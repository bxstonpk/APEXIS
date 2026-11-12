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
)

// ไม่พบ (404)
var (
	ErrUomNotFound       = errors.New("unit of measure not found")
	ErrCategoryNotFound  = errors.New("item category not found")
	ErrItemNotFound      = errors.New("item not found") // detailed
	ErrWarehouseNotFound = errors.New("warehouse not found")
	ErrDocumentNotFound  = errors.New("stock document not found")
	ErrBranchNotFound    = errors.New("branch not found")
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists = errors.New("code already exists")
	ErrInUse      = errors.New("record is referenced by other records")
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
)

// สิทธิ์ (403)
var ErrForbidden = errors.New("permission denied")
