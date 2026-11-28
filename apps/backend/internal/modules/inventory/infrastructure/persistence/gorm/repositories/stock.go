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
	"apexis/backend/internal/modules/inventory/domain/services"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	gp "apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/inventory/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.StockRepository = (*StockRepository)(nil)

type StockRepository struct{ db *gorm.DB }

func NewStockRepository(db *gorm.DB) *StockRepository { return &StockRepository{db: db} }

const batchSize = 200

// lockOrder คือลำดับการ lock ชั้นต้นทุนที่ทุก transaction ใช้เหมือนกัน (กัน deadlock)
const lockOrder = "item_id, warehouse_id, layer_date, id"

// LockLayers — 1 query: ชั้นที่ยังเหลือของ (สินค้า, คลัง) ที่ขอ ลงวันที่ไม่เกิน asOf ... FOR UPDATE
func (r *StockRepository) LockLayers(ctx context.Context, companyID vo.CompanyID, keys []services.StockKey, asOf time.Time) ([]*services.Layer, error) {
	pairs := make([][]any, len(keys))
	for i, k := range keys {
		pairs[i] = []any{uuidOf(k.ItemID), uuidOf(k.WarehouseID)}
	}
	var ms []models.LayerModel
	err := database.Conn(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("company_id = ? AND remaining_quantity > 0 AND layer_date <= ? AND (item_id, warehouse_id) IN ?", uuidOf(companyID), asOf, pairs).
		Order(lockOrder).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	out := make([]*services.Layer, len(ms))
	for i, m := range ms {
		out[i] = gp.LayerToDomain(m)
	}
	return out, nil
}

// ApplyPosting — statement แบบ batch: แก้ชั้นเดิม (1), เพิ่มชั้น / การเคลื่อนไหว / allocation (ละ 1 ต่อ 200 แถว)
func (r *StockRepository) ApplyPosting(ctx context.Context, companyID vo.CompanyID, plan services.PostingPlan) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	if err := updateLayers(tx, companyID, plan.UpdatedLayers); err != nil {
		return err
	}
	if len(plan.NewLayers) > 0 {
		layers := make([]models.LayerModel, len(plan.NewLayers))
		for i, l := range plan.NewLayers {
			layers[i] = gp.LayerToModel(companyID, l)
		}
		if err := tx.Omit(clause.Associations).CreateInBatches(&layers, batchSize).Error; err != nil {
			return err
		}
	}
	return insertMovements(tx, companyID, plan.Movements)
}

// LoadForCancel — 3 query: การเคลื่อนไหวของเอกสาร, allocation, ชั้นที่เกี่ยวข้อง (FOR UPDATE ตามลำดับเดียวกับ LockLayers)
func (r *StockRepository) LoadForCancel(ctx context.Context, companyID vo.CompanyID, docID vo.DocumentID) (services.CancelInput, error) {
	db := database.Conn(ctx, r.db)
	var mvs []models.MovementModel
	if err := db.Where("company_id = ? AND document_id = ?", uuidOf(companyID), uuidOf(docID)).Order("id").Find(&mvs).Error; err != nil {
		return services.CancelInput{}, err
	}
	in := services.CancelInput{Layers: map[vo.LayerID]*services.Layer{}}
	if len(mvs) == 0 {
		return in, nil
	}
	ids := make([]uuid.UUID, len(mvs))
	for i, m := range mvs {
		ids[i] = m.ID
	}
	var allocs []models.AllocationModel
	if err := db.Where("movement_id IN ?", ids).Find(&allocs).Error; err != nil {
		return services.CancelInput{}, err
	}
	byMovement := map[uuid.UUID][]services.Allocation{}
	layerIDs := []uuid.UUID{}
	for _, a := range allocs {
		byMovement[a.MovementID] = append(byMovement[a.MovementID], services.Allocation{LayerID: vo.LayerID(a.LayerID), Quantity: a.Quantity, Value: a.Value})
		layerIDs = append(layerIDs, a.LayerID)
	}
	for _, m := range mvs {
		mv := gp.MovementToDomain(m)
		mv.Allocations = byMovement[m.ID]
		in.Movements = append(in.Movements, mv)
	}
	q := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("company_id = ?", uuidOf(companyID))
	if len(layerIDs) > 0 {
		q = q.Where("(document_id = ? OR id IN ?)", uuidOf(docID), layerIDs)
	} else {
		q = q.Where("document_id = ?", uuidOf(docID))
	}
	var layers []models.LayerModel
	if err := q.Order(lockOrder).Find(&layers).Error; err != nil {
		return services.CancelInput{}, err
	}
	for _, m := range layers {
		l := gp.LayerToDomain(m)
		in.Layers[l.ID] = l
		if m.DocumentID == uuidOf(docID) {
			in.CreatedLayerIDs = append(in.CreatedLayerIDs, l.ID)
		}
	}
	return in, nil
}

