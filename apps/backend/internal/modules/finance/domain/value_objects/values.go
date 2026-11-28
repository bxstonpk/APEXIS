// Package valueobjects คือ value object ของ finance
package valueobjects

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/finance/domain/errors"
)

// CompanyID / UserID / FiscalPeriodID ของ organization / identity; GLAccountID / JournalID / EntryID ของ ledger;
// CustomerID / SupplierID / InvoiceID ของ sales / purchasing (เก็บอ้างอิงเท่านั้น — ไม่มี FK ข้าม module)
type (
	CompanyID      uuid.UUID
	UserID         uuid.UUID
	FiscalPeriodID uuid.UUID
	CashAccountID  uuid.UUID
	GLAccountID    uuid.UUID
	JournalID      uuid.UUID
	EntryID        uuid.UUID
	ReceiptID      uuid.UUID
	PaymentID      uuid.UUID
	TransferID     uuid.UUID
	WhtTypeID      uuid.UUID
	CustomerID     uuid.UUID
	SupplierID     uuid.UUID
	InvoiceID      uuid.UUID
)

// Money คือจำนวนเงิน (ทศนิยมตามที่บริษัทกำหนด)
type Money = decimal.Decimal

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
func (id ReceiptID) String() string     { return uuid.UUID(id).String() }
func (id PaymentID) String() string     { return uuid.UUID(id).String() }
func (id TransferID) String() string    { return uuid.UUID(id).String() }
func (id WhtTypeID) String() string     { return uuid.UUID(id).String() }

