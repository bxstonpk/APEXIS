package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

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

// InvoiceUseCases: ซื้อสินค้า (ใบแจ้งหนี้ / ใบกำกับภาษีผู้ขาย ตามของที่รับแล้ว) และซื้อบริการ (ตามงานที่ตรวจรับแล้ว)
//
//	ซื้อสินค้า: Dr GR/IR (ล้าง) ± ผลต่างราคา + Dr ภาษีซื้อ — Cr เจ้าหนี้ (ใบแจ้งหนี้ของผู้ขาย = ใบกำกับภาษี)
//	ซื้อบริการ: Dr ค่าใช้จ่าย + Dr ภาษีซื้อยังไม่ถึงกำหนด — Cr เจ้าหนี้ (ผู้ขายออกใบกำกับเมื่อได้รับเงิน ม.78/1)
//	           ได้รับใบกำกับภายหลัง (บันทึกที่จัดซื้อ): Dr ภาษีซื้อ / Cr ภาษีซื้อยังไม่ถึงกำหนด (ม.82/3)
//	           ใบกำกับมาพร้อมใบแจ้งหนี้: Dr ภาษีซื้อ ทันที
type InvoiceUseCases struct {
	base
	invoices    repositories.InvoiceRepository
	orders      repositories.OrderRepository
	suppliers   repositories.SupplierRepository
	credits     repositories.CreditNoteRepository
	taxInvoices repositories.TaxInvoiceRepository
	orderUC     *OrderUseCases
	receiptUC   *ReceiptUseCases
}

func (uc *InvoiceUseCases) Get(ctx context.Context, in dto.ByID[vo.InvoiceID]) (dto.Invoice, error) {
	kind := kindOf(in.Kind)
	return read(ctx, uc.base, in.Actor, kind.Perms().InvoiceRead, func() (dto.Invoice, error) {
		out, err := uc.get(ctx, in.CompanyID, in.ID)
		if err == nil && out.Kind != string(kind) {
			return dto.Invoice{}, domain.ErrInvoiceNotFound
		}
		return out, err
	})
}

