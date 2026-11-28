// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ purchasing
//   - mapper.go       แปลง domain <-> gorm model (ที่เดียวที่รู้จักทั้งสองฝั่ง)
//   - transaction.go  implement ports.TransactionManager
//   - models/         รูปร่างตาราง
//   - repositories/   implement application/ports/repositories
package gormpersistence

import (
	"apexis/backend/internal/platform/thaiaddress"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/domain/entities"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
)

func OptUUID[T ~[16]byte](id *T) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

func OptID[T ~[16]byte](u *uuid.UUID) *T {
	if u == nil {
		return nil
	}
	id := T(*u)
	return &id
}

// DateUTC: คอลัมน์ date อ่านกลับมาอาจมี location ของ driver — แปลงเป็นเที่ยงคืน UTC ตามข้อตกลงของ domain
func DateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func optDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := DateUTC(*t)
	return &d
}

// ---------- ค่าตั้ง / ผู้ขาย ----------

func SettingsToModel(companyID vo.CompanyID, s entities.Settings) models.SettingsModel {
	return models.SettingsModel{
		CompanyID: uuid.UUID(companyID), VatRate: s.VatRate, JournalID: OptUUID(s.JournalID),
		GRIRAccountID: OptUUID(s.GRIRAccountID), APAccountID: OptUUID(s.APAccountID),
		InputVatAccountID: OptUUID(s.InputVatAccountID), PriceVarianceAccountID: OptUUID(s.PriceVarianceAccountID),
		PurchaseReturnsAccountID: OptUUID(s.PurchaseReturnsAccountID), UndueInputVatAccountID: OptUUID(s.UndueInputVatAccountID),
		ServiceExpenseAccountID: OptUUID(s.ServiceExpenseAccountID),
	}
}

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{
		VatRate: m.VatRate, JournalID: OptID[vo.JournalID](m.JournalID),
		GRIRAccountID: OptID[vo.AccountID](m.GRIRAccountID), APAccountID: OptID[vo.AccountID](m.APAccountID),
		InputVatAccountID: OptID[vo.AccountID](m.InputVatAccountID), PriceVarianceAccountID: OptID[vo.AccountID](m.PriceVarianceAccountID),
		PurchaseReturnsAccountID: OptID[vo.AccountID](m.PurchaseReturnsAccountID),
		UndueInputVatAccountID:   OptID[vo.AccountID](m.UndueInputVatAccountID),
		ServiceExpenseAccountID:  OptID[vo.AccountID](m.ServiceExpenseAccountID),
	}
}

func SupplierToModel(s *entities.Supplier) models.SupplierModel {
	d := s.Data()
	return models.SupplierModel{
		ID: uuid.UUID(s.ID()), CompanyID: uuid.UUID(s.CompanyID()), Code: d.Code, Name: d.Name, TaxID: d.TaxID,
		BranchNo: d.BranchNo, Address: d.Address, AddressDetail: addressToModel(d.AddressDetail), PaymentTermDays: d.PaymentTermDays, VatRegistered: d.VatRegistered, TaxType: string(d.TaxType),
		Status: string(d.Status),
	}
}

// SupplierBranchesToModel: สาขาเพิ่มเติมตามลำดับที่บันทึก
func SupplierBranchesToModel(sp *entities.Supplier) []models.SupplierBranchModel {
	bs := sp.Data().Branches
	out := make([]models.SupplierBranchModel, len(bs))
	for i, b := range bs {
		out[i] = models.SupplierBranchModel{SupplierID: uuid.UUID(sp.ID()), BranchNo: b.BranchNo, Name: b.Name, Address: b.Address,
			AddressDetail: addressToModel(b.AddressDetail), Position: i + 1}
	}
	return out
}

func SupplierBranchesToDomain(ms []models.SupplierBranchModel) []entities.SupplierBranch {
	out := make([]entities.SupplierBranch, len(ms))
	for i, m := range ms {
		out[i] = entities.SupplierBranch{BranchNo: m.BranchNo, Name: m.Name, Address: m.Address, AddressDetail: addressToDomain(m.AddressDetail)}
	}
	return out
}

// SupplierToEntity: branches = สาขาเพิ่มเติมที่โหลดแยก (nil = ไม่มี)
func SupplierToEntity(m models.SupplierModel, branches ...models.SupplierBranchModel) *entities.Supplier {
	return entities.RehydrateSupplier(vo.SupplierID(m.ID), vo.CompanyID(m.CompanyID), entities.SupplierData{
		Code: m.Code, Name: m.Name, TaxID: m.TaxID, BranchNo: m.BranchNo, Address: m.Address, AddressDetail: addressToDomain(m.AddressDetail),
		PaymentTermDays: m.PaymentTermDays, VatRegistered: m.VatRegistered, TaxType: vo.TaxType(m.TaxType), Status: vo.Status(m.Status),
		Branches: SupplierBranchesToDomain(branches),
	})
}

