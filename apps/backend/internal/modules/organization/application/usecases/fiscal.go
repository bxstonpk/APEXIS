package usecases

import (
	"context"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// FiscalUseCases จัดการปีบัญชีและงวดบัญชี
type FiscalUseCases struct {
	base
	fiscal repositories.FiscalRepository
	hooks  ports.FiscalYearListener
}

// CreateYear สร้างปีบัญชี + 12 งวดใน transaction เดียว
//
// การตรวจช่วงวันที่ซ้อนกันทำในระดับ application (ให้ error ที่สื่อความหมาย)
// ส่วน unique (company_id, year) ใน DB เป็นด่านสุดท้ายกันการสร้างซ้ำพร้อมกัน
func (uc *FiscalUseCases) CreateYear(ctx context.Context, in dto.CreateFiscalYearInput) (dto.FiscalYear, error) {
	fy, err := entities.NewFiscalYear(in.CompanyID, in.Year, in.StartDate)
	if err != nil {
		return dto.FiscalYear{}, err
	}
	err = uc.write(ctx, in.Actor, vo.PermFiscalPeriodMng, func(ctx context.Context) error {
		overlaps, err := uc.fiscal.ExistsOverlapping(ctx, in.CompanyID, fy.StartDate(), fy.EndDate())
		if err != nil {
			return err
		}
		if overlaps {
			return domain.ErrFiscalYearOverlaps
		}
		if err := uc.fiscal.CreateYear(ctx, fy); err != nil {
			return err
		}
		if uc.hooks == nil {
			return nil
		}
		return uc.hooks.FiscalYearCreated(ctx, in.CompanyID, in.Year)
	})
	if err != nil {
		return dto.FiscalYear{}, err
	}
	return uc.fiscal.GetYear(ctx, in.CompanyID, fy.ID())
}

func (uc *FiscalUseCases) GetYear(ctx context.Context, in dto.ByID[vo.FiscalYearID]) (dto.FiscalYear, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermFiscalPeriodRead); err != nil {
		return dto.FiscalYear{}, err
	}
	return uc.fiscal.GetYear(ctx, in.CompanyID, in.ID)
}

func (uc *FiscalUseCases) ListYears(ctx context.Context, in dto.List[dto.FiscalYearFilter]) (listing.Result[dto.FiscalYear], error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermFiscalPeriodRead); err != nil {
		return listing.Result[dto.FiscalYear]{}, err
	}
	return uc.fiscal.ListYears(ctx, in.CompanyID, in.Filter)
}

func (uc *FiscalUseCases) GetPeriod(ctx context.Context, in dto.ByID[vo.FiscalPeriodID]) (dto.FiscalPeriod, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermFiscalPeriodRead); err != nil {
		return dto.FiscalPeriod{}, err
	}
	return uc.fiscal.GetPeriod(ctx, in.CompanyID, in.ID)
}

func (uc *FiscalUseCases) ListPeriods(ctx context.Context, in dto.List[dto.FiscalPeriodFilter]) (listing.Result[dto.FiscalPeriod], error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermFiscalPeriodRead); err != nil {
		return listing.Result[dto.FiscalPeriod]{}, err
	}
	return uc.fiscal.ListPeriods(ctx, in.CompanyID, in.Filter)
}

// ChangePeriodStatus ปิด / เปิดงวด: lock ปีบัญชี -> ตรวจลำดับงวดใน domain -> update (transaction เดียว)
func (uc *FiscalUseCases) ChangePeriodStatus(ctx context.Context, in dto.ChangePeriodStatusInput) (dto.FiscalPeriod, error) {
	err := uc.write(ctx, in.Actor, vo.PermFiscalPeriodClose, func(ctx context.Context) error {
		fy, err := uc.fiscal.FindYearByPeriodForUpdate(ctx, in.CompanyID, in.PeriodID)
		if err != nil {
			return err
		}
		p, err := fy.ChangePeriodStatus(in.PeriodID, in.Status)
		if err != nil {
			return err
		}
		return uc.fiscal.UpdatePeriodStatus(ctx, p)
	})
	if err != nil {
		return dto.FiscalPeriod{}, err
	}
	return uc.fiscal.GetPeriod(ctx, in.CompanyID, in.PeriodID)
}
