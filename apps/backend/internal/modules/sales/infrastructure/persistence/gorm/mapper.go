// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ sales
//   - mapper.go แปลง domain <-> gorm model, transaction.go, models/, repositories/
package gormpersistence

import (
	"apexis/backend/internal/platform/thaiaddress"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/domain/entities"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/modules/sales/infrastructure/persistence/gorm/models"
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

// ---------- ค่าตั้ง / ลูกค้า ----------

func SettingsToModel(companyID vo.CompanyID, s entities.Settings) models.SettingsModel {
	return models.SettingsModel{
		CompanyID: uuid.UUID(companyID), VatRegistered: s.VatRegistered, VatRate: s.VatRate, JournalID: OptUUID(s.JournalID),
		ARAccountID: OptUUID(s.ARAccountID), OutputVatAccountID: OptUUID(s.OutputVatAccountID), RevenueAccountID: OptUUID(s.RevenueAccountID),
		CogsAccountID: OptUUID(s.CogsAccountID),
	}
}

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{
		VatRegistered: m.VatRegistered, VatRate: m.VatRate, JournalID: OptID[vo.JournalID](m.JournalID),
		ARAccountID: OptID[vo.AccountID](m.ARAccountID), OutputVatAccountID: OptID[vo.AccountID](m.OutputVatAccountID),
		RevenueAccountID: OptID[vo.AccountID](m.RevenueAccountID), CogsAccountID: OptID[vo.AccountID](m.CogsAccountID),
	}
}

func CustomerToModel(c *entities.Customer) models.CustomerModel {
	d := c.Data()
	return models.CustomerModel{
		ID: uuid.UUID(c.ID()), CompanyID: uuid.UUID(c.CompanyID()), Code: d.Code, Name: d.Name, TaxID: d.TaxID, BranchNo: d.BranchNo,
		Address: d.Address, AddressDetail: addressToModel(d.AddressDetail), PaymentTermDays: d.PaymentTermDays, TaxType: string(d.TaxType), CreditLimit: d.CreditLimit, Status: string(d.Status),
	}
}

// CustomerBranchesToModel: สาขาเพิ่มเติมตามลำดับที่บันทึก
func CustomerBranchesToModel(c *entities.Customer) []models.CustomerBranchModel {
	bs := c.Data().Branches
	out := make([]models.CustomerBranchModel, len(bs))
	for i, b := range bs {
		out[i] = models.CustomerBranchModel{CustomerID: uuid.UUID(c.ID()), BranchNo: b.BranchNo, Name: b.Name, Address: b.Address,
			AddressDetail: addressToModel(b.AddressDetail), Position: i + 1}
	}
	return out
}

func CustomerBranchesToDomain(ms []models.CustomerBranchModel) []entities.CustomerBranch {
	out := make([]entities.CustomerBranch, len(ms))
	for i, m := range ms {
		out[i] = entities.CustomerBranch{BranchNo: m.BranchNo, Name: m.Name, Address: m.Address, AddressDetail: addressToDomain(m.AddressDetail)}
	}
	return out
}

// CustomerToEntity: branches = สาขาเพิ่มเติมที่โหลดแยก (nil = ไม่มี)
func CustomerToEntity(m models.CustomerModel, branches ...models.CustomerBranchModel) *entities.Customer {
	return entities.RehydrateCustomer(vo.CustomerID(m.ID), vo.CompanyID(m.CompanyID), entities.CustomerData{
		Code: m.Code, Name: m.Name, TaxID: m.TaxID, BranchNo: m.BranchNo, Address: m.Address, AddressDetail: addressToDomain(m.AddressDetail), PaymentTermDays: m.PaymentTermDays,
		TaxType: vo.TaxType(m.TaxType), CreditLimit: m.CreditLimit, Status: vo.Status(m.Status),
		Branches: CustomerBranchesToDomain(branches),
	})
}

// ---------- ใบเสนอราคา / ใบสั่งขาย ----------

func salesDoc(customerID vo.CustomerID, branchNo, reference, description string, terms entities.Terms, p entities.Priced, by vo.UserID) models.SalesDocModel {
	return models.SalesDocModel{
		CustomerID: uuid.UUID(customerID), CustomerBranchNo: branchNo, Reference: reference, Description: description,
		VatRegistered: terms.Vat.Registered, VatRate: terms.Vat.Rate, PriceIncludesVat: terms.Vat.Inclusive, PaymentTermDays: terms.PaymentTermDays,
		TotalNet: p.Totals.Net, TotalVat: p.Totals.Vat, TotalGross: p.Totals.Gross, CreatedBy: uuid.UUID(by),
	}
}

