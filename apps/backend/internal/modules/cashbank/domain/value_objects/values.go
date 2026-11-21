// Package valueobjects คือ value object ของ cashbank
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/cashbank/domain/errors"
)

// CompanyID / UserID เป็น id ของ organization / identity; GLAccountID เป็นบัญชีของ ledger
type (
	CompanyID     uuid.UUID
	UserID        uuid.UUID
	CashAccountID uuid.UUID
	GLAccountID   uuid.UUID
)

func NewID[T ~[16]byte]() T { return T(uuid.Must(uuid.NewV7())) }

func ParseID[T ~[16]byte](s string) (T, error) {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return T{}, domain.ErrInvalidID
	}
	return T(id), nil
}

func (id CashAccountID) String() string { return uuid.UUID(id).String() }
func (id GLAccountID) String() string   { return uuid.UUID(id).String() }

var (
	codePattern      = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._/-]{0,29}$`)
	accountNoPattern = regexp.MustCompile(`^[0-9][0-9-]{0,29}$`)
)

func NewCode(raw string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if !codePattern.MatchString(c) {
		return "", domain.ErrInvalidCode
	}
	return c, nil
}

func NewName(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || utf8.RuneCountInString(s) > max {
		return "", domain.ErrInvalidName
	}
	return s, nil
}

func OptionalText(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > max {
		return "", domain.ErrInvalidText
	}
	return s, nil
}

func AccountNo(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s != "" && !accountNoPattern.MatchString(s) {
		return "", domain.ErrInvalidAccountNo
	}
	return s, nil
}

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

var Statuses = []Status{StatusActive, StatusInactive}

func ParseStatus(raw string) (Status, error) {
	return parseEnum(raw, Statuses, domain.ErrInvalidStatus)
}

// AccountType: cash = เงินสด / เงินสดย่อย, bank = บัญชีเงินฝากธนาคาร
type AccountType string

const (
	TypeCash AccountType = "cash"
	TypeBank AccountType = "bank"
)

var AccountTypes = []AccountType{TypeCash, TypeBank}

func ParseAccountType(raw string) (AccountType, error) {
	return parseEnum(raw, AccountTypes, domain.ErrInvalidType)
}

func parseEnum[T ~string](raw string, allowed []T, invalid error) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", invalid
}

// Permission ของ cashbank
type Permission string

const (
	PermAccountRead   Permission = "cashbank.account:read"
	PermAccountManage Permission = "cashbank.account:manage"
)

func (p Permission) String() string { return string(p) }