// ---------- ใบสั่งซื้อ ----------

func OrderToModel(o *entities.Order) models.OrderModel {
	s := o.State()
	m := models.OrderModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), SupplierID: uuid.UUID(s.Data.SupplierID), SupplierBranchNo: s.Data.SupplierBranchNo,
		OrderDate: s.Data.Date, DeliveryDate: s.Data.DeliveryDate, Reference: s.Data.Reference, Description: s.Data.Description,
		VatRegistered: s.Terms.Vat.Registered, VatRate: s.Terms.Vat.Rate, PriceIncludesVat: s.Terms.Vat.Inclusive,
		PaymentTermDays: s.Terms.PaymentTermDays, TotalNet: s.Totals.Net, TotalVat: s.Totals.Vat, TotalGross: s.Totals.Gross,
		Status: string(s.Status), CreatedBy: uuid.UUID(s.CreatedBy), Kind: string(vo.KindOrDefault(string(s.Data.Kind))),
	}
	if a := s.Approval; a != nil {
		no, year, by, at := a.OrderNo, a.FiscalYear, uuid.UUID(a.By), a.At
		m.OrderNo, m.FiscalYear, m.ApprovedBy, m.ApprovedAt = &no, &year, &by, &at
	}
	if c := s.Closure; c != nil {
		at := c.At
		m.ClosedBy, m.ClosedAt = OptUUID(c.By), &at
	}
	if c := s.Cancellation; c != nil {
		by, at := uuid.UUID(c.By), c.At
		m.CancelledBy, m.CancelledAt = &by, &at
	}
	m.Lines = make([]models.OrderLineModel, len(s.Data.Lines))
	for i, l := range s.Data.Lines {
		p, a := s.Progress[i], s.Amounts[i]
		m.Lines[i] = models.OrderLineModel{
			OrderID: m.ID, LineNo: i + 1, ItemID: uuid.UUID(l.ItemID), Description: l.Description, Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, Net: a.Net, Vat: a.Vat, Gross: a.Gross, ReceivedQty: p.Received, InvoicedQty: p.Invoiced,
			ReceivedValue: p.ReceivedValue, ClearedValue: p.ClearedValue,
		}
	}
	return m
}

