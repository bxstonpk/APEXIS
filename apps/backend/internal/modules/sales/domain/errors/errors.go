// Package domainerrors เก็บ error ของ sales — import ด้วย alias "domain"
// error ที่ระบุว่า "detailed" ถูก wrap ด้วยรายละเอียดที่ปลอดภัยต่อการแสดง
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
	ErrInvalidTerm     = errors.New("payment term must be 0 to 365 days")
	ErrInvalidTaxType  = errors.New("tax_type must be individual or juristic")
	ErrInvalidVatRate  = errors.New("vat rate must be 0 to 100 with at most 2 decimal places")
	ErrInvalidAmount   = errors.New("invalid amount") // detailed
	ErrInvalidLine     = errors.New("invalid line")   // detailed
)

// ไม่พบ (404)
var (
	ErrCustomerNotFound  = errors.New("customer not found")
	ErrQuotationNotFound = errors.New("quotation not found")
	ErrOrderNotFound     = errors.New("sales order not found")
	ErrDeliveryNotFound  = errors.New("delivery not found")
	ErrInvoiceNotFound   = errors.New("sales invoice not found") // detailed
	ErrItemNotFound      = errors.New("item not found")          // detailed
)

// ขัดแย้ง (409)
var (
	ErrCodeExists     = errors.New("code already exists")
	ErrInUse          = errors.New("record is referenced by other records")
	ErrDocumentNoUsed = errors.New("document number is already used (check the document number format)")
	ErrQuotationUsed  = errors.New("quotation already has a sales order")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive   = errors.New("company is inactive (read-only)")
	ErrCustomerInactive  = errors.New("customer is inactive")
	ErrItemInactive      = errors.New("item is inactive") // detailed
	ErrVatNotApplicable  = errors.New("company is not VAT registered (prices cannot include VAT)")
	ErrNotDraft          = errors.New("only draft documents can be changed")
	ErrQuotationState    = errors.New("quotation status does not allow this change") // detailed
	ErrQuotationExpired  = errors.New("quotation has expired")
	ErrOrderNotApproved  = errors.New("sales order is not approved")
	ErrOrderNotOpen      = errors.New("sales order is not approved or closed")
	ErrOrderHasActivity  = errors.New("sales order already has deliveries or invoices")
	ErrOverDelivery      = errors.New("quantity exceeds what is left to deliver")                          // detailed
	ErrOverInvoice       = errors.New("quantity exceeds delivered but not invoiced quantity")              // detailed
	ErrDeliveryInvoiced  = errors.New("delivered quantity is already invoiced (cancel the invoice first)") // detailed
	ErrAlreadyCancelled  = errors.New("document is already cancelled")
	ErrInvoicePaid       = errors.New("invoice has receipts (cancel the receipts first)")
	ErrOverReceipt       = errors.New("amount exceeds the invoice outstanding balance")   // detailed
	ErrInvoiceNotPayable = errors.New("invoice is not a posted invoice of this customer") // detailed
	ErrNoFiscalPeriod    = errors.New("no fiscal period covers the date")
	ErrPeriodClosed      = errors.New("fiscal period is closed")
	ErrSettingsMissing   = errors.New("sales settings are incomplete")                                      // detailed
	ErrAccountInvalid    = errors.New("account cannot be used for posting (not found, header or inactive)") // detailed
	ErrJournalInvalid    = errors.New("journal not found or inactive")
	ErrWarehouseRequired = errors.New("warehouse_id is required to deliver stock items")
)

var ErrForbidden = errors.New("permission denied")

var all = []error{
	ErrInvalidID, ErrInvalidCode, ErrInvalidName, ErrInvalidText, ErrInvalidStatus, ErrInvalidDate, ErrInvalidTaxID,
	ErrInvalidBranchNo, ErrInvalidTerm, ErrInvalidTaxType, ErrInvalidVatRate, ErrInvalidAmount, ErrInvalidLine,
	ErrCustomerNotFound, ErrQuotationNotFound, ErrOrderNotFound, ErrDeliveryNotFound, ErrInvoiceNotFound,
	ErrItemNotFound,
	ErrCodeExists, ErrInUse, ErrDocumentNoUsed, ErrQuotationUsed,
	ErrCompanyInactive, ErrCustomerInactive, ErrItemInactive, ErrVatNotApplicable, ErrNotDraft, ErrQuotationState,
	ErrQuotationExpired, ErrOrderNotApproved, ErrOrderNotOpen, ErrOrderHasActivity, ErrOverDelivery, ErrOverInvoice,
	ErrDeliveryInvoiced, ErrAlreadyCancelled, ErrInvoicePaid, ErrOverReceipt, ErrInvoiceNotPayable,
	ErrNoFiscalPeriod, ErrPeriodClosed, ErrSettingsMissing, ErrAccountInvalid, ErrJournalInvalid, ErrWarehouseRequired,
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
