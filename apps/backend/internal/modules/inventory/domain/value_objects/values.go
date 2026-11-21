package valueobjects

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
	"apexis/backend/internal/platform/numeric"
)

var codePattern = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._/-]{0,29}$`)

// NewCode: รหัสสินค้า / คลัง / หน่วยนับ / หมวด (ตัวพิมพ์ใหญ่ ตัวเลข . _ / - ไม่เกิน 30 ตัว)
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

// Quantity / Money เป็น decimal ฐาน 10 — จำนวนทศนิยมที่ยอมรับมาจากค่าตั้งของบริษัท (ไม่ fix ไว้)
type (
	Quantity = decimal.Decimal
	Money    = decimal.Decimal
)

// ParseDecimal แปลงข้อความเป็น decimal (ไม่เกิน 18 หลัก + ทศนิยม 6 ตำแหน่ง) — invalid = error ที่ระบุ field
func ParseDecimal(field, raw string) (decimal.Decimal, error) {
	d, ok := numeric.Parse(strings.TrimSpace(raw))
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: %s %q must be a decimal number", domain.ErrInvalidQuantity, field, raw)
	}
	return d, nil
}

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

// ItemType: stock = นับสต็อก / คิดต้นทุน FIFO; non_stock = ซื้อขายได้แต่ไม่นับสต็อก; service = บริการ
type ItemType string

const (
	ItemStock    ItemType = "stock"
	ItemNonStock ItemType = "non_stock"
	ItemService  ItemType = "service"
)

var ItemTypes = []ItemType{ItemStock, ItemNonStock, ItemService}

func ParseItemType(raw string) (ItemType, error) {
	return parseEnum(raw, ItemTypes, domain.ErrInvalidItemType)
}

// AccountingSystem คือระบบบัญชีสินค้า (ตั้งต่อปีบัญชี)
type AccountingSystem string

const (
	Perpetual AccountingSystem = "perpetual" // บันทึกสินค้าคงเหลือ / ต้นทุนขายทันที
	Periodic  AccountingSystem = "periodic"  // ลงบัญชีซื้อระหว่างงวด ปรับปรุงสินค้าคงเหลือสิ้นงวด
)

var AccountingSystems = []AccountingSystem{Perpetual, Periodic}

func ParseAccountingSystem(raw string) (AccountingSystem, error) {
	return parseEnum(raw, AccountingSystems, domain.ErrInvalidSystem)
}

// GLAccounts คือบัญชีแยกประเภท (ของ ledger) ที่ผูกกับสินค้า / หมวด / ค่าเริ่มต้นของบริษัท — nil = ไม่ได้ตั้ง
type GLAccounts struct {
	Purchase  *AccountID // บัญชีซื้อ (periodic, สินค้าไม่นับสต็อก / บริการ)
	Inventory *AccountID // บัญชีสินค้าคงเหลือ (perpetual)
}

// IDs คืน id ที่ตั้งไว้ (ไม่ซ้ำ)
func (g GLAccounts) IDs() []AccountID {
	var out []AccountID
	for _, p := range []*AccountID{g.Purchase, g.Inventory} {
		if p != nil && !slices.Contains(out, *p) {
			out = append(out, *p)
		}
	}
	return out
}

// Or: ใช้ค่าของ g ถ้าตั้งไว้ ไม่งั้นใช้ของ fallback (สินค้า -> หมวด -> ค่าเริ่มต้นของบริษัท)
func (g GLAccounts) Or(fallback GLAccounts) GLAccounts {
	out := g
	if out.Purchase == nil {
		out.Purchase = fallback.Purchase
	}
	if out.Inventory == nil {
		out.Inventory = fallback.Inventory
	}
	return out
}

// DocumentType ของเอกสารเคลื่อนไหวสต็อก
type DocumentType string

const (
	DocReceipt    DocumentType = "receipt"    // รับเข้า (มีต้นทุน)
	DocIssue      DocumentType = "issue"      // เบิกออก (ต้นทุน FIFO)
	DocTransfer   DocumentType = "transfer"   // โอนระหว่างคลัง (ต้นทุนตามชั้นที่ตัดออก)
	DocAdjustment DocumentType = "adjustment" // ปรับปรุงยอด: จำนวนบวก = เพิ่ม (มีต้นทุน), ลบ = ลด (FIFO)
)

var DocumentTypes = []DocumentType{DocReceipt, DocIssue, DocTransfer, DocAdjustment}

func ParseDocumentType(raw string) (DocumentType, error) {
	return parseEnum(raw, DocumentTypes, domain.ErrInvalidDocType)
}

// Prefix นำหน้าเลขที่เอกสาร เช่น GR2026-000001
func (t DocumentType) Prefix() string {
	switch t {
	case DocReceipt:
		return "GR"
	case DocIssue:
		return "GI"
	case DocTransfer:
		return "TR"
	default:
		return "AJ"
	}
}

// DocumentStatus: draft -> posted -> cancelled
type DocumentStatus string

const (
	DocDraft     DocumentStatus = "draft"
	DocPosted    DocumentStatus = "posted"
	DocCancelled DocumentStatus = "cancelled"
)

var DocumentStatuses = []DocumentStatus{DocDraft, DocPosted, DocCancelled}

func ParseDocumentStatus(raw string) (DocumentStatus, error) {
	return parseEnum(raw, DocumentStatuses, domain.ErrInvalidStatus)
}

func parseEnum[T ~string](raw string, allowed []T, invalid error) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", invalid
}
