package usecases

import (
	"apexis/backend/internal/platform/dashperiod"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

func shipments(lines []dto.ShipmentInput) []entities.Shipment {
	out := make([]entities.Shipment, len(lines))
	for i, l := range lines {
		out[i] = entities.Shipment{LineNo: l.OrderLineNo, Quantity: l.Quantity}
	}
	return out
}

// post ลงบัญชี (ถ้ามีบรรทัด) แล้วคืน id / เลขที่ของรายการ
func (b base) post(ctx context.Context, a dto.Actor, journal vo.JournalID, date time.Time, description, reference string, lines []services.JournalLine) (*vo.EntryID, string, error) {
	if len(lines) == 0 {
		return nil, "", nil
	}
	id, no, err := b.ledger.Post(ctx, ports.LedgerEntry{
		CompanyID: a.CompanyID, UserID: a.UserID, JournalID: journal, Date: date, Description: description, Reference: reference, Lines: lines,
	})
	return &id, no, err
}

func (b base) unpost(ctx context.Context, a dto.Actor, p entities.Posting, date time.Time, what string) error {
	if p.EntryID == nil {
		return nil
	}
	return b.ledger.Reverse(ctx, a.CompanyID, a.UserID, *p.EntryID, date, "Cancel "+what+" "+p.DocumentNo)
}

func zeroAccount() vo.AccountID { return vo.AccountID{} }

func accountOr(p *vo.AccountID) vo.AccountID {
	if p == nil {
		return zeroAccount()
	}
	return *p
}

// ---------- เอกสารขาย (ขายสินค้า / ขายบริการ / มัดจำ) ----------
//
// จุดเกิดภาษี: ขายสินค้า = วันที่ใบ (ส่งมอบพร้อมใบกำกับ), ขายบริการ = รับชำระที่ finance (หรือใบกำกับภาษีก่อนรับเงิน),
// มัดจำ = วันที่ใบรับเงินมัดจำ / ใบกำกับภาษี — เงินเข้าทั้งหมดรับที่ finance (sales ออกเอกสารเท่านั้น)
//
// ประเภทธุรกิจกำหนดเอกสารที่ออกได้: ซื้อขาย / ผลิต = ขายสินค้า, บริการ / ก่อสร้าง = ขายบริการ (มัดจำออกได้ทุกประเภท)

type InvoiceUseCases struct {
	base
	invoices     repositories.InvoiceRepository
	orders       repositories.OrderRepository
	customers    repositories.CustomerRepository
	creditNotes  repositories.CreditNoteRepository
	settlements  repositories.SettlementRepository
	recognitions repositories.VatRecognitionRepository
}

// kindPermitted: ประเภทธุรกิจที่ออกเอกสารชนิดนี้ได้ (ไม่ทราบประเภท = ได้ทุกชนิด)
func kindPermitted(bt integration.BusinessType, kind vo.InvoiceKind) bool {
	switch kind {
	case vo.KindInvoice:
		return bt == "" || bt == integration.BusinessTrading || bt == integration.BusinessManufacturing
	case vo.KindServiceInvoice:
		return bt == "" || bt == integration.BusinessService || bt == integration.BusinessConstruction
	}
	return true
}

func (b base) checkKind(ctx context.Context, companyID vo.CompanyID, kind vo.InvoiceKind) error {
	bt, err := b.org.BusinessType(ctx, companyID)
	if err != nil {
		return err
	}
	if !kindPermitted(bt, kind) {
		return fmt.Errorf("%w: %s (%s)", domain.ErrDocumentNotAvailable, kind, bt)
	}
	return nil
}

func (uc *InvoiceUseCases) Get(ctx context.Context, in dto.ByID[vo.InvoiceID]) (dto.Invoice, error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (dto.Invoice, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *InvoiceUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.InvoiceID) (dto.Invoice, error) {
	i, err := uc.invoices.Get(ctx, companyID, id)
	if err != nil {
		return i, err
	}
	refs := make([]*dto.Ref, len(i.Lines))
	for n := range i.Lines {
		refs[n] = &i.Lines[n].Item
	}
	return i, uc.itemRefs(ctx, companyID, refs)
}

// GetKind: เอกสารชนิดที่ endpoint รองรับ (ชนิดอื่น = ไม่พบ)
func (uc *InvoiceUseCases) GetKind(ctx context.Context, in dto.ByID[vo.InvoiceID], kind vo.InvoiceKind) (dto.Invoice, error) {
	out, err := uc.Get(ctx, in)
	if err == nil && out.Kind != string(kind) {
		return dto.Invoice{}, fmt.Errorf("%w: %s", domain.ErrInvoiceNotFound, in.ID)
	}
	return out, err
}

func (uc *InvoiceUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.InvoiceSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (listing.Result[dto.InvoiceSummary], error) {
		return uc.invoices.List(ctx, in.CompanyID, in.Filter)
	})
}

// OpenDeposits: มัดจำที่รับเงินครบแล้วและยังหักได้ของลูกค้า (หน้าออกเอกสารขาย)
func (uc *InvoiceUseCases) OpenDeposits(ctx context.Context, a dto.Actor, customerID vo.CustomerID) ([]dto.OpenDeposit, error) {
	return read(ctx, uc.base, a, vo.PermInvoiceRead, func() ([]dto.OpenDeposit, error) {
		return uc.invoices.OpenDeposits(ctx, a.CompanyID, customerID)
	})
}

// saleHead คือผู้ซื้อ / เงื่อนไข / บรรทัดที่ resolve แล้วของเอกสารขาย
type saleHead struct {
	order      *entities.Order
	ships      []entities.Shipment
	customerID vo.CustomerID
	branchNo   string
	terms      entities.Terms
	lines      []entities.LineData
	orderLines []int // เลขบรรทัดใบสั่งขายของแต่ละบรรทัด (0 = ออกตรง)
	ref        string
}

// resolve: อ้างใบสั่งขาย (lock + ตรวจจำนวนที่เหลือ) หรือออกตรง (ลูกค้า active + เงื่อนไข VAT จากค่าตั้ง / ที่ส่งมา)
func (uc *InvoiceUseCases) resolve(ctx context.Context, companyID vo.CompanyID, s ports.CompanySettings, settings entities.Settings, d dto.SaleInput) (saleHead, error) {
	if d.OrderID != nil {
		o, err := uc.orders.FindForUpdate(ctx, companyID, *d.OrderID)
		if err != nil {
			return saleHead{}, err
		}
		ships := shipments(d.OrderLines)
		if err := o.CheckDeliver(ships, s.QuantityDecimals); err != nil {
			return saleHead{}, err
		}
		st := o.State()
		h := saleHead{order: o, ships: ships, customerID: st.Data.CustomerID, branchNo: st.Data.CustomerBranchNo, terms: st.Terms}
		if st.Approval != nil {
			h.ref = st.Approval.OrderNo
		}
		for _, sh := range ships {
			l, _, _ := o.Line(sh.LineNo)
			h.lines = append(h.lines, entities.LineData{ItemID: l.ItemID, Description: l.Description, Quantity: sh.Quantity, UnitPrice: l.UnitPrice})
			h.orderLines = append(h.orderLines, sh.LineNo)
		}
		return h, nil
	}
	if len(d.Lines) == 0 || len(d.Lines) > entities.MaxLines {
		return saleHead{}, fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, entities.MaxLines)
	}
	c, err := uc.customers.FindByID(ctx, companyID, d.CustomerID)
	if err != nil {
		return saleHead{}, err
	}
	if err := c.EnsureActive(); err != nil {
		return saleHead{}, err
	}
	if _, err := c.Branch(d.CustomerBranchNo); err != nil {
		return saleHead{}, err
	}
	vat, err := saleVat(settings, d.PriceIncludesVat, d.VatRate)
	if err != nil {
		return saleHead{}, err
	}
	h := saleHead{customerID: c.ID(), branchNo: d.CustomerBranchNo, terms: entities.Terms{Vat: vat, PaymentTermDays: c.Data().PaymentTermDays}}
	for n, l := range d.Lines {
		if !l.Quantity.IsPositive() || !numeric.FitsScale(l.Quantity, s.QuantityDecimals) {
			return saleHead{}, fmt.Errorf("%w: line %d quantity", domain.ErrInvalidLine, n+1)
		}
		if l.UnitPrice.IsNegative() || !numeric.FitsScale(l.UnitPrice, 6) {
			return saleHead{}, fmt.Errorf("%w: line %d unit price", domain.ErrInvalidLine, n+1)
		}
		desc, err := vo.OptionalText(l.Description, 500)
		if err != nil {
			return saleHead{}, err
		}
		h.lines = append(h.lines, entities.LineData{ItemID: l.ItemID, Description: desc, Quantity: l.Quantity, UnitPrice: l.UnitPrice})
		h.orderLines = append(h.orderLines, 0)
	}
	return h, nil
}

