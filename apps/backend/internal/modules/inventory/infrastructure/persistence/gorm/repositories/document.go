package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/inventory/application/dto"
	ports "apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	gp "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.DocumentRepository = (*DocumentRepository)(nil)

type DocumentRepository struct{ db *gorm.DB }

func NewDocumentRepository(db *gorm.DB) *DocumentRepository { return &DocumentRepository{db: db} }

// Create — 2 statement (หัวเอกสาร + บรรทัดแบบ batch)
func (r *DocumentRepository) Create(ctx context.Context, d *entities.Document) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.DocumentToModel(d)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// Update แทนที่ draft: แก้หัวเอกสาร (เฉพาะ draft) -> ลบบรรทัดเดิม -> insert บรรทัดใหม่
func (r *DocumentRepository) Update(ctx context.Context, d *entities.Document) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.DocumentToModel(d)
	res := tx.Model(&models.DocumentModel{}).
		Where("company_id = ? AND id = ? AND status = ?", m.CompanyID, m.ID, vo.DocDraft).
		Updates(map[string]any{
			"doc_type": m.DocType, "doc_date": m.DocDate, "warehouse_id": m.WarehouseID, "to_warehouse_id": m.ToWarehouseID,
			"reference": m.Reference, "description": m.Description, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrDocumentNotDraft
	}
	if err := tx.Where("document_id = ?", m.ID).Delete(&models.DocumentLineModel{}).Error; err != nil {
		return err
	}
	return tx.Create(&m.Lines).Error
}

// UpdateStatus — 2 statement: หัวเอกสาร + มูลค่าของทุกบรรทัด (UPDATE ... FROM (VALUES ...))
func (r *DocumentRepository) UpdateStatus(ctx context.Context, d *entities.Document) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.DocumentToModel(d)
	res := tx.Model(&models.DocumentModel{}).
		Where("company_id = ? AND id = ?", m.CompanyID, m.ID).
		Updates(map[string]any{
			"status": m.Status, "document_no": m.DocumentNo, "fiscal_period_id": m.FiscalPeriodID,
			"posted_at": m.PostedAt, "posted_by": m.PostedBy, "cancelled_at": m.CancelledAt, "cancelled_by": m.CancelledBy,
			"total_value": m.TotalValue, "updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return domain.ErrCodeExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrDocumentNotFound
	}
	values := make([]string, len(m.Lines))
	args := make([]any, 0, 2*len(m.Lines)+1)
	for i, l := range m.Lines {
		values[i] = "(?::int, ?::numeric)"
		args = append(args, l.LineNo, l.Value)
	}
	args = append(args, m.ID)
	return tx.Exec(`UPDATE inventory_document_lines AS l SET value = v.value
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(line_no, value)
		WHERE l.line_no = v.line_no AND l.document_id = ?`, args...).Error
}

