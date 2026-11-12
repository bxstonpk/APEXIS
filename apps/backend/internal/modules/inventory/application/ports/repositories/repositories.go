// Package repositories คือสัญญา (port) ที่ application ของ inventory ต้องการจาก persistence
//
// กติกา: ทุก method รับ CompanyID, ไม่เห็น record ที่ถูก soft delete, ไม่เปิด transaction เอง
// Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ, Create / Update คืน domain.ErrCodeExists เมื่อชน unique
package repositories

import (
	"context"
	"time"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/domain/entities"
	"apexis/backend/internal/modules/inventory/domain/services"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// MasterRepository คือ method ร่วมของ master data (หน่วยนับ / หมวด / คลัง / สินค้า)
type MasterRepository[ID any, E any, R any, F any] interface {
	Create(ctx context.Context, e E) error
	Update(ctx context.Context, e E) error
	Delete(ctx context.Context, companyID vo.CompanyID, id ID) error // soft delete
	FindByID(ctx context.Context, companyID vo.CompanyID, id ID) (E, error)
	// InUse: ถูกอ้างถึงโดย record อื่น (ลบไม่ได้)
	InUse(ctx context.Context, companyID vo.CompanyID, id ID) (bool, error)
	Get(ctx context.Context, companyID vo.CompanyID, id ID) (R, error)
	List(ctx context.Context, companyID vo.CompanyID, f F) (listing.Result[R], error)
}

type UomRepository = MasterRepository[vo.UomID, *entities.Uom, dto.Master, dto.MasterFilter]
type CategoryRepository = MasterRepository[vo.CategoryID, *entities.Category, dto.Master, dto.MasterFilter]

type WarehouseRepository interface {
	MasterRepository[vo.WarehouseID, *entities.Warehouse, dto.Master, dto.MasterFilter]
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.WarehouseID) ([]*entities.Warehouse, error)
}

type ItemRepository interface {
	MasterRepository[vo.ItemID, *entities.Item, dto.Item, dto.ItemFilter]
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) ([]*entities.Item, error)
	// HasMovements: เคยมีการเคลื่อนไหวสต็อก (ชนิด / หน่วยนับเปลี่ยนไม่ได้)
	HasMovements(ctx context.Context, companyID vo.CompanyID, id vo.ItemID) (bool, error)
}

type DocumentRepository interface {
	// Create / Update เขียนหัวเอกสาร + บรรทัด — ต้องอยู่ใน transaction
	Create(ctx context.Context, d *entities.Document) error
	Update(ctx context.Context, d *entities.Document) error
	// UpdateStatus เขียนสถานะ / เลขที่ / ข้อมูลการผ่าน-ยกเลิก / มูลค่าบรรทัด
	UpdateStatus(ctx context.Context, d *entities.Document) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) error // ลบจริง (เฉพาะ draft)
	// FindForUpdate โหลดเอกสารพร้อม lock แถวหัวเอกสาร (กันผ่าน / ยกเลิกซ้อนกัน)
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) (*entities.Document, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.DocumentID) (dto.Document, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error)
	// NextNumber: เลขลำดับถัดไปของชนิดเอกสาร + ปีบัญชี — lock แถวลำดับจน commit (ไม่ซ้ำ / ไม่ขาดตอน)
	NextNumber(ctx context.Context, companyID vo.CompanyID, t vo.DocumentType, fiscalYear int) (int64, error)
}

// StockRepository คือชั้นต้นทุน FIFO และการเคลื่อนไหวสต็อก
type StockRepository interface {
	// LockLayers โหลดชั้นที่ยังเหลือของ keys ที่ลงวันที่ไม่เกิน asOf พร้อม lock FOR UPDATE — query เดียว
	// เรียงตาม (สินค้า, คลัง, วันที่, ลำดับสร้าง) ทุกครั้ง เพื่อให้ลำดับการ lock คงที่ (กัน deadlock)
	LockLayers(ctx context.Context, companyID vo.CompanyID, keys []services.StockKey, asOf time.Time) ([]*services.Layer, error)
	// ApplyPosting เขียนผล FIFO: แก้ชั้นเดิม, เพิ่มชั้นใหม่, การเคลื่อนไหว, allocation (statement แบบ batch)
	ApplyPosting(ctx context.Context, companyID vo.CompanyID, plan services.PostingPlan) error
	// LoadForCancel โหลดการเคลื่อนไหวของเอกสาร + allocation + ชั้นที่เกี่ยวข้อง (lock FOR UPDATE)
	LoadForCancel(ctx context.Context, companyID vo.CompanyID, docID vo.DocumentID) (services.CancelInput, error)
	ApplyCancel(ctx context.Context, companyID vo.CompanyID, plan services.CancelPlan) error

	StockOnHand(ctx context.Context, companyID vo.CompanyID, f dto.StockOnHandFilter) (listing.Result[dto.StockOnHandRow], vo.Money, error)
	// StockCardOpening: ยอดยกมาก่อนวันที่ start
	StockCardOpening(ctx context.Context, companyID vo.CompanyID, f dto.StockCardFilter) (vo.Quantity, vo.Money, error)
	// StockCardLines: การเคลื่อนไหวในช่วงวันที่ พร้อมยอดสะสม (ไม่รวมยอดยกมา) — แบ่งหน้าใน DB
	StockCardLines(ctx context.Context, companyID vo.CompanyID, f dto.StockCardFilter) ([]dto.StockCardLine, int64, error)
}