// saleVat: เงื่อนไข VAT ของเอกสารออกตรง — อัตราตามค่าตั้ง (ส่ง vat_rate = ใช้อัตรานั้น เฉพาะบริษัทจด VAT)
func saleVat(settings entities.Settings, inclusive bool, rate *vo.Money) (services.VatTerms, error) {
	vat, err := settings.Terms(inclusive)
	if err != nil || rate == nil {
		return vat, err
	}
	if !settings.VatRegistered {
		if rate.IsZero() {
			return vat, nil
		}
		return vat, domain.ErrVatNotApplicable
	}
	if rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(100)) || !numeric.FitsScale(*rate, 2) {
		return vat, domain.ErrInvalidVatRate
	}
	vat.Rate = *rate
	return vat, nil
}

// cutStock ตัดสต็อกบรรทัดสินค้า stock (ใบเบิก FIFO) แล้วคืนต้นทุนต่อบรรทัด
// perpetual: บรรทัด stock ต้องมีบัญชีสินค้าคงเหลือ และค่าตั้งต้องมีบัญชีต้นทุนขาย
func (b base) cutStock(ctx context.Context, a dto.Actor, settings entities.Settings, system vo.AccountingSystem, items map[vo.ItemID]ports.Item,
	lines []entities.LineData, warehouseID *vo.WarehouseID, date time.Time, no, ref string) (entities.Stock, []entities.CostLine, error) {
	stock := entities.Stock{System: system}
	costs := make([]entities.CostLine, len(lines))
	var stockLines []ports.StockLine
	var idx []int
	for i, l := range lines {
		item := items[l.ItemID]
		costs[i] = entities.CostLine{Stock: item.Stock, Cost: decimal.Zero}
		if !item.Stock {
			continue
		}
		if system == vo.Perpetual {
			if item.InventoryAccountID == nil {
				return stock, nil, fmt.Errorf("%w: inventory account of item %s", domain.ErrSettingsMissing, item.Code)
			}
			costs[i].InventoryAccountID = item.InventoryAccountID
		}
		stockLines = append(stockLines, ports.StockLine{ItemID: l.ItemID, Quantity: l.Quantity})
		idx = append(idx, i)
	}
	if len(stockLines) == 0 {
		return stock, costs, nil
	}
	if warehouseID == nil {
		return stock, nil, domain.ErrWarehouseRequired
	}
	if system == vo.Perpetual {
		if err := settings.Require("cogs_account_id"); err != nil {
			return stock, nil, err
		}
	}
	desc := "Sales " + no
	if ref != "" {
		desc += " / " + ref
	}
	doc, err := b.inventory.PostIssue(ctx, a.CompanyID, a.UserID, *warehouseID, date, no, desc, stockLines)
	if err != nil {
		return stock, nil, err
	}
	for k, i := range idx {
		costs[i].Cost = doc.LineValues[k]
	}
	stock.WarehouseID, stock.StockDocumentID, stock.StockDocumentNo = warehouseID, &doc.DocumentID, doc.DocumentNo
	return stock, costs, nil
}

