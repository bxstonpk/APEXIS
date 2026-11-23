// Package valueobjects คือ value object ของ sales
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
)

// ---------- ID ----------
//
// CompanyID / UserID / FiscalPeriodID ของ organization / identity; ItemID / WarehouseID / StockDocumentID ของ inventory;
// AccountID / JournalID / EntryID ของ ledger
type (
	CompanyID       uuid.UUID
	UserID          uuid.UUID
	FiscalPeriodID  uuid.UUID
	CustomerID      uuid.UUID
	QuotationID     uuid.UUID
	OrderID         uuid.UUID
	DeliveryID      uuid.UUID
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

func (id CustomerID) String() string  { return IDString(id) }
func (id QuotationID) String() string { return IDString(id) }
func (id OrderID) String() string     { return IDString(id) }
func (id DeliveryID) String() string  { return IDString(id) }
func (id InvoiceID) String() string   { return IDString(id) }
func (id ItemID) String() string      { return IDString(id) }
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

func TaxID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s != "" && !taxIDPattern.MatchString(s) {
		return "", domain.ErrInvalidTaxID
	}
	return s, nil
}

// BranchNo: ว่าง = สำนักงานใหญ่ (00000)
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

func IsDateOnly(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t.Equal(t.Truncate(24*time.Hour))
}

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

func ParseStatus(raw string) (Status, error) {
	return parseEnum(raw, []Status{StatusActive, StatusInactive}, domain.ErrInvalidStatus)
}

// TaxType ของลูกค้า: individual = บุคคลธรรมดา (ภ.ง.ด.3), juristic = นิติบุคคล (ภ.ง.ด.53)
type TaxType string

const (
	TaxIndividual TaxType = "individual"
	TaxJuristic   TaxType = "juristic"
)

func ParseTaxType(raw string) (TaxType, error) {
	return parseEnum(raw, []TaxType{TaxIndividual, TaxJuristic}, domain.ErrInvalidTaxType)
}

// QuotationStatus: draft -> sent -> accepted / rejected (แปลงเป็นใบสั่งขาย = accepted)
type QuotationStatus string

const (
	QuotationDraft    QuotationStatus = "draft"
	QuotationSent     QuotationStatus = "sent"
	QuotationAccepted QuotationStatus = "accepted"
	QuotationRejected QuotationStatus = "rejected"
)

var QuotationStatuses = []QuotationStatus{QuotationDraft, QuotationSent, QuotationAccepted, QuotationRejected}

func ParseQuotationStatus(raw string) (QuotationStatus, error) {
	return parseEnum(raw, QuotationStatuses, domain.ErrInvalidStatus)
}

// OrderStatus: draft -> approved -> closed (ส่งครบ + ออกบิลครบ หรือปิดเอง) / cancelled
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

// DocStatus ของใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ: posted -> cancelled
type DocStatus string

const (
	DocPosted    DocStatus = "posted"
	DocCancelled DocStatus = "cancelled"
)

var DocStatuses = []DocStatus{DocPosted, DocCancelled}

func ParseDocStatus(raw string) (DocStatus, error) {
	return parseEnum(raw, DocStatuses, domain.ErrInvalidStatus)
}

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

// ---------- ชนิดเอกสาร (ตั้งรูปแบบเลขที่ที่ organization) ----------

type DocType string

const (
	DocQuotation DocType = "sales.quotation"
	DocOrder     DocType = "sales.order"
	DocDelivery  DocType = "sales.delivery"
	DocInvoice   DocType = "sales.invoice" // ใบแจ้งหนี้ / ใบกำกับภาษี (รวมแบบส่งของพร้อมออกบิล)
)

var DocTypes = []DocType{DocQuotation, DocOrder, DocDelivery, DocInvoice}

func (t DocType) Prefix() string {
	return map[DocType]string{DocQuotation: "QT", DocOrder: "SO", DocDelivery: "DO", DocInvoice: "IV"}[t]
}

func (t DocType) Name() string {
	return map[DocType]string{
		DocQuotation: "ใบเสนอราคา", DocOrder: "ใบสั่งขาย", DocDelivery: "ใบส่งของ",
		DocInvoice: "ใบแจ้งหนี้ / ใบกำกับภาษี",
	}[t]
}

// ---------- สิทธิ์ ----------

type Permission string

const (
	PermSettingsRead    Permission = "sales.settings:read"
	PermSettingsManage  Permission = "sales.settings:manage"
	PermCustomerRead    Permission = "sales.customer:read"
	PermCustomerManage  Permission = "sales.customer:manage"
	PermQuotationRead   Permission = "sales.quotation:read"
	PermQuotationManage Permission = "sales.quotation:manage"
	PermOrderRead       Permission = "sales.order:read"
	PermOrderManage     Permission = "sales.order:manage"
	PermOrderApprove    Permission = "sales.order:approve"
	PermDeliveryRead    Permission = "sales.delivery:read"
	PermDeliveryPost    Permission = "sales.delivery:post"
	PermInvoiceRead     Permission = "sales.invoice:read"
	PermInvoicePost     Permission = "sales.invoice:post"
	PermReportRead      Permission = "sales.report:read"
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ sales (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermSettingsRead,
	PermSettingsManage,
	PermCustomerRead,
	PermCustomerManage,
	PermQuotationRead,
	PermQuotationManage,
	PermOrderRead,
	PermOrderManage,
	PermOrderApprove,
	PermDeliveryRead,
	PermDeliveryPost,
	PermInvoiceRead,
	PermInvoicePost,
	PermReportRead,
}
