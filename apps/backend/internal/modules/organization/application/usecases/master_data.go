package usecases

import (
	"context"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// ---------- Branch ----------

type BranchUseCases struct {
	master[vo.BranchID, *entities.Branch, dto.Branch, dto.BranchFilter]
	repo repositories.BranchRepository
}

func (uc *BranchUseCases) Create(ctx context.Context, in dto.Create[entities.BranchData]) (dto.Branch, error) {
	b, err := entities.NewBranch(in.CompanyID, in.Data)
	if err != nil {
		return dto.Branch{}, err
	}
	return uc.save(ctx, in.Actor, b.ID(), func(ctx context.Context) error { return uc.repo.Create(ctx, b) })
}

func (uc *BranchUseCases) Update(ctx context.Context, in dto.Update[vo.BranchID, entities.BranchData]) (dto.Branch, error) {
	return uc.edit(ctx, in.Actor, in.ID, func(_ context.Context, b *entities.Branch) error { return b.Update(in.Data) })
}

func (uc *BranchUseCases) Delete(ctx context.Context, in dto.ByID[vo.BranchID]) error {
	return uc.remove(ctx, in, func(ctx context.Context, b *entities.Branch) error {
		if err := b.EnsureDeletable(); err != nil {
			return err
		}
		return unused(uc.repo.HasDependents(ctx, in.CompanyID, in.ID))
	})
}

// ---------- Business unit ----------

type BusinessUnitUseCases struct {
	master[vo.BusinessUnitID, *entities.BusinessUnit, dto.BusinessUnit, dto.BusinessUnitFilter]
	repo repositories.BusinessUnitRepository
}

func (uc *BusinessUnitUseCases) Create(ctx context.Context, in dto.Create[entities.BusinessUnitData]) (dto.BusinessUnit, error) {
	u, err := entities.NewBusinessUnit(in.CompanyID, in.Data)
	if err != nil {
		return dto.BusinessUnit{}, err
	}
	return uc.save(ctx, in.Actor, u.ID(), func(ctx context.Context) error {
		if err := uc.checkRefs(ctx, in.CompanyID, u.ID(), in.Data); err != nil {
			return err
		}
		return uc.repo.Create(ctx, u)
	})
}

func (uc *BusinessUnitUseCases) Update(ctx context.Context, in dto.Update[vo.BusinessUnitID, entities.BusinessUnitData]) (dto.BusinessUnit, error) {
	return uc.edit(ctx, in.Actor, in.ID, func(ctx context.Context, u *entities.BusinessUnit) error {
		if err := uc.checkRefs(ctx, in.CompanyID, in.ID, in.Data); err != nil {
			return err
		}
		return u.Update(in.Data)
	})
}

func (uc *BusinessUnitUseCases) Delete(ctx context.Context, in dto.ByID[vo.BusinessUnitID]) error {
	return uc.remove(ctx, in, func(ctx context.Context, _ *entities.BusinessUnit) error {
		return unused(uc.repo.HasDependents(ctx, in.CompanyID, in.ID))
	})
}

func (uc *BusinessUnitUseCases) checkRefs(ctx context.Context, companyID vo.CompanyID, self vo.BusinessUnitID, d entities.BusinessUnitData) error {
	return requireParent(ctx, companyID, self, d.ParentID, uc.repo.Ancestry, domain.ErrBusinessUnitNotFound)
}

// ---------- Department ----------

type DepartmentUseCases struct {
	master[vo.DepartmentID, *entities.Department, dto.Department, dto.DepartmentFilter]
	repo     repositories.DepartmentRepository
	units    repositories.BusinessUnitRepository
	branches repositories.BranchRepository
}

func (uc *DepartmentUseCases) Create(ctx context.Context, in dto.Create[entities.DepartmentData]) (dto.Department, error) {
	d, err := entities.NewDepartment(in.CompanyID, in.Data)
	if err != nil {
		return dto.Department{}, err
	}
	return uc.save(ctx, in.Actor, d.ID(), func(ctx context.Context) error {
		if err := uc.checkRefs(ctx, in.CompanyID, d.ID(), in.Data); err != nil {
			return err
		}
		return uc.repo.Create(ctx, d)
	})
}

func (uc *DepartmentUseCases) Update(ctx context.Context, in dto.Update[vo.DepartmentID, entities.DepartmentData]) (dto.Department, error) {
	return uc.edit(ctx, in.Actor, in.ID, func(ctx context.Context, d *entities.Department) error {
		if err := uc.checkRefs(ctx, in.CompanyID, in.ID, in.Data); err != nil {
			return err
		}
		return d.Update(in.Data)
	})
}

func (uc *DepartmentUseCases) Delete(ctx context.Context, in dto.ByID[vo.DepartmentID]) error {
	return uc.remove(ctx, in, func(ctx context.Context, _ *entities.Department) error {
		return unused(uc.repo.HasDependents(ctx, in.CompanyID, in.ID))
	})
}

func (uc *DepartmentUseCases) checkRefs(ctx context.Context, companyID vo.CompanyID, self vo.DepartmentID, d entities.DepartmentData) error {
	if err := requireParent(ctx, companyID, self, d.ParentID, uc.repo.Ancestry, domain.ErrDepartmentNotFound); err != nil {
		return err
	}
	if err := requireRef(ctx, companyID, d.BusinessUnitID, uc.units.Exists, domain.ErrBusinessUnitNotFound); err != nil {
		return err
	}
	return requireRef(ctx, companyID, d.BranchID, uc.branches.Exists, domain.ErrBranchNotFound)
}

// ---------- Cost center ----------

type CostCenterUseCases struct {
	master[vo.CostCenterID, *entities.CostCenter, dto.CostCenter, dto.CostCenterFilter]
	repo        repositories.CostCenterRepository
	departments repositories.DepartmentRepository
}

func (uc *CostCenterUseCases) Create(ctx context.Context, in dto.Create[entities.CostCenterData]) (dto.CostCenter, error) {
	c, err := entities.NewCostCenter(in.CompanyID, in.Data)
	if err != nil {
		return dto.CostCenter{}, err
	}
	return uc.save(ctx, in.Actor, c.ID(), func(ctx context.Context) error {
		if err := requireRef(ctx, in.CompanyID, in.Data.DepartmentID, uc.departments.Exists, domain.ErrDepartmentNotFound); err != nil {
			return err
		}
		return uc.repo.Create(ctx, c)
	})
}

func (uc *CostCenterUseCases) Update(ctx context.Context, in dto.Update[vo.CostCenterID, entities.CostCenterData]) (dto.CostCenter, error) {
	return uc.edit(ctx, in.Actor, in.ID, func(ctx context.Context, c *entities.CostCenter) error {
		if err := requireRef(ctx, in.CompanyID, in.Data.DepartmentID, uc.departments.Exists, domain.ErrDepartmentNotFound); err != nil {
			return err
		}
		return c.Update(in.Data)
	})
}

func (uc *CostCenterUseCases) Delete(ctx context.Context, in dto.ByID[vo.CostCenterID]) error {
	return uc.remove(ctx, in, func(ctx context.Context, _ *entities.CostCenter) error {
		return unused(uc.repo.HasDependents(ctx, in.CompanyID, in.ID))
	})
}

// ---------- Project / site ----------

type ProjectSiteUseCases struct {
	master[vo.ProjectSiteID, *entities.ProjectSite, dto.ProjectSite, dto.ProjectSiteFilter]
	repo        repositories.ProjectSiteRepository
	branches    repositories.BranchRepository
	costCenters repositories.CostCenterRepository
}

func (uc *ProjectSiteUseCases) Create(ctx context.Context, in dto.Create[entities.ProjectSiteData]) (dto.ProjectSite, error) {
	p, err := entities.NewProjectSite(in.CompanyID, in.Data)
	if err != nil {
		return dto.ProjectSite{}, err
	}
	return uc.save(ctx, in.Actor, p.ID(), func(ctx context.Context) error {
		if err := uc.checkRefs(ctx, in.CompanyID, in.Data); err != nil {
			return err
		}
		return uc.repo.Create(ctx, p)
	})
}

func (uc *ProjectSiteUseCases) Update(ctx context.Context, in dto.Update[vo.ProjectSiteID, entities.ProjectSiteData]) (dto.ProjectSite, error) {
	return uc.edit(ctx, in.Actor, in.ID, func(ctx context.Context, p *entities.ProjectSite) error {
		if err := uc.checkRefs(ctx, in.CompanyID, in.Data); err != nil {
			return err
		}
		return p.Update(in.Data)
	})
}

func (uc *ProjectSiteUseCases) Delete(ctx context.Context, in dto.ByID[vo.ProjectSiteID]) error {
	return uc.remove(ctx, in, nil)
}

func (uc *ProjectSiteUseCases) checkRefs(ctx context.Context, companyID vo.CompanyID, d entities.ProjectSiteData) error {
	if err := requireRef(ctx, companyID, d.BranchID, uc.branches.Exists, domain.ErrBranchNotFound); err != nil {
		return err
	}
	return requireRef(ctx, companyID, d.CostCenterID, uc.costCenters.Exists, domain.ErrCostCenterNotFound)
}