// applyDeposits: lock มัดจำ (ต้องรับเงินครบแล้ว) แล้วหักตามที่ขอ — คืนรายการมัดจำที่หัก
func (uc *InvoiceUseCases) applyDeposits(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, in []dto.DepositInput, amountDecimals int) ([]entities.DepositUse, error) {
	out := make([]entities.DepositUse, 0, len(in))
	for _, d := range in {
		dep, err := uc.invoices.FindForUpdate(ctx, companyID, d.DepositID)
		if err != nil {
			return nil, err
		}
		vat, err := dep.ApplyDeposit(customerID, d.Amount, amountDecimals)
		if err != nil {
			return nil, err
		}
		if err := uc.invoices.SaveState(ctx, dep); err != nil {
			return nil, err
		}
		out = append(out, entities.DepositUse{DepositID: dep.ID(), DepositNo: dep.State().Posting.DocumentNo, Amount: d.Amount, Vat: vat})
	}
	return out, nil
}

// journalAccounts: บัญชีที่เอกสารต้องใช้ (ตรวจค่าตั้งที่จำเป็น)
func journalAccounts(settings entities.Settings, inv *entities.Invoice) (entities.JournalAccounts, error) {
	st := inv.State()
	depBase, depVat := inv.DepositTotals()
	need := []string{}
	vatDue := st.Totals.Vat.Sub(depVat)
	switch {
	case st.Kind == vo.KindServiceInvoice && vatDue.IsPositive():
		need = append(need, "undue_output_vat_account_id")
	case vatDue.IsPositive():
		need = append(need, "output_vat_account_id")
	}
	if st.Kind == vo.KindDeposit || depBase.IsPositive() {
		need = append(need, "deposit_account_id")
	}
	if err := settings.Require(need...); err != nil {
		return entities.JournalAccounts{}, err
	}
	return entities.JournalAccounts{
		AR: *settings.ARAccountID, OutputVat: accountOr(settings.OutputVatAccountID), UndueVat: accountOr(settings.UndueOutputVatAccountID),
		Deposit: accountOr(settings.DepositAccountID), Cogs: accountOr(settings.CogsAccountID),
	}, nil
}

