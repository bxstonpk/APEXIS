package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
)

var (
	// รหัสบัญชี เช่น 1101, 1101-01, 11.01.001
	accountCodePattern = regexp.MustCompile(`^[0-9A-Z][0-9A-Z._-]{0,19}$`)
	// รหัสสมุดรายวัน ใช้นำหน้าเลขที่เอกสาร เช่น JV -> JV2026-000001
	journalCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}$`)
)

func NewAccountCode(raw string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if !accountCodePattern.MatchString(c) {
		return "", domain.ErrInvalidCode
	}
	return c, nil
}

func NewJournalCode(raw string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if !journalCodePattern.MatchString(c) {
		return "", domain.ErrInvalidCode
	}
	return c, nil
}

// NewName: ต้องมีค่า และยาวไม่เกิน max ตัวอักษร
func NewName(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || utf8.RuneCountInString(s) > max {
		return "", domain.ErrInvalidName
	}
	return s, nil
}

// OptionalText: ว่างได้ ยาวไม่เกิน max ตัวอักษร
func OptionalText(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > max {
		return "", domain.ErrInvalidText
	}
	return s, nil
}

// IsDateOnly: วันที่ใน domain ต้องเป็นเที่ยงคืน UTC (presentation แปลงจาก "YYYY-MM-DD" ให้แล้ว)
func IsDateOnly(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t.Equal(t.Truncate(24*time.Hour))
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

// AccountType คือหมวดบัญชี 5 หมวด + บัญชีพัก
//
// บัญชีพัก (suspense) ใช้พักรายการที่ยังระบุไม่ได้ เช่น เงินเข้าบัญชีธนาคารแต่ยังไม่รู้ว่าเป็นของใคร
// (พักรับ) — เมื่อตรวจสอบได้จึงโอนออกไปบัญชีที่ถูกต้อง ยอดจึงควรกลับเป็นศูนย์
// ยอดสะสมต่อเนื่องแบบบัญชีงบฐานะการเงิน และไม่เข้างบกำไรขาดทุน
type AccountType string

const (
	AccountAsset     AccountType = "asset"
	AccountLiability AccountType = "liability"
	AccountEquity    AccountType = "equity"
	AccountRevenue   AccountType = "revenue"
	AccountExpense   AccountType = "expense"
	AccountSuspense  AccountType = "suspense"
)

var AccountTypes = []AccountType{AccountAsset, AccountLiability, AccountEquity, AccountRevenue, AccountExpense, AccountSuspense}

func ParseAccountType(raw string) (AccountType, error) {
	return parseEnum(raw, AccountTypes, domain.ErrInvalidAccountType)
}

// DebitNormal: สินทรัพย์ / ค่าใช้จ่าย มียอดคงเหลือปกติด้านเดบิต, ที่เหลือด้านเครดิต
// บัญชีพักแสดงยอดแบบเดบิตเป็นบวก (พักจ่ายค้าง = บวก, พักรับค้าง = ลบ)
func (t AccountType) DebitNormal() bool {
	return t == AccountAsset || t == AccountExpense || t == AccountSuspense
}

// IsBalanceSheet: บัญชีงบแสดงฐานะการเงิน (ยอดสะสมตลอด) — รายได้ / ค่าใช้จ่ายเริ่มนับใหม่ทุกปีบัญชี
func (t AccountType) IsBalanceSheet() bool {
	return t == AccountAsset || t == AccountLiability || t == AccountEquity || t == AccountSuspense
}

// EntryStatus ของรายการบัญชี: draft -> posted -> reversed (ถูกกลับรายการแล้ว)
type EntryStatus string

const (
	EntryDraft    EntryStatus = "draft"
	EntryPosted   EntryStatus = "posted"
	EntryReversed EntryStatus = "reversed"
)

var EntryStatuses = []EntryStatus{EntryDraft, EntryPosted, EntryReversed}

func ParseEntryStatus(raw string) (EntryStatus, error) {
	return parseEnum(raw, EntryStatuses, domain.ErrInvalidStatus)
}

// Affects: รายการที่มีผลต่อยอดบัญชี (posted และ reversed — การกลับรายการเป็นรายการใหม่อีกใบ)
func (s EntryStatus) Affects() bool { return s == EntryPosted || s == EntryReversed }

func parseEnum[T ~string](raw string, allowed []T, invalid error) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", invalid
}
