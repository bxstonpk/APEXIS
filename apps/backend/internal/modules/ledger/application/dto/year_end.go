package dto

import (
	"time"

	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// ---------- ปิดบัญชีสิ้นปี ----------

// CloseDeadlineDays: ระยะเวลาปิดบัญชี / จัดทำงบการเงินหลังวันสิ้นปีบัญชี
const CloseDeadlineDays = 120

// YearEndInput: POST /year-end-closings/{fiscalYear}[/reopen]
type YearEndInput struct {
	Actor
	FiscalYear int
}

// YearEndRecord คือแถวการปิดบัญชีที่บันทึกไว้ (พร้อมเลขที่รายการ)
type YearEndRecord struct {
	FiscalYear int
	Status     string // closed / reopened
	NetIncome  vo.Money
	EntryID    *vo.EntryID
	EntryNo    string
	ReversalNo string
	ClosedAt   time.Time
	ClosedBy   vo.UserID
	ReopenedAt *time.Time
}

// SuspenseBalance: บัญชีพักที่ยังมียอด ณ วันสิ้นปี (เดบิตเป็นบวก)
type SuspenseBalance struct {
	Account AccountInfo
	Balance vo.Money
}

// YearEnd คือสถานะการปิดบัญชีของปีบัญชีหนึ่ง พร้อมเงื่อนไขก่อนปิด
//
//	Status: open (ยังไม่ปิด — งวดปรับปรุงเปิดอยู่) / closed
//	DeadlineStatus: on_time / overdue (ยังไม่ปิด) / closed
type YearEnd struct {
	FiscalYear     int
	Start, End     time.Time
	Deadline       time.Time
	Status         string
	DeadlineStatus string
	Periods        int
	OpenPeriods    int
	Suspense       []SuspenseBalance // คำนวณเฉพาะตอน GET รายปี
	SettingsReady  bool
	CanClose       bool
	CanReopen      bool
	Record         *YearEndRecord
}

// LedgerSettings คือค่าตั้งของ ledger (พร้อมรหัส / ชื่อ)
type LedgerSettings struct {
	RetainedEarningsAccount *Ref
	IncomeSummaryAccount    *Ref
	ClosingJournal          *Ref
}

// AdjustmentTemplate: บัญชีตั้งต้นของแบบรายการปรับปรุง (nil = ไม่พบในผังของบริษัท)
type AdjustmentTemplate struct {
	Template       string
	BalanceDebit   bool
	BalanceAccount *Ref
	ResultAccount  *Ref
	AutoReverse    bool
}

// ---------- งบการเงินตามแบบกรมพัฒนาธุรกิจการค้า ----------

// DBDColumn คือหัวคอลัมน์ของงบ (ปีปัจจุบัน / ปีก่อน)
type DBDColumn struct {
	Start      *time.Time // งบกำไรขาดทุน
	End        time.Time
	FiscalYear int
}

type DBDStatement struct {
	Columns []DBDColumn
	Rows    []services.DBDRow
}