// Create ออกขายสินค้า (IV) / ขายบริการ (SV):
// ประเภทธุรกิจ -> ใบสั่งขาย / ลูกค้า -> งวดเปิด -> สินค้า -> เลขที่ -> ตัดสต็อก (สินค้า stock) -> หักมัดจำ -> ลงบัญชี -> บันทึก
func (uc *InvoiceUseCases) Create(ctx context.Context, in dto.Create[dto.SaleInput]) (dto.Invoice, error) {
	var id vo.InvoiceID
	d := in.Data
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		if err := uc.checkKind(ctx, in.CompanyID, d.Kind); err != nil {
			return err
		}
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ar_account_id"); err != nil {
			return err
		}
		h, err := uc.resolve(ctx, in.CompanyID, s, settings, d)
		if err != nil {
			return err
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		ids := make([]vo.ItemID, 0, len(h.lines))
		for _, l := range h.lines {
			if !slices.Contains(ids, l.ItemID) {
				ids = append(ids, l.ItemID)
			}
		}
		system, items, err := uc.inventory.Context(ctx, in.CompanyID, d.Date, ids)
		if err != nil {
			return err
		}
		if err := checkItems(items, ids, true); err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, d.Kind.DocType(), d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		lines := make([]entities.InvoiceLine, len(h.lines))
		for i, l := range h.lines {
			item := items[l.ItemID]
			if d.Kind == vo.KindServiceInvoice && !item.Service {
				return fmt.Errorf("%w: line %d (%s)", domain.ErrServiceInvoiceGoodsLine, i+1, item.Code)
			}
			revenue := item.RevenueAccountID
			if revenue == nil {
				revenue = settings.RevenueAccountID
			}
			if revenue == nil {
				return fmt.Errorf("%w: revenue account of item %s (or revenue_account_id) not set", domain.ErrSettingsMissing, item.Code)
			}
			lines[i] = entities.InvoiceLine{
				OrderLineNo: h.orderLines[i], ItemID: l.ItemID, Description: l.Description, Service: item.Service, Quantity: l.Quantity,
				UnitPrice: l.UnitPrice, Amounts: services.LineAmounts(l.Quantity, l.UnitPrice, h.terms.Vat, s.AmountDecimals), RevenueAccountID: *revenue,
				CostLine: entities.CostLine{Cost: decimal.Zero},
			}
		}
		stock := entities.Stock{System: system}
		if d.Kind == vo.KindInvoice {
			var costs []entities.CostLine
			if stock, costs, err = uc.cutStock(ctx, in.Actor, settings, system, items, h.lines, d.WarehouseID, d.Date, no, h.ref); err != nil {
				return err
			}
			for i := range lines {
				lines[i].CostLine = costs[i]
			}
		}
		deposits, err := uc.applyDeposits(ctx, in.CompanyID, h.customerID, d.Deposits, s.AmountDecimals)
		if err != nil {
			return err
		}
		if h.order != nil {
			if err := h.order.Invoice(h.ships, s.QuantityDecimals, true, uc.now()); err != nil {
				return err
			}
		}
		billTo, err := uc.billTo(ctx, in.CompanyID, h.customerID, h.branchNo)
		if err != nil {
			return err
		}
		state := entities.InvoiceState{
			CompanyID: in.CompanyID, Kind: d.Kind, CustomerID: h.customerID, BillTo: billTo, Date: d.Date,
			Reference: d.Reference, Description: d.Description, Vat: h.terms.Vat, Stock: stock, Lines: lines, Deposits: deposits,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}
		if h.order != nil {
			oid := h.order.ID()
			state.OrderID = &oid
		}
		inv, err := entities.NewInvoice(state, h.terms.PaymentTermDays)
		if err != nil {
			return err
		}
		accounts, err := journalAccounts(settings, inv)
		if err != nil {
			return err
		}
		desc := map[vo.InvoiceKind]string{vo.KindInvoice: "Sales invoice ", vo.KindServiceInvoice: "Service invoice "}[d.Kind] + no
		if h.ref != "" {
			desc += " / " + h.ref
		}
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, d.Date, desc, no, inv.Journal(accounts))
		if err != nil {
			return err
		}
		if entryID != nil {
			inv.SetEntry(*entryID, entryNo)
		}
		id = inv.ID()
		if err := uc.invoices.Create(ctx, inv); err != nil {
			return err
		}
		// มัดจำที่หัก = ตัดยอดลูกหนี้ของใบนี้ (ประวัติการตัดยอด: document_id = ใบนี้ ใช้คำนวณอายุลูกหนี้)
		if b, v := inv.DepositTotals(); b.Add(v).IsPositive() {
			if err := uc.settlements.Create(ctx, in.CompanyID, dto.SettlementDoc{ID: uuid.UUID(inv.ID()), No: no, Date: d.Date},
				[]dto.SettlementLine{{InvoiceID: inv.ID(), Amount: b.Add(v)}}); err != nil {
				return err
			}
		}
		if h.order != nil {
			return uc.orders.SaveProgress(ctx, h.order)
		}
		return nil
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// CreateDeposit ออกใบรับเงินมัดจำ / ใบกำกับภาษี: Dr ลูกหนี้ / Cr เงินมัดจำรับ / Cr ภาษีขาย — รับเงินจริงที่ finance
func (uc *InvoiceUseCases) CreateDeposit(ctx context.Context, in dto.Create[dto.DepositDocInput]) (dto.Invoice, error) {
	var id vo.InvoiceID
	d := in.Data
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ar_account_id", "deposit_account_id"); err != nil {
			return err
		}
		customerID, branchNo, paymentTerm := d.CustomerID, d.CustomerBranchNo, 0
		var vat services.VatTerms
		ref := ""
		if d.OrderID != nil {
			o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, *d.OrderID)
			if err != nil {
				return err
			}
			st := o.State()
			if st.Status != vo.OrderApproved {
				return domain.ErrOrderNotApproved
			}
			customerID, branchNo, vat, paymentTerm = st.Data.CustomerID, st.Data.CustomerBranchNo, st.Terms.Vat, 0
			vat.Inclusive = d.PriceIncludesVat && vat.Registered
			if st.Approval != nil {
				ref = st.Approval.OrderNo
			}
		} else {
			c, err := uc.customers.FindByID(ctx, in.CompanyID, customerID)
			if err != nil {
				return err
			}
			if err := c.EnsureActive(); err != nil {
				return err
			}
			if vat, err = saleVat(settings, d.PriceIncludesVat, d.VatRate); err != nil {
				return err
			}
		}
		if !d.Amount.IsPositive() || !numeric.FitsScale(d.Amount, s.AmountDecimals) {
			return fmt.Errorf("%w: amount must be greater than 0 with at most %d decimal places", domain.ErrInvalidAmount, s.AmountDecimals)
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		amounts := services.LineAmounts(decimal.NewFromInt(1), d.Amount, vat, s.AmountDecimals)
		no, err := uc.number(ctx, in.CompanyID, vo.DocDeposit, d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		billTo, err := uc.billTo(ctx, in.CompanyID, customerID, branchNo)
		if err != nil {
			return err
		}
		reference := d.Reference
		if reference == "" {
			reference = ref
		}
		state := entities.InvoiceState{
			CompanyID: in.CompanyID, Kind: vo.KindDeposit, OrderID: d.OrderID, CustomerID: customerID, BillTo: billTo, Date: d.Date,
			Reference: reference, Description: d.Description, Vat: vat, Totals: amounts, Stock: entities.Stock{System: vo.Periodic},
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}
		dep, err := entities.NewInvoice(state, paymentTerm)
		if err != nil {
			return err
		}
		accounts, err := journalAccounts(settings, dep)
		if err != nil {
			return err
		}
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, d.Date, "Deposit "+no, no, dep.Journal(accounts))
		if err != nil {
			return err
		}
		if entryID != nil {
			dep.SetEntry(*entryID, entryNo)
		}
		id = dep.ID()
		return uc.invoices.Create(ctx, dep)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// IssueTaxInvoice: ใบกำกับภาษีของใบแจ้งหนี้บริการก่อนรับเงิน — Dr ภาษีขายยังไม่ถึงกำหนด / Cr ภาษีขาย (ที่ยังไม่ถึงกำหนดทั้งหมด)
func (uc *InvoiceUseCases) IssueTaxInvoice(ctx context.Context, in dto.Create[dto.TaxInvoiceInput]) (dto.Invoice, error) {
	d := in.Data
	err := uc.write(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "output_vat_account_id", "undue_output_vat_account_id"); err != nil {
			return err
		}
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocTaxInvoice, d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		base, vat, err := inv.IssueTaxInvoice(no, d.Date)
		if err != nil {
			return err
		}
		j := []services.JournalLine{
			{AccountID: *settings.UndueOutputVatAccountID, Debit: vat, Credit: decimal.Zero},
			{AccountID: *settings.OutputVatAccountID, Debit: decimal.Zero, Credit: vat},
		}
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, d.Date, "Tax invoice "+no+" / "+inv.State().Posting.DocumentNo, no, j)
		if err != nil {
			return err
		}
		if entryID != nil {
			inv.SetTaxInvoiceEntry(*entryID, entryNo)
		}
		if err := uc.invoices.SaveState(ctx, inv); err != nil {
			return err
		}
		return uc.recognitions.Create(ctx, in.CompanyID, []repositories.VatRecognition{{
			Source: "tax_invoice", DocumentID: uuid.UUID(inv.ID()), DocumentNo: no, Date: d.Date, InvoiceID: inv.ID(), Base: base, Vat: vat,
		}})
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, d.InvoiceID)
}

