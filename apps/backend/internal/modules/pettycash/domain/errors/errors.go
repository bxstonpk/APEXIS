// Package domainerrors เก็บ error ของ pettycash ไว้เป็น package ล่างสุด
// ทุกที่ import ด้วย alias "domain" — error ที่ระบุว่า "detailed" ถูก wrap ด้วยรายละเอียดที่ปลอดภัยต่อการแสดง
package domainerrors

import "errors"

// รูปแบบข้อมูลไม่ถูกต้อง (400)
var (
	ErrInvalidID      = errors.New("invalid id")
	ErrInvalidCode    = errors.New("invalid code")
	ErrInvalidName    = errors.New("invalid name")
	ErrInvalidText    = errors.New("text is too long")
	ErrInvalidStatus  = errors.New("invalid status")
	ErrInvalidKind    = errors.New("invalid kind")
	ErrInvalidDate    = errors.New("invalid date")
	ErrInvalidAmount  = errors.New("invalid amount")   // detailed
	ErrInvalidLine    = errors.New("invalid line")     // detailed
	ErrInvalidVatRate = errors.New("invalid vat rate") // detailed
)

// ไม่พบ (404)
var (
	ErrFundNotFound          = errors.New("petty cash fund not found")
	ErrVoucherNotFound       = errors.New("petty cash voucher not found") // detailed
	ErrReplenishmentNotFound = errors.New("replenishment request not found")
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists     = errors.New("code already exists")
	ErrDocumentNoUsed = errors.New("document number already used")
	ErrFundInUse      = errors.New("fund has documents")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive      = errors.New("company is inactive")
	ErrFundInactive         = errors.New("petty cash fund is inactive")
	ErrCustodianNotMember   = errors.New("custodian must be an active member of the company")
	ErrTaxInvoiceRequired   = errors.New("a tax invoice number and date are required when the voucher has VAT")
	ErrVoucherNotDraft      = errors.New("voucher is not a draft")
	ErrVoucherNotApproved   = errors.New("voucher is not approved")                             // detailed
	ErrVoucherInRequest     = errors.New("voucher is already in an open replenishment request") // detailed
	ErrVoucherOtherFund     = errors.New("voucher belongs to a different fund")                 // detailed
	ErrVoucherReplenished   = errors.New("replenished vouchers cannot be cancelled")
	ErrAlreadyCancelled     = errors.New("document is already cancelled")
	ErrInsufficientCash     = errors.New("petty cash on hand is not enough for this voucher") // detailed
	ErrFundAlreadyFunded    = errors.New("the fund is already established")
	ErrFundNotFunded        = errors.New("establish the fund before this request")
	ErrRequestNotRequested  = errors.New("replenishment request is not open")
	ErrRequestPaid          = errors.New("paid replenishment requests cannot be cancelled")
	ErrOpenRequestExists    = errors.New("the fund already has an open establish / increase request")
	ErrNoFiscalPeriod       = errors.New("no fiscal period covers the date")
	ErrSettingsMissing      = errors.New("petty cash settings are incomplete")       // detailed
	ErrAccountInvalid       = errors.New("account is not a postable active account") // detailed
	ErrDateBeforeVouchers   = errors.New("request date is before a voucher date")
	ErrPaymentBeforeRequest = errors.New("payment date is before the request date")
)

var ErrForbidden = errors.New("permission denied")

var all = []error{
	ErrInvalidID, ErrInvalidCode, ErrInvalidName, ErrInvalidText, ErrInvalidStatus, ErrInvalidKind, ErrInvalidDate,
	ErrInvalidAmount, ErrInvalidLine, ErrInvalidVatRate,
	ErrFundNotFound, ErrVoucherNotFound, ErrReplenishmentNotFound,
	ErrCodeExists, ErrDocumentNoUsed, ErrFundInUse,
	ErrCompanyInactive, ErrFundInactive, ErrCustodianNotMember, ErrTaxInvoiceRequired, ErrVoucherNotDraft,
	ErrVoucherNotApproved, ErrVoucherInRequest, ErrVoucherOtherFund, ErrVoucherReplenished, ErrAlreadyCancelled,
	ErrInsufficientCash, ErrFundAlreadyFunded, ErrFundNotFunded, ErrRequestNotRequested, ErrRequestPaid,
	ErrOpenRequestExists, ErrNoFiscalPeriod, ErrSettingsMissing, ErrAccountInvalid, ErrDateBeforeVouchers,
	ErrPaymentBeforeRequest, ErrForbidden,
}

// IsBusiness: err เป็นการปฏิเสธด้วยกฎธุรกิจของ module นี้ (public API ใช้แปลงเป็น integration.ErrRejected)
func IsBusiness(err error) bool {
	for _, e := range all {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
