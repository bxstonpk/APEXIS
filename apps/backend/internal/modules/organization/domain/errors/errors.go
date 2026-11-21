// Package domainerrors เก็บ error ของ organization ไว้เป็น package ล่างสุด
// เพื่อให้ entities / value_objects / services import ได้โดยไม่เกิด import cycle
//
// ทุกที่ import ด้วย alias "domain" เช่น domain.ErrBranchNotFound
package domainerrors

import "errors"

// รูปแบบข้อมูลไม่ถูกต้อง (400)
var (
	ErrInvalidID          = errors.New("invalid id")
	ErrInvalidCode        = errors.New("invalid code")
	ErrInvalidName        = errors.New("invalid name")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrInvalidAddress     = errors.New("invalid address")
	ErrInvalidPostalCode  = errors.New("invalid postal code")
	ErrInvalidCountry     = errors.New("invalid country code")
	ErrInvalidTaxBranchNo = errors.New("invalid tax branch number")
	ErrInvalidTaxID       = errors.New("invalid tax id")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPhone       = errors.New("invalid phone")
	ErrInvalidWebsite     = errors.New("invalid website")
	ErrInvalidCurrency    = errors.New("invalid currency code")
	ErrInvalidFiscalMonth = errors.New("fiscal year start month must be between 1 and 12")
	ErrInvalidDecimals    = errors.New("decimal places must be between 0 and 6")
	ErrInvalidDateRange   = errors.New("end date must not be before start date")
	ErrInvalidFiscalYear  = errors.New("invalid fiscal year")
	ErrInvalidFiscalStart = errors.New("fiscal year must start on the first day of a month")
	ErrInvalidNumbering   = errors.New("invalid document number format") // detailed
)

// ไม่พบ (404)
var (
	ErrCompanyNotFound      = errors.New("company not found")
	ErrBranchNotFound       = errors.New("branch not found")
	ErrBusinessUnitNotFound = errors.New("business unit not found")
	ErrDepartmentNotFound   = errors.New("department not found")
	ErrCostCenterNotFound   = errors.New("cost center not found")
	ErrProjectSiteNotFound  = errors.New("project site not found")
	ErrFiscalYearNotFound   = errors.New("fiscal year not found")
	ErrFiscalPeriodNotFound = errors.New("fiscal period not found")
	ErrDocumentTypeNotFound = errors.New("document type not found")
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists         = errors.New("code already exists")
	ErrBranchExists       = errors.New("branch code or tax branch number already exists")
	ErrInUse              = errors.New("record is referenced by other records")
	ErrFiscalYearExists   = errors.New("fiscal year already exists")
	ErrFiscalYearOverlaps = errors.New("fiscal year overlaps an existing fiscal year")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive        = errors.New("company is inactive (read-only)")
	ErrDecimalsDecrease       = errors.New("decimal places cannot be decreased")
	ErrHierarchyCycle         = errors.New("parent would create a cycle")
	ErrHeadOfficeImmutable    = errors.New("head office cannot be deactivated or deleted")
	ErrProjectEndDateRequired = errors.New("closed project site requires an end date")
	ErrPeriodCloseOrder       = errors.New("earlier periods must be closed first")
	ErrPeriodReopenOrder      = errors.New("later periods must be reopened first")
)

// สิทธิ์ (403)
var ErrForbidden = errors.New("permission denied")