// billTo: ผู้ซื้อ ณ วันออกใบ — ชื่อ / เลขผู้เสียภาษีของลูกค้า + เลขสาขา / ที่อยู่ของสาขา
func (uc *InvoiceUseCases) billTo(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, branchNo string) (entities.BillTo, error) {
	c, err := uc.customers.FindByID(ctx, companyID, customerID)
	if err != nil {
		return entities.BillTo{}, err
	}
	b, err := c.Branch(branchNo)
	if err != nil {
		return entities.BillTo{}, err
	}
	d := c.Data()
	return entities.BillTo{Name: d.Name, TaxID: d.TaxID, BranchNo: b.BranchNo, Address: b.Address}, nil
}

// Cancel: ต้องยกเลิกใบรับเงิน / ใบลดหนี้ก่อน; คืนมัดจำที่หัก, คืนสต็อก, กลับรายการ (ใบกำกับภาษีก่อนรับเงิน + ใบหลัก) ณ วันที่เดิม
func (uc *InvoiceUseCases) Cancel(ctx context.Context, in dto.ByID[vo.InvoiceID], kind vo.InvoiceKind) (dto.Invoice, error) {
	err := uc.write(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context) error {
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		s := inv.State()
		if s.Kind != kind {
			return fmt.Errorf("%w: %s", domain.ErrInvoiceNotFound, in.ID)
		}
		if s.Status == vo.DocPosted {
			has, err := uc.creditNotes.HasPosted(ctx, in.CompanyID, in.ID)
			if err != nil {
				return err
			}
			if has {
				return domain.ErrInvoiceHasCredits
			}
		}
		if err := inv.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		if s.OrderID != nil {
			o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, *s.OrderID)
			if err != nil {
				return err
			}
			if ships := inv.Shipments(); len(ships) > 0 {
				if err := o.Uninvoice(ships, true); err != nil {
					return err
				}
				if err := uc.orders.SaveProgress(ctx, o); err != nil {
					return err
				}
			}
		}
		for _, d := range s.Deposits {
			dep, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, d.DepositID)
			if err != nil {
				return err
			}
			dep.UnapplyDeposit(d.Amount, d.Vat)
			if err := uc.invoices.SaveState(ctx, dep); err != nil {
				return err
			}
		}
		if len(s.Deposits) > 0 {
			if err := uc.settlements.Cancel(ctx, in.CompanyID, uuid.UUID(inv.ID())); err != nil {
				return err
			}
		}
		if s.Stock.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.Stock.StockDocumentID); err != nil {
				return err
			}
		}
		if t := s.TaxInvoice; t != nil {
			if t.EntryID != nil {
				if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *t.EntryID, t.Date, "Cancel tax invoice "+t.No); err != nil {
					return err
				}
			}
			if err := uc.recognitions.Cancel(ctx, in.CompanyID, uuid.UUID(inv.ID())); err != nil {
				return err
			}
		}
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, string(kind)); err != nil {
			return err
		}
		return uc.invoices.UpdateStatus(ctx, inv)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

