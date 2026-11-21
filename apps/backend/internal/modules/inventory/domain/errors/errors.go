// Package domainerrors เก็บ error ของ inventory ไว้เป็น package ล่างสุด
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
	ErrInvalidItemType = errors.New("invalid item type")
	ErrInvalidDocType  = errors.New("invalid document type")
	ErrInvalidQuantity = errors.New("invalid quantity") // detailed
	ErrInvalidLine     = errors.New("invalid line")     // detailed
	ErrInvalidDate     = errors.New("invalid date")
	ErrInvalidSystem   = errors.New("accounting system must be perpetual or periodic")
	ErrInvalidYear     = errors.New("invalid fiscal year")
)

// ไม่พบ (404)
var (
	ErrUomNotFound       = errors.New("unit of measure not found")
	ErrCategoryNotFound  = errors.New("item category not found")
	ErrItemNotFound      = errors.New("item not found") // detailed
	ErrWarehouseNotFound = errors.New("warehouse not found")
	ErrDocumentNotFound  = errors.New("stock document not found")
	ErrBranchNotFound    = errors.New("branch not found")
	ErrClosingNotFound   = errors.New("inventory period closing not found")
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists    = errors.New("code already exists")
	ErrInUse         = errors.New("record is referenced by other records")
	ErrClosingExists = errors.New("inventory of this fiscal period is already closed (cancel the existing closing first)")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive   = errors.New("company is inactive (read-only)")
	ErrItemNotStocked    = errors.New("item is not a stock item or is inactive") // detailed
	ErrWarehouseInactive = errors.New("warehouse is inactive")
	ErrSameWarehouse     = errors.New("transfer requires a different destination warehouse")
	ErrDocumentNotDraft  = errors.New("only draft documents can be changed")
	ErrDocumentNotPosted = errors.New("only posted documents can be cancelled")
	ErrInsufficientStock = errors.New("insufficient stock")                                    // detailed
	ErrStockAlreadyUsed  = errors.New("stock received by this document has already been used") // detailed
	ErrNoFiscalPeriod    = errors.New("no fiscal period covers the date")
	ErrPeriodClosed      = errors.New("fiscal period is closed")
	ErrItemHasStock      = errors.New("item type or unit cannot change after stock movements")
	ErrAccountInvalid    = errors.New("account cannot be used for posting (not found, header or inactive)") // detailed
	ErrJournalInvalid    = errors.New("journal not found or inactive")
	ErrSystemLocked      = errors.New("accounting system cannot change for a fiscal year that already has posted stock documents")
	ErrNotPeriodic       = errors.New("period-end inventory adjustment is only for the periodic system")
	ErrSettingsMissing   = errors.New("inventory settings are incomplete") // detailed
	ErrInventoryClosed   = errors.New("inventory of the period is already closed (cancel the period-end adjustment first)")
)

// สิทธิ์ (403)
var ErrForbidden = errors.New("permission denied")

// all คือ error ทั้งหมดของ module นี้ (ล้วนเป็นกฎธุรกิจ / ข้อมูลไม่ถูกต้อง ไม่ใช่ปัญหาของระบบ)
var all = []error{
	ErrInvalidID,
	ErrInvalidCode,
	ErrInvalidName,
	ErrInvalidText,
	ErrInvalidStatus,
	ErrInvalidItemType,
	ErrInvalidDocType,
	ErrInvalidQuantity,
	ErrInvalidLine,
	ErrInvalidDate,
	ErrInvalidSystem,
	ErrInvalidYear,
	ErrUomNotFound,
	ErrCategoryNotFound,
	ErrItemNotFound,
	ErrWarehouseNotFound,
	ErrDocumentNotFound,
	ErrBranchNotFound,
	ErrClosingNotFound,
	ErrCodeExists,
	ErrInUse,
	ErrClosingExists,
	ErrCompanyInactive,
	ErrItemNotStocked,
	ErrWarehouseInactive,
	ErrSameWarehouse,
	ErrDocumentNotDraft,
	ErrDocumentNotPosted,
	ErrInsufficientStock,
	ErrStockAlreadyUsed,
	ErrNoFiscalPeriod,
	ErrPeriodClosed,
	ErrItemHasStock,
	ErrAccountInvalid,
	ErrJournalInvalid,
	ErrSystemLocked,
	ErrNotPeriodic,
	ErrSettingsMissing,
	ErrInventoryClosed,
	ErrForbidden,
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