func (r *DocumentRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) error {
	res := database.Conn(ctx, r.db).
		Where("company_id = ? AND id = ? AND status = ?", uuidOf(companyID), uuidOf(id), vo.DocDraft).
		Delete(&models.DocumentModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

// FindForUpdate — 2 query: lock หัวเอกสาร (FOR UPDATE) แล้วโหลดบรรทัด
func (r *DocumentRepository) FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) (*entities.Document, error) {
	db := database.Conn(ctx, r.db)
	var m models.DocumentModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Take(&m).Error
	if database.IsNotFound(err) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := db.Where("document_id = ?", m.ID).Order("line_no").Find(&m.Lines).Error; err != nil {
		return nil, err
	}
	return gp.DocumentToEntity(m), nil
}

// NextNumber — statement เดียว: INSERT ... ON CONFLICT DO UPDATE ... RETURNING (lock แถวลำดับจน commit)
func (r *DocumentRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, t vo.DocumentType, fiscalYear int) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO inventory_sequences (company_id, doc_type, fiscal_year, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (company_id, doc_type, fiscal_year) DO UPDATE SET last_no = inventory_sequences.last_no + 1
		RETURNING last_no`, uuidOf(companyID), string(t), fiscalYear).Scan(&n).Error
	return n, err
}

type documentRow struct {
	ID              uuid.UUID
	DocType         string
	DocumentNo      *string
	DocDate         time.Time
	WarehouseID     uuid.UUID
	WarehouseCode   string
	WarehouseName   string
	ToWarehouseID   *uuid.UUID
	ToWarehouseCode *string
	ToWarehouseName *string
	Reference       string
	Description     string
	Status          string
	TotalValue      decimal.Decimal
	PostedAt        *time.Time
	FiscalPeriodID  *uuid.UUID
	CreatedBy       uuid.UUID
	PostedBy        *uuid.UUID
	CancelledAt     *time.Time
	CancelledBy     *uuid.UUID
	Total           int64
}

func (r documentRow) summary() dto.DocumentSummary {
	out := dto.DocumentSummary{
		ID: r.ID.String(), Type: r.DocType, Date: gp.DateUTC(r.DocDate),
		Warehouse: dto.Ref{ID: r.WarehouseID.String(), Code: r.WarehouseCode, Name: r.WarehouseName},
		Reference: r.Reference, Description: r.Description, Status: r.Status, TotalValue: r.TotalValue, PostedAt: r.PostedAt,
	}
	if r.DocumentNo != nil {
		out.DocumentNo = *r.DocumentNo
	}
	if r.ToWarehouseID != nil && r.ToWarehouseCode != nil {
		out.ToWarehouse = &dto.Ref{ID: r.ToWarehouseID.String(), Code: *r.ToWarehouseCode, Name: *r.ToWarehouseName}
	}
	return out
}

const documentColumns = `d.id, d.doc_type, d.document_no, d.doc_date, d.warehouse_id, w.code AS warehouse_code, w.name AS warehouse_name,
	d.to_warehouse_id, tw.code AS to_warehouse_code, tw.name AS to_warehouse_name,
	d.reference, d.description, d.status, d.total_value, d.posted_at`

func (r *DocumentRepository) base(ctx context.Context, companyID vo.CompanyID) *gorm.DB {
	return database.Conn(ctx, r.db).Table("inventory_documents AS d").
		Joins("JOIN inventory_warehouses AS w ON w.id = d.warehouse_id").
		Joins("LEFT JOIN inventory_warehouses AS tw ON tw.id = d.to_warehouse_id").
		Where("d.company_id = ?", uuidOf(companyID))
}

// Get — 2 query: หัวเอกสาร JOIN คลัง + บรรทัด JOIN สินค้า / หน่วยนับ (one-to-many แยก query)
func (r *DocumentRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) (dto.Document, error) {
	var heads []documentRow
	err := r.base(ctx, companyID).
		Select(documentColumns+", d.fiscal_period_id, d.created_by, d.posted_by, d.cancelled_at, d.cancelled_by").
		Where("d.id = ?", uuidOf(id)).Limit(1).Scan(&heads).Error
	if err != nil {
		return dto.Document{}, err
	}
	if len(heads) == 0 {
		return dto.Document{}, domain.ErrDocumentNotFound
	}
	h := heads[0]
	var lines []struct {
		LineNo      int
		ItemID      uuid.UUID
		ItemCode    string
		ItemName    string
		UomCode     string
		Quantity    decimal.Decimal
		UnitCost    *decimal.Decimal
		Value       decimal.Decimal
		Description string
	}
	err = database.Conn(ctx, r.db).Table("inventory_document_lines AS l").
		Select(`l.line_no, l.item_id, i.code AS item_code, i.name AS item_name, u.code AS uom_code,
			l.quantity, l.unit_cost, l.value, l.description`).
		Joins("JOIN inventory_items AS i ON i.id = l.item_id").
		Joins("JOIN inventory_uoms AS u ON u.id = i.uom_id").
		Where("l.document_id = ?", h.ID).Order("l.line_no").Scan(&lines).Error
	if err != nil {
		return dto.Document{}, err
	}
	out := dto.Document{
		DocumentSummary: h.summary(), FiscalPeriodID: optString(h.FiscalPeriodID), CreatedBy: h.CreatedBy.String(),
		PostedBy: optString(h.PostedBy), CancelledAt: h.CancelledAt, CancelledBy: optString(h.CancelledBy),
		Lines: make([]dto.DocumentLine, len(lines)),
	}
	for i, l := range lines {
		out.Lines[i] = dto.DocumentLine{
			LineNo: l.LineNo, Item: dto.Ref{ID: l.ItemID.String(), Code: l.ItemCode, Name: l.ItemName}, UomCode: l.UomCode,
			Quantity: l.Quantity, UnitCost: l.UnitCost, Value: l.Value, Description: l.Description,
		}
	}
	return out, nil
}

var documentSortColumns = map[string]string{"date": "d.doc_date", "document_no": "d.document_no", "created_at": "d.created_at"}

// List — 1 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล) — มูลค่ารวมอ่านจากหัวเอกสาร
func (r *DocumentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error) {
	order, err := database.OrderBy(f.Sort, documentSortColumns, "d.id")
	if err != nil {
		return listing.Result[dto.DocumentSummary]{}, err
	}
	base := func() *gorm.DB {
		q := r.base(ctx, companyID)
		if len(f.Types) > 0 {
			q = q.Where("d.doc_type IN ?", f.Types)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("d.status IN ?", f.Statuses)
		}
		if f.WarehouseID != nil {
			q = q.Where("(d.warehouse_id = ? OR d.to_warehouse_id = ?)", uuidOf(*f.WarehouseID), uuidOf(*f.WarehouseID))
		}
		if f.ItemID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM inventory_document_lines AS l WHERE l.document_id = d.id AND l.item_id = ?)", uuidOf(*f.ItemID))
		}
		if f.StartDate != nil {
			q = q.Where("d.doc_date >= ?", *f.StartDate)
		}
		if f.EndDate != nil {
			q = q.Where("d.doc_date <= ?", *f.EndDate)
		}
		return search(q, f.Search, "d.document_no", "d.reference", "d.description")
	}
	rows, total, err := database.FetchPage(base, documentColumns, order, f.Page, func(r documentRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, documentRow.summary), err
}
