// Package valueobjects คือ value object ของ purchasing
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
)

// ---------- ID ----------
//
// CompanyID / UserID / FiscalPeriodID เป็น id ของ organization / identity
// ItemID / WarehouseID / StockDocumentID เป็น id ของ inventory; AccountID / JournalID / EntryID เป็นของ ledger
type (
	CompanyID       uuid.UUID
	UserID          uuid.UUID
	FiscalPeriodID  uuid.UUID
	SupplierID      uuid.UUID
	OrderID         uuid.UUID
	ReceiptID       uuid.UUID
	InvoiceID       uuid.UUID
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
func (id SupplierID) String() string  { return IDString(id) }
func (id OrderID) String() string     { return IDString(id) }
func (id ReceiptID) String() string   { return IDString(id) }
func (id InvoiceID) String() string   { return IDString(id) }
func (id ItemID) String() string      { return IDString(id) }
func (id WarehouseID) String() string { return IDString(id) }
func (id AccountID) String() string   { return IDString(id) }
func (id JournalID) String() string   { return IDString(id) }
func (id EntryID) String() string     { return IDString(id) }

// ---------- ข้อความ ----------

var (
	codePattern     = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._/-]{0,29}$`)
	taxIDPattern    = regexp.MustCompile(`^[0-9]{13}$`)
	branchNoPattern = regexp.MustCompile(`^[0-9]{5}$`)
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

// TaxID: เลขประจำตัวผู้เสียภาษี 13 หลัก (ว่างได้ เช่น ผู้ขายต่างประเทศ)
func TaxID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s != "" && !taxIDPattern.MatchString(s) {
		return "", domain.ErrInvalidTaxID
	}
	return s, nil
}

// BranchNo: เลขสาขาตามใบกำกับภาษี 5 หลัก — ว่าง = สำนักงานใหญ่ (00000)
func BranchNo(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "00000", nil
	}
	if !branchNoPattern.MatchString(s) {
		return "", domain.ErrInvalidBranchNo
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

// OrderStatus: draft -> approved -> closed (รับครบ + แจ้งหนี้ครบ หรือปิดเอง) / cancelled (ยังไม่มีการรับ)
type OrderStatus string

const (
	OrderDraft     OrderStatus = "draft"
	OrderApproved  OrderStatus = "approved"
	OrderClosed    OrderStatus = "closed"
	OrderCancelled OrderStatus = "cancelled"
)

var OrderStatuses = []OrderStatus{OrderDraft, OrderApproved, OrderClosed, OrderCancelled}

func ParseOrderStatus(raw string) (OrderStatus, error) {
	return parseEnum(raw, OrderStatuses, domain.ErrInvalidStatus)
}

// DocStatus ของใบรับสินค้า / ใบแจ้งหนี้: สร้างแล้วลงบัญชีทันที (posted) -> ยกเลิกด้วยการกลับรายการ (cancelled)
type DocStatus string

const (
	DocPosted    DocStatus = "posted"
	DocCancelled DocStatus = "cancelled"
)

var DocStatuses = []DocStatus{DocPosted, DocCancelled}

func ParseDocStatus(raw string) (DocStatus, error) {
	return parseEnum(raw, DocStatuses, domain.ErrInvalidStatus)
}

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