func OrderToEntity(m models.OrderModel) *entities.Order {
	s := entities.OrderState{
		ID: vo.OrderID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Status: vo.OrderStatus(m.Status), CreatedBy: vo.UserID(m.CreatedBy),
		Data: entities.OrderData{
			Kind: vo.KindOrDefault(m.Kind), SupplierID: vo.SupplierID(m.SupplierID), SupplierBranchNo: m.SupplierBranchNo, Date: DateUTC(m.OrderDate), DeliveryDate: optDate(m.DeliveryDate),
			Reference: m.Reference, Description: m.Description, PriceIncludesVat: m.PriceIncludesVat,
			Lines: make([]entities.OrderLineData, len(m.Lines)),
		},
		Terms: entities.Terms{
			Vat:             services.VatTerms{Registered: m.VatRegistered, Rate: m.VatRate, Inclusive: m.PriceIncludesVat},
			PaymentTermDays: m.PaymentTermDays,
		},
		Totals:   services.Amounts{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross},
		Amounts:  make([]services.Amounts, len(m.Lines)),
		Progress: make([]entities.Progress, len(m.Lines)),
	}
	if m.OrderNo != nil && m.FiscalYear != nil && m.ApprovedBy != nil && m.ApprovedAt != nil {
		s.Approval = &entities.Approval{OrderNo: *m.OrderNo, FiscalYear: *m.FiscalYear, By: vo.UserID(*m.ApprovedBy), At: *m.ApprovedAt}
	}
	if m.ClosedAt != nil {
		s.Closure = &entities.Closure{By: OptID[vo.UserID](m.ClosedBy), At: *m.ClosedAt}
	}
	if m.CancelledBy != nil && m.CancelledAt != nil {
		s.Cancellation = &entities.Cancellation{By: vo.UserID(*m.CancelledBy), At: *m.CancelledAt}
	}
	for i, l := range m.Lines {
		s.Data.Lines[i] = entities.OrderLineData{ItemID: vo.ItemID(l.ItemID), Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
		s.Amounts[i] = services.Amounts{Net: l.Net, Vat: l.Vat, Gross: l.Gross}
		s.Progress[i] = entities.Progress{Received: l.ReceivedQty, Invoiced: l.InvoicedQty, ReceivedValue: l.ReceivedValue, ClearedValue: l.ClearedValue}
	}
	return entities.RehydrateOrder(s)
}

// ---------- ใบรับสินค้า / ใบแจ้งหนี้ ----------

func cancellation(c *entities.Cancellation) (*uuid.UUID, *time.Time) {
	if c == nil {
		return nil, nil
	}
	by, at := uuid.UUID(c.By), c.At
	return &by, &at
}

func toCancellation(by *uuid.UUID, at *time.Time) *entities.Cancellation {
	if by == nil || at == nil {
		return nil
	}
	return &entities.Cancellation{By: vo.UserID(*by), At: *at}
}

func ReceiptToModel(r *entities.Receipt) models.ReceiptModel {
	s := r.State()
	m := models.ReceiptModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, FiscalYear: s.Posting.FiscalYear,
		FiscalPeriodID: uuid.UUID(s.Posting.PeriodID), OrderID: uuid.UUID(s.OrderID), SupplierID: uuid.UUID(s.SupplierID),
		ReceiptDate: s.Date, WarehouseID: OptUUID(s.WarehouseID), Reference: s.Reference, Description: s.Description,
		System: string(s.System), StockDocumentID: OptUUID(s.StockDocumentID), StockDocumentNo: s.StockDocumentNo,
		TotalValue: decimal.Zero, EntryID: OptUUID(s.Posting.EntryID), EntryNo: s.Posting.EntryNo, Status: string(s.Status),
		CreatedBy: uuid.UUID(s.Posting.By), PostedAt: s.Posting.At, Kind: string(vo.KindOrDefault(string(s.Kind))),
	}
	m.CancelledBy, m.CancelledAt = cancellation(s.Cancellation)
	m.Lines = make([]models.ReceiptLineModel, len(s.Lines))
	for i, l := range s.Lines {
		m.TotalValue = m.TotalValue.Add(l.Value)
		m.Lines[i] = models.ReceiptLineModel{
			ReceiptID: m.ID, LineNo: i + 1, OrderLineNo: l.OrderLineNo, ItemID: uuid.UUID(l.ItemID), Stock: l.Stock,
			Quantity: l.Quantity, UnitCost: l.UnitCost, Value: l.Value, AccountID: uuid.UUID(l.AccountID),
		}
	}
	return m
}

func ReceiptToEntity(m models.ReceiptModel) *entities.Receipt {
	s := entities.ReceiptState{
		ID: vo.ReceiptID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Kind: vo.KindOrDefault(m.Kind), OrderID: vo.OrderID(m.OrderID), SupplierID: vo.SupplierID(m.SupplierID),
		Date: DateUTC(m.ReceiptDate), WarehouseID: OptID[vo.WarehouseID](m.WarehouseID), Reference: m.Reference,
		Description: m.Description, System: vo.AccountingSystem(m.System), StockDocumentID: OptID[vo.StockDocumentID](m.StockDocumentID),
		StockDocumentNo: m.StockDocumentNo, Status: vo.DocStatus(m.Status), Lines: make([]entities.ReceiptLine, len(m.Lines)),
		Posting: entities.Posting{
			DocumentNo: m.DocumentNo, FiscalYear: m.FiscalYear, PeriodID: vo.FiscalPeriodID(m.FiscalPeriodID),
			EntryID: OptID[vo.EntryID](m.EntryID), EntryNo: m.EntryNo, By: vo.UserID(m.CreatedBy), At: m.PostedAt,
		},
		Cancellation: toCancellation(m.CancelledBy, m.CancelledAt),
	}
	for i, l := range m.Lines {
		s.Lines[i] = entities.ReceiptLine{
			OrderLineNo: l.OrderLineNo, ItemID: vo.ItemID(l.ItemID), Stock: l.Stock, Quantity: l.Quantity,
			UnitCost: l.UnitCost, Value: l.Value, AccountID: vo.AccountID(l.AccountID),
		}
	}
	return entities.RehydrateReceipt(s)
}

