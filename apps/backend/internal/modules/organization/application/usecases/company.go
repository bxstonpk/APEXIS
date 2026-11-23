package usecases

import (
	"context"
	"slices"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// CompanyUseCases จัดการบริษัท (tenant) — สิทธิ์ / สมาชิกของบริษัทอยู่ที่ identity (ผ่าน ports.Membership)
type CompanyUseCases struct {
	base
	members ports.Membership
}

// Create สร้างบริษัท + ให้ผู้สร้างเป็น owner ใน transaction เดียว
//
//	BEGIN -> INSERT organization_companies -> identity: INSERT owner role + membership -> COMMIT
//
// identity เข้าร่วม transaction เดียวกันผ่าน ctx — ถ้าขั้นใดล้มเหลวจะ rollback ทั้งหมด
// ผู้ใช้ที่ active ทุกคนสร้างบริษัทได้ (ถ้าต้องการจำกัดเฉพาะ platform admin ให้เพิ่มการตรวจตรงนี้)
func (uc *CompanyUseCases) Create(ctx context.Context, in dto.CreateCompanyInput) (dto.Company, error) {
	c, err := entities.NewCompany(in.Data)
	if err != nil {
		return dto.Company{}, err
	}
	err = uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.companies.Create(ctx, c); err != nil {
			return err
		}
		return uc.members.ProvisionOwner(ctx, c.ID(), in.UserID)
	})
	if err != nil {
		return dto.Company{}, err
	}
	return companyDTO(c), nil
}

func (uc *CompanyUseCases) Get(ctx context.Context, a dto.Actor) (dto.Company, error) {
	if err := uc.authorize(ctx, a, vo.PermCompanyRead); err != nil {
		return dto.Company{}, err
	}
	c, err := uc.companies.FindByID(ctx, a.CompanyID)
	if err != nil {
		return dto.Company{}, err
	}
	return companyDTO(c), nil
}

// Update แทนที่ข้อมูลบริษัท (ยกเว้นรหัส) — ไม่ผ่าน base.write เพราะต้องแก้บริษัทที่ inactive ได้ (เพื่อเปิดใช้งานกลับ)
func (uc *CompanyUseCases) Update(ctx context.Context, in dto.UpdateCompanyInput) (dto.Company, error) {
	var saved *entities.Company
	err := uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.authorize(ctx, in.Actor, vo.PermCompanyManage); err != nil {
			return err
		}
		c, err := uc.companies.FindByID(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := c.Update(in.Data); err != nil {
			return err
		}
		saved = c
		return uc.companies.Update(ctx, c)
	})
	if err != nil {
		return dto.Company{}, err
	}
	return companyDTO(saved), nil
}

// ListMine คืนบริษัทที่ผู้ใช้เป็นสมาชิก — 2 query:
//  1. identity: id ของบริษัทที่เป็นสมาชิก
//  2. organization: กรอง / เรียง / แบ่งหน้าใน DB เฉพาะ id เหล่านั้น
//
// แยก query เพราะตารางอยู่คนละ module (JOIN ข้าม module ไม่ได้) และจำนวนบริษัทต่อผู้ใช้มีน้อย
func (uc *CompanyUseCases) ListMine(ctx context.Context, in dto.ListMyCompaniesInput) (listing.Result[dto.CompanySummary], error) {
	ids, err := uc.members.CompanyIDsOf(ctx, in.UserID)
	if err != nil {
		return listing.Result[dto.CompanySummary]{}, err
	}
	return uc.companies.ListByIDs(ctx, ids, in.Filter)
}

// GetMine คืนข้อมูลสรุปของบริษัทที่ผู้ใช้เป็นสมาชิก (ไม่ต้องมีสิทธิ์ organization.company:read)
// ใช้แสดงชื่อบริษัทในหน้าระบบ — ไม่ใช่สมาชิก = ErrCompanyNotFound (ไม่บอกว่าบริษัทมีอยู่จริง)
func (uc *CompanyUseCases) GetMine(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (dto.CompanySummary, error) {
	ids, err := uc.members.CompanyIDsOf(ctx, userID)
	if err != nil {
		return dto.CompanySummary{}, err
	}
	if !slices.Contains(ids, companyID) {
		return dto.CompanySummary{}, domain.ErrCompanyNotFound
	}
	res, err := uc.companies.ListByIDs(ctx, []vo.CompanyID{companyID}, dto.CompanyFilter{
		Sort: dto.DefaultCompanySort, Page: listing.Page{Number: 1, Size: 1},
	})
	if err != nil {
		return dto.CompanySummary{}, err
	}
	if len(res.Items) == 0 {
		return dto.CompanySummary{}, domain.ErrCompanyNotFound
	}
	return res.Items[0], nil
}

// Letterhead คืนข้อมูลหัวเอกสาร (ชื่อนิติบุคคล เลขผู้เสียภาษี ที่อยู่ ติดต่อ) ให้สมาชิกทุกคน — ใช้พิมพ์ใบเสนอราคา / ใบกำกับภาษี
// ไม่ต้องมีสิทธิ์ organization.company:read (ไม่มีค่าตั้งบัญชี / ทศนิยม); ไม่ใช่สมาชิก = ErrCompanyNotFound
func (uc *CompanyUseCases) Letterhead(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (dto.Company, error) {
	ids, err := uc.members.CompanyIDsOf(ctx, userID)
	if err != nil {
		return dto.Company{}, err
	}
	if !slices.Contains(ids, companyID) {
		return dto.Company{}, domain.ErrCompanyNotFound
	}
	c, err := uc.companies.FindByID(ctx, companyID)
	if err != nil {
		return dto.Company{}, err
	}
	return companyDTO(c), nil
}

func companyDTO(c *entities.Company) dto.Company {
	d := c.Data()
	return dto.Company{
		ID: c.ID().String(), Code: d.Code, Name: d.Name, TaxID: d.TaxID,
		LegalNameTH: d.LegalNameTH, LegalNameEN: d.LegalNameEN,
		Address: d.Address, Phone: d.Phone, Email: d.Email, Website: d.Website,
		BaseCurrency: d.BaseCurrency, FiscalYearStartMonth: d.FiscalYearStartMonth,
		AmountDecimals: d.AmountDecimals, QuantityDecimals: d.QuantityDecimals, Status: string(d.Status),
	}
}
