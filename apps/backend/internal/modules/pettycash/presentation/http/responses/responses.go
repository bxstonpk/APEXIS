// Package responses คือรูปแบบ JSON ขาออกของ pettycash (Response DTO — ไม่เปิดเผย gorm model)
// ตัวเลขเป็น string (ไม่เสียความแม่นยำ), วันที่ YYYY-MM-DD, เวลาเป็น RFC 3339
package responses

import (
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	"apexis/backend/internal/platform/httpx"
)

func str(d decimal.Decimal) string { return d.String() }

func date(t time.Time) string { return t.Format(time.DateOnly) }

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func ref(r dto.Ref) Ref { return Ref{ID: r.ID, Code: r.Code, Name: r.Name} }

func optRef(r *dto.Ref) *Ref {
	if r == nil {
		return nil
	}
	out := ref(*r)
	return &out
}

// DocRef คือเอกสารที่อ้างถึง (คำขอเบิกชดเชย / ใบสำคัญจ่าย)
type DocRef struct {
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
}

func docRef(r *dto.Ref) *DocRef {
	if r == nil {
		return nil
	}
	return &DocRef{ID: r.ID, DocumentNo: r.Code}
}

// UserRef คือผู้ใช้ (ชื่อว่าง = ไม่ใช่สมาชิกแล้ว)
type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func userRef(r dto.Ref) UserRef { return UserRef{ID: r.ID, Name: r.Name} }

type Totals struct {
	Net   string `json:"net"`
	Vat   string `json:"vat"`
	Gross string `json:"gross"`
}

func totals(t entities.Totals) Totals {
	return Totals{Net: str(t.Net), Vat: str(t.Vat), Gross: str(t.Gross)}
}

// ---------- ค่าตั้ง ----------

type Settings struct {
	InputVatAccountID *string `json:"input_vat_account_id"`
	InputVatAccount   *Ref    `json:"input_vat_account"`
}

func FromSettings(s dto.Settings) Settings {
	out := Settings{InputVatAccount: optRef(s.InputVatAccount)}
	if s.InputVatAccount != nil {
		out.InputVatAccountID = &s.InputVatAccount.ID
	}
	return out
}

// ---------- กองทุน ----------

type Balance struct {
	Funded               bool   `json:"funded"`
	CashOnHand           string `json:"cash_on_hand"`
	PendingReplenishment string `json:"pending_replenishment"`
	Requested            string `json:"requested"`
}

func balance(b dto.Balance) Balance {
	return Balance{Funded: b.Funded, CashOnHand: str(b.CashOnHand), PendingReplenishment: str(b.Pending), Requested: str(b.Requested)}
}