// ApplyCancel — แก้ชั้น (1 statement) + การเคลื่อนไหวกลับรายการ (batch)
func (r *StockRepository) ApplyCancel(ctx context.Context, companyID vo.CompanyID, plan services.CancelPlan) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	if err := updateLayers(tx, companyID, plan.UpdatedLayers); err != nil {
		return err
	}
	return insertMovements(tx, companyID, plan.Movements)
}

// updateLayers — UPDATE ... FROM (VALUES ...) statement เดียวสำหรับทุกชั้น
func updateLayers(tx *gorm.DB, companyID vo.CompanyID, layers []*services.Layer) error {
	if len(layers) == 0 {
		return nil
	}
	values := make([]string, len(layers))
	args := make([]any, 0, 3*len(layers)+1)
	for i, l := range layers {
		values[i] = "(?::uuid, ?::numeric, ?::numeric)"
		args = append(args, uuidOf(l.ID), l.RemainingQuantity, l.RemainingValue)
	}
	args = append(args, uuidOf(companyID))
	return tx.Exec(`UPDATE inventory_layers AS l SET remaining_quantity = v.rq, remaining_value = v.rv
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(id, rq, rv)
		WHERE l.id = v.id AND l.company_id = ?`, args...).Error
}

func insertMovements(tx *gorm.DB, companyID vo.CompanyID, mvs []services.Movement) error {
	if len(mvs) == 0 {
		return nil
	}
	rows := make([]models.MovementModel, len(mvs))
	var allocs []models.AllocationModel
	for i, m := range mvs {
		rows[i] = gp.MovementToModel(companyID, m)
		for _, a := range m.Allocations {
			allocs = append(allocs, models.AllocationModel{MovementID: uuidOf(m.ID), LayerID: uuidOf(a.LayerID), Quantity: a.Quantity, Value: a.Value})
		}
	}
	if err := tx.Omit(clause.Associations).CreateInBatches(&rows, batchSize).Error; err != nil {
		return err
	}
	if len(allocs) == 0 {
		return nil
	}
	return tx.Omit(clause.Associations).CreateInBatches(&allocs, batchSize).Error
}

// ---------- รายงาน ----------

var stockOnHandSort = map[string]string{
	"item_code": "i.code", "item_name": "i.name", "warehouse_code": "w.code",
	"quantity": "SUM(m.quantity)", "value": "SUM(m.value)",
}