func InvoiceToModel(inv *entities.Invoice) models.InvoiceModel {
	s := inv.State()
	m := models.InvoiceModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, FiscalYear: s.Posting.FiscalYear,
		FiscalPeriodID: uuid.UUID(s.Posting.PeriodID), OrderID: uuid.UUID(s.OrderID), SupplierID: uuid.UUID(s.SupplierID),
		SupplierInvoiceNo: s.SupplierInvoiceNo, InvoiceDate: s.InvoiceDate, PostingDate: s.Date, DueDate: s.DueDate,
		BillFromName: s.BillFrom.Name, BillFromTaxID: s.BillFrom.TaxID, BillFromBranchNo: s.BillFrom.BranchNo, BillFromAddress: s.BillFrom.Address,
		Description: s.Description, VatRate: s.Vat.Rate, PriceIncludesVat: s.Vat.Inclusive,
		TotalNet: s.Totals.Net, TotalVat: s.Totals.Vat, TotalGross: s.Totals.Gross, PaidAmount: s.Paid,
		TaxInvoiceReceived: s.TaxInvoiceReceived, VatRecognized: s.Deferral.VatRecognized, BaseRecognized: s.Deferral.BaseRecognized,
		VatCredited: s.Deferral.VatCredited, BaseCredited: s.Deferral.BaseCredited,
		Kind: string(vo.KindOrDefault(string(s.Kind))), TaxInvoiceNo: s.TaxInvoiceNo, TaxInvoiceDate: s.TaxInvoiceDate,
		EntryID: OptUUID(s.Posting.EntryID), EntryNo: s.Posting.EntryNo, Status: string(s.Status),
		CreatedBy: uuid.UUID(s.Posting.By), PostedAt: s.Posting.At,
	}
	m.CancelledBy, m.CancelledAt = cancellation(s.Cancellation)
	m.Lines = make([]models.InvoiceLineModel, len(s.Lines))
	for i, l := range s.Lines {
		m.Lines[i] = models.InvoiceLineModel{
			InvoiceID: m.ID, LineNo: i + 1, OrderLineNo: l.OrderLineNo, ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, Net: l.Amounts.Net, Vat: l.Amounts.Vat, Gross: l.Amounts.Gross, Cleared: l.Cleared,
			VarianceAccountID: OptUUID(l.VarianceAccountID),
		}
	}
	return m
}

func InvoiceToEntity(m models.InvoiceModel) *entities.Invoice {
	s := entities.InvoiceState{
		ID: vo.InvoiceID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), OrderID: vo.OrderID(m.OrderID), SupplierID: vo.SupplierID(m.SupplierID),
		SupplierInvoiceNo: m.SupplierInvoiceNo, InvoiceDate: DateUTC(m.InvoiceDate), Date: DateUTC(m.PostingDate),
		DueDate: DateUTC(m.DueDate), Description: m.Description,
		BillFrom: entities.BillFrom{Name: m.BillFromName, TaxID: m.BillFromTaxID, BranchNo: m.BillFromBranchNo, Address: m.BillFromAddress},
		Vat:      services.VatTerms{Registered: !m.VatRate.IsZero(), Rate: m.VatRate, Inclusive: m.PriceIncludesVat},
		Totals:   services.Amounts{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross}, Paid: m.PaidAmount,
		TaxInvoiceReceived: m.TaxInvoiceReceived, Kind: vo.KindOrDefault(m.Kind), TaxInvoiceNo: m.TaxInvoiceNo, TaxInvoiceDate: optDate(m.TaxInvoiceDate),
		Deferral: entities.VatDeferral{VatRecognized: m.VatRecognized, BaseRecognized: m.BaseRecognized, VatCredited: m.VatCredited, BaseCredited: m.BaseCredited},
		Status:   vo.DocStatus(m.Status), Lines: make([]entities.InvoiceLine, len(m.Lines)),
		Posting: entities.Posting{
			DocumentNo: m.DocumentNo, FiscalYear: m.FiscalYear, PeriodID: vo.FiscalPeriodID(m.FiscalPeriodID),
			EntryID: OptID[vo.EntryID](m.EntryID), EntryNo: m.EntryNo, By: vo.UserID(m.CreatedBy), At: m.PostedAt,
		},
		Cancellation: toCancellation(m.CancelledBy, m.CancelledAt),
	}
	for i, l := range m.Lines {
		s.Lines[i] = entities.InvoiceLine{
			OrderLineNo: l.OrderLineNo, ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: services.Amounts{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, Cleared: l.Cleared,
			VarianceAccountID: OptID[vo.AccountID](l.VarianceAccountID),
		}
	}
	return entities.RehydrateInvoice(s)
}

// ---------- ใบลดหนี้ผู้ขาย ----------

