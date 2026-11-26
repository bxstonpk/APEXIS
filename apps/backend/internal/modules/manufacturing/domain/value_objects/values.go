// Package valueobjects คือ value object ของ manufacturing (ผลิต)
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	"apexis/backend/internal/platform/numeric"
)

// ---------- ID ----------
//
// CompanyID / UserID / FiscalPeriodID เป็น id ของ organization / identity
// ItemID / WarehouseID / StockDocumentID เป็น id ของ inventory; AccountID / JournalID / EntryID เป็นของ ledger
type (
	CompanyID       uuid.UUID
	UserID          uuid.UUID
	FiscalPeriodID  uuid.UUID
	BomID           uuid.UUID
	OrderID         uuid.UUID
	MovementID      uuid.UUID
	ItemID          uuid.UUID
	WarehouseID     uuid.UUID
	StockDocumentID uuid.UUID
	AccountID       uuid.UUID
	JournalID       uuid.UUID
	EntryID         uuid.UUID
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

func (id CompanyID) String() string   { return IDString(id) }
func (id UserID) String() string      { return IDString(id) }
func (id BomID) String() string       { return IDString(id) }
func (id OrderID) String() string     { return IDString(id) }
func (id MovementID) String() string  { return IDString(id) }
func (id ItemID) String() string      { return IDString(id) }
func (id WarehouseID) String() string { return IDString(id) }
func (id AccountID) String() string   { return IDString(id) }
func (id JournalID) String() string   { return IDString(id) }
func (id EntryID) String() string     { return IDString(id) }

// ---------- ข้อความ ----------

var codePattern = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._/-]{0,29}$`)

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

type (
	Quantity = decimal.Decimal
	Money    = decimal.Decimal
)

func MustDecimal(s string) decimal.Decimal { return decimal.RequireFromString(s) }

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

// OrderStatus: draft -> released -> completed; draft / released / completed -> cancelled
type OrderStatus string

const (
	OrderDraft     OrderStatus = "draft"
	OrderReleased  OrderStatus = "released"
	OrderCompleted OrderStatus = "completed"
	OrderCancelled OrderStatus = "cancelled"
)

var OrderStatuses = []OrderStatus{OrderDraft, OrderReleased, OrderCompleted, OrderCancelled}

func ParseOrderStatus(raw string) (OrderStatus, error) {
	return parseEnum(raw, OrderStatuses, domain.ErrInvalidStatus)
}

// MovementKind คือการเคลื่อนไหวสต็อกของใบสั่งผลิต:
// issue = เบิกวัตถุดิบ, return = คืนวัตถุดิบ, receipt = รับสินค้าสำเร็จรูปเข้าคลัง (ปิดงาน)
type MovementKind string

const (
	MoveIssue   MovementKind = "issue"
	MoveReturn  MovementKind = "return"
	MoveReceipt MovementKind = "receipt"
)

// AccountingSystem ของปีบัญชี (กำหนดใน inventory)
type AccountingSystem string

const (
	Perpetual AccountingSystem = "perpetual"
	Periodic  AccountingSystem = "periodic"
)

func parseEnum[T ~string](raw string, allowed []T, invalid error) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", invalid
}

// DocType คือชนิดเอกสารที่ลงทะเบียนกับ organization เพื่อตั้งรูปแบบเลขที่
type DocType string

const DocProductionOrder DocType = "manufacturing.production_order"

var DocTypes = []DocType{DocProductionOrder}

// Prefix คือ prefix เริ่มต้นของเลขที่ (บริษัทเปลี่ยนได้)
func (t DocType) Prefix() string { return "MO" }

func (t DocType) Name() string { return "ใบสั่งผลิต" }
