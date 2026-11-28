package gormpersistence

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm/models"
)

func optUUID[T ~[16]byte](id *T) *uuid.UUID {
	if id == nil {
		return nil
	}
	u := uuid.UUID(*id)
	return &u
}

func optID[T ~[16]byte](u *uuid.UUID) *T {
	if u == nil {
		return nil
	}
	id := T(*u)
	return &id
}

// ---------- ค่าตั้ง ----------

func SettingsToEntity(m models.SettingsModel) entities.Settings {
	return entities.Settings{InputVatAccountID: optID[vo.AccountID](m.InputVatAccountID)}
}

func SettingsToModel(companyID vo.CompanyID, s entities.Settings) models.SettingsModel {
	return models.SettingsModel{CompanyID: uuid.UUID(companyID), InputVatAccountID: optUUID(s.InputVatAccountID)}
}

// ---------- กองทุน ----------

func FundToModel(companyID vo.CompanyID, f *entities.Fund) models.FundModel {
	d := f.Data()
	return models.FundModel{ID: uuid.UUID(f.ID()), CompanyID: uuid.UUID(companyID), Code: d.Code, Name: d.Name,
		CustodianID: uuid.UUID(d.CustodianID), GLAccountID: uuid.UUID(d.GLAccountID), ImprestAmount: d.ImprestAmount, Status: string(d.Status)}
}

func FundToEntity(m models.FundModel) *entities.Fund {
	return entities.RehydrateFund(vo.FundID(m.ID), entities.FundData{Code: m.Code, Name: m.Name, CustodianID: vo.UserID(m.CustodianID),
		GLAccountID: vo.AccountID(m.GLAccountID), ImprestAmount: m.ImprestAmount, Status: vo.Status(m.Status)})
}

// ---------- ใบสำคัญจ่าย ----------

func VoucherToModels(v *entities.Voucher) (models.VoucherModel, []models.VoucherLineModel) {
	s := v.State()
	m := models.VoucherModel{ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.DocumentNo, FundID: uuid.UUID(s.FundID),
		DocDate: s.Date, PayeeName: s.Payee.Name, PayeeTaxID: s.Payee.TaxID, PayeeBranchNo: s.Payee.BranchNo, Description: s.Description,
		VatRate: s.VatRate, PriceIncludesVat: s.PriceIncludesVat, TotalNet: s.Totals.Net, TotalVat: s.Totals.Vat, TotalGross: s.Totals.Gross,
		Status: string(s.Status), ReplenishmentID: optUUID(s.ReplenishmentID), CreatedBy: uuid.UUID(s.CreatedBy),
		ApprovedBy: optUUID(s.ApprovedBy), ApprovedAt: s.ApprovedAt, CancelledAt: s.CancelledAt}
	if s.TaxInvoice != nil {
		d := s.TaxInvoice.Date
		m.TaxInvoiceNo, m.TaxInvoiceDate = s.TaxInvoice.No, &d
	}
	lines := make([]models.VoucherLineModel, len(s.Lines))
	for i, l := range s.Lines {
		lines[i] = models.VoucherLineModel{VoucherID: m.ID, LineNo: i + 1, AccountID: uuid.UUID(l.AccountID), Description: l.Description,
			Amount: l.Amount, Net: l.Net}
	}
	return m, lines
}

func VoucherToEntity(m models.VoucherModel) *entities.Voucher {
	s := entities.VoucherState{ID: vo.VoucherID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), DocumentNo: m.DocumentNo,
		VoucherData: entities.VoucherData{FundID: vo.FundID(m.FundID), Date: utcDate(m.DocDate),
			Payee:       entities.Payee{Name: m.PayeeName, TaxID: m.PayeeTaxID, BranchNo: m.PayeeBranchNo},
			Description: m.Description, VatRate: m.VatRate, PriceIncludesVat: m.PriceIncludesVat},
		Totals: entities.Totals{Net: m.TotalNet, Vat: m.TotalVat, Gross: m.TotalGross}, Status: vo.VoucherStatus(m.Status),
		ReplenishmentID: optID[vo.ReplenishmentID](m.ReplenishmentID), CreatedBy: vo.UserID(m.CreatedBy),
		ApprovedBy: optID[vo.UserID](m.ApprovedBy), ApprovedAt: m.ApprovedAt, CancelledAt: m.CancelledAt}
	if m.TaxInvoiceDate != nil {
		s.TaxInvoice = &entities.TaxInvoice{No: m.TaxInvoiceNo, Date: utcDate(*m.TaxInvoiceDate)}
	}
	s.Lines = make([]entities.VoucherLine, len(m.Lines))
	for i, l := range m.Lines {
		s.Lines[i] = entities.VoucherLine{AccountID: vo.AccountID(l.AccountID), Description: l.Description, Amount: l.Amount, Net: l.Net}
	}
	return entities.RehydrateVoucher(s)
}

// ---------- คำขอ ----------

func ReplenishmentToModel(r *entities.Replenishment) models.ReplenishmentModel {
	s := r.State()
	m := models.ReplenishmentModel{ID: uuid.UUID(s.ID), CompanyID: uuid.UUID(s.CompanyID), DocumentNo: s.DocumentNo, FundID: uuid.UUID(s.FundID),
		Kind: string(s.Kind), DocDate: s.Date, Description: s.Description, Amount: s.Amount, Status: string(s.Status),
		CreatedBy: uuid.UUID(s.CreatedBy), CreatedAt: s.CreatedAt, CancelledAt: s.CancelledAt}
	if p := s.Payment; p != nil {
		id, d := p.DocumentID, p.Date
		m.PaymentDocumentID, m.PaymentDocumentNo, m.PaidDate = &id, p.DocumentNo, &d
	}
	return m
}

func ReplenishmentToEntity(m models.ReplenishmentModel, voucherIDs []vo.VoucherID) *entities.Replenishment {
	s := entities.ReplenishmentState{ID: vo.ReplenishmentID(m.ID), CompanyID: vo.CompanyID(m.CompanyID), DocumentNo: m.DocumentNo,
		FundID: vo.FundID(m.FundID), Kind: vo.RequestKind(m.Kind), Date: utcDate(m.DocDate), Description: m.Description, Amount: m.Amount,
		VoucherIDs: voucherIDs, Status: vo.RequestStatus(m.Status), CreatedBy: vo.UserID(m.CreatedBy), CreatedAt: m.CreatedAt,
		CancelledAt: m.CancelledAt}
	if m.PaymentDocumentID != nil && m.PaidDate != nil {
		s.Payment = &entities.Payment{DocumentID: *m.PaymentDocumentID, DocumentNo: m.PaymentDocumentNo, Date: utcDate(*m.PaidDate)}
	}
	return entities.RehydrateReplenishment(s)
}

// utcDate: column DATE อ่านกลับมาเป็นเที่ยงคืน UTC (domain ใช้วันที่ UTC)
func utcDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
