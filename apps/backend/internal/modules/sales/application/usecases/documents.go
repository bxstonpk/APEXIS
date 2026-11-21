package usecases

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

func shipments(lines []dto.ShipmentInput) []entities.Shipment {
	out := make([]entities.Shipment, len(lines))
	for i, l := range lines {
		out[i] = entities.Shipment{LineNo: l.OrderLineNo, Quantity: l.Quantity}
	}
	return out
}

// orderItems: สินค้าของบรรทัดที่ส่ง / ออกบิล + ระบบบัญชีของปี (inventory) — สินค้าต้องมีอยู่ (active เมื่อตัดสต็อก)
func (b base) orderItems(ctx context.Context, companyID vo.CompanyID, o *entities.Order, ships []entities.Shipment, date time.Time, requireActive bool) (vo.AccountingSystem, map[vo.ItemID]ports.Item, error) {
	ids := make([]vo.ItemID, 0, len(ships))
	for _, s := range ships {
		l, _, err := o.Line(s.LineNo)
		if err != nil {
			return "", nil, err
		}
		if !slices.Contains(ids, l.ItemID) {
			ids = append(ids, l.ItemID)
		}
	}
	system, found, err := b.inventory.Context(ctx, companyID, date, ids)
	if err != nil {
		return "", nil, err
	}
	return system, found, checkItems(found, ids, requireActive)
}

