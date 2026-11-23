package gormpersistence

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
)

func OptUUID[T ~[16]byte](p *T) *uuid.UUID {
	if p == nil {
		return nil
	}
	u := uuid.UUID(*p)
	return &u
}

func OptID[T ~[16]byte](p *uuid.UUID) *T {
	if p == nil {
		return nil
	}
	t := T(*p)
	return &t
}

// DateUTC: คอลัมน์ date ที่อ่านกลับมาเป็นเที่ยงคืน UTC (ตรงกับ vo.IsDateOnly)
func DateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ---------- ค่าตั้ง / ประเภทภาษี ----------

func SettingsToModel(companyID vo.CompanyID, s entities.Settings) models.SettingsModel {
	return models.SettingsModel{
		CompanyID: uuid.UUID(companyID), ReceiptJournalID: OptUUID(s.ReceiptJournalID), PaymentJournalID: OptUUID(s.PaymentJournalID),
		TransferJournalID: OptUUID(s.TransferJournalID), WhtReceivedAccountID: OptUUID(s.WhtReceivedAccountID),
		WhtPnd3AccountID: OptUUID(s.WhtPnd3AccountID), WhtPnd53AccountID: OptUUID(s.WhtPnd53AccountID),
	}
}

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{
		ReceiptJournalID: OptID[vo.JournalID](m.ReceiptJournalID), PaymentJournalID: OptID[vo.JournalID](m.PaymentJournalID),
		TransferJournalID: OptID[vo.JournalID](m.TransferJournalID), WhtReceivedAccountID: OptID[vo.GLAccountID](m.WhtReceivedAccountID),
		WhtPnd3AccountID: OptID[vo.GLAccountID](m.WhtPnd3AccountID), WhtPnd53AccountID: OptID[vo.GLAccountID](m.WhtPnd53AccountID),
	}
}

func WhtTypeToModel(w *entities.WhtType) models.WhtTypeModel {
	d := w.Data()
	return models.WhtTypeModel{ID: uuid.UUID(w.ID()), CompanyID: uuid.UUID(w.CompanyID()), Code: d.Code, Name: d.Name, Rate: d.Rate, Status: string(d.Status)}
}

func WhtTypeToEntity(m models.WhtTypeModel) *entities.WhtType {
	return entities.RehydrateWhtType(vo.WhtTypeID(m.ID), vo.CompanyID(m.CompanyID), entities.WhtTypeData{
		Code: m.Code, Name: m.Name, Rate: m.Rate, Status: vo.Status(m.Status),
	})
}

// ---------- ส่วนร่วมของเอกสาร ----------

func docModel(id uuid.UUID, description string, status vo.DocStatus, p entities.Posting, c *entities.Cancellation) models.DocumentModel {
	m := models.DocumentModel{
		ID: id, Description: description, Status: string(status), FiscalPeriodID: uuid.UUID(p.PeriodID),
		EntryID: OptUUID(p.EntryID), EntryNo: p.EntryNo, CreatedBy: uuid.UUID(p.By), CreatedAt: p.At, UpdatedAt: p.At,
	}
	if c != nil {
		by, at := uuid.UUID(c.By), c.At
		m.CancelledBy, m.CancelledAt = &by, &at
	}
	return m
}

func posting(no string, m models.DocumentModel) entities.Posting {
	return entities.Posting{
		DocumentNo: no, PeriodID: vo.FiscalPeriodID(m.FiscalPeriodID), EntryID: OptID[vo.EntryID](m.EntryID), EntryNo: m.EntryNo,
		By: vo.UserID(m.CreatedBy), At: m.CreatedAt,
	}
}

func cancellation(m models.DocumentModel) *entities.Cancellation {
	if m.CancelledBy == nil || m.CancelledAt == nil {
		return nil
	}
	return &entities.Cancellation{By: vo.UserID(*m.CancelledBy), At: *m.CancelledAt}
}

