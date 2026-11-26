package usecases

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// SettlementUseCases: ตัดยอด / คืนยอดใบแจ้งหนี้ตามคำขอของ finance (เอกสารรับเงิน)
//
// เรียกภายใน transaction ของ finance เสมอ — finance ตรวจสิทธิ์ finance.receipt:* และสถานะบริษัทแล้ว
// จึงไม่ตรวจสิทธิ์ของขายซ้ำ (ผู้รับเงินไม่จำเป็นต้องมีสิทธิ์ใบแจ้งหนี้ขาย)
type SettlementUseCases struct {
	base
	invoices    repositories.InvoiceRepository
	customers   repositories.CustomerRepository
	settlements repositories.SettlementRepository
}

// Settle: lock ใบแจ้งหนี้ (เรียง id) -> ต้องเป็นของลูกค้านี้และยอดไม่เกินค้าง -> บันทึกยอดรับ + ประวัติการตัดยอด
func (uc *SettlementUseCases) Settle(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID, doc dto.SettlementDoc, lines []dto.SettlementLine) (dto.SettlementResult, error) {
	settings, err := uc.settings.Get(ctx, companyID)
	if err != nil {
		return dto.SettlementResult{}, err
	}
	if err := settings.Require("ar_account_id"); err != nil {
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
	out := dto.SettlementResult{Customer: dto.SettlementParty{ID: customer.ID(), Code: c.Code, Name: c.Name, TaxID: c.TaxID, BranchNo: c.BranchNo, Address: c.Address}, ARAccountID: *settings.ARAccountID, Invoices: make([]dto.SettledInvoice, len(lines))}
	for i, l := range lines {
		inv, ok := byID[l.InvoiceID]
		if !ok {
			return dto.SettlementResult{}, fmt.Errorf("%w: line %d", domain.ErrInvoiceNotFound, i+1)
		}
		if err := inv.Receive(customer.ID(), l.Amount); err != nil {
			return dto.SettlementResult{}, err
		}
		st := inv.State()
		out.Invoices[i] = dto.SettledInvoice{ID: inv.ID(), DocumentNo: st.Posting.DocumentNo, Net: st.Totals.Net, Gross: st.Totals.Gross}
	}
	if err := uc.invoices.SaveReceived(ctx, invoices); err != nil {
		return dto.SettlementResult{}, err
	}
	return out, uc.settlements.Create(ctx, companyID, doc, lines)
}

// Unsettle: คืนยอดค้างให้ใบแจ้งหนี้ของเอกสารนี้ (ยกเลิกใบรับเงิน) — ไม่พบรายการที่มีผล = ไม่ทำอะไร
func (uc *SettlementUseCases) Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return unsettle(ctx, uc.invoices, uc.settlements, companyID, documentID)
}

// unsettle: คืนยอดของเอกสารที่ตัดยอดใบแจ้งหนี้ (ใบรับเงินของ finance / ใบลดหนี้) แล้วยกเลิกประวัติการตัดยอด
func unsettle(ctx context.Context, invoices repositories.InvoiceRepository, settlements repositories.SettlementRepository, companyID vo.CompanyID, documentID uuid.UUID) error {
	lines, err := settlements.LockDocument(ctx, companyID, documentID)
	if err != nil || len(lines) == 0 {
		return err
	}
	ids := make([]vo.InvoiceID, len(lines))
	amounts := make(map[vo.InvoiceID]vo.Money, len(lines))
	for i, l := range lines {
		ids[i] = l.InvoiceID
		amounts[l.InvoiceID] = amounts[l.InvoiceID].Add(l.Amount)
	}
	locked, err := invoices.LockForSettlement(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, inv := range locked {
		inv.Unreceive(amounts[inv.ID()])
	}
	if err := invoices.SaveReceived(ctx, locked); err != nil {
		return err
	}
	return settlements.Cancel(ctx, companyID, documentID)
}

// Open: ใบแจ้งหนี้ค้างรับของลูกค้า (หน้ารับเงินของ finance)
func (uc *SettlementUseCases) Open(ctx context.Context, companyID vo.CompanyID, customerID vo.CustomerID) ([]dto.OpenInvoice, error) {
	return uc.invoices.Open(ctx, companyID, customerID)
}
