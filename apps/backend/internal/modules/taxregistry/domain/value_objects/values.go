// Package value_objects คือค่าที่ตรวจแล้วของ taxregistry
package value_objects

import (
	"strings"

	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
)

type (
	UserID    uuid.UUID
	CompanyID uuid.UUID
)

func digits(s string, n int) bool {
	return len(s) == n && strings.Trim(s, "0123456789") == ""
}

// TaxID: เลขประจำตัวผู้เสียภาษี 13 หลัก
func TaxID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !digits(s, 13) {
		return "", domain.ErrInvalidTaxID
	}
	return s, nil
}

// BranchNo: เลขสาขา 5 หลัก — ว่าง = ทุกสาขา
func BranchNo(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s != "" && !digits(s, 5) {
		return "", domain.ErrInvalidBranchNo
	}
	return s, nil
}
