package usecases

import (
	"context"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

func decimals(s ports.CompanySettings) entities.Decimals {
	return entities.Decimals{Quantity: s.QuantityDecimals, Amount: s.AmountDecimals}
}

// customerTerms: ลูกค้าต้องมีอยู่และ active และมีสาขาที่เลือก (*branchNo ว่าง = สาขาหลัก → เขียนเลขสาขาหลักกลับ)
// VAT จากค่าตั้งขาย ณ ตอนนี้ + เครดิตเทอมของลูกค้า
func (b base) customerTerms(ctx context.Context, customers repositories.CustomerRepository, companyID vo.CompanyID, id vo.CustomerID, branchNo *string, inclusive bool) (*entities.Customer, entities.Terms, error) {
	c, err := customers.FindByID(ctx, companyID, id)
	if err != nil {
		return nil, entities.Terms{}, err
	}
	if err := c.EnsureActive(); err != nil {
		return nil, entities.Terms{}, err
	}
	branch, err := c.Branch(*branchNo)
	if err != nil {
		return nil, entities.Terms{}, err
	}
	*branchNo = branch.BranchNo
	s, err := b.settings.Get(ctx, companyID)
	if err != nil {
		return nil, entities.Terms{}, err
	}
	vat, err := s.Terms(inclusive)
	if err != nil {
		return nil, entities.Terms{}, err
	}
	return c, entities.Terms{Vat: vat, PaymentTermDays: c.Data().PaymentTermDays}, nil
}

func (b base) requireItems(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) error {
	found, err := b.inventory.Items(ctx, companyID, ids)
	if err != nil {
		return err
	}
	return checkItems(found, ids, true)
}

// ---------- ใบเสนอราคา ----------

type QuotationUseCases struct {
	base
	quotations repositories.QuotationRepository
	customers  repositories.CustomerRepository
}

func (uc *QuotationUseCases) Get(ctx context.Context, in dto.ByID[vo.QuotationID]) (dto.Quotation, error) {
	return read(ctx, uc.base, in.Actor, vo.PermQuotationRead, func() (dto.Quotation, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *QuotationUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.QuotationID) (dto.Quotation, error) {
	q, err := uc.quotations.Get(ctx, companyID, id)
	if err != nil {
		return q, err
	}
	return q, uc.itemRefs(ctx, companyID, lineRefs(q.Lines))
}

func (uc *QuotationUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.SalesDocSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermQuotationRead, func() (listing.Result[dto.SalesDocSummary], error) {
		return uc.quotations.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *QuotationUseCases) Create(ctx context.Context, in dto.Create[entities.QuotationData]) (dto.Quotation, error) {
	var id vo.QuotationID
	err := uc.writeWith(ctx, in.Actor, vo.PermQuotationManage, func(ctx context.Context, s ports.CompanySettings) error {
		_, terms, err := uc.customerTerms(ctx, uc.customers, in.CompanyID, in.Data.CustomerID, &in.Data.CustomerBranchNo, in.Data.PriceIncludesVat)
		if err != nil {
			return err
		}
		q, err := entities.NewQuotation(in.CompanyID, in.UserID, in.Data, terms, decimals(s))
		if err != nil {
			return err
		}
		if err := uc.requireItems(ctx, in.CompanyID, q.ItemIDs()); err != nil {
			return err
		}
		id = q.ID()
		return uc.quotations.Create(ctx, q)
	})
	if err != nil {
		return dto.Quotation{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Update แทนที่ draft — เงื่อนไข VAT / เครดิตเทอมตามค่าตั้ง / ลูกค้า ณ ตอนแก้ (ยังเป็น draft)
func (uc *QuotationUseCases) Update(ctx context.Context, in dto.Update[vo.QuotationID, entities.QuotationData]) (dto.Quotation, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermQuotationManage, func(ctx context.Context, s ports.CompanySettings) error {
		q, err := uc.quotations.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := q.EnsureDraft(); err != nil {
			return err
		}
		_, terms, err := uc.customerTerms(ctx, uc.customers, in.CompanyID, in.Data.CustomerID, &in.Data.CustomerBranchNo, in.Data.PriceIncludesVat)
		if err != nil {
			return err
		}
		if err := q.Update(in.Data, terms, decimals(s)); err != nil {
			return err
		}
		if err := uc.requireItems(ctx, in.CompanyID, q.ItemIDs()); err != nil {
			return err
		}
		return uc.quotations.Update(ctx, q)
	})
	if err != nil {
		return dto.Quotation{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *QuotationUseCases) Delete(ctx context.Context, in dto.ByID[vo.QuotationID]) error {
	return uc.write(ctx, in.Actor, vo.PermQuotationManage, func(ctx context.Context) error {
		q, err := uc.quotations.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := q.EnsureDraft(); err != nil {
			return err
		}
		return uc.quotations.Delete(ctx, in.CompanyID, in.ID)
	})
}

// SetStatus: sent (ออกเลขที่ตามปีบัญชีของวันที่ใบเสนอราคา) / accepted / rejected
func (uc *QuotationUseCases) SetStatus(ctx context.Context, in dto.ByID[vo.QuotationID], status vo.QuotationStatus) (dto.Quotation, error) {
	err := uc.write(ctx, in.Actor, vo.PermQuotationManage, func(ctx context.Context) error {
		q, err := uc.quotations.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if status == vo.QuotationSent {
			date := q.State().Data.Date
			p, err := uc.org.PeriodAt(ctx, in.CompanyID, date)
			if err != nil {
				return err
			}
			no, err := uc.number(ctx, in.CompanyID, vo.DocQuotation, date, p.FiscalYear)
			if err != nil {
				return err
			}
			err = q.Send(no, p.FiscalYear)
			if err != nil {
				return err
			}
		} else if err := q.Respond(status); err != nil {
			return err
		}
		return uc.quotations.SaveStatus(ctx, q)
	})
	if err != nil {
		return dto.Quotation{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

// ---------- ใบสั่งขาย ----------

type OrderUseCases struct {
	base
	orders     repositories.OrderRepository
	quotations repositories.QuotationRepository
	customers  repositories.CustomerRepository
}

func (uc *OrderUseCases) Get(ctx context.Context, in dto.ByID[vo.OrderID]) (dto.Order, error) {
	return read(ctx, uc.base, in.Actor, vo.PermOrderRead, func() (dto.Order, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *OrderUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.OrderID) (dto.Order, error) {
	o, err := uc.orders.Get(ctx, companyID, id)
	if err != nil {
		return o, err
	}
	return o, uc.itemRefs(ctx, companyID, lineRefs(o.Lines))
}

func (uc *OrderUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.SalesDocSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermOrderRead, func() (listing.Result[dto.SalesDocSummary], error) {
		return uc.orders.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create สร้าง draft — มี QuotationID = คัดลอกลูกค้า / บรรทัด / เงื่อนไขจากใบเสนอราคา (ต้องส่งแล้ว / ยอมรับ, ยังไม่หมดอายุ)
func (uc *OrderUseCases) Create(ctx context.Context, in dto.Create[entities.OrderData]) (dto.Order, error) {
	var id vo.OrderID
	err := uc.writeWith(ctx, in.Actor, vo.PermOrderManage, func(ctx context.Context, s ports.CompanySettings) error {
		data := in.Data
		var terms entities.Terms
		var quotation *entities.Quotation
		if data.QuotationID != nil {
			q, err := uc.quotations.FindForUpdate(ctx, in.CompanyID, *data.QuotationID)
			if err != nil {
				return err
			}
			qs := q.State()
			data.CustomerID, data.CustomerBranchNo, data.PriceIncludesVat, data.Lines, terms = qs.Data.CustomerID, qs.Data.CustomerBranchNo, qs.Terms.Vat.Inclusive, qs.Data.Lines, qs.Terms
			if data.Reference == "" {
				data.Reference = qs.QuotationNo
			}
			if data.Description == "" {
				data.Description = qs.Data.Description
			}
			quotation = q
		} else {
			var err error
			if _, terms, err = uc.customerTerms(ctx, uc.customers, in.CompanyID, data.CustomerID, &data.CustomerBranchNo, data.PriceIncludesVat); err != nil {
				return err
			}
		}
		o, err := entities.NewOrder(in.CompanyID, in.UserID, data, terms, decimals(s))
		if err != nil {
			return err
		}
		if err := uc.requireItems(ctx, in.CompanyID, o.ItemIDs()); err != nil {
			return err
		}
		if quotation != nil {
			if err := quotation.Convert(o.ID(), data.Date); err != nil {
				return err
			}
		}
		id = o.ID()
		if err := uc.orders.Create(ctx, o); err != nil {
			return err
		}
		if quotation != nil {
			return uc.quotations.SaveStatus(ctx, quotation)
		}
		return nil
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Update แทนที่ draft — ใบที่มาจากใบเสนอราคาคงเงื่อนไข VAT เดิม
func (uc *OrderUseCases) Update(ctx context.Context, in dto.Update[vo.OrderID, entities.OrderData]) (dto.Order, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermOrderManage, func(ctx context.Context, s ports.CompanySettings) error {
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := o.EnsureDraft(); err != nil {
			return err
		}
		st := o.State()
		data, terms := in.Data, st.Terms
		data.QuotationID = st.Data.QuotationID
		if data.QuotationID != nil {
			// ลูกค้า / VAT ตามใบเสนอราคา — เปลี่ยนได้เฉพาะสาขาของลูกค้ารายเดิม
			data.CustomerID, data.PriceIncludesVat = st.Data.CustomerID, st.Data.PriceIncludesVat
			c, err := uc.customers.FindByID(ctx, in.CompanyID, data.CustomerID)
			if err != nil {
				return err
			}
			b, err := c.Branch(data.CustomerBranchNo)
			if err != nil {
				return err
			}
			data.CustomerBranchNo = b.BranchNo
		} else if _, terms, err = uc.customerTerms(ctx, uc.customers, in.CompanyID, data.CustomerID, &data.CustomerBranchNo, data.PriceIncludesVat); err != nil {
			return err
		}
		if err := o.Update(data, terms, decimals(s)); err != nil {
			return err
		}
		if err := uc.requireItems(ctx, in.CompanyID, o.ItemIDs()); err != nil {
			return err
		}
		return uc.orders.Update(ctx, o)
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *OrderUseCases) Delete(ctx context.Context, in dto.ByID[vo.OrderID]) error {
	return uc.write(ctx, in.Actor, vo.PermOrderManage, func(ctx context.Context) error {
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := o.EnsureDraft(); err != nil {
			return err
		}
		if o.State().Data.QuotationID != nil {
			return domain.ErrQuotationUsed // ใบที่แปลงจากใบเสนอราคา: ยกเลิกแทนการลบ (คงประวัติ)
		}
		return uc.orders.Delete(ctx, in.CompanyID, in.ID)
	})
}

// SetStatus: approved (สิทธิ์ approve, ออกเลขที่, เตือนวงเงินเครดิต) / closed / cancelled
func (uc *OrderUseCases) SetStatus(ctx context.Context, in dto.ByID[vo.OrderID], status vo.OrderStatus) (dto.Order, error) {
	perm := vo.PermOrderManage
	if status == vo.OrderApproved {
		perm = vo.PermOrderApprove
	}
	var warnings []string
	err := uc.writeWith(ctx, in.Actor, perm, func(ctx context.Context, s ports.CompanySettings) error {
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		switch status {
		case vo.OrderApproved:
			if warnings, err = uc.approve(ctx, in.Actor, o, s.AmountDecimals); err != nil {
				return err
			}
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
	out, err := uc.get(ctx, in.CompanyID, in.ID)
	out.Warnings = warnings
	return out, err
}

// approve: ลูกค้า / สินค้า active, เลขที่ตามปีบัญชีของวันที่ใบสั่งขาย, เตือน (ไม่ห้าม) เมื่อยอดค้างเกินวงเงิน
func (uc *OrderUseCases) approve(ctx context.Context, a dto.Actor, o *entities.Order, amountDecimals int) ([]string, error) {
	if err := o.EnsureDraft(); err != nil {
		return nil, err
	}
	st := o.State()
	c, err := uc.customers.FindByID(ctx, a.CompanyID, st.Data.CustomerID)
	if err != nil {
		return nil, err
	}
	if err := c.EnsureActive(); err != nil {
		return nil, err
	}
	if err := uc.requireItems(ctx, a.CompanyID, o.ItemIDs()); err != nil {
		return nil, err
	}
	p, err := uc.org.PeriodAt(ctx, a.CompanyID, st.Data.Date)
	if err != nil {
		return nil, err
	}
	no, err := uc.number(ctx, a.CompanyID, vo.DocOrder, st.Data.Date, p.FiscalYear)
	if err != nil {
		return nil, err
	}
	if err := o.Approve(entities.Approval{OrderNo: no, FiscalYear: p.FiscalYear, By: a.UserID, At: uc.now()}); err != nil {
		return nil, err
	}
	var warnings []string
	if c.Data().CreditLimit != nil {
		exposure, err := uc.orders.Exposure(ctx, a.CompanyID, c.ID(), o.ID(), amountDecimals)
		if err != nil {
			return nil, err
		}
		if w := c.CreditWarning(exposure.Add(o.OpenAmount(amountDecimals))); w != "" {
			warnings = append(warnings, w)
		}
	}
	return warnings, nil
}