type Fund struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Custodian     UserRef `json:"custodian"`
	GLAccount     Ref     `json:"gl_account"`
	ImprestAmount string  `json:"imprest_amount"`
	Status        string  `json:"status"`
	Balance
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func FromFund(f dto.Fund) Fund {
	return Fund{ID: f.ID, Code: f.Code, Name: f.Name, Custodian: userRef(f.Custodian), GLAccount: ref(f.GLAccount),
		ImprestAmount: str(f.ImprestAmount), Status: string(f.Status), Balance: balance(f.Balance), CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}

// ---------- ใบสำคัญจ่าย ----------

type Payee struct {
	Name     string `json:"name"`
	TaxID    string `json:"tax_id"`
	BranchNo string `json:"branch_no"`
}

type TaxInvoice struct {
	No   string `json:"no"`
	Date string `json:"date"`
}

type VoucherLine struct {
	LineNo      int    `json:"line_no"`
	Account     Ref    `json:"account"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	Net         string `json:"net"`
}

type Voucher struct {
	ID               string        `json:"id"`
	DocumentNo       string        `json:"document_no"`
	Fund             Ref           `json:"fund"`
	Date             string        `json:"date"`
	Payee            Payee         `json:"payee"`
	Description      string        `json:"description"`
	Lines            []VoucherLine `json:"lines"`
	VatRate          string        `json:"vat_rate"`
	PriceIncludesVat bool          `json:"price_includes_vat"`
	TaxInvoice       *TaxInvoice   `json:"tax_invoice"`
	Totals           Totals        `json:"totals"`
	Status           string        `json:"status"`
	Replenishment    *DocRef       `json:"replenishment"`
	CreatedBy        UserRef       `json:"created_by"`
	ApprovedBy       *UserRef      `json:"approved_by"`
	ApprovedAt       *time.Time    `json:"approved_at"`
	CancelledAt      *time.Time    `json:"cancelled_at"`
	CreatedAt        time.Time     `json:"created_at"`
}

func FromVoucher(v dto.Voucher) Voucher {
	out := Voucher{ID: v.ID, DocumentNo: v.DocumentNo, Fund: ref(v.Fund), Date: date(v.Date),
		Payee: Payee{Name: v.Payee.Name, TaxID: v.Payee.TaxID, BranchNo: v.Payee.BranchNo}, Description: v.Description,
		VatRate: str(v.VatRate), PriceIncludesVat: v.PriceIncludesVat, Totals: totals(v.Totals), Status: string(v.Status),
		Replenishment: docRef(v.Replenishment), CreatedBy: userRef(v.CreatedBy), ApprovedAt: v.ApprovedAt, CancelledAt: v.CancelledAt,
		CreatedAt: v.CreatedAt, Lines: make([]VoucherLine, len(v.Lines))}
	if v.TaxInvoice != nil {
		out.TaxInvoice = &TaxInvoice{No: v.TaxInvoice.No, Date: date(v.TaxInvoice.Date)}
	}
	if v.ApprovedBy != nil {
		u := userRef(*v.ApprovedBy)
		out.ApprovedBy = &u
	}
	for i, l := range v.Lines {
		out.Lines[i] = VoucherLine{LineNo: l.LineNo, Account: ref(l.Account), Description: l.Description, Amount: str(l.Amount), Net: str(l.Net)}
	}
	return out
}

type VoucherRow struct {
	ID            string  `json:"id"`
	DocumentNo    string  `json:"document_no"`
	Date          string  `json:"date"`
	Fund          Ref     `json:"fund"`
	PayeeName     string  `json:"payee_name"`
	Description   string  `json:"description"`
	TaxInvoiceNo  string  `json:"tax_invoice_no"`
	Totals        Totals  `json:"totals"`
	Status        string  `json:"status"`
	Replenishment *DocRef `json:"replenishment"`
}

func FromVoucherRow(v dto.VoucherSummary) VoucherRow {
	return VoucherRow{ID: v.ID, DocumentNo: v.DocumentNo, Date: date(v.Date), Fund: ref(v.Fund), PayeeName: v.PayeeName,
		Description: v.Description, TaxInvoiceNo: v.TaxInvoiceNo, Totals: totals(v.Totals), Status: string(v.Status),
		Replenishment: docRef(v.Replenishment)}
}

// ---------- คำขอ ----------

type Payment struct {
	DocumentID string `json:"document_id"`
	DocumentNo string `json:"document_no"`
	Date       string `json:"date"`
}

func payment(p *dto.Payment) *Payment {
	if p == nil {
		return nil
	}
	return &Payment{DocumentID: p.DocumentID, DocumentNo: p.DocumentNo, Date: date(p.Date)}
}

type Replenishment struct {
	ID          string       `json:"id"`
	DocumentNo  string       `json:"document_no"`
	Fund        Ref          `json:"fund"`
	Kind        string       `json:"kind"`
	Date        string       `json:"date"`
	Description string       `json:"description"`
	Amount      string       `json:"amount"`
	Status      string       `json:"status"`
	Vouchers    []VoucherRow `json:"vouchers"`
	Payment     *Payment     `json:"payment"`
	CreatedBy   UserRef      `json:"created_by"`
	CreatedAt   time.Time    `json:"created_at"`
	CancelledAt *time.Time   `json:"cancelled_at"`
}

func FromReplenishment(r dto.Replenishment) Replenishment {
	out := Replenishment{ID: r.ID, DocumentNo: r.DocumentNo, Fund: ref(r.Fund), Kind: string(r.Kind), Date: date(r.Date),
		Description: r.Description, Amount: str(r.Amount), Status: string(r.Status), Payment: payment(r.Payment),
		CreatedBy: userRef(r.CreatedBy), CreatedAt: r.CreatedAt, CancelledAt: r.CancelledAt, Vouchers: make([]VoucherRow, len(r.Vouchers))}
	for i, v := range r.Vouchers {
		out.Vouchers[i] = FromVoucherRow(v)
	}
	return out
}

type ReplenishmentRow struct {
	ID          string   `json:"id"`
	DocumentNo  string   `json:"document_no"`
	Fund        Ref      `json:"fund"`
	Kind        string   `json:"kind"`
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Amount      string   `json:"amount"`
	Status      string   `json:"status"`
	Vouchers    int      `json:"vouchers"`
	Payment     *Payment `json:"payment"`
}

func FromReplenishmentRow(r dto.ReplenishmentSummary) ReplenishmentRow {
	return ReplenishmentRow{ID: r.ID, DocumentNo: r.DocumentNo, Fund: ref(r.Fund), Kind: string(r.Kind), Date: date(r.Date),
		Description: r.Description, Amount: str(r.Amount), Status: string(r.Status), Vouchers: r.Vouchers, Payment: payment(r.Payment)}
}

// ---------- รายงาน ----------

type RegisterReport struct {
	Book   string       `json:"book"`
	Rows   []VoucherRow `json:"rows"`
	Totals Totals       `json:"totals"`
}

type RegisterBody struct {
	Data RegisterReport `json:"data"`
	Meta httpx.Meta     `json:"meta"`
}

func FromRegister(book string, r dto.RegisterReport) RegisterBody {
	list := httpx.ToList(r.Rows, FromVoucherRow)
	return RegisterBody{Data: RegisterReport{Book: book, Rows: list.Data, Totals: totals(r.Totals)}, Meta: list.Meta}
}

type FundBalanceRow struct {
	Fund          Ref     `json:"fund"`
	Custodian     UserRef `json:"custodian"`
	ImprestAmount string  `json:"imprest_amount"`
	Balance
}

type FundBalanceReport struct {
	Book    string           `json:"book"`
	EndDate string           `json:"end_date"`
	Rows    []FundBalanceRow `json:"rows"`
}

func FromFundBalance(book string, asOf time.Time, rows []dto.FundBalanceRow) httpx.Data[FundBalanceReport] {
	out := FundBalanceReport{Book: book, EndDate: date(asOf), Rows: make([]FundBalanceRow, len(rows))}
	for i, r := range rows {
		out.Rows[i] = FundBalanceRow{Fund: ref(r.Fund), Custodian: userRef(r.Custodian), ImprestAmount: str(r.ImprestAmount), Balance: balance(r.Balance)}
	}
	return httpx.Data[FundBalanceReport]{Data: out}
}

type InputVatRow struct {
	Date         string `json:"date"`
	TaxInvoiceNo string `json:"tax_invoice_no"`
	Voucher      DocRef `json:"voucher"`
	PayeeName    string `json:"payee_name"`
	TaxID        string `json:"tax_id"`
	BranchNo     string `json:"branch_no"`
	Base         string `json:"base"`
	Vat          string `json:"vat"`
}

type InputVatTotals struct {
	Base string `json:"base"`
	Vat  string `json:"vat"`
}

type InputVatReport struct {
	Book   string         `json:"book"`
	Rows   []InputVatRow  `json:"rows"`
	Totals InputVatTotals `json:"totals"`
}

type InputVatBody struct {
	Data InputVatReport `json:"data"`
	Meta httpx.Meta     `json:"meta"`
}

func FromInputVat(book string, r dto.InputVatReport) InputVatBody {
	list := httpx.ToList(r.Rows, func(x dto.InputVatRow) InputVatRow {
		return InputVatRow{Date: date(x.Date), TaxInvoiceNo: x.TaxInvoiceNo, Voucher: DocRef{ID: x.Voucher.ID, DocumentNo: x.Voucher.Code},
			PayeeName: x.PayeeName, TaxID: x.TaxID, BranchNo: x.BranchNo, Base: str(x.Base), Vat: str(x.Vat)}
	})
	return InputVatBody{Data: InputVatReport{Book: book, Rows: list.Data, Totals: InputVatTotals{Base: str(r.Totals.Base), Vat: str(r.Totals.Vat)}},
		Meta: list.Meta}
}