func CreditNoteToModel(c *entities.CreditNote) models.CreditNoteModel {
	s := c.State()
	m := models.CreditNoteModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, FiscalYear: s.Posting.FiscalYear,
		FiscalPeriodID: uuid.UUID(s.Posting.PeriodID), InvoiceID: uuid.UUID(s.InvoiceID), OrderID: uuid.UUID(s.OrderID),
		SupplierID: uuid.UUID(s.SupplierID), SupplierCreditNoteNo: s.SupplierCreditNoteNo,
		BillFromName: s.BillFrom.Name, BillFromTaxID: s.BillFrom.TaxID, BillFromBranchNo: s.BillFrom.BranchNo, BillFromAddress: s.BillFrom.Address,
		CreditNoteDate: s.CreditNoteDate, PostingDate: s.Date, Kind: string(s.Kind), VatKind: vatKind(s.VatKind), Reason: s.Reason, Description: s.Description,
		VatRate: s.Vat.Rate, PriceIncludesVat: s.Vat.Inclusive, TotalNet: s.Totals.Net, TotalVat: s.Totals.Vat, TotalGross: s.Totals.Gross,
		WarehouseID: OptUUID(s.WarehouseID), System: string(s.System), StockDocumentID: OptUUID(s.StockDocumentID), StockDocumentNo: s.StockDocumentNo,
		EntryID: OptUUID(s.Posting.EntryID), EntryNo: s.Posting.EntryNo, Status: string(s.Status),
		CreatedBy: uuid.UUID(s.Posting.By), PostedAt: s.Posting.At,
	}
	m.CancelledBy, m.CancelledAt = cancellation(s.Cancellation)
	m.Lines = make([]models.CreditNoteLineModel, len(s.Lines))
	for i, l := range s.Lines {
		m.Lines[i] = models.CreditNoteLineModel{
			CreditNoteID: m.ID, LineNo: i + 1, InvoiceLineNo: l.InvoiceLineNo, ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, Net: l.Amounts.Net, Vat: l.Amounts.Vat, Gross: l.Amounts.Gross, AccountID: uuid.UUID(l.AccountID),
			Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptUUID(l.InventoryAccountID), Variance: l.Variance,
		}
	}
	return m
}

func CreditNoteToEntity(m models.CreditNoteModel) *entities.CreditNote {
	s := entities.CreditNoteState{
		ID: vo.CreditNoteID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), InvoiceID: vo.InvoiceID(m.InvoiceID), OrderID: vo.OrderID(m.OrderID),
		SupplierID: vo.SupplierID(m.SupplierID), SupplierCreditNoteNo: m.SupplierCreditNoteNo,
		BillFrom:       entities.BillFrom{Name: m.BillFromName, TaxID: m.BillFromTaxID, BranchNo: m.BillFromBranchNo, Address: m.BillFromAddress},
		CreditNoteDate: DateUTC(m.CreditNoteDate), Date: DateUTC(m.PostingDate), Kind: vo.CreditNoteKind(m.Kind), VatKind: m.VatKind, Reason: m.Reason,
		Description: m.Description, Vat: services.VatTerms{Registered: !m.VatRate.IsZero(), Rate: m.VatRate, Inclusive: m.PriceIncludesVat},
		WarehouseID: OptID[vo.WarehouseID](m.WarehouseID), System: vo.AccountingSystem(m.System),
		StockDocumentID: OptID[vo.StockDocumentID](m.StockDocumentID), StockDocumentNo: m.StockDocumentNo,
		Totals: services.Amounts{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross}, Status: vo.DocStatus(m.Status),
		Lines: make([]entities.CreditNoteLine, len(m.Lines)),
		Posting: entities.Posting{
			DocumentNo: m.DocumentNo, FiscalYear: m.FiscalYear, PeriodID: vo.FiscalPeriodID(m.FiscalPeriodID),
			EntryID: OptID[vo.EntryID](m.EntryID), EntryNo: m.EntryNo, By: vo.UserID(m.CreatedBy), At: m.PostedAt,
		},
		Cancellation: toCancellation(m.CancelledBy, m.CancelledAt),
	}
	for i, l := range m.Lines {
		s.Lines[i] = entities.CreditNoteLine{
			InvoiceLineNo: l.InvoiceLineNo, ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: services.Amounts{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, AccountID: vo.AccountID(l.AccountID),
			Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptID[vo.AccountID](l.InventoryAccountID), Variance: l.Variance,
		}
	}
	return entities.RehydrateCreditNote(s)
}

// ---------- ภาษีหัก ณ ที่จ่าย / ใบจ่าย ----------

func addressToModel(a thaiaddress.Address) models.AddressColumns { return models.AddressColumns(a) }

func addressToDomain(m models.AddressColumns) thaiaddress.Address { return thaiaddress.Address(m) }

// vatKind: ใบลดหนี้เดิม / ไม่ระบุ = output (ลดภาษีซื้อ)
func vatKind(k string) string {
	if k == "" {
		return "output"
	}
	return k
}