func partyModel(p entities.Party) models.PartyModel {
	return models.PartyModel{PartyID: p.ID, PartyCode: p.Code, PartyName: p.Name, PartyTaxID: p.TaxID, PartyBranchNo: p.BranchNo,
		PartyAddress: p.Address, PartyTaxType: string(p.TaxType)}
}

func party(m models.PartyModel) entities.Party {
	return entities.Party{ID: m.PartyID, Code: m.PartyCode, Name: m.PartyName, TaxID: m.PartyTaxID, BranchNo: m.PartyBranchNo,
		Address: m.PartyAddress, TaxType: vo.TaxType(m.PartyTaxType)}
}

func methodModel(i int, x entities.Method) models.MethodModel {
	return models.MethodModel{LineNo: i + 1, CashAccountID: uuid.UUID(x.CashAccountID), GLAccountID: uuid.UUID(x.GLAccountID), Amount: x.Amount, Reference: x.Reference}
}

func method(m models.MethodModel) entities.Method {
	return entities.Method{CashAccountID: vo.CashAccountID(m.CashAccountID), GLAccountID: vo.GLAccountID(m.GLAccountID), Amount: m.Amount, Reference: m.Reference}
}

// ---------- ใบรับเงิน ----------

func ReceiptToModel(r *entities.Receipt) models.ReceiptModel {
	s := r.State()
	m := models.ReceiptModel{
		CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, DocDate: s.Date,
		DocumentModel: docModel(uuid.UUID(s.ID), s.Description, s.Status, s.Posting, s.Cancellation),
		Kind:          string(s.Kind), PartyModel: partyModel(s.Party),
		TotalAmount: s.Totals.Amount, TotalWht: s.Totals.Wht, TotalReceived: s.Totals.Received,
	}
	for i, l := range s.Lines {
		m.Lines = append(m.Lines, models.ReceiptLineModel{
			ReceiptID: m.ID, LineNo: i + 1, InvoiceID: OptUUID(l.InvoiceID), InvoiceNo: l.InvoiceNo, AccountID: OptUUID(l.AccountID),
			Description: l.Description, Amount: l.Amount, Wht: l.Wht,
		})
	}
	for i, x := range s.Methods {
		m.Methods = append(m.Methods, models.ReceiptMethodModel{ReceiptID: m.ID, MethodModel: methodModel(i, x)})
	}
	return m
}

func ReceiptToEntity(m models.ReceiptModel) *entities.Receipt {
	s := entities.ReceiptState{
		ID: vo.ReceiptID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Kind: vo.ReceiptKind(m.Kind), Party: party(m.PartyModel),
		Date: DateUTC(m.DocDate), Description: m.Description, Status: vo.DocStatus(m.Status),
		Totals:  entities.ReceiptTotals{Amount: m.TotalAmount, Wht: m.TotalWht, Received: m.TotalReceived},
		Posting: posting(m.DocumentNo, m.DocumentModel), Cancellation: cancellation(m.DocumentModel),
	}
	for _, l := range m.Lines {
		s.Lines = append(s.Lines, entities.ReceiptLine{
			InvoiceID: OptID[vo.InvoiceID](l.InvoiceID), InvoiceNo: l.InvoiceNo, AccountID: OptID[vo.GLAccountID](l.AccountID),
			Description: l.Description, Amount: l.Amount, Wht: l.Wht,
		})
	}
	for _, x := range m.Methods {
		s.Methods = append(s.Methods, method(x.MethodModel))
	}
	return entities.RehydrateReceipt(s)
}

// ---------- ใบสำคัญจ่าย ----------

