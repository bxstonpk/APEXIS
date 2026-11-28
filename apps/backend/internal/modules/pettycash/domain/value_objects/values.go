// Package valueobjects คือ value object ของ pettycash (เงินสดย่อย)
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	"apexis/backend/internal/platform/numeric"
)

// ---------- ID ----------
//
// CompanyID / UserID / FiscalPeriodID เป็น id ของ organization / identity; AccountID เป็นของ ledger
type (
	CompanyID       uuid.UUID
	UserID          uuid.UUID
	FundID          uuid.UUID
	VoucherID       uuid.UUID
	ReplenishmentID uuid.UUID
	AccountID       uuid.UUID
)

type anyID interface{ ~[16]byte }

func NewID[T anyID]() T { return T(uuid.Must(uuid.NewV7())) }

func ParseID[T anyID](s string) (T, error) {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return T{}, domain.ErrInvalidID
	}
	return T(id), nil
}

func IDString[T anyID](id T) string { return uuid.UUID(id).String() }

func (id CompanyID) String() string       { return IDString(id) }
func (id UserID) String() string          { return IDString(id) }
func (id FundID) String() string          { return IDString(id) }
func (id VoucherID) String() string       { return IDString(id) }
func (id ReplenishmentID) String() string { return IDString(id) }
func (id AccountID) String() string       { return IDString(id) }

// ---------- ข้อความ ----------

var codePattern = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._/-]{0,19}$`)

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

// IsDateOnly: วันที่ใน domain ต้องเป็นเที่ยงคืน UTC
func IsDateOnly(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t.Equal(t.Truncate(24*time.Hour))
}

// ---------- ตัวเลข ----------

type Money = decimal.Decimal

// FitsCompany: อยู่ในช่วง NUMERIC(24,6) และทศนิยมไม่เกินที่บริษัทกำหนด
func FitsCompany(v decimal.Decimal, decimals int) bool {
	return numeric.InRange(v) && numeric.FitsScale(v, decimals)
}

// ---------- enum ----------

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

var Statuses = []Status{StatusActive, StatusInactive}

func ParseStatus(raw string) (Status, error) {
	return parseEnum(raw, Statuses, domain.ErrInvalidStatus)
}

// VoucherStatus: draft -> approved -> replenished; draft / approved -> cancelled
type VoucherStatus string

const (
	VoucherDraft       VoucherStatus = "draft"
	VoucherApproved    VoucherStatus = "approved"
	VoucherReplenished VoucherStatus = "replenished"
	VoucherCancelled   VoucherStatus = "cancelled"
)

var VoucherStatuses = []VoucherStatus{VoucherDraft, VoucherApproved, VoucherReplenished, VoucherCancelled}

func ParseVoucherStatus(raw string) (VoucherStatus, error) {
	return parseEnum(raw, VoucherStatuses, domain.ErrInvalidStatus)
}

// RequestKind: establish = ตั้งวงเงิน, increase = เพิ่มวงเงิน, replenish = เบิกชดเชยตามใบสำคัญจ่าย
type RequestKind string

const (
	RequestEstablish RequestKind = "establish"
	RequestIncrease  RequestKind = "increase"
	RequestReplenish RequestKind = "replenish"
)

var RequestKinds = []RequestKind{RequestEstablish, RequestIncrease, RequestReplenish}

func ParseRequestKind(raw string) (RequestKind, error) {
	return parseEnum(raw, RequestKinds, domain.ErrInvalidKind)
}

// RequestStatus: requested -> paid (การเงินจ่ายแล้ว); requested -> cancelled; paid -> requested (การเงินยกเลิกใบจ่าย)
type RequestStatus string

const (
	RequestRequested RequestStatus = "requested"
	RequestPaid      RequestStatus = "paid"
	RequestCancelled RequestStatus = "cancelled"
)

var RequestStatuses = []RequestStatus{RequestRequested, RequestPaid, RequestCancelled}

func ParseRequestStatus(raw string) (RequestStatus, error) {
	return parseEnum(raw, RequestStatuses, domain.ErrInvalidStatus)
}

func parseEnum[T ~string](raw string, allowed []T, invalid error) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", invalid
}

// ---------- ชนิดเอกสาร (เลขที่ตั้งรูปแบบได้) ----------

type DocType string

const (
	DocVoucher       DocType = "petty_cash.voucher"
	DocReplenishment DocType = "petty_cash.replenishment"
)

var DocTypes = []DocType{DocVoucher, DocReplenishment}

// Prefix คือ prefix เริ่มต้นของเลขที่ (บริษัทเปลี่ยนได้)
func (t DocType) Prefix() string {
	if t == DocReplenishment {
		return "PCR"
	}
	return "PCV"
}

func (t DocType) Name() string {
	if t == DocReplenishment {
		return "ใบขอเบิกชดเชยเงินสดย่อย"
	}
	return "ใบสำคัญจ่ายเงินสดย่อย"
}