func lineModels(p entities.Priced) []models.LineModel {
	out := make([]models.LineModel, len(p.Lines))
	for i, l := range p.Lines {
		a := p.Amounts[i]
		out[i] = models.LineModel{
			LineNo: i + 1, ItemID: uuid.UUID(l.ItemID), Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Net: a.Net, Vat: a.Vat, Gross: a.Gross,
		}
	}
	return out
}

func termsOf(m models.SalesDocModel) entities.Terms {
	return entities.Terms{
		Vat:             services.VatTerms{Registered: m.VatRegistered, Rate: m.VatRate, Inclusive: m.PriceIncludesVat},
		PaymentTermDays: m.PaymentTermDays,
	}
}

func pricedOf(m models.SalesDocModel, lines []models.LineModel) entities.Priced {
	p := entities.Priced{
		Lines: make([]entities.LineData, len(lines)), Amounts: make([]services.Amounts, len(lines)),
		Totals: services.Amounts{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross},
	}
	for i, l := range lines {
		p.Lines[i] = entities.LineData{ItemID: vo.ItemID(l.ItemID), Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
		p.Amounts[i] = services.Amounts{Net: l.Net, Vat: l.Vat, Gross: l.Gross}
	}
	return p
}

func QuotationToModel(q *entities.Quotation) models.QuotationModel {
	s := q.State()
	m := models.QuotationModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocDate: s.Data.Date, ValidUntil: s.Data.ValidUntil,
		SalesDocModel: salesDoc(s.Data.CustomerID, s.Data.CustomerBranchNo, s.Data.Reference, s.Data.Description, s.Terms, s.Priced, s.CreatedBy),
		Status:        string(s.Status), OrderID: OptUUID(s.OrderID),
	}
	if s.QuotationNo != "" {
		no, fy := s.QuotationNo, s.FiscalYear
		m.QuotationNo, m.FiscalYear = &no, &fy
	}
	for _, l := range lineModels(s.Priced) {
		m.Lines = append(m.Lines, models.QuotationLineModel{QuotationID: m.ID, LineModel: l})
	}
	return m
}

func QuotationToEntity(m models.QuotationModel) *entities.Quotation {
	lines := make([]models.LineModel, len(m.Lines))
	for i, l := range m.Lines {
		lines[i] = l.LineModel
	}
	p := pricedOf(m.SalesDocModel, lines)
	s := entities.QuotationState{
		ID: vo.QuotationID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Terms: termsOf(m.SalesDocModel), Priced: p,
		Status: vo.QuotationStatus(m.Status), OrderID: OptID[vo.OrderID](m.OrderID), CreatedBy: vo.UserID(m.SalesDocModel.CreatedBy),
		Data: entities.QuotationData{
			CustomerID: vo.CustomerID(m.SalesDocModel.CustomerID), CustomerBranchNo: m.CustomerBranchNo, Date: DateUTC(m.DocDate), ValidUntil: DateUTC(m.ValidUntil),
			Reference: m.SalesDocModel.Reference, Description: m.SalesDocModel.Description, PriceIncludesVat: m.SalesDocModel.PriceIncludesVat, Lines: p.Lines,
		},
	}
	if m.QuotationNo != nil && m.FiscalYear != nil {
		s.QuotationNo, s.FiscalYear = *m.QuotationNo, *m.FiscalYear
	}
	return entities.RehydrateQuotation(s)
}

func OrderToModel(o *entities.Order) models.OrderModel {
	s := o.State()
	m := models.OrderModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), QuotationID: OptUUID(s.Data.QuotationID), DocDate: s.Data.Date,
		SalesDocModel: salesDoc(s.Data.CustomerID, s.Data.CustomerBranchNo, s.Data.Reference, s.Data.Description, s.Terms, s.Priced, s.CreatedBy),
		DeliveryDate:  s.Data.DeliveryDate, Status: string(s.Status),
	}
	if a := s.Approval; a != nil {
		no, fy, by, at := a.OrderNo, a.FiscalYear, uuid.UUID(a.By), a.At
		m.OrderNo, m.FiscalYear, m.ApprovedBy, m.ApprovedAt = &no, &fy, &by, &at
	}
	if c := s.Closure; c != nil {
		at := c.At
		m.ClosedBy, m.ClosedAt = OptUUID(c.By), &at
	}
	if c := s.Cancellation; c != nil {
		by, at := uuid.UUID(c.By), c.At
		m.CancelledBy, m.CancelledAt = &by, &at
	}
	for i, l := range lineModels(s.Priced) {
		p := s.Progress[i]
		m.Lines = append(m.Lines, models.OrderLineModel{OrderID: m.ID, LineModel: l, DeliveredQty: p.Delivered, InvoicedQty: p.Invoiced})
	}
	return m
}