// StockOnHand — 1 query: รวมการเคลื่อนไหวถึงวันที่ GROUP BY สินค้า × คลัง, แบ่งหน้าใน DB,
// จำนวนแถวทั้งหมดและมูลค่ารวมด้วย window function (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
func (r *StockRepository) StockOnHand(ctx context.Context, companyID vo.CompanyID, f dto.StockOnHandFilter) (listing.Result[dto.StockOnHandRow], vo.Money, error) {
	order, err := database.OrderBy(f.Sort, stockOnHandSort, "m.item_id ASC, m.warehouse_id")
	if err != nil {
		return listing.Result[dto.StockOnHandRow]{}, decimal.Zero, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("inventory_movements AS m").
			Joins("JOIN inventory_items AS i ON i.id = m.item_id").
			Joins("JOIN inventory_uoms AS u ON u.id = i.uom_id").
			Joins("JOIN inventory_warehouses AS w ON w.id = m.warehouse_id").
			Where("m.company_id = ? AND m.movement_date <= ?", uuidOf(companyID), f.AsOf)
		if f.WarehouseID != nil {
			q = q.Where("m.warehouse_id = ?", uuidOf(*f.WarehouseID))
		}
		if f.ItemID != nil {
			q = q.Where("m.item_id = ?", uuidOf(*f.ItemID))
		}
		if f.CategoryID != nil {
			q = q.Where("i.category_id = ?", uuidOf(*f.CategoryID))
		}
		return search(q, f.Search, "i.code", "i.name").
			Group("m.item_id, i.code, i.name, u.code, m.warehouse_id, w.code, w.name").
			Having("SUM(m.quantity) <> 0 OR SUM(m.value) <> 0")
	}
	var rows []struct {
		ItemID        uuid.UUID
		ItemCode      string
		ItemName      string
		UomCode       string
		WarehouseID   uuid.UUID
		WarehouseCode string
		WarehouseName string
		Quantity      decimal.Decimal
		Value         decimal.Decimal
		Total         int64
		TotalValue    decimal.Decimal
	}
	err = base().
		Select(`m.item_id, i.code AS item_code, i.name AS item_name, u.code AS uom_code,
			m.warehouse_id, w.code AS warehouse_code, w.name AS warehouse_name,
			SUM(m.quantity) AS quantity, SUM(m.value) AS value,
			COUNT(*) OVER () AS total, SUM(SUM(m.value)) OVER () AS total_value`).
		Order(order).Limit(f.Page.Limit()).Offset(f.Page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return listing.Result[dto.StockOnHandRow]{}, decimal.Zero, err
	}
	res := listing.Result[dto.StockOnHandRow]{Items: make([]dto.StockOnHandRow, len(rows)), Page: f.Page}
	totalValue := decimal.Zero
	if len(rows) > 0 {
		res.Total, totalValue = rows[0].Total, rows[0].TotalValue
	} else if f.Page.Offset() > 0 {
		// หน้าที่เกินข้อมูล: นับจำนวนกลุ่ม + มูลค่ารวมจาก sub-query
		var agg struct {
			Total      int64
			TotalValue decimal.Decimal
		}
		err := database.Conn(ctx, r.db).Table("(?) AS g", base().Select("SUM(m.value) AS value")).
			Select("COUNT(*) AS total, COALESCE(SUM(g.value), 0) AS total_value").Scan(&agg).Error
		if err != nil {
			return listing.Result[dto.StockOnHandRow]{}, decimal.Zero, err
		}
		res.Total, totalValue = agg.Total, agg.TotalValue
	}
	for i, x := range rows {
		res.Items[i] = dto.StockOnHandRow{
			Item: dto.Ref{ID: x.ItemID.String(), Code: x.ItemCode, Name: x.ItemName}, UomCode: x.UomCode,
			Warehouse: dto.Ref{ID: x.WarehouseID.String(), Code: x.WarehouseCode, Name: x.WarehouseName},
			Quantity:  x.Quantity, Value: x.Value,
		}
	}
	return res, totalValue, nil
}

func (r *StockRepository) cardBase(ctx context.Context, companyID vo.CompanyID, f dto.StockCardFilter) *gorm.DB {
	q := database.Conn(ctx, r.db).Table("inventory_movements AS m").
		Where("m.company_id = ? AND m.item_id = ?", uuidOf(companyID), uuidOf(f.ItemID))
	if f.WarehouseID != nil {
		q = q.Where("m.warehouse_id = ?", uuidOf(*f.WarehouseID))
	}
	return q
}

// StockCardOpening — 1 query: ยอดรวมก่อนวันที่เริ่ม
func (r *StockRepository) StockCardOpening(ctx context.Context, companyID vo.CompanyID, f dto.StockCardFilter) (vo.Quantity, vo.Money, error) {
	var o struct {
		Quantity decimal.Decimal
		Value    decimal.Decimal
	}
	err := r.cardBase(ctx, companyID, f).Where("m.movement_date < ?", f.Start).
		Select("COALESCE(SUM(m.quantity), 0) AS quantity, COALESCE(SUM(m.value), 0) AS value").Scan(&o).Error
	return o.Quantity, o.Value, err
}

