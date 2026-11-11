package usecases

import (
	"context"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// JournalUseCases จัดการสมุดรายวัน
type JournalUseCases struct {
	base
	journals repositories.JournalRepository
}

func (uc *JournalUseCases) Get(ctx context.Context, in dto.ByID[vo.JournalID]) (dto.Journal, error) {
	return read(ctx, uc.base, in.Actor, vo.PermJournalRead, func() (dto.Journal, error) {
		return uc.journals.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *JournalUseCases) List(ctx context.Context, in dto.List[dto.JournalFilter]) (listing.Result[dto.Journal], error) {
	return read(ctx, uc.base, in.Actor, vo.PermJournalRead, func() (listing.Result[dto.Journal], error) {
		return uc.journals.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *JournalUseCases) Create(ctx context.Context, in dto.Create[entities.JournalData]) (dto.Journal, error) {
	j, err := entities.NewJournal(in.CompanyID, in.Data)
	if err != nil {
		return dto.Journal{}, err
	}
	if err := uc.write(ctx, in.Actor, vo.PermJournalManage, func(ctx context.Context) error {
		return uc.journals.Create(ctx, j)
	}); err != nil {
		return dto.Journal{}, err
	}
	return uc.journals.Get(ctx, in.CompanyID, j.ID())
}

func (uc *JournalUseCases) Update(ctx context.Context, in dto.Update[vo.JournalID, entities.JournalData]) (dto.Journal, error) {
	err := uc.write(ctx, in.Actor, vo.PermJournalManage, func(ctx context.Context) error {
		j, err := uc.journals.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := j.Update(in.Data); err != nil {
			return err
		}
		return uc.journals.Update(ctx, j)
	})
	if err != nil {
		return dto.Journal{}, err
	}
	return uc.journals.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete ได้เมื่อยังไม่มีรายการในสมุด
func (uc *JournalUseCases) Delete(ctx context.Context, in dto.ByID[vo.JournalID]) error {
	return uc.write(ctx, in.Actor, vo.PermJournalManage, func(ctx context.Context) error {
		j, err := uc.journals.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		used, err := uc.journals.HasEntries(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := j.EnsureDeletable(used); err != nil {
			return err
		}
		return uc.journals.Delete(ctx, in.CompanyID, in.ID)
	})
}