// ship ตัดสต็อกสินค้า stock ผ่าน inventory (ใบเบิก FIFO) แล้วคืนต้นทุนต่อบรรทัด
// perpetual: บรรทัด stock ต้องมีบัญชีสินค้าคงเหลือ และค่าตั้งต้องมีบัญชีต้นทุนขาย
func (b base) ship(ctx context.Context, a dto.Actor, settings entities.Settings, system vo.AccountingSystem, items map[vo.ItemID]ports.Item,
	o *entities.Order, ships []entities.Shipment, warehouseID *vo.WarehouseID, date time.Time, no string) (entities.Stock, []entities.CostLine, error) {
	stock := entities.Stock{System: system}
	costs := make([]entities.CostLine, len(ships))
	var lines []ports.StockLine
	var idx []int
	for i, s := range ships {
		l, _, _ := o.Line(s.LineNo)
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
		lines = append(lines, ports.StockLine{ItemID: l.ItemID, Quantity: s.Quantity})
		idx = append(idx, i)
	}
	if len(lines) == 0 {
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
	doc, err := b.inventory.PostIssue(ctx, a.CompanyID, a.UserID, *warehouseID, date, no, "Sales "+no+" / "+o.State().Approval.OrderNo, lines)
	if err != nil {
		return stock, nil, err
	}
	for k, i := range idx {
		costs[i].Cost = doc.LineValues[k]
	}
	stock.WarehouseID, stock.StockDocumentID, stock.StockDocumentNo = warehouseID, &doc.DocumentID, doc.DocumentNo
	return stock, costs, nil
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

// ---------- ใบส่งของ ----------

type DeliveryUseCases struct {
	base
	deliveries repositories.DeliveryRepository
	orders     repositories.OrderRepository
}

func (uc *DeliveryUseCases) Get(ctx context.Context, in dto.ByID[vo.DeliveryID]) (dto.Delivery, error) {
	return read(ctx, uc.base, in.Actor, vo.PermDeliveryRead, func() (dto.Delivery, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *DeliveryUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.DeliveryID) (dto.Delivery, error) {
	d, err := uc.deliveries.Get(ctx, companyID, id)
	if err != nil {
		return d, err
	}
	refs := make([]*dto.Ref, len(d.Lines))
	for i := range d.Lines {
		refs[i] = &d.Lines[i].Item
	}
	return d, uc.itemRefs(ctx, companyID, refs)
}

func (uc *DeliveryUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.PostedSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermDeliveryRead, func() (listing.Result[dto.PostedSummary], error) {
		return uc.deliveries.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create ส่งของ (แยกจากใบแจ้งหนี้): lock ใบสั่งขาย -> ตรวจจำนวน -> งวดเปิด -> ตัดสต็อก FIFO
// -> perpetual: Dr ต้นทุนขาย / Cr สินค้าคงเหลือ (periodic ไม่มีรายการบัญชี)
func (uc *DeliveryUseCases) Create(ctx context.Context, in dto.Create[dto.DeliveryInput]) (dto.Delivery, error) {
	var id vo.DeliveryID
	err := uc.writeWith(ctx, in.Actor, vo.PermDeliveryPost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.Data.OrderID)
		if err != nil {
			return err
		}
		ships := shipments(in.Data.Lines)
		if err := o.CheckDeliver(ships, s.QuantityDecimals); err != nil {
			return err
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		system, items, err := uc.orderItems(ctx, in.CompanyID, o, ships, in.Data.Date, true)
		if err != nil {
			return err
		}
		if system == vo.Perpetual {
			if err := settings.Require("journal_id"); err != nil {
				return err
			}
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocDelivery, in.Data.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		stock, costs, err := uc.ship(ctx, in.Actor, settings, system, items, o, ships, in.Data.WarehouseID, in.Data.Date, no)
		if err != nil {
			return err
		}
		if err := o.Deliver(ships, s.QuantityDecimals, uc.now()); err != nil {
			return err
		}
		st := o.State()
		lines := make([]entities.DeliveryLine, len(ships))
		for i, sh := range ships {
			l, _, _ := o.Line(sh.LineNo)
			lines[i] = entities.DeliveryLine{OrderLineNo: sh.LineNo, ItemID: l.ItemID, Quantity: sh.Quantity, CostLine: costs[i]}
		}
		d, err := entities.NewDelivery(entities.DeliveryState{
			CompanyID: in.CompanyID, OrderID: o.ID(), CustomerID: st.Data.CustomerID, Date: in.Data.Date,
			Reference: in.Data.Reference, Description: in.Data.Description, Stock: stock, Lines: lines,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		})
		if err != nil {
			return err
		}
		if journal := d.Journal(accountOr(settings.CogsAccountID)); len(journal) > 0 {
			entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, in.Data.Date, "Delivery "+no+" / "+st.Approval.OrderNo, no, journal)
			if err != nil {
				return err
			}
			d.SetEntry(*entryID, entryNo)
		}
		id = d.ID()
		if err := uc.deliveries.Create(ctx, d); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Delivery{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Cancel: ส่วนที่ออกบิลแล้วต้องยกเลิกบิลก่อน -> คืนสต็อก -> กลับรายการบัญชี ณ วันที่เดิม
func (uc *DeliveryUseCases) Cancel(ctx context.Context, in dto.ByID[vo.DeliveryID]) (dto.Delivery, error) {
	err := uc.write(ctx, in.Actor, vo.PermDeliveryPost, func(ctx context.Context) error {
		d, err := uc.deliveries.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := d.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := d.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, s.OrderID)
		if err != nil {
			return err
		}
		if err := o.Undeliver(d.Shipments()); err != nil {
			return err
		}
		if s.Stock.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.Stock.StockDocumentID); err != nil {
				return err
			}
		}
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "delivery"); err != nil {
			return err
		}
		if err := uc.deliveries.UpdateStatus(ctx, d); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Delivery{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

// ---------- ใบแจ้งหนี้ / ใบกำกับภาษี ----------

type InvoiceUseCases struct {
	base
	invoices repositories.InvoiceRepository
	orders   repositories.OrderRepository
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

func (uc *InvoiceUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.InvoiceSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (listing.Result[dto.InvoiceSummary], error) {
		return uc.invoices.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create ออกใบแจ้งหนี้ / ใบกำกับภาษี:
//   - แยกใบ: ไม่เกินจำนวนที่ส่งแล้วยังไม่ออกบิล
//   - รวมใบ (WithDelivery): ไม่เกินที่เหลือให้ส่ง, ตัดสต็อกพร้อมกัน
//
// ลงบัญชีรายการเดียว: Dr ลูกหนี้ / Cr รายได้ (สินค้า -> หมวด -> ค่าตั้ง) + ภาษีขาย [+ Dr ต้นทุนขาย / Cr สินค้าคงเหลือ]
func (uc *InvoiceUseCases) Create(ctx context.Context, in dto.Create[dto.InvoiceInput]) (dto.Invoice, error) {
	var id vo.InvoiceID
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ar_account_id"); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.Data.OrderID)
		if err != nil {
			return err
		}
		ships := shipments(in.Data.Lines)
		if in.Data.WithDelivery {
			err = o.CheckDeliver(ships, s.QuantityDecimals)
		} else {
			err = o.CheckInvoice(ships, s.QuantityDecimals)
		}
		if err != nil {
			return err
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		system, items, err := uc.orderItems(ctx, in.CompanyID, o, ships, in.Data.Date, in.Data.WithDelivery)
		if err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocInvoice, in.Data.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		var stock entities.Stock
		costs := make([]entities.CostLine, len(ships))
		if in.Data.WithDelivery {
			if stock, costs, err = uc.ship(ctx, in.Actor, settings, system, items, o, ships, in.Data.WarehouseID, in.Data.Date, no); err != nil {
				return err
			}
		}
		st := o.State()
		lines := make([]entities.InvoiceLine, len(ships))
		for i, sh := range ships {
			l, _, _ := o.Line(sh.LineNo)
			item := items[l.ItemID]
			revenue := item.RevenueAccountID
			if revenue == nil {
				revenue = settings.RevenueAccountID
			}
			if revenue == nil {
				return fmt.Errorf("%w: revenue account of item %s (or revenue_account_id) not set", domain.ErrSettingsMissing, item.Code)
			}
			lines[i] = entities.InvoiceLine{
				OrderLineNo: sh.LineNo, ItemID: l.ItemID, Quantity: sh.Quantity, UnitPrice: l.UnitPrice,
				Amounts: services.LineAmounts(sh.Quantity, l.UnitPrice, st.Terms.Vat, s.AmountDecimals), RevenueAccountID: *revenue,
				CostLine: costs[i],
			}
		}
		if err := o.Invoice(ships, s.QuantityDecimals, in.Data.WithDelivery, uc.now()); err != nil {
			return err
		}
		inv, err := entities.NewInvoice(entities.InvoiceState{
			CompanyID: in.CompanyID, OrderID: o.ID(), CustomerID: st.Data.CustomerID, Date: in.Data.Date,
			Reference: in.Data.Reference, Description: in.Data.Description, Vat: st.Terms.Vat,
			WithDelivery: in.Data.WithDelivery, Stock: stock, Lines: lines,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}, st.Terms.PaymentTermDays)
		if err != nil {
			return err
		}
		if !inv.State().Totals.Vat.IsZero() {
			if err := settings.Require("output_vat_account_id"); err != nil {
				return err
			}
		}
		journal := inv.Journal(*settings.ARAccountID, accountOr(settings.OutputVatAccountID), accountOr(settings.CogsAccountID))
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, in.Data.Date, "Sales invoice "+no+" / "+st.Approval.OrderNo, no, journal)
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
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Invoice{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Cancel: ใบที่รับเงินแล้วต้องยกเลิกใบเสร็จก่อน; รวมใบ = คืนสต็อกด้วย; กลับรายการ ณ วันที่เดิม
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
		if err := o.Uninvoice(inv.Shipments(), s.WithDelivery); err != nil {
			return err
		}
		if s.Stock.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.Stock.StockDocumentID); err != nil {
				return err
			}
		}
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "sales invoice"); err != nil {
			return err
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

// ---------- ใบเสร็จรับเงิน ----------

type ReceiptUseCases struct {
	base
	receipts  repositories.ReceiptRepository
	invoices  repositories.InvoiceRepository
	customers repositories.CustomerRepository
	cashbank  ports.CashBank
}

func (uc *ReceiptUseCases) Get(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReceiptRead, func() (dto.Receipt, error) { return uc.receipts.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *ReceiptUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.ReceiptSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermReceiptRead, func() (listing.Result[dto.ReceiptSummary], error) {
		return uc.receipts.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create รับชำระ: lock ใบแจ้งหนี้ (เรียง id) -> ยอดค้าง -> ทะเบียนเงิน (cashbank) -> งวดเปิด -> เลขที่
// -> ลงบัญชีรายการเดียว: Dr เงินสด / ธนาคาร + ภาษีถูกหัก ณ ที่จ่าย / Cr ลูกหนี้
func (uc *ReceiptUseCases) Create(ctx context.Context, in dto.Create[dto.ReceiptInput]) (dto.Receipt, error) {
	var id vo.ReceiptID
	err := uc.writeWith(ctx, in.Actor, vo.PermReceiptPost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ar_account_id"); err != nil {
			return err
		}
		customer, err := uc.customers.FindByID(ctx, in.CompanyID, in.Data.CustomerID)
		if err != nil {
			return err
		}
		ids := make([]vo.InvoiceID, len(in.Data.Lines))
		for i, l := range in.Data.Lines {
			ids[i] = l.InvoiceID
		}
		invoices, err := uc.invoices.LockForReceipt(ctx, in.CompanyID, ids)
		if err != nil {
			return err
		}
		byID := make(map[vo.InvoiceID]*entities.Invoice, len(invoices))
		for _, inv := range invoices {
			byID[inv.ID()] = inv
		}
		lines := make([]entities.ReceiptLine, len(in.Data.Lines))
		for i, l := range in.Data.Lines {
			inv, ok := byID[l.InvoiceID]
			if !ok {
				return fmt.Errorf("%w: line %d", domain.ErrInvoiceNotFound, i+1)
			}
			if err := inv.Receive(customer.ID(), l.Amount); err != nil {
				return err
			}
			lines[i] = entities.ReceiptLine{InvoiceID: l.InvoiceID, Amount: l.Amount, Wht: l.Wht}
		}
		cashIDs := make([]vo.CashAccountID, len(in.Data.Methods))
		for i, m := range in.Data.Methods {
			cashIDs[i] = m.CashAccountID
		}
		cash, err := uc.cashbank.CashAccounts(ctx, in.CompanyID, cashIDs)
		if err != nil {
			return err
		}
		methods := make([]entities.ReceiptMethod, len(in.Data.Methods))
		for i, m := range in.Data.Methods {
			c, ok := cash[m.CashAccountID]
			switch {
			case !ok:
				return fmt.Errorf("%w: method %d", domain.ErrCashAccountNotFound, i+1)
			case !c.Active:
				return fmt.Errorf("%w: %s", domain.ErrCashAccountInactive, c.Code)
			}
			methods[i] = entities.ReceiptMethod{CashAccountID: m.CashAccountID, GLAccountID: c.GLAccountID, Amount: m.Amount, Reference: m.Reference}
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocReceipt, in.Data.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		r, err := entities.NewReceipt(entities.ReceiptState{
			CompanyID: in.CompanyID, CustomerID: customer.ID(), Date: in.Data.Date, Description: in.Data.Description,
			Lines: lines, Methods: methods,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}, s.AmountDecimals)
		if err != nil {
			return err
		}
		if !r.State().Totals.Wht.IsZero() {
			if err := settings.Require("wht_account_id"); err != nil {
				return err
			}
		}
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, in.Data.Date, "Customer receipt "+no+" / "+customer.Data().Code, no,
			r.Journal(*settings.ARAccountID, accountOr(settings.WhtAccountID)))
		if err != nil {
			return err
		}
		if entryID != nil {
			r.SetEntry(*entryID, entryNo)
		}
		id = r.ID()
		if err := uc.receipts.Create(ctx, r); err != nil {
			return err
		}
		return uc.invoices.SaveReceived(ctx, invoices)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.receipts.Get(ctx, in.CompanyID, id)
}

// Cancel: กลับรายการ ณ วันที่เดิม และคืนยอดค้างให้ใบแจ้งหนี้
func (uc *ReceiptUseCases) Cancel(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	err := uc.write(ctx, in.Actor, vo.PermReceiptPost, func(ctx context.Context) error {
		r, err := uc.receipts.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := r.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := r.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		ids := make([]vo.InvoiceID, len(s.Lines))
		amounts := map[vo.InvoiceID]vo.Money{}
		for i, l := range s.Lines {
			ids[i], amounts[l.InvoiceID] = l.InvoiceID, l.Amount
		}
		invoices, err := uc.invoices.LockForReceipt(ctx, in.CompanyID, ids)
		if err != nil {
			return err
		}
		for _, inv := range invoices {
			inv.Unreceive(amounts[inv.ID()])
		}
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "customer receipt"); err != nil {
			return err
		}
		if err := uc.receipts.UpdateStatus(ctx, r); err != nil {
			return err
		}
		return uc.invoices.SaveReceived(ctx, invoices)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.receipts.Get(ctx, in.CompanyID, in.ID)
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

func (uc *ReportUseCases) Aging(ctx context.Context, in dto.List[dto.AgingFilter]) (dto.Aging, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.Aging, error) {
		rows, totals, err := uc.reports.Aging(ctx, in.CompanyID, in.Filter)
		return dto.Aging{AsOf: in.Filter.AsOf, Rows: rows, Totals: totals}, err
	})
}

func (uc *ReportUseCases) OutputVat(ctx context.Context, in dto.List[dto.TaxFilter]) (dto.TaxReport, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.TaxReport, error) {
		rows, t, err := uc.reports.OutputVat(ctx, in.CompanyID, in.Filter)
		return dto.TaxReport{Start: in.Filter.Start, End: in.Filter.End, Rows: rows, TotalBase: t.Base, TotalTax: t.Tax}, err
	})
}

func (uc *ReportUseCases) WithholdingReceived(ctx context.Context, in dto.List[dto.TaxFilter]) (dto.TaxReport, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.TaxReport, error) {
		rows, t, err := uc.reports.WithholdingReceived(ctx, in.CompanyID, in.Filter)
		return dto.TaxReport{Start: in.Filter.Start, End: in.Filter.End, Rows: rows, TotalBase: t.Base, TotalTax: t.Tax}, err
	})
}