// ---------- ใบวางบิล ----------

type BillingNoteUseCases struct {
	base
	notes     repositories.BillingNoteRepository
	invoices  repositories.InvoiceRepository
	customers repositories.CustomerRepository
}

func (uc *BillingNoteUseCases) Get(ctx context.Context, in dto.ByID[vo.BillingNoteID]) (dto.BillingNote, error) {
	return read(ctx, uc.base, in.Actor, vo.PermBillingRead, func() (dto.BillingNote, error) { return uc.notes.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *BillingNoteUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.BillingNoteSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermBillingRead, func() (listing.Result[dto.BillingNoteSummary], error) {
		return uc.notes.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create ใบวางบิล: ลูกค้า -> ใบแจ้งหนี้ที่วางบิลได้ -> เลขที่ (ปีบัญชีของวันที่) — ไม่มีรายการบัญชี
func (uc *BillingNoteUseCases) Create(ctx context.Context, in dto.Create[dto.BillingNoteInput]) (dto.BillingNote, error) {
	var id vo.BillingNoteID
	d := in.Data
	err := uc.write(ctx, in.Actor, vo.PermBillingManage, func(ctx context.Context) error {
		if _, err := uc.customers.FindByID(ctx, in.CompanyID, d.CustomerID); err != nil {
			return err
		}
		if !vo.IsDateOnly(d.Date) {
			return domain.ErrInvalidDate
		}
		invoices, err := uc.invoices.Billable(ctx, in.CompanyID, d.InvoiceIDs)
		if err != nil {
			return err
		}
		period, err := uc.org.PeriodAt(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		b, err := entities.NewBillingNote(entities.BillingNoteState{
			CompanyID: in.CompanyID, CustomerID: d.CustomerID, Date: d.Date, DueDate: d.DueDate, Description: d.Description,
			InvoiceIDs: d.InvoiceIDs, By: in.UserID, At: uc.now(),
		}, invoices)
		if err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocBillingNote, d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		st := b.State()
		st.DocumentNo = no
		b = entities.RehydrateBillingNote(st)
		id = b.ID()
		return uc.notes.Create(ctx, b)
	})
	if err != nil {
		return dto.BillingNote{}, err
	}
	return uc.notes.Get(ctx, in.CompanyID, id)
}

func (uc *BillingNoteUseCases) Cancel(ctx context.Context, in dto.ByID[vo.BillingNoteID]) (dto.BillingNote, error) {
	err := uc.write(ctx, in.Actor, vo.PermBillingManage, func(ctx context.Context) error {
		b, err := uc.notes.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := b.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		return uc.notes.UpdateStatus(ctx, b)
	})
	if err != nil {
		return dto.BillingNote{}, err
	}
	return uc.notes.Get(ctx, in.CompanyID, in.ID)
}

// ---------- รายงาน ----------

type ReportUseCases struct {
	base
	reports repositories.ReportRepository
}

func (uc *ReportUseCases) Outstanding(ctx context.Context, in dto.List[dto.OutstandingFilter]) (listing.Result[dto.OutstandingRow], error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (listing.Result[dto.OutstandingRow], error) {
		res, err := uc.reports.Outstanding(ctx, in.CompanyID, in.Filter)
		if err != nil {
			return res, err
		}
		refs := make([]*dto.Ref, len(res.Items))
		for i := range res.Items {
			refs[i] = &res.Items[i].Item
		}
		return res, uc.itemRefs(ctx, in.CompanyID, refs)
	})
}

// Dashboard: ภาพรวมแอปขาย — สิทธิ์เดียวกับรายงาน (sales.report:read), อ่านอย่างเดียว ไม่ใช้ transaction
func (uc *ReportUseCases) Dashboard(ctx context.Context, in dto.List[dashperiod.Period]) (dto.Dashboard, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Dashboard, error) {
		// กราฟรายเดือน = ปีบัญชีที่ครอบคลุม end_date (เดือนเริ่มงวด -> สิ้นงวด) เทียบปีก่อนหน้า; ไม่มีปีบัญชี = 12 เดือนล่าสุด
		p := in.Filter
		fy, err := uc.org.PeriodAt(ctx, in.CompanyID, p.End)
		switch {
		case err == nil:
			p = p.InFiscalYear(fy.FiscalYear, fy.YearStart)
		case !errors.Is(err, domain.ErrNoFiscalPeriod):
			return dto.Dashboard{}, err
		}
		return uc.reports.Dashboard(ctx, in.CompanyID, p)
	})
}

func (uc *ReportUseCases) Aging(ctx context.Context, in dto.List[dto.AgingFilter]) (dto.Aging, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Aging, error) {
		rows, totals, err := uc.reports.Aging(ctx, in.CompanyID, in.Filter)
		return dto.Aging{AsOf: in.Filter.AsOf, Rows: rows, Totals: totals}, err
	})
}

