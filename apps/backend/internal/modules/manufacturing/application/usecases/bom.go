package usecases

import (
	"context"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/application/ports/repositories"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// BomUseCases คือสูตรการผลิต (BOM)
type BomUseCases struct {
	base
	boms repositories.BomRepository
}

// List: 1 query (หน้า + จำนวนทั้งหมด) + สินค้า 1 query
func (uc *BomUseCases) List(ctx context.Context, a dto.Actor, f dto.BomFilter) (listing.Result[dto.BomRow], error) {
	return read(ctx, uc.base, a, vo.PermBomRead, func() (listing.Result[dto.BomRow], error) {
		res, err := uc.boms.List(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, len(res.Items))
		for i := range res.Items {
			refs[i] = &res.Items[i].Item
		}
		return res, uc.itemRefs(ctx, a.CompanyID, refs)
	})
}

func (uc *BomUseCases) Get(ctx context.Context, a dto.Actor, id vo.BomID) (dto.Bom, error) {
	return read(ctx, uc.base, a, vo.PermBomRead, func() (dto.Bom, error) {
		b, err := uc.boms.FindByID(ctx, a.CompanyID, id)
		if err != nil {
			return dto.Bom{}, err
		}
		return uc.view(ctx, b)
	})
}

// Create: สินค้า 1 + บัญชี 1 + INSERT (หัว + บรรทัด)
func (uc *BomUseCases) Create(ctx context.Context, in dto.BomInput) (dto.Bom, error) {
	var b *entities.Bom
	err := uc.writeWith(ctx, in.Actor, vo.PermBomManage, func(ctx context.Context, dec entities.Decimals) error {
		var err error
		if b, err = entities.NewBom(in.CompanyID, in.Data, dec); err != nil {
			return err
		}
		b.Touch(uc.now())
		if err := uc.validate(ctx, b); err != nil {
			return err
		}
		return uc.boms.Create(ctx, b)
	})
	if err != nil {
		return dto.Bom{}, err
	}
	return uc.view(ctx, b)
}

// Update แทนที่สูตรทั้งก้อน — ใบสั่งผลิตที่สร้างแล้วใช้สำเนาสูตรเดิม ไม่ถูกกระทบ
func (uc *BomUseCases) Update(ctx context.Context, in dto.BomInput) (dto.Bom, error) {
	var b *entities.Bom
	err := uc.writeWith(ctx, in.Actor, vo.PermBomManage, func(ctx context.Context, dec entities.Decimals) error {
		var err error
		if b, err = uc.boms.FindByID(ctx, in.CompanyID, in.ID); err != nil {
			return err
		}
		if err := b.Update(in.Data, dec); err != nil {
			return err
		}
		b.Touch(uc.now())
		if err := uc.validate(ctx, b); err != nil {
			return err
		}
		return uc.boms.Update(ctx, b)
	})
	if err != nil {
		return dto.Bom{}, err
	}
	return uc.view(ctx, b)
}

// Delete ลบแบบ soft delete — สูตรที่มีใบสั่งผลิตอ้างอยู่ลบไม่ได้ (ปิดใช้งานแทน)
func (uc *BomUseCases) Delete(ctx context.Context, a dto.Actor, id vo.BomID) error {
	return uc.writeWith(ctx, a, vo.PermBomManage, func(ctx context.Context, _ entities.Decimals) error {
		if _, err := uc.boms.FindByID(ctx, a.CompanyID, id); err != nil {
			return err
		}
		used, err := uc.boms.InUse(ctx, a.CompanyID, id)
		if err != nil {
			return err
		}
		if used {
			return domain.ErrInUse
		}
		return uc.boms.Delete(ctx, a.CompanyID, id)
	})
}

// validate: สินค้าที่ผลิต + วัตถุดิบเป็นชนิด stock ที่ใช้งานอยู่, บัญชีในบรรทัดค่าแรงลงรายการได้
func (uc *BomUseCases) validate(ctx context.Context, b *entities.Bom) error {
	if _, err := uc.stockItems(ctx, b.CompanyID(), b.ItemIDs()); err != nil {
		return err
	}
	if ids := b.AccountIDs(); len(ids) > 0 {
		bad, err := uc.ledger.InvalidAccounts(ctx, b.CompanyID(), ids)
		if err != nil {
			return err
		}
		if len(bad) > 0 {
			return domain.ErrAccountInvalid
		}
	}
	return nil
}

// view: สูตร + ชื่อสินค้า (1 query) + ชื่อบัญชี (1 query ถ้ามี)
func (uc *BomUseCases) view(ctx context.Context, b *entities.Bom) (dto.Bom, error) {
	d := b.Data()
	created, updated := b.Times()
	out := dto.Bom{
		BomRow: dto.BomRow{ID: vo.IDString(b.ID()), Code: d.Code, Name: d.Name, Item: ref(d.ItemID), OutputQuantity: d.OutputQuantity,
			Status: d.Status, IsDefault: d.IsDefault, CreatedAt: created, UpdatedAt: updated},
		Components:  make([]dto.BomComponent, len(b.Components())),
		Conversions: make([]dto.BomConversion, len(b.Conversions())),
	}
	refs := []*dto.Ref{&out.Item}
	for i, c := range b.Components() {
		out.Components[i] = dto.BomComponent{LineNo: c.LineNo, Item: ref(c.ItemID), Quantity: c.Quantity, ScrapPercent: c.ScrapPercent}
	}
	for i := range out.Components {
		refs = append(refs, &out.Components[i].Item)
	}
	if err := uc.itemRefs(ctx, b.CompanyID(), refs); err != nil {
		return dto.Bom{}, err
	}
	accounts, err := uc.ledger.AccountRefs(ctx, b.CompanyID(), b.AccountIDs())
	if err != nil {
		return dto.Bom{}, err
	}
	for i, c := range b.Conversions() {
		out.Conversions[i] = dto.BomConversion{LineNo: c.LineNo, Description: c.Description, Amount: c.Amount}
		if c.AccountID != nil {
			r := accounts[*c.AccountID]
			out.Conversions[i].Account = &dto.Ref{ID: vo.IDString(*c.AccountID), Code: r.Code, Name: r.Name}
		}
	}
	return out, nil
}
