// Package domainerrors เก็บ error ของ ledger ไว้เป็น package ล่างสุด
// เพื่อให้ entities / value_objects / services import ได้โดยไม่เกิด import cycle
//
// ทุกที่ import ด้วย alias "domain" เช่น domain.ErrAccountNotFound
// error ที่ระบุว่า "detailed" ถูก wrap ด้วยรายละเอียดที่ปลอดภัยต่อการแสดง (เช่นบรรทัดที่ผิด)
package domainerrors

import "errors"

// รูปแบบข้อมูลไม่ถูกต้อง (400)
var (
	ErrInvalidID           = errors.New("invalid id")
	ErrInvalidCode         = errors.New("invalid code")
	ErrInvalidName         = errors.New("invalid name")
	ErrInvalidText         = errors.New("text is too long")
	ErrInvalidStatus       = errors.New("invalid status")
	ErrInvalidBusinessType = errors.New("invalid business type")
	ErrInvalidAccountType  = errors.New("invalid account type")
	ErrInvalidAmount       = errors.New("invalid amount") // detailed
	ErrInvalidLine         = errors.New("invalid line")   // detailed
	ErrInvalidDate         = errors.New("invalid date")
	ErrInvalidNumbering    = errors.New("invalid document number format") // detailed
)

// ไม่พบ (404)
var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrJournalNotFound   = errors.New("journal not found")
	ErrEntryNotFound     = errors.New("journal entry not found")
	ErrDimensionNotFound = errors.New("dimension not found") // detailed
)

// ขัดแย้งกับข้อมูลที่มีอยู่ (409)
var (
	ErrCodeExists  = errors.New("code already exists")
	ErrEntryNoUsed = errors.New("entry number is already used (check the journal number format)")
	ErrInUse       = errors.New("record is referenced by other records")
	ErrChartExists = errors.New("chart of accounts already has accounts")
)

// ผิดกฎธุรกิจ (422)
var (
	ErrCompanyInactive    = errors.New("company is inactive (read-only)")
	ErrHierarchyCycle     = errors.New("parent would create a cycle")
	ErrParentNotHeader    = errors.New("parent account must be a header (non-postable) account")
	ErrParentTypeMismatch = errors.New("parent account must have the same type")
	ErrAccountHasChildren = errors.New("account with sub-accounts must stay a header of the same type")
	ErrAccountNotPostable = errors.New("account is a header account or inactive") // detailed
	ErrJournalInactive    = errors.New("journal is inactive")
	ErrEntryNotDraft      = errors.New("only draft entries can be changed")
	ErrEntryNotPosted     = errors.New("only posted entries can be reversed")
	ErrEntryIncomplete    = errors.New("entry needs at least two lines and a non-zero total")
	ErrUnbalanced         = errors.New("total debit must equal total credit")
	ErrNoFiscalPeriod     = errors.New("no fiscal period covers the date")
	ErrRangeCrossesYear   = errors.New("start and end dates must be in the same fiscal year")
	ErrPeriodClosed       = errors.New("fiscal period is closed")
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
	ErrInvalidAccountType,
	ErrInvalidAmount,
	ErrInvalidLine,
	ErrInvalidDate,
	ErrAccountNotFound,
	ErrJournalNotFound,
	ErrEntryNotFound,
	ErrInvalidNumbering,
	ErrEntryNoUsed,
	ErrDimensionNotFound,
	ErrCodeExists,
	ErrInUse,
	ErrChartExists,
	ErrCompanyInactive,
	ErrHierarchyCycle,
	ErrParentNotHeader,
	ErrParentTypeMismatch,
	ErrAccountHasChildren,
	ErrAccountNotPostable,
	ErrJournalInactive,
	ErrEntryNotDraft,
	ErrEntryNotPosted,
	ErrEntryIncomplete,
	ErrUnbalanced,
	ErrNoFiscalPeriod,
	ErrRangeCrossesYear,
	ErrPeriodClosed,
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
