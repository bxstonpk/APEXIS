// Package repositories คือ interface ของ persistence ที่ application ของ manufacturing ต้องการ
// implementation อยู่ใน infrastructure/persistence/gorm/repositories — ทุก method เข้าร่วม transaction ใน ctx
package repositories

import (
	"context"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type BomRepository interface {
	// Create / Update เขียนหัวสูตร + บรรทัด — ต้องอยู่ใน transaction
	// รหัสซ้ำ -> domain.ErrCodeExists; สูตรหลักซ้ำ -> domain.ErrDefaultBomExists
	Create(ctx context.Context, b *entities.Bom) error
	Update(ctx context.Context, b *entities.Bom) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.BomID) error
	// FindByID คืน domain.ErrBomNotFound ถ้าไม่พบ
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.BomID) (*entities.Bom, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.BomFilter) (listing.Result[dto.BomRow], error)
	// InUse: มีใบสั่งผลิตอ้างสูตรนี้
	InUse(ctx context.Context, companyID vo.CompanyID, id vo.BomID) (bool, error)
	// Refs: รหัส / ชื่อสูตรตาม id (รวมที่ลบแล้ว)
	Refs(ctx context.Context, companyID vo.CompanyID, ids []vo.BomID) (map[vo.BomID]dto.Ref, error)
}

type OrderRepository interface {
	// Create / Save เขียนหัวใบสั่งผลิต + วัตถุดิบ + ค่าแรง + การเคลื่อนไหว — ต้องอยู่ใน transaction
	Create(ctx context.Context, o *entities.Order) error
	Save(ctx context.Context, o *entities.Order) error
	// FindByID คืน domain.ErrOrderNotFound ถ้าไม่พบ; forUpdate = lock แถวจน commit (กันเบิก / ปิดงานพร้อมกัน)
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.OrderID, forUpdate bool) (*entities.Order, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.OrderFilter) (listing.Result[dto.OrderRow], error)
}

type SettingsRepository interface {
	// Get คืนค่าว่างถ้ายังไม่เคยตั้ง
	Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error)
	Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error
}

// Sequence ออกเลขลำดับถัดไปของชนิดเอกสาร / ช่วง (lock จน commit)
type Sequence interface {
	NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error)
}

type ReportRepository interface {
	ProductionCost(ctx context.Context, companyID vo.CompanyID, f dto.CostFilter) (dto.CostReport, error)
	MaterialUsage(ctx context.Context, companyID vo.CompanyID, f dto.CostFilter) (listing.Result[dto.UsageRow], error)
	Wip(ctx context.Context, companyID vo.CompanyID, f dto.WipFilter) (dto.WipReport, error)
}
