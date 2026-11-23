// Package domainerrors เก็บ error ของ finance — import ด้วย alias "domain"
package domainerrors

import "errors"

// 400
var (
	ErrInvalidID        = errors.New("invalid id")
	ErrInvalidCode      = errors.New("invalid code")
	ErrInvalidName      = errors.New("invalid name")
	ErrInvalidText      = errors.New("text is too long")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidType      = errors.New("type must be cash or bank")
	ErrInvalidAccountNo = errors.New("bank account number may contain only digits and - (up to 30)")
	ErrBankDetails      = errors.New("bank accounts require bank_name and account_no")
	ErrInvalidDate      = errors.New("invalid date")
	ErrInvalidKind      = errors.New("invalid document kind")
	ErrInvalidTaxType   = errors.New("tax_type must be individual or juristic")
	ErrInvalidWhtRate   = errors.New("withholding tax rate must be more than 0 and at most 100 with at most 2 decimal places")
	ErrInvalidReceipt   = errors.New("invalid receipt")  // detailed
	ErrInvalidPayment   = errors.New("invalid payment")  // detailed
	ErrInvalidTransfer  = errors.New("invalid transfer") // detailed
)

// 404
var (
	ErrCashAccountNotFound = errors.New("cash / bank account not found") // detailed
	ErrReceiptNotFound     = errors.New("receipt not found")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrTransferNotFound    = errors.New("transfer not found")
	ErrWhtTypeNotFound     = errors.New("withholding tax type not found") // detailed
)

// 409
var (
	ErrCodeExists     = errors.New("code already exists")
	ErrDocumentNoUsed = errors.New("document number is already used (check the document number format)")
)

// 422
var (
	ErrCompanyInactive    = errors.New("company is inactive (read-only)")
	ErrAccountInvalid     = errors.New("GL account cannot be used for posting (not found, header or inactive)") // detailed
	ErrJournalInvalid     = errors.New("journal cannot be used (not found or inactive)")
	ErrSettingsMissing    = errors.New("finance settings are incomplete") // detailed
	ErrInactiveRef        = errors.New("referenced record is inactive")   // detailed
	ErrAlreadyCancelled   = errors.New("document is already cancelled")
	ErrNoFiscalPeriod     = errors.New("no fiscal period covers the date")
	ErrPeriodClosed       = errors.New("fiscal period is closed")
	ErrPayeeTaxIDRequired = errors.New("withholding tax requires the payee tax id and tax type")
)

var ErrForbidden = errors.New("permission denied")
