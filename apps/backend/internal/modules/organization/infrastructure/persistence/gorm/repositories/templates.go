package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/organization/application/dto"
	ports "apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var (
	_ ports.DocumentTemplateRepository = (*DocumentTemplateRepository)(nil)
	_ ports.DocumentAssetRepository    = (*DocumentAssetRepository)(nil)
)

type DocumentTemplateRepository struct{ db *gorm.DB }

func NewDocumentTemplateRepository(db *gorm.DB) *DocumentTemplateRepository {
	return &DocumentTemplateRepository{db: db}
}

// templateRow คือ column ที่อ่าน (รายการไม่อ่าน html — html อาจใหญ่ถึง 200 KB ต่อแถว)
type templateRow struct {
	ID        uuid.UUID
	DocType   string
	Name      string
	HTML      string
	Paper     string
	IsDefault bool
	Status    string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r templateRow) dto() dto.DocumentTemplate {
	return dto.DocumentTemplate{
		ID: r.ID.String(), DocType: r.DocType, Name: r.Name, HTML: r.HTML, Paper: r.Paper, IsDefault: r.IsDefault,
		Status: r.Status, CreatedBy: r.CreatedBy.String(), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

const (
	templateListCols = "id, doc_type, name, paper, is_default, status, created_by, created_at, updated_at"
	templateFullCols = templateListCols + ", html"
)

func (r *DocumentTemplateRepository) base(ctx context.Context, companyID vo.CompanyID, cols string) *gorm.DB {
	return database.Conn(ctx, r.db).Model(&models.DocumentTemplateModel{}).Select(cols).Where("company_id = ?", uuidOf(companyID))
}

func (r *DocumentTemplateRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.DocumentTemplateID) (*entities.DocumentTemplate, error) {
	m, err := findByID[models.DocumentTemplateModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrTemplateNotFound)
	if err != nil {
		return nil, err
	}
	return entities.RehydrateDocumentTemplate(vo.DocumentTemplateID(m.ID), vo.CompanyID(m.CompanyID), entities.DocumentTemplateData{
		DocType: m.DocType, Name: m.Name, HTML: m.HTML, Paper: vo.Paper(m.Paper), IsDefault: m.IsDefault, Status: vo.Status(m.Status),
	}), nil
}

func (r *DocumentTemplateRepository) Get(ctx context.Context, companyID vo.CompanyID, id vo.DocumentTemplateID) (dto.DocumentTemplate, error) {
	row, err := first[templateRow](r.base(ctx, companyID, templateFullCols).Where("id = ?", uuidOf(id)), domain.ErrTemplateNotFound)
	return row.dto(), err
}

func (r *DocumentTemplateRepository) GetDefault(ctx context.Context, companyID vo.CompanyID, docType string) (dto.DocumentTemplate, error) {
	row, err := first[templateRow](r.base(ctx, companyID, templateFullCols).Where("doc_type = ? AND is_default AND status = ?", docType, vo.StatusActive), domain.ErrTemplateNotFound)
	return row.dto(), err
}

// List: กรองชนิดเอกสาร / สถานะ / ค้นชื่อ, เรียงชนิดเอกสาร → ค่าเริ่มต้นก่อน → ชื่อ, แบ่งหน้าใน DB (COUNT + หน้าเดียว)
func (r *DocumentTemplateRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentTemplateFilter) (listing.Result[dto.DocumentTemplate], error) {
	q := r.base(ctx, companyID, templateListCols)
	if f.DocType != "" {
		q = q.Where("doc_type = ?", f.DocType)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	q = search(q, f.Search, "name")
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return listing.Result[dto.DocumentTemplate]{}, err
	}
	var rows []templateRow
	if err := q.Order("doc_type, is_default DESC, name, id").Limit(f.Page.Limit()).Offset(f.Page.Offset()).Scan(&rows).Error; err != nil {
		return listing.Result[dto.DocumentTemplate]{}, err
	}
	out := make([]dto.DocumentTemplate, len(rows))
	for i, row := range rows {
		out[i] = row.dto()
	}
	return listing.Result[dto.DocumentTemplate]{Items: out, Total: total, Page: f.Page}, nil
}

func (r *DocumentTemplateRepository) Create(ctx context.Context, t *entities.DocumentTemplate, createdBy vo.UserID) error {
	d, now := t.Data(), time.Now().UTC()
	m := models.DocumentTemplateModel{
		ID: uuidOf(t.ID()), CompanyID: uuidOf(t.CompanyID()), DocType: d.DocType, Name: d.Name, HTML: d.HTML, Paper: string(d.Paper),
		Status: string(d.Status), CreatedBy: uuidOf(createdBy), CreatedAt: now, UpdatedAt: now,
	}
	return create(ctx, r.db, &m, domain.ErrCodeExists)
}

func (r *DocumentTemplateRepository) Update(ctx context.Context, t *entities.DocumentTemplate) error {
	d := t.Data()
	m := models.DocumentTemplateModel{
		ID: uuidOf(t.ID()), Name: d.Name, HTML: d.HTML, Paper: string(d.Paper), IsDefault: d.IsDefault, Status: string(d.Status), UpdatedAt: time.Now().UTC(),
	}
	return update(ctx, r.db, &m, uuidOf(t.CompanyID()), []string{"name", "html", "paper", "is_default", "status"}, domain.ErrTemplateNotFound, domain.ErrTemplateNotFound)
}

// SetDefault: ล้างค่าเริ่มต้นเดิมก่อนตั้งใหม่ (partial unique index กันสองแถวพร้อมกัน — race แพ้ได้ unique violation)
func (r *DocumentTemplateRepository) SetDefault(ctx context.Context, companyID vo.CompanyID, docType string, id vo.DocumentTemplateID) error {
	conn, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := conn.Model(&models.DocumentTemplateModel{}).
		Where("company_id = ? AND doc_type = ? AND is_default AND id <> ?", uuidOf(companyID), docType, uuidOf(id)).
		Updates(map[string]any{"is_default": false, "updated_at": now}).Error; err != nil {
		return err
	}
	res := conn.Model(&models.DocumentTemplateModel{}).
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).
		Updates(map[string]any{"is_default": true, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// Delete: soft delete และเลิกเป็นค่าเริ่มต้น (แถวที่ลบไม่อยู่ใน partial unique index อยู่แล้ว)
func (r *DocumentTemplateRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.DocumentTemplateID) error {
	return softDelete[models.DocumentTemplateModel](ctx, r.db, uuidOf(companyID), uuidOf(id), domain.ErrTemplateNotFound)
}

// ---------- รูป ----------

type DocumentAssetRepository struct{ db *gorm.DB }

func NewDocumentAssetRepository(db *gorm.DB) *DocumentAssetRepository {
	return &DocumentAssetRepository{db: db}
}

// List: ไม่อ่านเนื้อไฟล์ (bytea) — รูปมีไม่กี่รายการต่อบริษัท จึงไม่แบ่งหน้า
func (r *DocumentAssetRepository) List(ctx context.Context, companyID vo.CompanyID) ([]dto.DocumentAsset, error) {
	var rows []struct {
		ID          uuid.UUID
		Name        string
		ContentType string
		Size        int
		CreatedAt   time.Time
	}
	err := database.Conn(ctx, r.db).Model(&models.DocumentAssetModel{}).
		Select("id, name, content_type, size, created_at").
		Where("company_id = ?", uuidOf(companyID)).Order("name").Limit(200).Scan(&rows).Error
	out := make([]dto.DocumentAsset, len(rows))
	for i, row := range rows {
		out[i] = dto.DocumentAsset{ID: row.ID.String(), Name: row.Name, ContentType: row.ContentType, Size: row.Size, CreatedAt: row.CreatedAt}
	}
	return out, err
}

func (r *DocumentAssetRepository) Content(ctx context.Context, companyID vo.CompanyID, id vo.DocumentAssetID) (dto.DocumentAssetContent, error) {
	row, err := first[struct {
		ContentType string
		Data        []byte
		UpdatedAt   time.Time
	}](database.Conn(ctx, r.db).Model(&models.DocumentAssetModel{}).Select("content_type, data, updated_at").
		Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)), domain.ErrAssetNotFound)
	return dto.DocumentAssetContent{ContentType: row.ContentType, Data: row.Data, UpdatedAt: row.UpdatedAt}, err
}

func (r *DocumentAssetRepository) Create(ctx context.Context, a *entities.DocumentAsset) error {
	now := time.Now().UTC()
	m := models.DocumentAssetModel{
		ID: uuidOf(a.ID), CompanyID: uuidOf(a.CompanyID), Name: a.Name, ContentType: a.ContentType, Size: len(a.Data), Data: a.Data,
		CreatedAt: now, UpdatedAt: now,
	}
	return create(ctx, r.db, &m, domain.ErrAssetNameExists)
}

func (r *DocumentAssetRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.DocumentAssetID) error {
	res := database.Conn(ctx, r.db).Where("company_id = ? AND id = ?", uuidOf(companyID), uuidOf(id)).Delete(&models.DocumentAssetModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrAssetNotFound
	}
	return nil
}
