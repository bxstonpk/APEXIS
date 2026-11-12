// Package responses คือรูปแบบ JSON ขาออกของ ledger (แปลงจาก application/dto)
//
//   - วันที่ "YYYY-MM-DD" (ค.ศ.)
//   - จำนวนเงินเป็น string ทศนิยมตามค่าจริง เช่น "1234.5" (ไม่ใช้ number เพื่อไม่ให้ client ปัดเศษผิด)
//     frontend จัดรูปแบบการแสดงผลตามจำนวนทศนิยมที่บริษัทกำหนด (company.amount_decimals)
//
// envelope (data / meta / error) อยู่ที่ platform/httpx
package responses

import (
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/services"
	"apexis/backend/internal/platform/httpx"
)

func date(t time.Time) string { return t.Format(time.DateOnly) }

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func ref(r dto.Ref) Ref { return Ref{ID: r.ID, Code: r.Code, Name: r.Name} }

// ---------- account / journal ----------

type Account struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	NameEN   string `json:"name_en"`
	Type     string `json:"type"`
	Parent   *Ref   `json:"parent"`
	Postable bool   `json:"postable"`
	Status   string `json:"status"`
}

func FromAccount(a dto.Account) Account {
	out := Account{ID: a.ID, Code: a.Code, Name: a.Name, NameEN: a.NameEN, Type: a.Type, Postable: a.Postable, Status: a.Status}
	if a.Parent != nil {
		p := ref(*a.Parent)
		out.Parent = &p
	}
	return out
}

type Journal struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func FromJournal(j dto.Journal) Journal {
	return Journal{ID: j.ID, Code: j.Code, Name: j.Name, Status: j.Status}
}

// ---------- journal entry ----------

type EntrySummary struct {
	ID          string     `json:"id"`
	EntryNo     *string    `json:"entry_no"` // null จนกว่าจะผ่านรายการ
	Date        string     `json:"date"`
	Journal     Ref        `json:"journal"`
	Description string     `json:"description"`
	Reference   string     `json:"reference"`
	Status      string     `json:"status"`
	TotalDebit  string     `json:"total_debit"`
	TotalCredit string     `json:"total_credit"`
	PostedAt    *time.Time `json:"posted_at"`
}

func FromEntrySummary(e dto.EntrySummary) EntrySummary {
	out := EntrySummary{
		ID: e.ID, Date: date(e.Date), Journal: ref(e.Journal), Description: e.Description, Reference: e.Reference,
		Status: e.Status, TotalDebit: e.TotalDebit.String(), TotalCredit: e.TotalCredit.String(), PostedAt: e.PostedAt,
	}
	if e.EntryNo != "" {
		no := e.EntryNo
		out.EntryNo = &no
	}
	return out
}

type EntryLine struct {
	LineNo        int     `json:"line_no"`
	Account       Ref     `json:"account"`
	Description   string  `json:"description"`
	Debit         string  `json:"debit"`
	Credit        string  `json:"credit"`
	BranchID      *string `json:"branch_id"`
	DepartmentID  *string `json:"department_id"`
	CostCenterID  *string `json:"cost_center_id"`
	ProjectSiteID *string `json:"project_site_id"`
}

type Entry struct {
	EntrySummary
	FiscalPeriodID *string     `json:"fiscal_period_id"`
	ReversalOfID   *string     `json:"reversal_of_id"`
	ReversedByID   *string     `json:"reversed_by_id"`
	CreatedBy      string      `json:"created_by"`
	PostedBy       *string     `json:"posted_by"`
	Lines          []EntryLine `json:"lines"`
}

func FromEntry(e dto.Entry) Entry {
	out := Entry{
		EntrySummary: FromEntrySummary(e.EntrySummary), FiscalPeriodID: e.FiscalPeriodID,
		ReversalOfID: e.ReversalOfID, ReversedByID: e.ReversedByID, CreatedBy: e.CreatedBy, PostedBy: e.PostedBy,
		Lines: make([]EntryLine, len(e.Lines)),
	}
	for i, l := range e.Lines {
		out.Lines[i] = EntryLine{
			LineNo: l.LineNo, Account: ref(l.Account), Description: l.Description,
			Debit: l.Debit.String(), Credit: l.Credit.String(),
			BranchID: l.BranchID, DepartmentID: l.DepartmentID, CostCenterID: l.CostCenterID, ProjectSiteID: l.ProjectSiteID,
		}
	}
	return out
}

// ---------- รายงาน ----------

type AccountInfo struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func info(a dto.AccountInfo) AccountInfo {
	return AccountInfo{ID: a.ID, Code: a.Code, Name: a.Name, Type: a.Type}
}

type TrialBalanceRow struct {
	Account        AccountInfo `json:"account"`
	OpeningBalance string      `json:"opening_balance"` // เดบิตเป็นบวก
	Debit          string      `json:"debit"`
	Credit         string      `json:"credit"`
	ClosingBalance string      `json:"closing_balance"` // เดบิตเป็นบวก
}

type TrialBalance struct {
	Book               string            `json:"book"`
	StartDate          string            `json:"start_date"`
	EndDate            string            `json:"end_date"`
	Rows               []TrialBalanceRow `json:"rows"`
	TotalDebit         string            `json:"total_debit"`
	TotalCredit        string            `json:"total_credit"`
	PriorYearsEarnings string            `json:"prior_years_earnings"` // เดบิตเป็นบวก
}