func OrderToEntity(m models.OrderModel) *entities.Order {
	lines := make([]models.LineModel, len(m.Lines))
	progress := make([]entities.Progress, len(m.Lines))
	for i, l := range m.Lines {
		lines[i] = l.LineModel
		progress[i] = entities.Progress{Delivered: l.DeliveredQty, Invoiced: l.InvoicedQty}
	}
	p := pricedOf(m.SalesDocModel, lines)
	s := entities.OrderState{
		ID: vo.OrderID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Terms: termsOf(m.SalesDocModel), Priced: p, Progress: progress,
		Status: vo.OrderStatus(m.Status), CreatedBy: vo.UserID(m.SalesDocModel.CreatedBy),
		Data: entities.OrderData{
			CustomerID: vo.CustomerID(m.SalesDocModel.CustomerID), CustomerBranchNo: m.CustomerBranchNo, QuotationID: OptID[vo.QuotationID](m.QuotationID), Date: DateUTC(m.DocDate),
			DeliveryDate: optDate(m.DeliveryDate), Reference: m.SalesDocModel.Reference, Description: m.SalesDocModel.Description,
			PriceIncludesVat: m.SalesDocModel.PriceIncludesVat, Lines: p.Lines,
		},
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
	return entities.RehydrateOrder(s)
}

// ---------- ใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ ----------

func posted(customerID vo.CustomerID, description string, p entities.Posting, status vo.DocStatus, c *entities.Cancellation) models.PostedModel {
	m := models.PostedModel{
		FiscalYear: p.FiscalYear, FiscalPeriodID: uuid.UUID(p.PeriodID), CustomerID: uuid.UUID(customerID), Description: description,
		EntryID: OptUUID(p.EntryID), EntryNo: p.EntryNo, Status: string(status), CreatedBy: uuid.UUID(p.By), PostedAt: p.At,
	}
	if c != nil {
		by, at := uuid.UUID(c.By), c.At
		m.CancelledBy, m.CancelledAt = &by, &at
	}
	return m
}

func postingOf(no string, m models.PostedModel) entities.Posting {
	return entities.Posting{
		DocumentNo: no, FiscalYear: m.FiscalYear, PeriodID: vo.FiscalPeriodID(m.FiscalPeriodID),
		EntryID: OptID[vo.EntryID](m.EntryID), EntryNo: m.EntryNo, By: vo.UserID(m.CreatedBy), At: m.PostedAt,
	}
}

func cancellationOf(m models.PostedModel) *entities.Cancellation {
	if m.CancelledBy == nil || m.CancelledAt == nil {
		return nil
	}
	return &entities.Cancellation{By: vo.UserID(*m.CancelledBy), At: *m.CancelledAt}
}

func stockModel(s entities.Stock) models.StockModel {
	system := string(s.System)
	if system == "" {
		system = string(vo.Perpetual)
	}
	return models.StockModel{
		WarehouseID: OptUUID(s.WarehouseID), System: system, StockDocumentID: OptUUID(s.StockDocumentID), StockDocumentNo: s.StockDocumentNo,
	}
}

func stockOf(m models.StockModel) entities.Stock {
	return entities.Stock{
		WarehouseID: OptID[vo.WarehouseID](m.WarehouseID), System: vo.AccountingSystem(m.System),
		StockDocumentID: OptID[vo.StockDocumentID](m.StockDocumentID), StockDocumentNo: m.StockDocumentNo,
	}
}

func DeliveryToModel(d *entities.Delivery) models.DeliveryModel {
	s := d.State()
	m := models.DeliveryModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, DocDate: s.Date,
		OrderID: uuid.UUID(s.OrderID), PostedModel: posted(s.CustomerID, s.Description, s.Posting, s.Status, s.Cancellation),
		StockModel: stockModel(s.Stock), Reference: s.Reference, TotalCost: decimal.Zero,
	}
	for i, l := range s.Lines {
		m.TotalCost = m.TotalCost.Add(l.Cost)
		m.Lines = append(m.Lines, models.DeliveryLineModel{
			DeliveryID: m.ID, LineNo: i + 1, OrderLineNo: l.OrderLineNo, ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity,
			Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptUUID(l.InventoryAccountID),
		})
	}
	return m
}