func PaymentToModel(p *entities.Payment) models.PaymentModel {
	s := p.State()
	m := models.PaymentModel{
		CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, DocDate: s.Date,
		DocumentModel: docModel(uuid.UUID(s.ID), s.Description, s.Status, s.Posting, s.Cancellation),
		Kind:          string(s.Kind), PartyModel: partyModel(s.Party),
		TotalAmount: s.Totals.Amount, TotalWht: s.Totals.Wht, TotalPaid: s.Totals.Paid,
	}
	for i, l := range s.Lines {
		m.Lines = append(m.Lines, models.PaymentLineModel{
			PaymentID: m.ID, LineNo: i + 1, InvoiceID: OptUUID(l.InvoiceID), InvoiceNo: l.InvoiceNo, SupplierInvoiceNo: l.SupplierInvoiceNo,
			AccountID: OptUUID(l.AccountID), Description: l.Description, Amount: l.Amount,
			WhtTypeID: OptUUID(l.WhtTypeID), WhtRate: l.WhtRate, WhtBase: l.WhtBase, Wht: l.Wht,
		})
	}
	for i, x := range s.Methods {
		m.Methods = append(m.Methods, models.PaymentMethodModel{PaymentID: m.ID, MethodModel: methodModel(i, x)})
	}
	return m
}

func PaymentToEntity(m models.PaymentModel) *entities.Payment {
	s := entities.PaymentState{
		ID: vo.PaymentID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Kind: vo.PaymentKind(m.Kind), Party: party(m.PartyModel),
		Date: DateUTC(m.DocDate), Description: m.Description, Status: vo.DocStatus(m.Status),
		Totals:  entities.PaymentTotals{Amount: m.TotalAmount, Wht: m.TotalWht, Paid: m.TotalPaid},
		Posting: posting(m.DocumentNo, m.DocumentModel), Cancellation: cancellation(m.DocumentModel),
	}
	for _, l := range m.Lines {
		s.Lines = append(s.Lines, entities.PaymentLine{
			InvoiceID: OptID[vo.InvoiceID](l.InvoiceID), InvoiceNo: l.InvoiceNo, SupplierInvoiceNo: l.SupplierInvoiceNo,
			AccountID: OptID[vo.GLAccountID](l.AccountID), Description: l.Description, Amount: l.Amount,
			WhtTypeID: OptID[vo.WhtTypeID](l.WhtTypeID), WhtRate: l.WhtRate, WhtBase: l.WhtBase, Wht: l.Wht,
		})
	}
	for _, x := range m.Methods {
		s.Methods = append(s.Methods, method(x.MethodModel))
	}
	return entities.RehydratePayment(s)
}

// ---------- โอนเงิน ----------

func TransferToModel(t *entities.Transfer) models.TransferModel {
	s := t.State()
	return models.TransferModel{
		CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.Posting.DocumentNo, DocDate: s.Date,
		DocumentModel: docModel(uuid.UUID(s.ID), s.Description, s.Status, s.Posting, s.Cancellation),
		Reference:     s.Reference,
		FromAccountID: uuid.UUID(s.From.CashAccountID), FromGLAccountID: uuid.UUID(s.From.GLAccountID),
		ToAccountID: uuid.UUID(s.To.CashAccountID), ToGLAccountID: uuid.UUID(s.To.GLAccountID), Amount: s.From.Amount,
	}
}

func TransferToEntity(m models.TransferModel) *entities.Transfer {
	return entities.RehydrateTransfer(entities.TransferState{
		ID: vo.TransferID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), Date: DateUTC(m.DocDate), Description: m.Description,
		Reference: m.Reference, Status: vo.DocStatus(m.Status),
		From:    entities.Method{CashAccountID: vo.CashAccountID(m.FromAccountID), GLAccountID: vo.GLAccountID(m.FromGLAccountID), Amount: m.Amount},
		To:      entities.Method{CashAccountID: vo.CashAccountID(m.ToAccountID), GLAccountID: vo.GLAccountID(m.ToGLAccountID), Amount: m.Amount},
		Posting: posting(m.DocumentNo, m.DocumentModel), Cancellation: cancellation(m.DocumentModel),
	})
}
