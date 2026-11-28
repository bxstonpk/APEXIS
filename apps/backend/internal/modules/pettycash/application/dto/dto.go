// Package dto คือ input / output ของ use case ของ pettycash (read model ไม่ใช่ gorm model)
package dto

import (
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

// Ref คือรหัส / ชื่อของข้อมูลอ้างอิง (บัญชี / กองทุน / ผู้ใช้ / เอกสาร) — Code ว่างได้
type Ref struct {
	ID   string
	Code string
	Name string
}

// ---------- ค่าตั้ง ----------

type Settings struct {
	InputVatAccount *Ref
}

type SettingsInput struct {
	Actor
	Settings entities.Settings
}

// ---------- กองทุน ----------

// Balance คือยอดของกองทุน (คำนวณจากเอกสาร):
//
//	CashOnHand = เงินที่การเงินจ่ายเข้ากองทุน (ตั้ง / เพิ่มวงเงิน + เบิกชดเชย) - ใบสำคัญจ่ายที่อนุมัติแล้ว (รวมที่ชดเชยแล้ว)
//	Pending = ใบสำคัญจ่ายที่อนุมัติแล้วแต่ยังไม่ได้ชดเชย, Requested = คำขอที่รอการเงินจ่าย
type Balance struct {
	Funded     bool
	CashOnHand vo.Money
	Pending    vo.Money
	Requested  vo.Money
}

type Fund struct {
	ID            string
	Code          string
	Name          string
	Custodian     Ref
	GLAccount     Ref
	ImprestAmount vo.Money
	Status        vo.Status
	Balance
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FundFilter struct {
	Search string
	Status []vo.Status
	Sort   []listing.Sort
	Page   listing.Page
}

var FundSortKeys = []string{"code", "name", "updated_at"}
var DefaultFundSort = []listing.Sort{{Key: "code"}}

// FundInput: GLAccountID nil = บัญชีเงินสดย่อยของผังมาตรฐาน (1101010200)
type FundInput struct {
	Actor
	ID            vo.FundID // update เท่านั้น
	Code          string
	Name          string
	CustodianID   vo.UserID
	GLAccountID   *vo.AccountID
	ImprestAmount vo.Money
	Status        vo.Status
}

// ---------- ใบสำคัญจ่าย ----------

type TaxInvoice struct {
	No   string
	Date time.Time
}

type VoucherLine struct {
	LineNo      int
	Account     Ref
	Description string
	Amount      vo.Money
	Net         vo.Money
}

type Voucher struct {
	ID               string
	DocumentNo       string
	Fund             Ref
	Date             time.Time
	Payee            entities.Payee
	Description      string
	Lines            []VoucherLine
	VatRate          vo.Money
	PriceIncludesVat bool
	TaxInvoice       *TaxInvoice
	Totals           entities.Totals
	Status           vo.VoucherStatus
	Replenishment    *Ref
	CreatedBy        Ref
	ApprovedBy       *Ref
	ApprovedAt       *time.Time
	CancelledAt      *time.Time
	CreatedAt        time.Time
}

type VoucherSummary struct {
	ID            string
	DocumentNo    string
	Date          time.Time
	Fund          Ref
	PayeeName     string
	Description   string
	TaxInvoiceNo  string
	Totals        entities.Totals
	Status        vo.VoucherStatus
	Replenishment *Ref
}

// VoucherFilter: Unrequested = เฉพาะใบที่อนุมัติแล้วและยังไม่อยู่ในคำขอเบิกชดเชย (เลือกในหน้าขอเบิก)
type VoucherFilter struct {
	Search      string
	FundID      *vo.FundID
	Status      []vo.VoucherStatus
	From        *time.Time
	To          *time.Time
	Unrequested bool
	Sort        []listing.Sort
	Page        listing.Page
}

var VoucherSortKeys = []string{"date", "document_no", "created_at"}
var DefaultVoucherSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}

type VoucherInput struct {
	Actor
	ID   vo.VoucherID // update เท่านั้น
	Data entities.VoucherData
}

type ByVoucher struct {
	Actor
	ID vo.VoucherID
}

// ---------- คำขอเบิกชดเชย ----------

type Payment struct {
	DocumentID string
	DocumentNo string
	Date       time.Time
}

type Replenishment struct {
	ID          string
	DocumentNo  string
	Fund        Ref
	Kind        vo.RequestKind
	Date        time.Time
	Description string
	Amount      vo.Money
	Status      vo.RequestStatus
	Vouchers    []VoucherSummary
	Payment     *Payment
	CreatedBy   Ref
	CreatedAt   time.Time
	CancelledAt *time.Time
}

type ReplenishmentSummary struct {
	ID          string
	DocumentNo  string
	Fund        Ref
	Kind        vo.RequestKind
	Date        time.Time
	Description string
	Amount      vo.Money
	Status      vo.RequestStatus
	Vouchers    int
	Payment     *Payment
}

type ReplenishmentFilter struct {
	Search string
	FundID *vo.FundID
	Kind   []vo.RequestKind
	Status []vo.RequestStatus
	From   *time.Time
	To     *time.Time
	Sort   []listing.Sort
	Page   listing.Page
}

var ReplenishmentSortKeys = []string{"date", "document_no", "created_at"}
var DefaultReplenishmentSort = []listing.Sort{{Key: "date", Desc: true}, {Key: "document_no", Desc: true}}

// ReplenishmentInput: establish ใช้วงเงินของกองทุน (Amount ไม่ใช้), increase ใช้ Amount, replenish ใช้ VoucherIDs
type ReplenishmentInput struct {
	Actor
	FundID      vo.FundID
	Kind        vo.RequestKind
	Date        time.Time
	Description string
	Amount      vo.Money
	VoucherIDs  []vo.VoucherID
}

type ByReplenishment struct {
	Actor
	ID vo.ReplenishmentID
}

// OpenRequest คือคำขอที่รอการเงินจ่าย (การเงินเลือกในหน้าจ่ายเงิน)
type OpenRequest struct {
	ID         uuid.UUID
	DocumentNo string
	FundCode   string
	FundName   string
	Kind       vo.RequestKind
	Date       time.Time
	Amount     vo.Money
}

// ---------- รายงาน ----------

type DateRange struct {
	Start time.Time
	End   time.Time
}

type RegisterFilter struct {
	DateRange
	FundID *vo.FundID
	Status []vo.VoucherStatus
	Page   listing.Page
}

type RegisterReport struct {
	Rows   listing.Result[VoucherSummary]
	Totals entities.Totals
}

type FundBalanceRow struct {
	Fund          Ref
	Custodian     Ref
	ImprestAmount vo.Money
	Balance
}

type InputVatFilter struct {
	DateRange
	Page listing.Page
}

type InputVatRow struct {
	Date         time.Time // วันที่ใบกำกับภาษี
	TaxInvoiceNo string
	Voucher      Ref
	PayeeName    string
	TaxID        string
	BranchNo     string
	Base         vo.Money
	Vat          vo.Money
}

type InputVatReport struct {
	Rows   listing.Result[InputVatRow]
	Totals struct{ Base, Vat vo.Money }
}

// VatTotals: ยอดรวมภาษีซื้อของช่วงวันที่ (ใบสำคัญจ่ายที่ชดเชยแล้วและมีใบกำกับภาษี) — finance ใช้ทำ ภ.พ.30
type VatTotals struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}