func FromTrialBalance(t dto.TrialBalance) TrialBalance {
	out := TrialBalance{
		StartDate: date(t.Start), EndDate: date(t.End), Rows: make([]TrialBalanceRow, len(t.Rows)),
		TotalDebit: t.TotalDebit.String(), TotalCredit: t.TotalCredit.String(), PriorYearsEarnings: t.PriorYearsEarnings.String(),
	}
	for i, r := range t.Rows {
		out.Rows[i] = TrialBalanceRow{
			Account: info(r.Account), OpeningBalance: r.Opening.String(),
			Debit: r.Debit.String(), Credit: r.Credit.String(), ClosingBalance: r.Closing.String(),
		}
	}
	return out
}

type LedgerLine struct {
	EntryID     string `json:"entry_id"`
	EntryNo     string `json:"entry_no"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Reference   string `json:"reference"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
	Balance     string `json:"balance"` // ยอดสะสมรวมยอดยกมา เดบิตเป็นบวก
}

type AccountLedger struct {
	Book           string       `json:"book"`
	Account        AccountInfo  `json:"account"`
	StartDate      string       `json:"start_date"`
	EndDate        string       `json:"end_date"`
	OpeningBalance string       `json:"opening_balance"`
	Lines          []LedgerLine `json:"lines"`
}

// AccountLedgerBody: {"data": {...}, "meta": {...}} — meta คือการแบ่งหน้าของ lines
type AccountLedgerBody struct {
	Data AccountLedger `json:"data"`
	Meta httpx.Meta    `json:"meta"`
}

func FromAccountLedger(l dto.AccountLedger) AccountLedgerBody {
	out := AccountLedger{
		Account: info(l.Account), StartDate: date(l.Start), EndDate: date(l.End),
		OpeningBalance: l.Opening.String(), Lines: make([]LedgerLine, len(l.Lines.Items)),
	}
	for i, x := range l.Lines.Items {
		out.Lines[i] = LedgerLine{
			EntryID: x.EntryID, EntryNo: x.EntryNo, Date: date(x.Date), Description: x.Description, Reference: x.Reference,
			Debit: x.Debit.String(), Credit: x.Credit.String(), Balance: x.Balance.String(),
		}
	}
	return AccountLedgerBody{Data: out, Meta: httpx.Meta{Page: l.Lines.Page.Number, PageSize: l.Lines.Page.Size, Total: l.Lines.Total}}
}

// StatementLine: amount ตามด้านปกติของหมวด รวมบัญชีย่อยแล้ว — level ใช้ย่อหน้า
type StatementLine struct {
	AccountID string `json:"account_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Level     int    `json:"level"`
	IsHeader  bool   `json:"is_header"`
	Amount    string `json:"amount"`
}

func lines(in []services.StatementLine) []StatementLine {
	out := make([]StatementLine, len(in))
	for i, l := range in {
		out[i] = StatementLine{
			AccountID: l.AccountID.String(), Code: l.Code, Name: l.Name, Level: l.Level, IsHeader: l.IsHeader, Amount: l.Amount.String(),
		}
	}
	return out
}

type BalanceSheet struct {
	Book                      string          `json:"book"`
	AsOf                      string          `json:"as_of"`
	Assets                    []StatementLine `json:"assets"`
	TotalAssets               string          `json:"total_assets"`
	Liabilities               []StatementLine `json:"liabilities"`
	TotalLiabilities          string          `json:"total_liabilities"`
	Equity                    []StatementLine `json:"equity"`
	RetainedEarningsPrior     string          `json:"retained_earnings_prior_years"`
	CurrentYearEarnings       string          `json:"current_year_earnings"`
	TotalEquity               string          `json:"total_equity"`
	TotalLiabilitiesAndEquity string          `json:"total_liabilities_and_equity"`
}

func FromBalanceSheet(b dto.BalanceSheet) BalanceSheet {
	return BalanceSheet{
		AsOf: date(b.AsOf), Assets: lines(b.Assets), TotalAssets: b.TotalAssets.String(),
		Liabilities: lines(b.Liabilities), TotalLiabilities: b.TotalLiabilities.String(),
		Equity: lines(b.Equity), RetainedEarningsPrior: b.RetainedEarningsPrior.String(),
		CurrentYearEarnings: b.CurrentYearEarnings.String(), TotalEquity: b.TotalEquity.String(),
		TotalLiabilitiesAndEquity: b.TotalLiabilitiesAndEquity.String(),
	}
}

type IncomeStatement struct {
	Book          string          `json:"book"`
	StartDate     string          `json:"start_date"`
	EndDate       string          `json:"end_date"`
	Revenue       []StatementLine `json:"revenue"`
	TotalRevenue  string          `json:"total_revenue"`
	Expenses      []StatementLine `json:"expenses"`
	TotalExpenses string          `json:"total_expenses"`
	NetIncome     string          `json:"net_income"`
}

func FromIncomeStatement(s dto.IncomeStatement) IncomeStatement {
	return IncomeStatement{
		StartDate: date(s.Start), EndDate: date(s.End),
		Revenue: lines(s.Revenue), TotalRevenue: s.TotalRevenue.String(),
		Expenses: lines(s.Expenses), TotalExpenses: s.TotalExpenses.String(), NetIncome: s.NetIncome.String(),
	}
}
