// Package domainerrors เก็บ error ของ cashbank — import ด้วย alias "domain"
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
)

// 404 / 409 / 422 / 403
var (
	ErrCashAccountNotFound = errors.New("cash / bank account not found")
	ErrCodeExists          = errors.New("code already exists")
	ErrCompanyInactive     = errors.New("company is inactive (read-only)")
	ErrAccountInvalid      = errors.New("GL account cannot be used for posting (not found, header or inactive)")
	ErrForbidden           = errors.New("permission denied")
)