func DeliveryToEntity(m models.DeliveryModel) *entities.Delivery {
	s := entities.DeliveryState{
		ID: vo.DeliveryID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), OrderID: vo.OrderID(m.OrderID), CustomerID: vo.CustomerID(m.PostedModel.CustomerID),
		Date: DateUTC(m.DocDate), Reference: m.Reference, Description: m.PostedModel.Description, Stock: stockOf(m.StockModel),
		Status: vo.DocStatus(m.PostedModel.Status), Posting: postingOf(m.DocumentNo, m.PostedModel), Cancellation: cancellationOf(m.PostedModel),
		Lines: make([]entities.DeliveryLine, len(m.Lines)),
	}
	for i, l := range m.Lines {
		s.Lines[i] = entities.DeliveryLine{
			OrderLineNo: l.OrderLineNo, ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity,
			CostLine: entities.CostLine{Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptID[vo.AccountID](l.InventoryAccountID)},
		}
	}
	return entities.RehydrateDelivery(s)
}

func InvoiceToModel(inv *entities.Invoice) models.InvoiceModel {
	s := inv.State()
	m := models.InvoiceModel{
		ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, DocDate: s.Date,
		OrderID: uuid.UUID(s.OrderID), PostedModel: posted(s.CustomerID, s.Description, s.Posting, s.Status, s.Cancellation),
		StockModel: stockModel(s.Stock), WithDelivery: s.WithDelivery, DueDate: s.DueDate, Reference: s.Reference,
		BillToName: s.BillTo.Name, BillToTaxID: s.BillTo.TaxID, BillToBranchNo: s.BillTo.BranchNo, BillToAddress: s.BillTo.Address,
		VatRate: s.Vat.Rate, PriceIncludesVat: s.Vat.Inclusive, TotalNet: s.Totals.Net, TotalVat: s.Totals.Vat, TotalGross: s.Totals.Gross,
		ReceivedAmount: s.Received,
	}
	for i, l := range s.Lines {
		m.Lines = append(m.Lines, models.InvoiceLineModel{
			InvoiceID: m.ID, LineNo: i + 1, OrderLineNo: l.OrderLineNo, ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, Net: l.Amounts.Net, Vat: l.Amounts.Vat, Gross: l.Amounts.Gross, RevenueAccountID: uuid.UUID(l.RevenueAccountID),
			Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptUUID(l.InventoryAccountID),
		})
	}
	return m
}

func InvoiceToEntity(m models.InvoiceModel) *entities.Invoice {
	s := entities.InvoiceState{
		ID: vo.InvoiceID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), OrderID: vo.OrderID(m.OrderID), CustomerID: vo.CustomerID(m.PostedModel.CustomerID),
		Date: DateUTC(m.DocDate), DueDate: DateUTC(m.DueDate), Reference: m.Reference, Description: m.PostedModel.Description,
		BillTo:       entities.BillTo{Name: m.BillToName, TaxID: m.BillToTaxID, BranchNo: m.BillToBranchNo, Address: m.BillToAddress},
		Vat:          services.VatTerms{Registered: !m.VatRate.IsZero(), Rate: m.VatRate, Inclusive: m.PriceIncludesVat},
		WithDelivery: m.WithDelivery, Stock: stockOf(m.StockModel), Received: m.ReceivedAmount,
		Totals: services.Amounts{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross},
		Status: vo.DocStatus(m.PostedModel.Status), Posting: postingOf(m.DocumentNo, m.PostedModel), Cancellation: cancellationOf(m.PostedModel),
		Lines: make([]entities.InvoiceLine, len(m.Lines)),
	}
	for i, l := range m.Lines {
		s.Lines[i] = entities.InvoiceLine{
			OrderLineNo: l.OrderLineNo, ItemID: vo.ItemID(l.ItemID), Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			Amounts: services.Amounts{Net: l.Net, Vat: l.Vat, Gross: l.Gross}, RevenueAccountID: vo.AccountID(l.RevenueAccountID),
			CostLine: entities.CostLine{Stock: l.Stock, Cost: l.Cost, InventoryAccountID: OptID[vo.AccountID](l.InventoryAccountID)},
		}
	}
	return entities.RehydrateInvoice(s)
}

func addressToModel(a thaiaddress.Address) models.AddressColumns { return models.AddressColumns(a) }

func addressToDomain(m models.AddressColumns) thaiaddress.Address { return thaiaddress.Address(m) }