// IsDateOnly: วันที่ไม่มีเวลา (UTC เที่ยงคืน)
func IsDateOnly(t time.Time) bool {
	return !t.IsZero() && t.Equal(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
}

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

// ---------- เอกสาร ----------

// DocStatus: เอกสารรับ / จ่าย / โอนลงบัญชีทันทีที่สร้าง (posted) และยกเลิกด้วยการกลับรายการ (cancelled)
type DocStatus string

const (
	DocPosted    DocStatus = "posted"
	DocCancelled DocStatus = "cancelled"
)

var DocStatuses = []DocStatus{DocPosted, DocCancelled}

func ParseDocStatus(raw string) (DocStatus, error) {
	return parseEnum(raw, DocStatuses, domain.ErrInvalidStatus)
}

// ReceiptKind: customer = รับชำระหนี้ลูกค้า (ตัดใบแจ้งหนี้ขาย), other = รับเงินอื่น (ระบุบัญชีเอง)
type ReceiptKind string

const (
	ReceiptCustomer ReceiptKind = "customer"
	ReceiptOther    ReceiptKind = "other"
)

var ReceiptKinds = []ReceiptKind{ReceiptCustomer, ReceiptOther}

func ParseReceiptKind(raw string) (ReceiptKind, error) {
	return parseEnum(raw, ReceiptKinds, domain.ErrInvalidKind)
}

// PaymentKind: supplier = จ่ายชำระหนี้ผู้ขาย (ตัดใบแจ้งหนี้ซื้อ), other = จ่ายเงินอื่น (ค่าใช้จ่าย / อื่นๆ ระบุบัญชีเอง),
// petty_cash = จ่ายคำขอเงินสดย่อย (ตั้ง / เพิ่มวงเงิน / เบิกชดเชย — บรรทัดเดบิตมาจาก pettycash)
type PaymentKind string

const (
	PaymentSupplier  PaymentKind = "supplier"
	PaymentOther     PaymentKind = "other"
	PaymentPettyCash PaymentKind = "petty_cash"
)

var PaymentKinds = []PaymentKind{PaymentSupplier, PaymentOther, PaymentPettyCash}

func ParsePaymentKind(raw string) (PaymentKind, error) {
	return parseEnum(raw, PaymentKinds, domain.ErrInvalidKind)
}

// TaxType ของผู้รับเงิน — กำหนดแบบยื่นภาษีหัก ณ ที่จ่าย: individual = ภ.ง.ด.3, juristic = ภ.ง.ด.53
type TaxType string

const (
	TaxIndividual TaxType = "individual"
	TaxJuristic   TaxType = "juristic"
)

var TaxTypes = []TaxType{TaxIndividual, TaxJuristic}

func ParseTaxType(raw string) (TaxType, error) {
	return parseEnum(raw, TaxTypes, domain.ErrInvalidTaxType)
}

// Form: แบบยื่น pnd3 / pnd53 ("" = ไม่ระบุ)
func (t TaxType) Form() string {
	switch t {
	case TaxIndividual:
		return "pnd3"
	case TaxJuristic:
		return "pnd53"
	}
	return ""
}

// IncomeType คือประเภทเงินได้ตามหนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) — ใช้เลือกช่องที่พิมพ์
type IncomeType string

const (
	Income40_1  IncomeType = "40_1"   // เงินเดือน ค่าจ้าง เบี้ยเลี้ยง โบนัส ฯลฯ ตามมาตรา 40 (1)
	Income40_2  IncomeType = "40_2"   // ค่าธรรมเนียม ค่านายหน้า ฯลฯ ตามมาตรา 40 (2)
	Income40_3  IncomeType = "40_3"   // ค่าแห่งลิขสิทธิ์ ฯลฯ ตามมาตรา 40 (3)
	Income40_4a IncomeType = "40_4a"  // ดอกเบี้ย ฯลฯ ตามมาตรา 40 (4) (ก)
	Income40_4b IncomeType = "40_4b"  // เงินปันผล เงินส่วนแบ่งกำไร ฯลฯ ตามมาตรา 40 (4) (ข)
	Income3Tres IncomeType = "3_tres" // ตามคำสั่งกรมสรรพากรที่ออกตามมาตรา 3 เตรส (ค่าบริการ ค่าเช่า ค่าจ้างทำของ ค่าโฆษณา ค่าขนส่ง ฯลฯ)
	IncomeOther IncomeType = "other"  // อื่น ๆ
)

var IncomeTypes = []IncomeType{Income40_1, Income40_2, Income40_3, Income40_4a, Income40_4b, Income3Tres, IncomeOther}

func ParseIncomeType(raw string) (IncomeType, error) {
	return parseEnum(raw, IncomeTypes, domain.ErrInvalidIncomeType)
}

// DocType คือชนิดเอกสารที่ลงทะเบียนกับ organization เพื่อตั้งรูปแบบเลขที่
type DocType string

const (
	DocReceipt  DocType = "finance.receipt"
	DocPayment  DocType = "finance.payment"
	DocTransfer DocType = "finance.transfer"
	// DocWhtCertificate: เลขที่หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) — ออกพร้อมใบสำคัญจ่ายที่มีภาษีหัก ณ ที่จ่าย
	DocWhtCertificate DocType = "finance.wht_certificate"
)

var DocTypes = []DocType{DocReceipt, DocPayment, DocTransfer, DocWhtCertificate}

func (t DocType) Prefix() string {
	return map[DocType]string{DocReceipt: "RV", DocPayment: "PV", DocTransfer: "TF", DocWhtCertificate: "WT"}[t]
}

func (t DocType) Name() string {
	return map[DocType]string{DocReceipt: "ใบเสร็จรับเงิน / ใบรับเงิน", DocPayment: "ใบสำคัญจ่าย", DocTransfer: "ใบโอนเงินระหว่างบัญชี",
		DocWhtCertificate: "หนังสือรับรองการหักภาษี ณ ที่จ่าย"}[t]
}

// Permission ของ finance
type Permission string

const (
	PermAccountRead    Permission = "finance.cash_account:read"
	PermAccountManage  Permission = "finance.cash_account:manage"
	PermReceiptRead    Permission = "finance.receipt:read"
	PermReceiptPost    Permission = "finance.receipt:post" // รับเงิน / ยกเลิก (ลงบัญชี + ตัดยอดลูกหนี้)
	PermPaymentRead    Permission = "finance.payment:read"
	PermPaymentPost    Permission = "finance.payment:post" // จ่ายเงิน / ยกเลิก (ลงบัญชี + ตัดยอดเจ้าหนี้)
	PermTransferRead   Permission = "finance.transfer:read"
	PermTransferPost   Permission = "finance.transfer:post"
	PermSettingsRead   Permission = "finance.settings:read"
	PermSettingsManage Permission = "finance.settings:manage" // ค่าตั้งการเงิน + ประเภทภาษีหัก ณ ที่จ่าย
	PermReportRead     Permission = "finance.report:read"
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ finance (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermAccountRead,
	PermAccountManage,
	PermReceiptRead,
	PermReceiptPost,
	PermPaymentRead,
	PermPaymentPost,
	PermTransferRead,
	PermTransferPost,
	PermSettingsRead,
	PermSettingsManage,
	PermReportRead,
}

// PermissionMigration คือการแปลงสิทธิ์เดิมของบทบาทเมื่อความรับผิดชอบย้าย module (ตัดสินใจ 2026-11-23)
// From = สิทธิ์เดิม (รวม wildcard), To = สิทธิ์ใหม่, Keep = คงสิทธิ์เดิมไว้ด้วย (ยังมีอยู่ใน module เดิม)
type PermissionMigration struct {
	From string
	To   []string
	Keep bool
}

// PermissionMigrations: ทะเบียนเงินจาก cashbank, รับเงินจาก sales, จ่ายเงิน / ภาษีหัก ณ ที่จ่ายจาก purchasing
var PermissionMigrations = []PermissionMigration{
	{From: "cashbank.*", To: []string{"finance.*"}},
	{From: "cashbank.account:*", To: []string{"finance.cash_account:*"}},
	{From: "cashbank.account:read", To: []string{"finance.cash_account:read"}},
	{From: "cashbank.account:manage", To: []string{"finance.cash_account:manage"}},
	{From: "sales.receipt:*", To: []string{"finance.receipt:*"}},
	{From: "sales.receipt:read", To: []string{"finance.receipt:read"}},
	{From: "sales.receipt:post", To: []string{"finance.receipt:post"}},
	{From: "purchasing.payment:*", To: []string{"finance.payment:*"}},
	{From: "purchasing.payment:read", To: []string{"finance.payment:read"}},
	{From: "purchasing.payment:post", To: []string{"finance.payment:post"}},
	// ประเภทภาษีหัก ณ ที่จ่ายเคยใช้สิทธิ์ค่าตั้งจัดซื้อ, ภ.ง.ด.3 / 53 เคยอยู่ในรายงานจัดซื้อ, ภาษีถูกหักเคยอยู่ในรายงานขาย
	{From: "purchasing.settings:read", To: []string{"finance.settings:read"}, Keep: true},
	{From: "purchasing.settings:manage", To: []string{"finance.settings:manage"}, Keep: true},
	{From: "purchasing.report:read", To: []string{"finance.report:read"}, Keep: true},
	{From: "sales.report:read", To: []string{"finance.report:read"}, Keep: true},
}

func IDString[T ~[16]byte](id T) string { return uuid.UUID(id).String() }
