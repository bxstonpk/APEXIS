package responses

import (
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
)

// ---------- งบการเงินตามแบบกรมพัฒนาธุรกิจการค้า ----------

type DBDColumn struct {
	StartDate  *string `json:"start_date"` // งบกำไรขาดทุนเท่านั้น
	EndDate    string  `json:"end_date"`
	FiscalYear int     `json:"fiscal_year"`
}

// DBDRow: key = รหัสบรรทัดคงที่ (หน้าเว็บแปลชื่อเอง), kind = header / line / subtotal / total,
// amounts = 1 ค่าต่อคอลัมน์ (ด้านปกติของบรรทัดเป็นบวก — ขาดทุนติดลบ)
type DBDRow struct {
	Key     string   `json:"key"`
	Kind    string   `json:"kind"`
	Level   int      `json:"level"`
	Amounts []string `json:"amounts"`
}

type DBDStatement struct {
	Book    string      `json:"book"`
	Columns []DBDColumn `json:"columns"`
	Rows    []DBDRow    `json:"rows"`
}

func FromDBD(book string, s dto.DBDStatement) DBDStatement {
	out := DBDStatement{Book: book, Columns: make([]DBDColumn, len(s.Columns)), Rows: make([]DBDRow, len(s.Rows))}
	for i, c := range s.Columns {
		out.Columns[i] = DBDColumn{EndDate: date(c.End), FiscalYear: c.FiscalYear}
		if c.Start != nil {
			d := date(*c.Start)
			out.Columns[i].StartDate = &d
		}
	}
	for i, r := range s.Rows {
		am := make([]string, len(r.Amounts))
		for j, v := range r.Amounts {
			am[j] = v.String()
		}
		out.Rows[i] = DBDRow{Key: r.Key, Kind: string(r.Kind), Level: r.Level, Amounts: am}
	}
	return out
}

// ---------- ปิดบัญชีสิ้นปี ----------

type SuspenseBalance struct {
	Account AccountInfo `json:"account"`
	Balance string      `json:"balance"`
}

type YearEndRecord struct {
	NetIncome  string     `json:"net_income"`
	EntryID    *string    `json:"entry_id"`
	EntryNo    string     `json:"entry_no"`
	ReversalNo string     `json:"reversal_no"`
	ClosedAt   time.Time  `json:"closed_at"`
	ClosedBy   string     `json:"closed_by"`
	ReopenedAt *time.Time `json:"reopened_at"`
}

// YearEnd: status = open / closed; deadline_status = on_time / overdue / closed
type YearEnd struct {
	FiscalYear     int               `json:"fiscal_year"`
	StartDate      string            `json:"start_date"`
	EndDate        string            `json:"end_date"`
	Deadline       string            `json:"deadline"`
	Status         string            `json:"status"`
	DeadlineStatus string            `json:"deadline_status"`
	Periods        int               `json:"periods"`
	OpenPeriods    int               `json:"open_periods"`
	Suspense       []SuspenseBalance `json:"suspense"`
	SettingsReady  bool              `json:"settings_ready"`
	CanClose       bool              `json:"can_close"`
	CanReopen      bool              `json:"can_reopen"`
	Closing        *YearEndRecord    `json:"closing"`
}

func FromYearEnd(y dto.YearEnd) YearEnd {
	out := YearEnd{
		FiscalYear: y.FiscalYear, StartDate: date(y.Start), EndDate: date(y.End), Deadline: date(y.Deadline),
		Status: y.Status, DeadlineStatus: y.DeadlineStatus, Periods: y.Periods, OpenPeriods: y.OpenPeriods,
		Suspense: make([]SuspenseBalance, len(y.Suspense)), SettingsReady: y.SettingsReady, CanClose: y.CanClose, CanReopen: y.CanReopen,
	}
	for i, s := range y.Suspense {
		out.Suspense[i] = SuspenseBalance{Account: info(s.Account), Balance: s.Balance.String()}
	}
	if r := y.Record; r != nil {
		out.Closing = &YearEndRecord{
			NetIncome: r.NetIncome.String(), EntryNo: r.EntryNo, ReversalNo: r.ReversalNo,
			ClosedAt: r.ClosedAt, ClosedBy: r.ClosedBy.String(), ReopenedAt: r.ReopenedAt,
		}
		if r.EntryID != nil {
			id := r.EntryID.String()
			out.Closing.EntryID = &id
		}
	}
	return out
}

func FromYearEnds(ys []dto.YearEnd) []YearEnd {
	out := make([]YearEnd, len(ys))
	for i, y := range ys {
		out[i] = FromYearEnd(y)
	}
	return out
}

// ---------- ค่าตั้ง ----------

type LedgerSettings struct {
	RetainedEarningsAccount *Ref `json:"retained_earnings_account"`
	IncomeSummaryAccount    *Ref `json:"income_summary_account"`
	ClosingJournal          *Ref `json:"closing_journal"`
}

func optRef(r *dto.Ref) *Ref {
	if r == nil {
		return nil
	}
	v := ref(*r)
	return &v
}

func FromLedgerSettings(s dto.LedgerSettings) LedgerSettings {
	return LedgerSettings{
		RetainedEarningsAccount: optRef(s.RetainedEarningsAccount), IncomeSummaryAccount: optRef(s.IncomeSummaryAccount),
		ClosingJournal: optRef(s.ClosingJournal),
	}
}

// ---------- แบบรายการปรับปรุง ----------

// AdjustmentTemplate: บัญชีตั้งต้นของแบบ (null = ไม่พบในผังบัญชีของบริษัท)
type AdjustmentTemplate struct {
	Template      string `json:"template"`
	BalanceDebit  bool   `json:"balance_debit"` // true = บัญชีงบแสดงฐานะอยู่ด้านเดบิต
	BalanceAcc    *Ref   `json:"balance_account"`
	ResultAccount *Ref   `json:"result_account"`
	AutoReverse   bool   `json:"auto_reverse"`
}

func FromTemplates(ts []dto.AdjustmentTemplate) []AdjustmentTemplate {
	out := make([]AdjustmentTemplate, len(ts))
	for i, t := range ts {
		out[i] = AdjustmentTemplate{Template: t.Template, BalanceDebit: t.BalanceDebit, BalanceAcc: optRef(t.BalanceAccount),
			ResultAccount: optRef(t.ResultAccount), AutoReverse: t.AutoReverse}
	}
	return out
}