func (uc *InvoiceUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.InvoiceSummary], error) {
	in.Filter.Kind = kindOf(in.Filter.Kind)
	return read(ctx, uc.base, in.Actor, in.Filter.Kind.Perms().InvoiceRead, func() (listing.Result[dto.InvoiceSummary], error) {
		return uc.invoices.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create บันทึกใบแจ้งหนี้ตามใบสั่ง (ซื้อสินค้า / ซื้อบริการ) ใน transaction เดียว
func (uc *InvoiceUseCases) Create(ctx context.Context, in dto.Create[dto.InvoiceInput]) (dto.Invoice, error) {
	kind := kindOf(in.Data.Kind)
	var id vo.InvoiceID
	err := uc.writeWith(ctx, in.Actor, kind.Perms().InvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		inv, err := uc.create(ctx, in.Actor, s, in.Data)
		if err != nil {
			return err
		}
		id = inv.ID()
		return nil
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// CreateDirect: ซื้อสินค้า / ซื้อบริการตรงไม่ผ่านใบสั่ง — ระบบออกใบสั่ง (อนุมัติ) + ใบรับสินค้า / ใบตรวจรับงานเต็มจำนวน
// + ใบแจ้งหนี้ ใน transaction เดียว (เอกสารครบชุดเหมือนผ่านใบสั่ง — รายงานค้างรับ / ค้างแจ้งหนี้ไม่มีค้าง)
// สิทธิ์: บันทึกใบแจ้งหนี้ของสายนั้น (ไม่ต้องมีสิทธิ์อนุมัติใบสั่ง / รับสินค้าแยก)
func (uc *InvoiceUseCases) CreateDirect(ctx context.Context, in dto.Create[dto.DirectInvoiceInput]) (dto.Invoice, error) {
	d := in.Data
	kind := kindOf(d.Kind)
	var id vo.InvoiceID
	err := uc.writeWith(ctx, in.Actor, kind.Perms().InvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		if !vo.IsDateOnly(d.Date) {
			return domain.ErrInvalidDate
		}
		lines := make([]entities.OrderLineData, len(d.Lines))
		for i, l := range d.Lines {
			lines[i] = entities.OrderLineData{ItemID: l.ItemID, Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
		}
		o, err := uc.orderUC.create(ctx, in.Actor, s, entities.OrderData{
			Kind: kind, SupplierID: d.SupplierID, SupplierBranchNo: d.SupplierBranchNo, Date: d.Date, Reference: d.Reference,
			Description: d.Description, PriceIncludesVat: d.PriceIncludesVat, Lines: lines,
		})
		if err != nil {
			return err
		}
		if err := uc.orderUC.approve(ctx, in.Actor, o); err != nil {
			return err
		}
		if err := uc.orders.SaveProgress(ctx, o); err != nil {
			return err
		}
		qty := make([]dto.LineQty, len(d.Lines))
		for i, l := range d.Lines {
			qty[i] = dto.LineQty{OrderLineNo: i + 1, Quantity: l.Quantity, AccountID: l.AccountID}
		}
		if _, err := uc.receiptUC.create(ctx, in.Actor, s, dto.ReceiptInput{
			Kind: kind, OrderID: o.ID(), Date: d.Date, WarehouseID: d.WarehouseID, Reference: d.SupplierInvoiceNo, Description: d.Description, Lines: qty,
		}); err != nil {
			return err
		}
		inv, err := uc.create(ctx, in.Actor, s, dto.InvoiceInput{
			Kind: kind, OrderID: o.ID(), SupplierInvoiceNo: d.SupplierInvoiceNo, InvoiceDate: d.InvoiceDate, Date: d.Date,
			Description: d.Description, Lines: qty, TaxInvoiceNo: d.TaxInvoiceNo, TaxInvoiceDate: d.TaxInvoiceDate,
		})
		if err != nil {
			return err
		}
		id = inv.ID()
		return nil
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// create: ภายใน transaction ที่ตรวจสิทธิ์แล้ว
//
//	lock ใบสั่ง -> จำนวนไม่เกินที่รับ / ตรวจรับแล้วยังไม่แจ้งหนี้ -> VAT ตามใบสั่ง
//	สินค้า: ล้าง GR/IR -> ผลต่างราคา (perpetual + stock: บัญชีผลต่างราคา, อื่นๆ: บัญชีซื้อ) -> Dr ภาษีซื้อ / Cr เจ้าหนี้
//	บริการ: Dr ค่าใช้จ่ายของบรรทัด -> Dr ภาษีซื้อยังไม่ถึงกำหนด (หรือภาษีซื้อเมื่อได้รับใบกำกับพร้อมกัน) / Cr เจ้าหนี้
func (uc *InvoiceUseCases) create(ctx context.Context, a dto.Actor, s ports.CompanySettings, in dto.InvoiceInput) (*entities.Invoice, error) {
	kind := kindOf(in.Kind)
	settings, err := uc.settings.Get(ctx, a.CompanyID)
	if err != nil {
		return nil, err
	}
	required := []string{"journal_id", "ap_account_id"}
	if kind == vo.KindGoods {
		required = append(required, "grir_account_id")
	}
	if err := settings.Require(required...); err != nil {
		return nil, err
	}
	o, err := uc.orders.FindForUpdate(ctx, a.CompanyID, in.OrderID)
	if err != nil {
		return nil, err
	}
	if o.Kind() != kind {
		return nil, domain.ErrOrderNotFound
	}
	if !vo.IsDateOnly(in.Date) || !vo.IsDateOnly(in.InvoiceDate) {
		return nil, domain.ErrInvalidDate
	}
	period, err := uc.openPeriod(ctx, a.CompanyID, in.Date)
	if err != nil {
		return nil, err
	}
	deliveries := make([]entities.Delivery, len(in.Lines))
	ids := make([]vo.ItemID, 0, len(in.Lines))
	for i, l := range in.Lines {
		deliveries[i] = entities.Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity}
		ol, _, err := o.Line(l.OrderLineNo)
		if err != nil {
			return nil, err
		}
		ids = append(ids, ol.ItemID)
	}
	cleared, err := o.Invoice(deliveries, s.QuantityDecimals, s.AmountDecimals, uc.now())
	if err != nil {
		return nil, err
	}
	system, found, err := uc.inventory.PurchaseContext(ctx, a.CompanyID, in.Date, ids)
	if err != nil {
		return nil, err
	}
	if err := items(found, ids, false); err != nil {
		return nil, err
	}

	terms := o.Terms()
	lines := make([]entities.InvoiceLine, len(deliveries))
	for i, d := range deliveries {
		ol, _, _ := o.Line(d.LineNo)
		price := ol.UnitPrice
		if p := in.Lines[i].UnitPrice; p != nil {
			if p.IsNegative() || !numeric.InRange(*p) || !numeric.FitsScale(*p, numeric.MaxScale) {
				return nil, fmt.Errorf("%w: line %d unit_price must be zero or positive with at most %d decimal places", domain.ErrInvalidLine, d.LineNo, numeric.MaxScale)
			}
			price = *p
		}
		line := entities.InvoiceLine{
			OrderLineNo: d.LineNo, ItemID: ol.ItemID, Quantity: d.Quantity, UnitPrice: price,
			Amounts: services.LineAmounts(d.Quantity, price, terms.Vat, s.AmountDecimals), Cleared: cleared[i],
		}
		item := found[ol.ItemID]
		switch {
		case kind == vo.KindService:
			// ค่าใช้จ่ายของบรรทัด: ระบุเอง -> ค่าตั้ง service_expense_account_id -> บัญชีซื้อของสินค้า
			acc := in.Lines[i].AccountID
			if acc == nil {
				acc = settings.ServiceExpenseAccountID
			}
			if acc == nil {
				acc = item.PurchaseAccountID
			}
			if acc == nil {
				return nil, fmt.Errorf("%w: service_expense_account_id or purchase account of item %s not set", domain.ErrSettingsMissing, item.Code)
			}
			line.VarianceAccountID = acc
		case !line.Variance().IsZero():
			acc, what := item.PurchaseAccountID, fmt.Sprintf("purchase account of item %s", item.Code)
			if system == vo.Perpetual && item.Stock {
				acc, what = settings.PriceVarianceAccountID, "price_variance_account_id"
			}
			if acc == nil {
				return nil, fmt.Errorf("%w: %s not set (invoice price differs from receipt)", domain.ErrSettingsMissing, what)
			}
			line.VarianceAccountID = acc
		}
		lines[i] = line
	}
	// บัญชีที่ระบุเองต่อบรรทัดต้องลงรายการได้ (ตรวจครั้งเดียว)
	if kind == vo.KindService {
		var custom []vo.AccountID
		for _, l := range in.Lines {
			if l.AccountID != nil {
				custom = append(custom, *l.AccountID)
			}
		}
		if len(custom) > 0 {
			bad, err := uc.ledger.InvalidAccounts(ctx, a.CompanyID, custom)
			if err != nil {
				return nil, err
			}
			if len(bad) > 0 {
				return nil, fmt.Errorf("%w: %s", domain.ErrAccountInvalid, bad[0])
			}
		}
	}
	no, err := uc.number(ctx, a.CompanyID, kind.InvoiceDoc(), in.Date, period.FiscalYear)
	if err != nil {
		return nil, err
	}
	// ตรวจหลังออกเลข: แถวลำดับถูก lock แล้ว ใบแจ้งหนี้ช่วงเดียวกันจึงต่อคิวกัน (unique ที่ DB กันกรณีที่เหลือ)
	used, err := uc.invoices.SupplierInvoiceNoUsed(ctx, a.CompanyID, o.Data().SupplierID, strings.TrimSpace(in.SupplierInvoiceNo))
	if err != nil {
		return nil, err
	}
	if used {
		return nil, domain.ErrInvoiceNoExists
	}
	billFrom, err := uc.billFrom(ctx, a.CompanyID, o.Data().SupplierID, o.Data().SupplierBranchNo)
	if err != nil {
		return nil, err
	}
	taxNo := ""
	var taxDate *time.Time
	if kind == vo.KindService {
		taxNo, taxDate = strings.TrimSpace(in.TaxInvoiceNo), in.TaxInvoiceDate
		if taxNo != "" && taxDate == nil {
			d := in.InvoiceDate
			taxDate = &d
		}
	}
	inv, err := entities.NewInvoice(entities.InvoiceState{
		CompanyID: a.CompanyID, Kind: kind, OrderID: o.ID(), SupplierID: o.Data().SupplierID, BillFrom: billFrom,
		SupplierInvoiceNo: in.SupplierInvoiceNo, InvoiceDate: in.InvoiceDate, Date: in.Date,
		Description: in.Description, Vat: terms.Vat, Lines: lines, TaxInvoiceNo: taxNo, TaxInvoiceDate: taxDate,
		Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: a.UserID, At: uc.now()},
	}, terms.PaymentTermDays)
	if err != nil {
		return nil, err
	}
	if taxNo != "" {
		if used, err := uc.taxInvoices.NoUsed(ctx, a.CompanyID, o.Data().SupplierID, taxNo); err != nil {
			return nil, err
		} else if used {
			return nil, domain.ErrTaxInvoiceNoExists
		}
	}
	vatAccount := vo.AccountID{}
	if key := inv.VatAccountKey(); key != "" {
		if err := settings.Require(key); err != nil {
			return nil, err
		}
		vatAccount = *settings.InputVatAccountID
		if key == "undue_input_vat_account_id" {
			vatAccount = *settings.UndueInputVatAccountID
		}
	}
	grir := vo.AccountID{}
	if settings.GRIRAccountID != nil {
		grir = *settings.GRIRAccountID
	}
	if journal := inv.Journal(grir, vatAccount, *settings.APAccountID); len(journal) > 0 {
		st := inv.State()
		what := "Supplier invoice "
		if kind == vo.KindService {
			what = "Supplier service invoice "
		}
		entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
			CompanyID: a.CompanyID, UserID: a.UserID, JournalID: *settings.JournalID, Date: st.Date,
			Description: what + st.SupplierInvoiceNo + " / " + o.State().Approval.OrderNo,
			Reference:   no, Lines: journal,
		})
		if err != nil {
			return nil, err
		}
		inv.SetEntry(entryID, entryNo)
	}
	if err := uc.invoices.Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, uc.orders.SaveProgress(ctx, o)
}

// Cancel ยกเลิกใบแจ้งหนี้: กลับจำนวนที่แจ้งหนี้ในใบสั่ง -> กลับรายการบัญชี ณ วันที่เดิม (งวดต้องยังเปิด)
// ห้ามเมื่อมีการจ่าย / ใบลดหนี้ / ใบกำกับภาษีที่บันทึกภายหลัง
func (uc *InvoiceUseCases) Cancel(ctx context.Context, in dto.ByID[vo.InvoiceID]) (dto.Invoice, error) {
	kind := kindOf(in.Kind)
	err := uc.write(ctx, in.Actor, kind.Perms().InvoicePost, func(ctx context.Context) error {
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if inv.Kind() != kind {
			return domain.ErrInvoiceNotFound
		}
		// ใบลดหนี้ตัดยอดผ่านการตัดยอดเหมือนใบจ่าย — แจ้งให้ยกเลิกใบลดหนี้ก่อน (ชัดกว่า "มีการจ่ายแล้ว")
		if credited, err := uc.credits.HasPosted(ctx, in.CompanyID, in.ID); err != nil {
			return err
		} else if credited {
			return domain.ErrInvoiceHasCredits
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

// RecordTaxInvoice: บันทึกใบกำกับภาษีที่ได้รับจากผู้ขาย (ซื้อบริการ) — ภาษีซื้อยังไม่ถึงกำหนดถึงกำหนด (ใช้สิทธิได้)
//
//	lock ใบแจ้งหนี้ -> เลขที่ใบกำกับไม่ซ้ำต่อผู้ขาย -> ภาษี (ไม่ส่ง = ตามสัดส่วนที่จ่ายแล้ว / ที่เหลือทั้งหมด) ไม่เกินที่รอ
//	-> งวดของวันที่ลงบัญชีต้องเปิด -> Dr ภาษีซื้อ / Cr ภาษีซื้อยังไม่ถึงกำหนด -> บันทึก
func (uc *InvoiceUseCases) RecordTaxInvoice(ctx context.Context, in dto.Create[dto.TaxInvoiceInput]) (dto.Invoice, error) {
	d := in.Data
	err := uc.writeWith(ctx, in.Actor, vo.PermServiceInvoicePost, func(ctx context.Context, cs ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "input_vat_account_id", "undue_input_vat_account_id"); err != nil {
			return err
		}
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Kind() != vo.KindService {
			return domain.ErrNoTaxInvoiceToRecord
		}
		no := strings.TrimSpace(d.TaxInvoiceNo)
		if no == "" || len([]rune(no)) > 50 {
			return domain.ErrInvalidText
		}
		if !vo.IsDateOnly(d.TaxInvoiceDate) {
			return domain.ErrInvalidDate
		}
		date := d.TaxInvoiceDate
		if d.Date != nil {
			if !vo.IsDateOnly(*d.Date) {
				return domain.ErrInvalidDate
			}
			date = *d.Date
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, date)
		if err != nil {
			return err
		}
		st := inv.State()
		if used, err := uc.taxInvoices.NoUsed(ctx, in.CompanyID, st.SupplierID, no); err != nil {
			return err
		} else if used {
			return domain.ErrTaxInvoiceNoExists
		}
		vat := inv.DefaultRecognition(uc.paidByPayments(ctx, in.CompanyID, inv), cs.AmountDecimals)
		if d.Vat != nil {
			vat = *d.Vat
		}
		base, err := inv.RecordTaxInvoice(vat, cs.AmountDecimals)
		if err != nil {
			return err
		}
		_ = period
		rec := dto.TaxInvoiceRecord{
			ID: uuid.Must(uuid.NewV7()), CompanyID: in.CompanyID, InvoiceID: inv.ID(), SupplierID: st.SupplierID,
			TaxInvoiceNo: no, TaxInvoiceDate: d.TaxInvoiceDate, Date: date, Base: base, Vat: vat, CreatedBy: in.UserID,
		}
		j := services.NewJournal()
		j.Debit(*settings.InputVatAccountID, vat)
		j.Credit(*settings.UndueInputVatAccountID, vat)
		entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
			CompanyID: in.CompanyID, UserID: in.UserID, JournalID: *settings.JournalID, Date: date,
			Description: "Supplier tax invoice " + no + " / " + st.Posting.DocumentNo, Reference: st.Posting.DocumentNo, Lines: j.Lines(),
		})
		if err != nil {
			return err
		}
		rec.EntryID, rec.EntryNo = &entryID, entryNo
		if err := uc.taxInvoices.Create(ctx, rec); err != nil {
			return err
		}
		return uc.invoices.SaveVat(ctx, inv)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, d.InvoiceID)
}

// CancelTaxInvoice: ยกเลิกใบกำกับที่บันทึกไว้ — กลับรายการ ณ วันที่เดิม (งวดต้องยังเปิด), ภาษีกลับเป็นยังไม่ถึงกำหนด
func (uc *InvoiceUseCases) CancelTaxInvoice(ctx context.Context, in dto.Create[dto.TaxInvoiceRef]) (dto.Invoice, error) {
	err := uc.write(ctx, in.Actor, vo.PermServiceInvoicePost, func(ctx context.Context) error {
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, in.Data.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Kind() != vo.KindService {
			return domain.ErrInvoiceNotFound
		}
		rec, err := uc.taxInvoices.FindForUpdate(ctx, in.CompanyID, in.Data.InvoiceID, in.Data.ID)
		if err != nil {
			return err
		}
		if rec.Status != vo.DocPosted {
			return domain.ErrAlreadyCancelled
		}
		if _, err := uc.openPeriod(ctx, in.CompanyID, rec.Date); err != nil {
			return err
		}
		if rec.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *rec.EntryID, rec.Date, "Cancel supplier tax invoice "+rec.TaxInvoiceNo); err != nil {
				return err
			}
		}
		inv.UnrecordTaxInvoice(rec.Base, rec.Vat)
		if err := uc.taxInvoices.Cancel(ctx, in.CompanyID, rec.ID, in.UserID, uc.now()); err != nil {
			return err
		}
		return uc.invoices.SaveVat(ctx, inv)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, in.Data.InvoiceID)
}

// paidByPayments: ยอดจ่ายด้วยใบจ่ายเงิน (ไม่รวมใบลดหนี้ที่ตัดยอดผ่านการตัดยอดเดียวกัน)
func (uc *InvoiceUseCases) paidByPayments(ctx context.Context, companyID vo.CompanyID, inv *entities.Invoice) vo.Money {
	out, err := uc.invoices.Get(ctx, companyID, inv.ID())
	if err != nil {
		return inv.State().Paid
	}
	return out.Paid
}

func (uc *InvoiceUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error) {
	out, err := uc.invoices.Get(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	// ภาษีซื้อตั้งต้นของใบกำกับถัดไป (ซื้อบริการที่ยังรอใบกำกับ)
	out.DefaultVat = zero
	if out.Kind == string(vo.KindService) && out.Status == string(vo.DocPosted) && out.VatUndue.IsPositive() {
		if cs, err := uc.org.CompanySettings(ctx, companyID); err == nil {
			out.DefaultVat = entities.SuggestRecognition(services.Amounts{Net: out.Totals.Net, Vat: out.Totals.Vat, Gross: out.Totals.Gross},
				out.VatRecognized, out.VatUndue, out.Paid, cs.AmountDecimals)
		}
	}
	refs := make([]*dto.Ref, len(out.Lines))
	for i := range out.Lines {
		refs[i] = &out.Lines[i].Item
	}
	return out, uc.itemRefs(ctx, companyID, refs)
}

// billFrom: ผู้ขาย ณ วันบันทึกใบ — ชื่อ / เลขผู้เสียภาษีของผู้ขาย + เลขสาขา / ที่อยู่ของสาขาตามใบสั่ง
func (uc *InvoiceUseCases) billFrom(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID, branchNo string) (entities.BillFrom, error) {
	s, err := uc.suppliers.FindByID(ctx, companyID, supplierID)
	if err != nil {
		return entities.BillFrom{}, err
	}
	b, err := s.Branch(branchNo)
	if err != nil {
		return entities.BillFrom{}, err
	}
	d := s.Data()
	return entities.BillFrom{Name: d.Name, TaxID: d.TaxID, BranchNo: b.BranchNo, Address: b.Address}, nil
}