// VatTotals: ภาษีขายของช่วงวันที่ (ใบแจ้งหนี้ - ใบลดหนี้) — เรียกโดย finance (สรุป ภ.พ.30) ซึ่งตรวจสิทธิ์รายงานของตัวเองแล้ว
func (uc *ReportUseCases) VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error) {
	if !vo.IsDateOnly(start) || !vo.IsDateOnly(end) || start.After(end) {
		return dto.VatTotals{}, domain.ErrInvalidDate
	}
	return uc.reports.VatTotals(ctx, companyID, start, end)
}

// VatRowsLimit: จำนวนแถวสูงสุดของรายงานภาษีขาย 1 เดือนที่ finance พิมพ์ประกอบ ภ.พ.30 (ไม่แบ่งหน้า)
const VatRowsLimit = 20000

// VatRows: แถวภาษีขายที่เกิดแล้วของช่วงวันที่ (ชุดเดียวกับ book=output-vat เรียงวันที่ / เลขที่) — เรียกโดย finance
// ซึ่งตรวจสิทธิ์รายงานของตัวเองแล้ว; query เดียวกับรายงาน (แบ่งหน้าใน DB ด้วย limit เดียว)
func (uc *ReportUseCases) VatRows(ctx context.Context, companyID vo.CompanyID, start, end time.Time) ([]dto.TaxRow, error) {
	if !vo.IsDateOnly(start) || !vo.IsDateOnly(end) || start.After(end) {
		return nil, domain.ErrInvalidDate
	}
	res, _, err := uc.reports.OutputVat(ctx, companyID, dto.TaxFilter{Start: start, End: end, Sort: dto.DefaultTaxSort,
		Page: listing.Page{Number: 1, Size: VatRowsLimit}})
	return res.Items, err
}

// UndueVat: ใบแจ้งหนี้บริการที่ภาษีขายยังไม่ถึงกำหนด ณ วันที่ (กระทบยอดบัญชีภาษีขายยังไม่ถึงกำหนด)
func (uc *ReportUseCases) UndueVat(ctx context.Context, in dto.List[dto.UndueVatFilter]) (dto.UndueVatReport, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.UndueVatReport, error) {
		rows, t, err := uc.reports.UndueVat(ctx, in.CompanyID, in.Filter)
		return dto.UndueVatReport{EndDate: in.Filter.End, Rows: rows, Totals: t}, err
	})
}

func (uc *ReportUseCases) OutputVat(ctx context.Context, in dto.List[dto.TaxFilter]) (dto.TaxReport, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.TaxReport, error) {
		rows, t, err := uc.reports.OutputVat(ctx, in.CompanyID, in.Filter)
		return dto.TaxReport{Start: in.Filter.Start, End: in.Filter.End, Rows: rows, TotalBase: t.Base, TotalTax: t.Tax}, err
	})
}
