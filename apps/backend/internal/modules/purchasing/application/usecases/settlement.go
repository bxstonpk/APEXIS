package usecases

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

// SettlementUseCases: ตัดยอด / คืนยอดใบแจ้งหนี้ผู้ขายตามคำขอของ finance (เอกสารจ่ายเงิน)
//
// เรียกภายใน transaction ของ finance เสมอ — finance ตรวจสิทธิ์ finance.payment:* และสถานะบริษัทแล้ว
type SettlementUseCases struct {
	base
	invoices    repositories.InvoiceRepository
	suppliers   repositories.SupplierRepository
	settlements repositories.SettlementRepository
}

// Settle: lock ใบแจ้งหนี้ (เรียง id) -> ต้องเป็นของผู้ขายนี้และยอดไม่เกินค้าง -> บันทึกยอดจ่าย + ประวัติการตัดยอด
func (uc *SettlementUseCases) Settle(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID, doc dto.SettlementDoc, lines []dto.SettlementLine) (dto.SettlementResult, error) {
	settings, err := uc.settings.Get(ctx, companyID)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	if err := settings.Require("ap_account_id"); err != nil {
		return dto.SettlementResult{}, err
	}
	supplier, err := uc.suppliers.FindByID(ctx, companyID, supplierID)
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
	d := supplier.Data()
	out := dto.SettlementResult{
		Supplier:    dto.SettlementParty{ID: supplier.ID(), Code: d.Code, Name: d.Name, TaxID: d.TaxID, BranchNo: d.BranchNo, Address: d.Address, TaxType: d.TaxType},
		APAccountID: *settings.APAccountID, Invoices: make([]dto.SettledInvoice, len(lines)),
	}
	for i, l := range lines {
		inv, ok := byID[l.InvoiceID]
		if !ok {
			return dto.SettlementResult{}, fmt.Errorf("%w: line %d", domain.ErrInvoiceNotFound, i+1)
		}
		if err := inv.Pay(supplier.ID(), l.Amount); err != nil {
			return dto.SettlementResult{}, err
		}
		st := inv.State()
		out.Invoices[i] = dto.SettledInvoice{ID: inv.ID(), DocumentNo: st.Posting.DocumentNo, SupplierInvoiceNo: st.SupplierInvoiceNo,
			Net: st.Totals.Net, Gross: st.Totals.Gross}
	}
	if err := uc.invoices.SavePaid(ctx, invoices); err != nil {
		return dto.SettlementResult{}, err
	}
	return out, uc.settlements.Create(ctx, companyID, doc, lines)
}

// Unsettle: คืนยอดค้างให้ใบแจ้งหนี้ของเอกสารนี้ (ยกเลิกใบจ่ายเงิน) — ไม่พบรายการที่มีผล = ไม่ทำอะไร
func (uc *SettlementUseCases) Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	lines, err := uc.settlements.LockDocument(ctx, companyID, documentID)
	if err != nil || len(lines) == 0 {
		return err
	}
	ids := make([]vo.InvoiceID, len(lines))
	amounts := make(map[vo.InvoiceID]vo.Money, len(lines))
	for i, l := range lines {
		ids[i] = l.InvoiceID
		amounts[l.InvoiceID] = amounts[l.InvoiceID].Add(l.Amount)
	}
	invoices, err := uc.invoices.LockForSettlement(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, inv := range invoices {
		inv.Unpay(amounts[inv.ID()])
	}
	if err := uc.invoices.SavePaid(ctx, invoices); err != nil {
		return err
	}
	return uc.settlements.Cancel(ctx, companyID, documentID)
}

// Open: ใบแจ้งหนี้ค้างจ่ายของผู้ขาย (หน้าจ่ายเงินของ finance)
func (uc *SettlementUseCases) Open(ctx context.Context, companyID vo.CompanyID, supplierID vo.SupplierID) ([]dto.OpenInvoice, error) {
	return uc.invoices.Open(ctx, companyID, supplierID)
}
