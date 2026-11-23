// Package errors คือ error ของ taxregistry (code = ข้อความแทนช่องว่างด้วย _)
package errors

import "errors"

var (
	ErrInvalidTaxID    = errors.New("invalid tax id")
	ErrInvalidBranchNo = errors.New("invalid branch no")
	ErrNotFound        = errors.New("vat registrant not found")
	ErrUnavailable     = errors.New("vat registry unavailable")
	ErrForbidden       = errors.New("permission denied")
)
