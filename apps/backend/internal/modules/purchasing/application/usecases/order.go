package usecases

import (
	"context"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// OrderUseCases: ใบสั่งซื้อ (สินค้า) / ใบสั่งจ้าง (บริการ) draft -> approved -> closed / cancelled
// สองสายใช้กฎเดียวกัน ต่างกันที่ชนิดสินค้าที่รับ, สิทธิ์ และชุดเลขที่ — endpoint ของสายส่ง Kind มา (เอกสารต่างสาย = ไม่พบ)
type OrderUseCases struct {
	base
	orders    repositories.OrderRepository
	suppliers repositories.SupplierRepository
}

func kindOf(k vo.PurchaseKind) vo.PurchaseKind { return vo.KindOrDefault(string(k)) }

func (uc *OrderUseCases) Get(ctx context.Context, in dto.ByID[vo.OrderID]) (dto.Order, error) {
	kind := kindOf(in.Kind)
	return read(ctx, uc.base, in.Actor, kind.Perms().OrderRead, func() (dto.Order, error) {
		out, err := uc.get(ctx, in.CompanyID, in.ID)
		if err == nil && out.Kind != string(kind) {
			return dto.Order{}, domain.ErrOrderNotFound
		}
		return out, err
	})
}

func (uc *OrderUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.OrderSummary], error) {
	in.Filter.Kind = kindOf(in.Filter.Kind)
	return read(ctx, uc.base, in.Actor, in.Filter.Kind.Perms().OrderRead, func() (listing.Result[dto.OrderSummary], error) {
		return uc.orders.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create สร้าง draft — เก็บเงื่อนไข VAT / เครดิตเทอมของผู้ขาย ณ ตอนนี้ไว้ในใบ
func (uc *OrderUseCases) Create(ctx context.Context, in dto.Create[entities.OrderData]) (dto.Order, error) {
	in.Data.Kind = kindOf(in.Data.Kind)
	var id vo.OrderID
	err := uc.writeWith(ctx, in.Actor, in.Data.Kind.Perms().OrderManage, func(ctx context.Context, s ports.CompanySettings) error {
		o, err := uc.create(ctx, in.Actor, s, in.Data)
		if err != nil {
			return err
		}
		id = o.ID()
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// create: ภายใน transaction ที่ตรวจสิทธิ์แล้ว (ใช้ร่วมกับซื้อตรงไม่ผ่านใบสั่ง)
func (uc *OrderUseCases) create(ctx context.Context, a dto.Actor, s ports.CompanySettings, d entities.OrderData) (*entities.Order, error) {
	terms, err := uc.terms(ctx, a.CompanyID, d)
	if err != nil {
		return nil, err
	}
	if err := uc.resolveBranch(ctx, a.CompanyID, &d); err != nil {
		return nil, err
	}
	o, err := entities.NewOrder(a.CompanyID, a.UserID, d, terms, entities.Decimals{Quantity: s.QuantityDecimals, Amount: s.AmountDecimals})
	if err != nil {
		return nil, err
	}
	if err := uc.checkItems(ctx, a.CompanyID, o); err != nil {
		return nil, err
	}
	return o, uc.orders.Create(ctx, o)
}

// find: ใบของบริษัทพร้อม lock — ต่างสายกับ endpoint = ไม่พบ
func (uc *OrderUseCases) find(ctx context.Context, companyID vo.CompanyID, id vo.OrderID, kind vo.PurchaseKind) (*entities.Order, error) {
	o, err := uc.orders.FindForUpdate(ctx, companyID, id)
	if err != nil {
		return nil, err
	}
	if o.Kind() != kind {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

// Update แทนที่ draft ทั้งก้อน — ผู้ขายเดิม: คงอัตรา VAT ที่เก็บไว้ (เปลี่ยนได้แค่รวม / แยก VAT); เปลี่ยนผู้ขาย: ใช้เงื่อนไขของผู้ขายใหม่
func (uc *OrderUseCases) Update(ctx context.Context, in dto.Update[vo.OrderID, entities.OrderData]) (dto.Order, error) {
	kind := kindOf(in.Data.Kind)
	err := uc.writeWith(ctx, in.Actor, kind.Perms().OrderManage, func(ctx context.Context, s ports.CompanySettings) error {
		o, err := uc.find(ctx, in.CompanyID, in.ID, kind)
		if err != nil {
			return err
		}
		if err := o.EnsureDraft(); err != nil {
			return err
		}
		terms := o.Terms()
		if in.Data.SupplierID != o.Data().SupplierID {
			if terms, err = uc.terms(ctx, in.CompanyID, in.Data); err != nil {
				return err
			}
		} else {
			if in.Data.PriceIncludesVat && !terms.Vat.Registered {
				return domain.ErrVatNotApplicable
			}
			terms.Vat.Inclusive = in.Data.PriceIncludesVat
		}
		if err := uc.resolveBranch(ctx, in.CompanyID, &in.Data); err != nil {
			return err
		}
		if err := o.Update(in.Data, terms, entities.Decimals{Quantity: s.QuantityDecimals, Amount: s.AmountDecimals}); err != nil {
			return err
		}
		if err := uc.checkItems(ctx, in.CompanyID, o); err != nil {
			return err
		}
		return uc.orders.Update(ctx, o)
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

// Delete ลบ draft (ลบจริง)
func (uc *OrderUseCases) Delete(ctx context.Context, in dto.ByID[vo.OrderID]) error {
	kind := kindOf(in.Kind)
	return uc.write(ctx, in.Actor, kind.Perms().OrderManage, func(ctx context.Context) error {
		o, err := uc.find(ctx, in.CompanyID, in.ID, kind)
		if err != nil {
			return err
		}
		if err := o.EnsureDraft(); err != nil {
			return err
		}
		return uc.orders.Delete(ctx, in.CompanyID, in.ID)
	})
}

type SetOrderStatus struct {
	dto.ByID[vo.OrderID]
	Status vo.OrderStatus
}

// SetStatus: approved (สิทธิ์ approve: ผู้ขาย / สินค้าต้อง active, ออกเลขที่ตามปีบัญชีของวันที่ใบสั่ง),
// closed (หยุดรับ), cancelled (ยังไม่มีการรับ / แจ้งหนี้)
func (uc *OrderUseCases) SetStatus(ctx context.Context, in SetOrderStatus) (dto.Order, error) {
	kind := kindOf(in.Kind)
	perm := kind.Perms().OrderManage
	if in.Status == vo.OrderApproved {
		perm = kind.Perms().OrderApprove
	}
	err := uc.write(ctx, in.Actor, perm, func(ctx context.Context) error {
		o, err := uc.find(ctx, in.CompanyID, in.ID, kind)
		if err != nil {
			return err
		}
		switch in.Status {
		case vo.OrderApproved:
			err = uc.approve(ctx, in.Actor, o)
		case vo.OrderClosed:
			err = o.Close(in.UserID, uc.now())
		case vo.OrderCancelled:
			err = o.Cancel(in.UserID, uc.now())
		default:
			err = domain.ErrInvalidStatus
		}
		if err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *OrderUseCases) approve(ctx context.Context, a dto.Actor, o *entities.Order) error {
	if err := o.EnsureDraft(); err != nil {
		return err
	}
	s, err := uc.suppliers.FindByID(ctx, a.CompanyID, o.Data().SupplierID)
	if err != nil {
		return err
	}
	if err := s.EnsureActive(); err != nil {
		return err
	}
	if err := uc.checkItems(ctx, a.CompanyID, o); err != nil {
		return err
	}
	p, err := uc.org.PeriodAt(ctx, a.CompanyID, o.Data().Date)
	if err != nil {
		return err
	}
	no, err := uc.number(ctx, a.CompanyID, o.Kind().OrderDoc(), o.Data().Date, p.FiscalYear)
	if err != nil {
		return err
	}
	return o.Approve(entities.Approval{OrderNo: no, FiscalYear: p.FiscalYear, By: a.UserID, At: uc.now()})
}

// terms: ผู้ขายต้องมีอยู่และ active — เงื่อนไข VAT ตามสถานะจด VAT ปัจจุบัน + อัตราในค่าตั้ง
func (uc *OrderUseCases) terms(ctx context.Context, companyID vo.CompanyID, d entities.OrderData) (entities.Terms, error) {
	s, err := uc.suppliers.FindByID(ctx, companyID, d.SupplierID)
	if err != nil {
		return entities.Terms{}, err
	}
	if err := s.EnsureActive(); err != nil {
		return entities.Terms{}, err
	}
	settings, err := uc.settings.Get(ctx, companyID)
	if err != nil {
		return entities.Terms{}, err
	}
	return entities.SupplierTerms(s, settings.VatRate, d.PriceIncludesVat)
}

// resolveBranch: สาขาผู้ขายต้องมีในทะเบียน (ว่าง = สาขาหลัก → เขียนเลขสาขาหลักกลับ)
func (uc *OrderUseCases) resolveBranch(ctx context.Context, companyID vo.CompanyID, d *entities.OrderData) error {
	s, err := uc.suppliers.FindByID(ctx, companyID, d.SupplierID)
	if err != nil {
		return err
	}
	b, err := s.Branch(d.SupplierBranchNo)
	if err != nil {
		return err
	}
	d.SupplierBranchNo = b.BranchNo
	return nil
}

// checkItems: สินค้าทุกบรรทัดต้องมีอยู่, active และตรงสายของใบ (สินค้า / บริการ) — 1 query ไม่ว่ามีกี่บรรทัด
func (uc *OrderUseCases) checkItems(ctx context.Context, companyID vo.CompanyID, o *entities.Order) error {
	ids := o.ItemIDs()
	found, err := uc.inventory.Items(ctx, companyID, ids)
	if err != nil {
		return err
	}
	if err := items(found, ids, true); err != nil {
		return err
	}
	return itemKinds(found, ids, o.Kind())
}

// itemKinds: สินค้าทุกตัวต้องตรงสายของเอกสาร
func itemKinds(found map[vo.ItemID]ports.Item, ids []vo.ItemID, kind vo.PurchaseKind) error {
	for _, id := range ids {
		i := found[id]
		if err := entities.CheckLineKind(kind, i.Service, i.Code); err != nil {
			return err
		}
	}
	return nil
}

func (uc *OrderUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error) {
	out, err := uc.orders.Get(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	refs := make([]*dto.Ref, len(out.Lines))
	for i := range out.Lines {
		refs[i] = &out.Lines[i].Item
	}
	return out, uc.itemRefs(ctx, companyID, refs)
}
