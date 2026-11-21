package usecases

import (
	"context"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- หน่วยนับ / หมวดสินค้า (รูปแบบเดียวกัน) ----------

type MasterUseCases[ID any, E any] struct {
	base
	repo         repositories.MasterRepository[ID, E, dto.Master, dto.MasterFilter]
	read, manage vo.Permission
	newEntity    func(vo.CompanyID, entities.MasterData) (E, error)
	update       func(E, entities.MasterData) error
	id           func(E) ID
	validate     func(context.Context, vo.CompanyID, E) error // optional: ตรวจกับข้อมูลภายนอก (เช่น บัญชีของหมวด)
}

func (uc *MasterUseCases[ID, E]) check(ctx context.Context, companyID vo.CompanyID, e E) error {
	if uc.validate == nil {
		return nil
	}
	return uc.validate(ctx, companyID, e)
}

func (uc *MasterUseCases[ID, E]) Get(ctx context.Context, in dto.ByID[ID]) (dto.Master, error) {
	return read(ctx, uc.base, in.Actor, uc.read, func() (dto.Master, error) { return uc.repo.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *MasterUseCases[ID, E]) List(ctx context.Context, in dto.List[dto.MasterFilter]) (listing.Result[dto.Master], error) {
	return read(ctx, uc.base, in.Actor, uc.read, func() (listing.Result[dto.Master], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *MasterUseCases[ID, E]) Create(ctx context.Context, in dto.Create[entities.MasterData]) (dto.Master, error) {
	e, err := uc.newEntity(in.CompanyID, in.Data)
	if err != nil {
		return dto.Master{}, err
	}
	err = uc.write(ctx, in.Actor, uc.manage, func(ctx context.Context) error {
		if err := uc.check(ctx, in.CompanyID, e); err != nil {
			return err
		}
		return uc.repo.Create(ctx, e)
	})
	if err != nil {
		return dto.Master{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, uc.id(e))
}

func (uc *MasterUseCases[ID, E]) Update(ctx context.Context, in dto.Update[ID, entities.MasterData]) (dto.Master, error) {
	err := uc.write(ctx, in.Actor, uc.manage, func(ctx context.Context) error {
		e, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := uc.update(e, in.Data); err != nil {
			return err
		}
		if err := uc.check(ctx, in.CompanyID, e); err != nil {
			return err
		}
		return uc.repo.Update(ctx, e)
	})
	if err != nil {
		return dto.Master{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete ได้เมื่อไม่มีสินค้าอ้างถึง
func (uc *MasterUseCases[ID, E]) Delete(ctx context.Context, in dto.ByID[ID]) error {
	return uc.write(ctx, in.Actor, uc.manage, func(ctx context.Context) error {
		if _, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID); err != nil {
			return err
		}
		used, err := uc.repo.InUse(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := entities.EnsureUnused(used); err != nil {
			return err
		}
		return uc.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}

// ---------- คลังสินค้า ----------

type WarehouseUseCases struct {
	base
	repo repositories.WarehouseRepository
}

func (uc *WarehouseUseCases) Get(ctx context.Context, in dto.ByID[vo.WarehouseID]) (dto.Master, error) {
	return read(ctx, uc.base, in.Actor, vo.PermWarehouseRead, func() (dto.Master, error) { return uc.repo.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *WarehouseUseCases) List(ctx context.Context, in dto.List[dto.MasterFilter]) (listing.Result[dto.Master], error) {
	return read(ctx, uc.base, in.Actor, vo.PermWarehouseRead, func() (listing.Result[dto.Master], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *WarehouseUseCases) Create(ctx context.Context, in dto.Create[entities.WarehouseData]) (dto.Master, error) {
	w, err := entities.NewWarehouse(in.CompanyID, in.Data)
	if err != nil {
		return dto.Master{}, err
	}
	err = uc.write(ctx, in.Actor, vo.PermWarehouseManage, func(ctx context.Context) error {
		if err := uc.requireBranch(ctx, in.CompanyID, in.Data.BranchID); err != nil {
			return err
		}
		return uc.repo.Create(ctx, w)
	})
	if err != nil {
		return dto.Master{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, w.ID())
}

func (uc *WarehouseUseCases) Update(ctx context.Context, in dto.Update[vo.WarehouseID, entities.WarehouseData]) (dto.Master, error) {
	err := uc.write(ctx, in.Actor, vo.PermWarehouseManage, func(ctx context.Context) error {
		w, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := uc.requireBranch(ctx, in.CompanyID, in.Data.BranchID); err != nil {
			return err
		}
		if err := w.Update(in.Data); err != nil {
			return err
		}
		return uc.repo.Update(ctx, w)
	})
	if err != nil {
		return dto.Master{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, in.ID)
}

// Delete: คลังที่มีเอกสาร / การเคลื่อนไหวแล้วลบไม่ได้ (ปิดใช้งานแทน)
func (uc *WarehouseUseCases) Delete(ctx context.Context, in dto.ByID[vo.WarehouseID]) error {
	return uc.write(ctx, in.Actor, vo.PermWarehouseManage, func(ctx context.Context) error {
		if _, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID); err != nil {
			return err
		}
		used, err := uc.repo.InUse(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := entities.EnsureUnused(used); err != nil {
			return err
		}
		return uc.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}

func (uc *WarehouseUseCases) requireBranch(ctx context.Context, companyID vo.CompanyID, id *vo.BranchID) error {
	if id == nil {
		return nil
	}
	ok, err := uc.org.BranchExists(ctx, companyID, *id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrBranchNotFound
	}
	return nil
}

// ---------- สินค้า ----------

type ItemUseCases struct {
	base
	repo       repositories.ItemRepository
	uoms       repositories.UomRepository
	categories repositories.CategoryRepository
}

func (uc *ItemUseCases) Get(ctx context.Context, in dto.ByID[vo.ItemID]) (dto.Item, error) {
	return read(ctx, uc.base, in.Actor, vo.PermItemRead, func() (dto.Item, error) { return uc.repo.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *ItemUseCases) List(ctx context.Context, in dto.List[dto.ItemFilter]) (listing.Result[dto.Item], error) {
	return read(ctx, uc.base, in.Actor, vo.PermItemRead, func() (listing.Result[dto.Item], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *ItemUseCases) Create(ctx context.Context, in dto.Create[entities.ItemData]) (dto.Item, error) {
	var id vo.ItemID
	err := uc.writeWith(ctx, in.Actor, vo.PermItemManage, func(ctx context.Context, s ports.CompanySettings) error {
		if err := uc.requireRefs(ctx, in.CompanyID, in.Data); err != nil {
			return err
		}
		i, err := entities.NewItem(in.CompanyID, in.Data, s.QuantityDecimals)
		if err != nil {
			return err
		}
		id = i.ID()
		return uc.repo.Create(ctx, i)
	})
	if err != nil {
		return dto.Item{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, id)
}

func (uc *ItemUseCases) Update(ctx context.Context, in dto.Update[vo.ItemID, entities.ItemData]) (dto.Item, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermItemManage, func(ctx context.Context, s ports.CompanySettings) error {
		i, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := uc.requireRefs(ctx, in.CompanyID, in.Data); err != nil {
			return err
		}
		moved, err := uc.repo.HasMovements(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := i.Update(in.Data, s.QuantityDecimals, moved); err != nil {
			return err
		}
		return uc.repo.Update(ctx, i)
	})
	if err != nil {
		return dto.Item{}, err
	}
	return uc.repo.Get(ctx, in.CompanyID, in.ID)
}

// Delete: สินค้าที่อยู่ในเอกสาร / มีการเคลื่อนไหวแล้วลบไม่ได้ (ปิดใช้งานแทน)
func (uc *ItemUseCases) Delete(ctx context.Context, in dto.ByID[vo.ItemID]) error {
	return uc.write(ctx, in.Actor, vo.PermItemManage, func(ctx context.Context) error {
		if _, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID); err != nil {
			return err
		}
		used, err := uc.repo.InUse(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := entities.EnsureUnused(used); err != nil {
			return err
		}
		return uc.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}

// requireRefs: หน่วยนับ (บังคับ) และหมวด (ถ้าระบุ) ต้องมีอยู่ในบริษัท, บัญชีที่ตั้งต้องลงรายการได้
func (uc *ItemUseCases) requireRefs(ctx context.Context, companyID vo.CompanyID, d entities.ItemData) error {
	if _, err := uc.uoms.FindByID(ctx, companyID, d.UomID); err != nil {
		return err
	}
	if d.CategoryID != nil {
		if _, err := uc.categories.FindByID(ctx, companyID, *d.CategoryID); err != nil {
			return err
		}
	}
	return uc.checkAccounts(ctx, companyID, d.Accounts.IDs())
}
