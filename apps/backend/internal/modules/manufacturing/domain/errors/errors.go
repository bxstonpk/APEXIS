// Package domainerrors เก็บ error ของ manufacturing ไว้เป็น package ล่างสุด
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
	ErrInvalidQuantity = errors.New("invalid quantity") // detailed
	ErrInvalidLine     = errors.New("invalid line")     // detailed
	ErrInvalidReason   = errors.New("reason is required (up to 500 characters)")
)

// ไม่พบ (404)
var (
	ErrBomNotFound       = errors.New("bill of materials not found")
	ErrOrderNotFound     = errors.New("production order not found")
	ErrItemNotFound      = errors.New("item not found") // detailed
	ErrWarehouseNotFound = errors.New("warehouse not found")
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists       = errors.New("code already exists")
	ErrDefaultBomExists = errors.New("the item already has a default active bill of materials")
	ErrInUse            = errors.New("record is referenced by production orders")
	ErrDocumentNoUsed   = errors.New("document number already used")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrNotManufacturing   = errors.New("the manufacturing app is available only to manufacturing companies")
	ErrCompanyInactive    = errors.New("company is inactive")
	ErrItemNotStock       = errors.New("item must be a stock item") // detailed
	ErrItemInactive       = errors.New("item is inactive")          // detailed
	ErrWarehouseInactive  = errors.New("warehouse is inactive")
	ErrBomInactive        = errors.New("bill of materials is inactive")
	ErrBomItemMismatch    = errors.New("bill of materials is for a different item")
	ErrOrderNotDraft      = errors.New("production order is not a draft")
	ErrOrderNotReleased   = errors.New("production order is not released")
	ErrAlreadyCancelled   = errors.New("production order is already cancelled")
	ErrMaterialNotOnOrder = errors.New("material is not on the production order") // detailed
	ErrOverReturn         = errors.New("return exceeds the net issued quantity")  // detailed
	ErrNoFiscalPeriod     = errors.New("no fiscal period covers the date")
	ErrPeriodClosed       = errors.New("fiscal period is closed")
	ErrSettingsMissing    = errors.New("manufacturing settings are incomplete") // detailed
	ErrAccountInvalid     = errors.New("account is not a postable active account")
	ErrJournalInvalid     = errors.New("journal is not active")
	ErrAccountMissing     = errors.New("item has no inventory account") // detailed
	ErrDateBeforeOrder    = errors.New("date is before the production order date")
)

var ErrForbidden = errors.New("permission denied")

var all = []error{
	ErrInvalidID, ErrInvalidCode, ErrInvalidName, ErrInvalidText, ErrInvalidStatus, ErrInvalidDate, ErrInvalidQuantity,
	ErrInvalidLine, ErrInvalidReason,
	ErrBomNotFound, ErrOrderNotFound, ErrItemNotFound, ErrWarehouseNotFound,
	ErrCodeExists, ErrDefaultBomExists, ErrInUse, ErrDocumentNoUsed,
	ErrNotManufacturing, ErrCompanyInactive, ErrItemNotStock, ErrItemInactive, ErrWarehouseInactive, ErrBomInactive,
	ErrBomItemMismatch, ErrOrderNotDraft, ErrOrderNotReleased, ErrAlreadyCancelled, ErrMaterialNotOnOrder, ErrOverReturn,
	ErrNoFiscalPeriod, ErrPeriodClosed, ErrSettingsMissing, ErrAccountInvalid, ErrJournalInvalid, ErrAccountMissing,
	ErrDateBeforeOrder,
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
