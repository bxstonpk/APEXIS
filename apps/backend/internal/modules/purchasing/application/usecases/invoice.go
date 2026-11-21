package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

// InvoiceUseCases: ใบแจ้งหนี้ผู้ขาย (ตั้งเจ้าหนี้) ตามจำนวนที่รับแล้ว
type InvoiceUseCases struct {
	base
	invoices repositories.InvoiceRepository
	orders   repositories.OrderRepository
}

func (uc *InvoiceUseCases) Get(ctx context.Context, in dto.ByID[vo.InvoiceID]) (dto.Invoice, error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (dto.Invoice, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *InvoiceUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.InvoiceSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (listing.Result[dto.InvoiceSummary], error) {
		return uc.invoices.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create บันทึกใบแจ้งหนี้ใน transaction เดียว:
// lock ใบสั่งซื้อ -> จำนวนไม่เกินที่รับแล้วยังไม่แจ้งหนี้ -> VAT ตามใบสั่งซื้อ -> ล้าง GR/IR
// -> ผลต่างราคา (perpetual + stock: บัญชีผลต่างราคา, อื่นๆ: บัญชีซื้อ) -> Dr ภาษีซื้อ / Cr เจ้าหนี้
func (uc *InvoiceUseCases) Create(ctx context.Context, in dto.Create[dto.InvoiceInput]) (dto.Invoice, error) {
	var id vo.InvoiceID
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "grir_account_id", "ap_account_id"); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.Data.OrderID)
		if err != nil {
			return err
		}
		if !vo.IsDateOnly(in.Data.Date) || !vo.IsDateOnly(in.Data.InvoiceDate) {
			return domain.ErrInvalidDate
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		deliveries := make([]entities.Delivery, len(in.Data.Lines))
		ids := make([]vo.ItemID, 0, len(in.Data.Lines))
		for i, l := range in.Data.Lines {
			deliveries[i] = entities.Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity}
			ol, _, err := o.Line(l.OrderLineNo)
			if err != nil {
				return err
			}
			ids = append(ids, ol.ItemID)
		}
		cleared, err := o.Invoice(deliveries, s.QuantityDecimals, s.AmountDecimals, uc.now())
		if err != nil {
			return err
		}
		system, found, err := uc.inventory.PurchaseContext(ctx, in.CompanyID, in.Data.Date, ids)
		if err != nil {
			return err
		}
		if err := items(found, ids, false); err != nil {
			return err
		}

		terms := o.Terms()
		lines := make([]entities.InvoiceLine, len(deliveries))
		for i, d := range deliveries {
			ol, _, _ := o.Line(d.LineNo)
			price := ol.UnitPrice
			if p := in.Data.Lines[i].UnitPrice; p != nil {
				if p.IsNegative() || !numeric.InRange(*p) || !numeric.FitsScale(*p, numeric.MaxScale) {
					return fmt.Errorf("%w: line %d unit_price must be zero or positive with at most %d decimal places", domain.ErrInvalidLine, d.LineNo, numeric.MaxScale)
				}
				price = *p
			}
			line := entities.InvoiceLine{
				OrderLineNo: d.LineNo, ItemID: ol.ItemID, Quantity: d.Quantity, UnitPrice: price,
				Amounts: services.LineAmounts(d.Quantity, price, terms.Vat, s.AmountDecimals), Cleared: cleared[i],
			}
			if !line.Variance().IsZero() {
				item := found[ol.ItemID]
				acc, what := item.PurchaseAccountID, fmt.Sprintf("purchase account of item %s", item.Code)
				if system == vo.Perpetual && item.Stock {
					acc, what = settings.PriceVarianceAccountID, "price_variance_account_id"
				}
				if acc == nil {
					return fmt.Errorf("%w: %s not set (invoice price differs from receipt)", domain.ErrSettingsMissing, what)
				}
				line.VarianceAccountID = acc
			}
			lines[i] = line
		}
		no, err := uc.number(ctx, in.CompanyID, "PI", period.FiscalYear)
		if err != nil {
			return err
		}
		inv, err := entities.NewInvoice(entities.InvoiceState{
			CompanyID: in.CompanyID, OrderID: o.ID(), SupplierID: o.Data().SupplierID,
			SupplierInvoiceNo: in.Data.SupplierInvoiceNo, InvoiceDate: in.Data.InvoiceDate, Date: in.Data.Date,
			Description: in.Data.Description, Vat: terms.Vat, Lines: lines,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}, terms.PaymentTermDays)
		if err != nil {
			return err
		}
		inputVat := vo.AccountID{}
		if st := inv.State(); !st.Totals.Vat.IsZero() {
			if err := settings.Require("input_vat_account_id"); err != nil {
				return err
			}
			inputVat = *settings.InputVatAccountID
		}
		if journal := inv.Journal(*settings.GRIRAccountID, inputVat, *settings.APAccountID); len(journal) > 0 {
			st := inv.State()
			entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
				CompanyID: in.CompanyID, UserID: in.UserID, JournalID: *settings.JournalID, Date: st.Date,
				Description: "Supplier invoice " + st.SupplierInvoiceNo + " / " + o.State().Approval.OrderNo,
				Reference:   no, Lines: journal,
			})
			if err != nil {
				return err
			}
			inv.SetEntry(entryID, entryNo)
		}
		id = inv.ID()
		if err := uc.invoices.Create(ctx, inv); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Cancel ยกเลิกใบแจ้งหนี้: กลับจำนวนที่แจ้งหนี้ในใบสั่งซื้อ -> กลับรายการบัญชี ณ วันที่เดิม (งวดต้องยังเปิด)
func (uc *InvoiceUseCases) Cancel(ctx context.Context, in dto.ByID[vo.InvoiceID]) (dto.Invoice, error) {
	err := uc.write(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context) error {
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := inv.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := inv.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, s.OrderID)
		if err != nil {
			return err
		}
		if err := o.Uninvoice(inv.Deliveries()); err != nil {
			return err
		}
		if s.Posting.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *s.Posting.EntryID, s.Date, "Cancel supplier invoice "+s.Posting.DocumentNo); err != nil {
				return err
			}
		}
		if err := uc.invoices.UpdateStatus(ctx, inv); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *InvoiceUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error) {
	out, err := uc.invoices.Get(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	refs := make([]*dto.Ref, len(out.Lines))
	for i := range out.Lines {
		refs[i] = &out.Lines[i].Item
	}
	return out, uc.itemRefs(ctx, companyID, refs)
}