// StockCardLines — 1 query: ยอดสะสมคำนวณด้วย window function ก่อนตัดหน้า จึงถูกต้องทุกหน้า
// (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
func (r *StockRepository) StockCardLines(ctx context.Context, companyID vo.CompanyID, f dto.StockCardFilter) ([]dto.StockCardLine, int64, error) {
	inner := r.cardBase(ctx, companyID, f).
		Joins("JOIN inventory_documents AS d ON d.id = m.document_id").
		Joins("JOIN inventory_warehouses AS w ON w.id = m.warehouse_id").
		Where("m.movement_date BETWEEN ? AND ?", f.Start, f.End)
	type row struct {
		MovementDate  time.Time
		ID            uuid.UUID
		DocumentID    uuid.UUID
		DocumentNo    *string
		DocType       string
		WarehouseID   uuid.UUID
		WarehouseCode string
		WarehouseName string
		Quantity      decimal.Decimal
		Value         decimal.Decimal
		RunQuantity   decimal.Decimal
		RunValue      decimal.Decimal
		Total         int64
	}
	var rows []row
	err := database.Conn(ctx, r.db).Table("(?) AS t", inner.Select(`m.movement_date, m.id, m.document_id, d.document_no, d.doc_type,
			m.warehouse_id, w.code AS warehouse_code, w.name AS warehouse_name, m.quantity, m.value,
			SUM(m.quantity) OVER (ORDER BY m.movement_date, m.id ROWS UNBOUNDED PRECEDING) AS run_quantity,
			SUM(m.value) OVER (ORDER BY m.movement_date, m.id ROWS UNBOUNDED PRECEDING) AS run_value,
			COUNT(*) OVER () AS total`)).
		Order("movement_date, id").Limit(f.Page.Limit()).Offset(f.Page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var total int64
	switch {
	case len(rows) > 0:
		total = rows[0].Total
	case f.Page.Offset() > 0:
		if err := inner.Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}
	out := make([]dto.StockCardLine, len(rows))
	for i, x := range rows {
		out[i] = dto.StockCardLine{
			Date: gp.DateUTC(x.MovementDate), DocumentID: x.DocumentID.String(), DocumentType: x.DocType,
			Warehouse: dto.Ref{ID: x.WarehouseID.String(), Code: x.WarehouseCode, Name: x.WarehouseName},
			Quantity:  x.Quantity, Value: x.Value, BalanceQuantity: x.RunQuantity, BalanceValue: x.RunValue,
		}
		if x.DocumentNo != nil {
			out[i].DocumentNo = *x.DocumentNo
		}
	}
	return out, total, nil
}

// ValueByItem — 1 query: รวมมูลค่าการเคลื่อนไหวถึงวันที่ต่อสินค้า (subquery) แล้ว JOIN สินค้า / หมวดเพื่อเอาบัญชี
// (รวมสินค้าที่ถูก soft delete — มูลค่าที่ยังเหลือต้องถูกนับ)
func (r *StockRepository) ValueByItem(ctx context.Context, companyID vo.CompanyID, asOf time.Time) ([]dto.ItemStockValue, error) {
	var rows []struct {
		ItemAccountsRow
		Value decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Raw(`
		SELECT `+itemAccountColumns+`, v.value
		  FROM (SELECT item_id, SUM(value) AS value FROM inventory_movements
		         WHERE company_id = ? AND movement_date <= ?
		         GROUP BY item_id HAVING SUM(value) <> 0) AS v
		  JOIN inventory_items AS i ON i.id = v.item_id
		  LEFT JOIN inventory_item_categories AS c ON c.id = i.category_id AND c.deleted_at IS NULL
		  LEFT JOIN inventory_uoms AS u ON u.id = i.uom_id
		 ORDER BY i.code`, uuidOf(companyID), asOf).Scan(&rows).Error
	out := make([]dto.ItemStockValue, len(rows))
	for i, row := range rows {
		out[i] = dto.ItemStockValue{ItemAccounts: row.ItemAccountsRow.dto(), Value: row.Value}
	}
	return out, err
}
