// Package domainerrors เก็บ error ของ purchasing ไว้เป็น package ล่างสุด
// ทุกที่ import ด้วย alias "domain" — error ที่ระบุว่า "detailed" ถูก wrap ด้วยรายละเอียดที่ปลอดภัยต่อการแสดง
package domainerrors

import "errors"

// รูปแบบข้อมูลไม่ถูกต้อง (400)
var (
	ErrInvalidID       = errors.New("invalid id")
	ErrInvalidCode     = errors.New("invalid code")
	ErrInvalidName     = errors.New("invalid name")
	ErrInvalidText     = errors.New("text is too long")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidDate     = errors.New("invalid date")
	ErrInvalidTaxID    = errors.New("tax id must be 13 digits")
	ErrInvalidBranchNo = errors.New("branch number must be 5 digits")
	ErrDuplicateBranch = errors.New("duplicate supplier branch number") // detailed
	ErrInvalidTerm     = errors.New("payment term must be 0 to 365 days")
	ErrInvalidVatRate  = errors.New("vat rate must be 0 to 100 with at most 2 decimal places")
	ErrInvalidLine     = errors.New("invalid line") // detailed
	ErrInvalidTaxType  = errors.New("tax_type must be individual or juristic")
)

// ไม่พบ (404)
var (
	ErrSupplierNotFound = errors.New("supplier not found")
	ErrOrderNotFound    = errors.New("purchase order not found")
	ErrReceiptNotFound  = errors.New("goods receipt not found")
	ErrInvoiceNotFound  = errors.New("supplier invoice not found")
	ErrItemNotFound     = errors.New("item not found") // detailed
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists      = errors.New("code already exists")
	ErrInUse           = errors.New("record is referenced by other records")
	ErrInvoiceNoExists = errors.New("supplier invoice number already recorded for this supplier")
	ErrDocumentNoUsed  = errors.New("document number is already used (check the document number format)")
	ErrTaxIDExists     = errors.New("tax id already belongs to another supplier") // detailed: รหัสที่มีอยู่ — เพิ่มสาขาที่รหัสนั้นแทน
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive   = errors.New("company is inactive (read-only)")
	ErrSupplierInactive  = errors.New("supplier is inactive")
	ErrItemInactive      = errors.New("item is inactive") // detailed
	ErrVatNotApplicable  = errors.New("supplier is not VAT registered (prices cannot include VAT)")
	ErrOrderNotDraft     = errors.New("only draft purchase orders can be changed")
	ErrOrderNotApproved  = errors.New("purchase order is not approved")
	ErrOrderNotOpen      = errors.New("purchase order is not approved or closed")
	ErrOrderHasActivity  = errors.New("purchase order already has receipts or invoices")
	ErrOverReceipt       = errors.New("quantity exceeds what is left to receive")                         // detailed
	ErrOverInvoice       = errors.New("quantity exceeds received but not invoiced quantity")              // detailed
	ErrReceiptInvoiced   = errors.New("received quantity is already invoiced (cancel the invoice first)") // detailed
	ErrAlreadyCancelled  = errors.New("document is already cancelled")
	ErrNoFiscalPeriod    = errors.New("no fiscal period covers the date")
	ErrPeriodClosed      = errors.New("fiscal period is closed")
	ErrSettingsMissing   = errors.New("purchasing settings are incomplete")                                 // detailed
	ErrAccountInvalid    = errors.New("account cannot be used for posting (not found, header or inactive)") // detailed
	ErrJournalInvalid    = errors.New("journal not found or inactive")
	ErrWarehouseRequired = errors.New("warehouse_id is required to receive stock items")
	ErrInvoicePaid       = errors.New("invoice has payments (cancel the payments first)")
	ErrOverPayment       = errors.New("amount exceeds the invoice outstanding balance")          // detailed
	ErrInvoiceNotPayable = errors.New("invoice is not a posted invoice of this supplier")        // detailed
	ErrInactiveRef       = errors.New("withholding tax type or cash / bank account is inactive") // detailed
	ErrBranchNotFound    = errors.New("supplier branch not found")                               // detailed
)

// สิทธิ์ (403)
var ErrForbidden = errors.New("permission denied")

var all = []error{
	ErrInvalidID, ErrInvalidCode, ErrInvalidName, ErrInvalidText, ErrInvalidStatus, ErrInvalidDate, ErrInvalidTaxID,
	ErrInvalidBranchNo, ErrDuplicateBranch, ErrInvalidTerm, ErrInvalidVatRate, ErrInvalidLine, ErrInvalidTaxType,
	ErrInvoicePaid, ErrOverPayment, ErrInvoiceNotPayable, ErrInactiveRef,
	ErrSupplierNotFound, ErrOrderNotFound, ErrReceiptNotFound, ErrInvoiceNotFound, ErrItemNotFound,
	ErrCodeExists, ErrInUse, ErrInvoiceNoExists, ErrDocumentNoUsed, ErrTaxIDExists,
	ErrCompanyInactive, ErrSupplierInactive, ErrItemInactive, ErrVatNotApplicable, ErrOrderNotDraft, ErrOrderNotApproved,
	ErrOrderNotOpen, ErrOrderHasActivity, ErrOverReceipt, ErrOverInvoice, ErrReceiptInvoiced, ErrAlreadyCancelled,
	ErrNoFiscalPeriod, ErrPeriodClosed, ErrSettingsMissing, ErrAccountInvalid, ErrJournalInvalid, ErrWarehouseRequired, ErrBranchNotFound,
	ErrForbidden,
}

// IsBusiness: err เป็นการปฏิเสธด้วยกฎธุรกิจของ module นี้
func IsBusiness(err error) bool {
	for _, e := range all {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
