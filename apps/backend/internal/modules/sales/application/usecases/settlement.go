package usecases

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// SettlementUseCases: ตัดยอด / คืนยอดเอกสารขายตามคำขอของ finance (ใบรับเงิน) — เงินเข้าทั้งหมดรับที่ finance
//
// เรียกภายใน transaction ของ finance เสมอ — finance ตรวจสิทธิ์ finance.receipt:* และสถานะบริษัทแล้ว
// จึงไม่ตรวจสิทธิ์ของขายซ้ำ (ผู้รับเงินไม่จำเป็นต้องมีสิทธิ์ใบแจ้งหนี้ขาย)
//
// ขายบริการ (ม.78/1): การรับชำระทำให้ภาษีขายถึงกำหนดตามสัดส่วนที่รับ — คืนบรรทัด Dr ภาษีขายยังไม่ถึงกำหนด / Cr ภาษีขาย
// ให้ finance ลงในรายการรับเงินเดียวกัน และบันทึกประวัติ (รายงานภาษีขายตามวันที่รับเงิน)
type SettlementUseCases struct {
	base
	invoices     repositories.InvoiceRepository
	customers    repositories.CustomerRepository
	settlements  repositories.SettlementRepository
	recognitions repositories.VatRecognitionRepository
}

// Settle: lock เอกสาร (เรียง id) -> ต้องเป็นของลูกค้านี้และยอดไม่เกินค้าง -> ภาษีที่ถึงกำหนด (ขายบริการ) -> บันทึกยอดรับ + ประวัติ
func (uc *SettlementUseCases) Settle(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, doc dto.SettlementDoc, lines []dto.SettlementLine) (dto.SettlementResult, error) {
	settings, err := uc.settings.Get(ctx, companyID)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	if err := settings.Require("ar_account_id"); err != nil {
		return dto.SettlementResult{}, err
	}
	cs, err := uc.org.CompanySettings(ctx, companyID)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	customer, err := uc.customers.FindByID(ctx, companyID, customerID)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	ids := make([]vo.InvoiceID, len(lines))
	for i, l := range lines {
		ids[i] = l.InvoiceID
	}
	invoices, err := uc.invoices.LockForSettlement(ctx, companyID, ids)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	byID := make(map[vo.InvoiceID]*entities.Invoice, len(invoices))
	for _, inv := range invoices {
		byID[inv.ID()] = inv
	}
	c := customer.Data()
	out := dto.SettlementResult{Customer: dto.SettlementParty{ID: customer.ID(), Code: c.Code, Name: c.Name, TaxID: c.TaxID, BranchNo: c.BranchNo, Address: c.Address},
		ARAccountID: *settings.ARAccountID, Invoices: make([]dto.SettledInvoice, len(lines))}
	var recognized []repositories.VatRecognition
	totalVat := decimal.Zero
	for i, l := range lines {
		inv, ok := byID[l.InvoiceID]
		if !ok {
			return dto.SettlementResult{}, fmt.Errorf("%w: line %d", domain.ErrInvoiceNotFound, i+1)
		}
		// ภาษีถึงกำหนดคำนวณจากยอดค้างก่อนรับ — ต้องเรียกก่อน Receive
		base, vat := inv.Recognize(l.Amount, cs.AmountDecimals)
		if err := inv.Receive(customer.ID(), l.Amount); err != nil {
			return dto.SettlementResult{}, err
		}
		st := inv.State()
		out.Invoices[i] = dto.SettledInvoice{ID: inv.ID(), DocumentNo: st.Posting.DocumentNo, Kind: st.Kind, Net: st.Totals.Net, Gross: st.Totals.Gross,
			VatBase: base, Vat: vat}
		if vat.IsPositive() {
			totalVat = totalVat.Add(vat)
			recognized = append(recognized, repositories.VatRecognition{Source: "receipt", DocumentID: doc.ID, DocumentNo: doc.No, Date: doc.Date,
				InvoiceID: inv.ID(), Base: base, Vat: vat})
		}
	}
	if totalVat.IsPositive() {
		if err := settings.Require("output_vat_account_id", "undue_output_vat_account_id"); err != nil {
			return dto.SettlementResult{}, err
		}
		out.ExtraJournal = []dto.JournalLine{
			{AccountID: *settings.UndueOutputVatAccountID, Debit: totalVat, Credit: decimal.Zero},
			{AccountID: *settings.OutputVatAccountID, Debit: decimal.Zero, Credit: totalVat},
		}
	}
	if err := uc.invoices.SaveReceived(ctx, invoices); err != nil {
		return dto.SettlementResult{}, err
	}
	if err := uc.recognitions.Create(ctx, companyID, recognized); err != nil {
		return dto.SettlementResult{}, err
	}
	return out, uc.settlements.Create(ctx, companyID, doc, lines)
}

// Unsettle: คืนยอดค้าง + ภาษีที่ถึงกำหนดให้เอกสารของใบรับเงินนี้ (ยกเลิกใบรับเงิน) — ไม่พบรายการที่มีผล = ไม่ทำอะไร
func (uc *SettlementUseCases) Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return unsettle(ctx, uc.invoices, uc.settlements, uc.recognitions, companyID, documentID)
}

// unsettle: คืนยอดของเอกสารที่ตัดยอด (ใบรับเงินของ finance / ใบลดหนี้) + ภาษีที่ถึงกำหนดจากเอกสารนั้น แล้วยกเลิกประวัติ
func unsettle(ctx context.Context, invoices repositories.InvoiceRepository, settlements repositories.SettlementRepository,
	recognitions repositories.VatRecognitionRepository, companyID vo.CompanyID, documentID uuid.UUID) error {
	lines, err := settlements.LockDocument(ctx, companyID, documentID)
	if err != nil || len(lines) == 0 {
		return err
	}
	recs, err := recognitions.LockDocument(ctx, companyID, documentID)
	if err != nil {
		return err
	}
	ids := make([]vo.InvoiceID, len(lines))
	amounts := make(map[vo.InvoiceID]vo.Money, len(lines))
	for i, l := range lines {
		ids[i] = l.InvoiceID
		amounts[l.InvoiceID] = amounts[l.InvoiceID].Add(l.Amount)
	}
	type vb struct{ base, vat vo.Money }
	undo := map[vo.InvoiceID]vb{}
	for _, r := range recs {
		u := undo[r.InvoiceID]
		undo[r.InvoiceID] = vb{u.base.Add(r.Base), u.vat.Add(r.Vat)}
	}
	locked, err := invoices.LockForSettlement(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, inv := range locked {
		inv.Unreceive(amounts[inv.ID()])
		if u, ok := undo[inv.ID()]; ok {
			inv.Unrecognize(u.base, u.vat)
		}
	}
	if err := invoices.SaveReceived(ctx, locked); err != nil {
		return err
	}
	if len(recs) > 0 {
		if err := recognitions.Cancel(ctx, companyID, documentID); err != nil {
			return err
		}
	}
	return settlements.Cancel(ctx, companyID, documentID)
}

// Open: เอกสารค้างรับของลูกค้า (หน้ารับเงินของ finance) — billingNoteID = เฉพาะใบในใบวางบิล
func (uc *SettlementUseCases) Open(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, billingNoteID *vo.BillingNoteID) ([]dto.OpenInvoice, error) {
	return uc.invoices.Open(ctx, companyID, customerID, billingNoteID)
}
