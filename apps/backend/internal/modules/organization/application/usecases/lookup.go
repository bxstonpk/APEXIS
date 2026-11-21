package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// LookupUseCases คือข้อมูลที่ module อื่นต้องใช้ประกอบการทำงาน (เช่น ledger ก่อนลงรายการบัญชี)
//
// ไม่ตรวจสิทธิ์: ถูกเรียกผ่าน public API ของ module (organization.Module) โดย module ที่ตรวจสิทธิ์ของตัวเองแล้ว
// ไม่มี HTTP route
type LookupUseCases struct {
	companies  repositories.CompanyRepository
	fiscal     repositories.FiscalRepository
	dimensions repositories.DimensionRepository
}

func (uc *LookupUseCases) CompanySettings(ctx context.Context, id vo.CompanyID) (dto.CompanySettings, error) {
	return uc.companies.Settings(ctx, id)
}

func (uc *LookupUseCases) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (dto.PeriodAt, error) {
	return uc.fiscal.PeriodAt(ctx, companyID, date)
}

func (uc *LookupUseCases) MissingDimensions(ctx context.Context, companyID vo.CompanyID, d dto.Dimensions) ([]string, error) {
	return uc.dimensions.Missing(ctx, companyID, d)
}
