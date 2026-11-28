// Package domainerrors เก็บ error ของ sales — import ด้วย alias "domain"
// error ที่ระบุว่า "detailed" ถูก wrap ด้วยรายละเอียดที่ปลอดภัยต่อการแสดง
package domainerrors

import "errors"

// รูปแบบข้อมูลไม่ถูกต้อง (400)
var (
	ErrInvalidID         = errors.New("invalid id")
	ErrInvalidCode       = errors.New("invalid code")
	ErrInvalidName       = errors.New("invalid name")
	ErrInvalidText       = errors.New("text is too long")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidDate       = errors.New("invalid date")
	ErrInvalidTaxID      = errors.New("tax id must be 13 digits")
	ErrInvalidBranchNo   = errors.New("branch number must be 5 digits")
	ErrDuplicateBranch   = errors.New("duplicate customer branch number") // detailed
	ErrInvalidTerm       = errors.New("payment term must be 0 to 365 days")
	ErrInvalidTaxType    = errors.New("tax_type must be individual or juristic")
	ErrInvalidVatRate    = errors.New("vat rate must be 0 to 100 with at most 2 decimal places")
	ErrInvalidAmount     = errors.New("invalid amount") // detailed
	ErrInvalidLine       = errors.New("invalid line")   // detailed
	ErrInvalidCreditKind = errors.New("kind must be return or allowance")
	ErrReasonRequired    = errors.New("reason is required (at most 500 characters)")
)

// ไม่พบ (404)
var (
	ErrCustomerNotFound    = errors.New("customer not found")
	ErrQuotationNotFound   = errors.New("quotation not found")
	ErrOrderNotFound       = errors.New("sales order not found")
	ErrBillingNoteNotFound = errors.New("billing note not found")
	ErrInvoiceNotFound     = errors.New("sales invoice not found") // detailed
	ErrItemNotFound        = errors.New("item not found")          // detailed
	ErrCreditNoteNotFound  = errors.New("credit note not found")
)

// ขัดแย้ง (409)
var (
	ErrCodeExists     = errors.New("code already exists")
	ErrInUse          = errors.New("record is referenced by other records")
	ErrDocumentNoUsed = errors.New("document number is already used (check the document number format)")
	ErrQuotationUsed  = errors.New("quotation already has a sales order")
	ErrTaxIDExists    = errors.New("tax id already belongs to another customer") // detailed: รหัสที่มีอยู่ — เพิ่มสาขาที่รหัสนั้นแทน
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
	ErrOrderHasActivity  = errors.New("sales order already has invoices")
	ErrOverDelivery      = errors.New("quantity exceeds what is left to invoice")                          // detailed
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
	ErrBranchNotFound    = errors.New("customer branch not found") // detailed
	// ใบลดหนี้
	ErrOverCredit          = errors.New("credit exceeds what is left to credit on the invoice")                   // detailed
	ErrOverReturn          = errors.New("returned quantity exceeds invoiced quantity not yet returned")           // detailed
	ErrCreditPrice         = errors.New("unit price must be greater than 0 and not above the invoice unit price") // detailed
	ErrCreditBeforeInvoice = errors.New("credit note date must not be before the invoice date")
	ErrInvoiceHasCredits   = errors.New("invoice has credit notes (cancel the credit notes first)")
	ErrCostUnknown         = errors.New("original cost of the returned item is unknown (no posted delivery)") // detailed
	// ขายบริการ / ใบกำกับภาษี / มัดจำ / ใบวางบิล
	ErrServiceInvoiceGoodsLine = errors.New("service invoice goods line")                                                   // detailed: ขายบริการรับเฉพาะสินค้าชนิดบริการ
	ErrNotServiceInvoice       = errors.New("document is not a service invoice")                                            //
	ErrTaxInvoiceIssued        = errors.New("tax invoice is already issued for this service invoice")                       //
	ErrNoUndueVat              = errors.New("service invoice has no undue output vat left")                                 //
	ErrDepositNotUsable        = errors.New("deposit is not a posted deposit of this customer")                             // detailed
	ErrDepositOverApplied      = errors.New("deposit amount exceeds the remaining deposit")                                 // detailed
	ErrDepositApplied          = errors.New("deposit is already applied to sales documents (cancel those documents first)") //
	ErrDepositExceedsSale      = errors.New("applied deposits exceed the document amount")                                  //
	ErrDocumentNotAvailable    = errors.New("document not available for business type")                                     // detailed: ซื้อขาย / ผลิต = ขายสินค้า; บริการ / ก่อสร้าง = ขายบริการ
	ErrDepositNotPaid          = errors.New("deposit not paid")                                                             // detailed: รับเงินมัดจำที่ finance ให้ครบก่อนหัก
	ErrBillingInvoice          = errors.New("invoice cannot be put on this billing note")                                   // detailed
	ErrBillingNoteCancelled    = errors.New("billing note is already cancelled")                                            //
)

var ErrForbidden = errors.New("permission denied")

var all = []error{
	ErrInvalidID, ErrInvalidCode, ErrInvalidName, ErrInvalidText, ErrInvalidStatus, ErrInvalidDate, ErrInvalidTaxID,
	ErrInvalidBranchNo, ErrDuplicateBranch, ErrInvalidTerm, ErrInvalidTaxType, ErrInvalidVatRate, ErrInvalidAmount, ErrInvalidLine,
	ErrInvalidCreditKind, ErrReasonRequired, ErrCreditNoteNotFound,
	ErrOverCredit, ErrOverReturn, ErrCreditPrice, ErrCreditBeforeInvoice, ErrInvoiceHasCredits, ErrCostUnknown,
	ErrCustomerNotFound, ErrQuotationNotFound, ErrOrderNotFound, ErrBillingNoteNotFound, ErrInvoiceNotFound,
	ErrServiceInvoiceGoodsLine, ErrNotServiceInvoice, ErrTaxInvoiceIssued, ErrNoUndueVat, ErrDepositNotUsable, ErrDepositOverApplied,
	ErrDepositApplied, ErrDepositExceedsSale, ErrDocumentNotAvailable, ErrDepositNotPaid, ErrBillingInvoice, ErrBillingNoteCancelled,
	ErrItemNotFound,
	ErrCodeExists, ErrInUse, ErrDocumentNoUsed, ErrQuotationUsed, ErrTaxIDExists,
	ErrCompanyInactive, ErrCustomerInactive, ErrItemInactive, ErrVatNotApplicable, ErrNotDraft, ErrQuotationState,
	ErrQuotationExpired, ErrOrderNotApproved, ErrOrderNotOpen, ErrOrderHasActivity, ErrOverDelivery, ErrOverInvoice,
	ErrDeliveryInvoiced, ErrAlreadyCancelled, ErrInvoicePaid, ErrOverReceipt, ErrInvoiceNotPayable,
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
