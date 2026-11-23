// Package dto คือข้อมูลขาเข้า (Input / Filter) และขาออก (read model) ของ use case ledger
package dto

import (
	"time"

	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- Input ----------

// Actor คือผู้เรียกและบริษัทที่กำลังทำงาน
type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

// Create / Update / ByID / List เป็น input รูปแบบกลาง — D = entities.*Data, ID = vo.*ID, F = *Filter
type Create[D any] struct {
	Actor
	Data D
}

type Update[ID any, D any] struct {
	Actor
	ID   ID
	Data D
}

type ByID[ID any] struct {
	Actor
	ID ID
}

type List[F any] struct {
	Actor
	Filter F
}

// CreateEntry: Post = true คือสร้างแล้วผ่านรายการทันทีใน transaction เดียว
type CreateEntry[D any] struct {
	Actor
	Data D
	Post bool
}

// ReverseEntry: กลับรายการ id ด้วยรายการใหม่ ณ วันที่ Date
type ReverseEntry struct {
	Actor
	ID          vo.EntryID
	Date        time.Time
	Description string
}

type DateRange struct {
	Actor
	Start time.Time
	End   time.Time
}

type AsOf struct {
	Actor
	Date time.Time
}

type AccountLedgerInput struct {
	Actor
	AccountID vo.AccountID
	Start     time.Time
	End       time.Time
	Page      listing.Page
}

// ---------- Filter (จาก URL query) ----------
//
// *SortKeys คือคีย์สาธารณะที่ client ส่งใน ?sort= ได้ (presentation ตรวจ, repository map เป็น column ผ่าน whitelist)

var (
	AccountSortKeys    = []string{"code", "name", "type"}
	DefaultAccountSort = []listing.Sort{{Key: "code"}}
)

// AccountFilter: GET /accounts?q=&type=&status=&postable=&parent_id=&sort=&page=&page_size=
type AccountFilter struct {
	Search   string
	Types    []vo.AccountType
	Statuses []vo.Status
	Postable *bool
	ParentID *vo.AccountID
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	JournalSortKeys    = []string{"code", "name"}
	DefaultJournalSort = []listing.Sort{{Key: "code"}}
)

// JournalFilter: GET /journals?q=&status=&sort=&page=&page_size=
type JournalFilter struct {
	Search   string
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

var (
	EntrySortKeys    = []string{"entry_date", "entry_no", "created_at"}
	DefaultEntrySort = []listing.Sort{{Key: "entry_date", Desc: true}, {Key: "entry_no", Desc: true}}
)

// EntryFilter: GET /journal-entries?q=&status=&journal_id=&account_id=&start_date=&end_date=&sort=&page=&page_size=
type EntryFilter struct {
	Search    string // เลขที่เอกสาร / คำอธิบาย / เลขอ้างอิง
	Statuses  []vo.EntryStatus
	JournalID *vo.JournalID
	AccountID *vo.AccountID // รายการที่มีบรรทัดใช้บัญชีนี้
	StartDate *time.Time
	EndDate   *time.Time
	Sort      []listing.Sort
	Page      listing.Page
}

// ---------- Output (read model) ----------

// Ref คือข้อมูลอ้างอิงสั้นๆ ของ record ที่เกี่ยวข้อง (ได้จาก JOIN)
type Ref struct {
	ID   string
	Code string
	Name string
}

type Account struct {
	ID       string
	Code     string
	Name     string
	NameEN   string
	Type     string
	Parent   *Ref
	Postable bool
	Status   string
	// Description คำอธิบายการใช้บัญชี
	Description string
}

// DefaultChartResult ผลการสร้างผังบัญชีมาตรฐาน
type DefaultChartResult struct {
	Accounts int // จำนวนบัญชีที่สร้าง
	Journals int // จำนวนสมุดรายวันที่สร้าง (ข้ามรหัสที่มีอยู่แล้ว)
}

type Journal struct {
	ID             string
	Code           string
	Name           string
	NumberTemplate string
	Status         string
}

type EntrySummary struct {
	ID          string
	EntryNo     string // ว่างจนกว่าจะผ่านรายการ
	Date        time.Time
	Journal     Ref
	Description string
	Reference   string
	Status      string
	TotalDebit  vo.Money
	TotalCredit vo.Money
	PostedAt    *time.Time
}

type EntryLine struct {
	LineNo        int
	Account       Ref
	Description   string
	Debit         vo.Money
	Credit        vo.Money
	BranchID      *string
	DepartmentID  *string
	CostCenterID  *string
	ProjectSiteID *string
}

type Entry struct {
	EntrySummary
	FiscalPeriodID *string
	ReversalOfID   *string
	ReversedByID   *string
	CreatedBy      string
	PostedBy       *string
	Lines          []EntryLine
}

// ---------- รายงาน ----------
//
// ยอดในงบทดลอง / บัญชีแยกประเภท เป็นแบบ "เดบิตเป็นบวก" (เดบิต - เครดิต)
// ยอดในงบการเงินเป็นตามด้านปกติของหมวด (ดู services.StatementLine)

type AccountInfo struct {
	ID   string
	Code string
	Name string
	Type string
}

type TrialBalanceRow struct {
	Account AccountInfo
	Opening vo.Money
	Debit   vo.Money
	Credit  vo.Money
	Closing vo.Money
}

type TrialBalance struct {
	Start, End         time.Time
	Rows               []TrialBalanceRow
	TotalDebit         vo.Money // ยอดเคลื่อนไหวรวม (เท่ากับ TotalCredit เสมอ)
	TotalCredit        vo.Money
	PriorYearsEarnings vo.Money // (เดบิต - เครดิต) ของรายได้ / ค่าใช้จ่ายปีก่อนๆ ที่ยังไม่ปิดบัญชี
}

type LedgerLine struct {
	EntryID     string
	EntryNo     string
	Date        time.Time
	Description string
	Reference   string
	Debit       vo.Money
	Credit      vo.Money
	Balance     vo.Money // ยอดคงเหลือสะสม (รวมยอดยกมา)
}

type AccountLedger struct {
	Account    AccountInfo
	Start, End time.Time
	Opening    vo.Money
	Lines      listing.Result[LedgerLine]
}

type BalanceSheet struct {
	AsOf                      time.Time
	Assets                    []services.StatementLine
	TotalAssets               vo.Money
	Liabilities               []services.StatementLine
	TotalLiabilities          vo.Money
	Equity                    []services.StatementLine
	RetainedEarningsPrior     vo.Money // กำไร (ขาดทุน) ของปีก่อนๆ ที่ยังไม่ได้ปิดเข้าบัญชีส่วนของเจ้าของ
	CurrentYearEarnings       vo.Money // กำไร (ขาดทุน) ปีปัจจุบันถึง AsOf
	TotalEquity               vo.Money
	TotalLiabilitiesAndEquity vo.Money
	Suspense                  []services.StatementLine // บัญชีพักที่ยังมียอดค้าง (ควรเป็นศูนย์)
	TotalSuspense             vo.Money                 // เดบิตเป็นบวก: สินทรัพย์ + บัญชีพัก = หนี้สิน + ส่วนของเจ้าของ
}

type IncomeStatement struct {
	Start, End    time.Time
	Revenue       []services.StatementLine
	TotalRevenue  vo.Money
	Expenses      []services.StatementLine
	TotalExpenses vo.Money
	NetIncome     vo.Money
	// Suspense: ยอดเคลื่อนไหวของบัญชีพักในช่วง (เดบิตเป็นบวก) — แสดงแยก ไม่รวมใน NetIncome
	// (ยอดคงเหลือของบัญชีพักอยู่ในงบแสดงฐานะการเงิน)
	Suspense      []services.StatementLine
	TotalSuspense vo.Money
}

// ---------- dashboard ----------

// IncomeSummary: รายได้ / ค่าใช้จ่าย (ด้านปกติเป็นบวก) และกำไรสุทธิ — ไม่รวมบัญชีพัก
type IncomeSummary struct {
	Revenue   vo.Money
	Expense   vo.Money
	NetIncome vo.Money
}

type MonthlyIncome struct {
	Month   time.Time // วันที่ 1 ของเดือน
	Revenue vo.Money
	Expense vo.Money
}

type RecentEntry struct {
	ID          string
	EntryNo     string
	Date        time.Time
	Description string
	Amount      vo.Money // ยอดเดบิตรวม
}

type DashboardPeriod struct {
	FiscalYear int
	Start      time.Time
	End        time.Time
	Open       bool
}

// AccountingDashboard คือภาพรวมแอปบัญชีของช่วง [Start, End]
type AccountingDashboard struct {
	Start           time.Time
	End             time.Time
	Income          IncomeSummary
	Previous        IncomeSummary // ช่วงก่อนหน้าความยาวเท่ากัน
	Monthly         []MonthlyIncome
	SuspenseBalance vo.Money // ยอดบัญชีพัก ณ End (เดบิตเป็นบวก) — ไม่ศูนย์ = ต้องโอนออกก่อนปิดงบ
	DraftCount      int64
	PostedCount     int64
	RecentEntries   []RecentEntry
	Period          *DashboardPeriod // งวดที่ครอบคลุม End (nil = ไม่มีงวด)
}
