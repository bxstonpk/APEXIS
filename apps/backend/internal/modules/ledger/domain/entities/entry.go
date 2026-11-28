package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// MaxLines คือจำนวนบรรทัดสูงสุดต่อรายการ
const MaxLines = 500

// LineData คือบรรทัดของรายการบัญชี: ต้องมีเดบิตหรือเครดิตอย่างใดอย่างหนึ่ง (> 0)
// มิติวิเคราะห์ (สาขา / แผนก / ศูนย์ต้นทุน / โครงการ) เป็น id ของ organization — ไม่บังคับ
type LineData struct {
	AccountID     vo.AccountID
	Description   string
	Debit         vo.Money
	Credit        vo.Money
	BranchID      *vo.BranchID
	DepartmentID  *vo.DepartmentID
	CostCenterID  *vo.CostCenterID
	ProjectSiteID *vo.ProjectSiteID
}

type EntryData struct {
	JournalID   vo.JournalID
	Date        time.Time // วันที่ลงบัญชี (กำหนดงวดบัญชี)
	Description string
	Reference   string // เลขที่เอกสารอ้างอิงภายนอก
	Lines       []LineData
	// AdjustingPeriod: ลงในงวดปรับปรุง (งวดที่ 13) ของปีบัญชี — วันที่ต้องเป็นวันสุดท้ายของปี,
	// ลงได้แม้งวด 1–12 ปิดแล้ว จนกว่าจะปิดบัญชีสิ้นปี
	AdjustingPeriod bool
	// Template: รายการปรับปรุงสิ้นงวด (accrued_revenue / accrued_expense / deferred_revenue / prepaid_expense / other)
	// ว่าง = รายการทั่วไป
	Template AdjustmentTemplate
	// AutoReverse: รายการปรับปรุงที่กลับรายการอัตโนมัติ ณ วันแรกของงวดถัดไป
	AutoReverse bool
}

// AdjustmentTemplate คือแบบของรายการปรับปรุงสิ้นงวด
type AdjustmentTemplate string

const (
	TemplateAccruedRevenue  AdjustmentTemplate = "accrued_revenue"  // รายได้ค้างรับ
	TemplateAccruedExpense  AdjustmentTemplate = "accrued_expense"  // ค่าใช้จ่ายค้างจ่าย
	TemplateDeferredRevenue AdjustmentTemplate = "deferred_revenue" // รายได้รับล่วงหน้า
	TemplatePrepaidExpense  AdjustmentTemplate = "prepaid_expense"  // ค่าใช้จ่ายจ่ายล่วงหน้า
	TemplateOther           AdjustmentTemplate = "other"            // อื่น ๆ
)

var AdjustmentTemplates = []AdjustmentTemplate{TemplateAccruedRevenue, TemplateAccruedExpense, TemplateDeferredRevenue, TemplatePrepaidExpense, TemplateOther}

func ParseTemplate(s string) (AdjustmentTemplate, error) {
	for _, t := range AdjustmentTemplates {
		if string(t) == s {
			return t, nil
		}
	}
	return "", domain.ErrInvalidTemplate
}

// TemplateAccounts: รหัสบัญชีตั้งต้นของแต่ละแบบในผังมาตรฐาน — [บัญชีงบแสดงฐานะ, บัญชีกำไรขาดทุน]
// และด้านของบัญชีงบแสดงฐานะ (true = เดบิต) — ผู้ใช้เปลี่ยนบัญชี / บรรทัดได้
var TemplateAccounts = map[AdjustmentTemplate]struct {
	BalanceCode, ResultCode string
	BalanceDebit            bool
}{
	TemplateAccruedRevenue:  {"1103020400", "4101000000", true},  // Dr รายได้ค้างรับ / Cr รายได้ค่าบริการ
	TemplateAccruedExpense:  {"2102020100", "5313000000", false}, // Dr ค่าใช้จ่าย / Cr ค่าใช้จ่ายค้างจ่าย
	TemplateDeferredRevenue: {"2102020200", "4101000000", false}, // Dr รายได้ / Cr รายได้รับล่วงหน้า
	TemplatePrepaidExpense:  {"1103020600", "5313000000", true},  // Dr ค่าใช้จ่ายจ่ายล่วงหน้า / Cr ค่าใช้จ่าย
}

// IsAdjustment: เป็นรายการปรับปรุงสิ้นงวด
func (d EntryData) IsAdjustment() bool { return d.Template != "" }

// Posting คือข้อมูลที่ได้ตอนผ่านรายการ
type Posting struct {
	EntryNo  string // เช่น JV2026-000001 — เรียงไม่ขาดตอนแยกตามสมุดและปีบัญชี
	PeriodID vo.FiscalPeriodID
	By       vo.UserID
	At       time.Time
}

// EntryState คือสถานะทั้งหมดของรายการ (ใช้ใน mapper / Rehydrate)
type EntryState struct {
	ID         vo.EntryID
	CompanyID  vo.CompanyID
	Data       EntryData
	Status     vo.EntryStatus
	Posting    *Posting
	ReversalOf *vo.EntryID // รายการนี้กลับรายการของ id นี้
	ReversedBy *vo.EntryID // รายการนี้ถูกกลับโดย id นี้
	CreatedBy  vo.UserID
	Closing    bool // รายการปิดบัญชีสิ้นปี (และรายการกลับของมัน)
	// AutoReversalID: รายการกลับอัตโนมัติของรายการปรับปรุงนี้ (draft = รอผ่านเมื่อมีงวดถัดไป)
	AutoReversalID *vo.EntryID
	// AutoReversal: รายการนี้คือรายการกลับอัตโนมัติ (ReversalOf = รายการปรับปรุงต้นฉบับ)
	AutoReversal bool
}

// Entry คือรายการบัญชี (ใบสำคัญ)
//
// กฎ:
//   - draft แก้ / ลบได้ และยังไม่ต้องสมดุล
//   - ผ่านรายการ (posted) ได้เมื่อมีอย่างน้อย 2 บรรทัด, ยอดรวม > 0 และเดบิต = เครดิต
//   - posted แก้ / ลบไม่ได้ — แก้ไขด้วยการกลับรายการ (reversal) ซึ่งสร้างรายการใหม่ที่สลับเดบิต / เครดิต
type Entry struct{ s EntryState }

// NewEntry: amountDecimals คือจำนวนทศนิยมของเงินที่บริษัทกำหนด (organization.amount_decimals)
func NewEntry(companyID vo.CompanyID, createdBy vo.UserID, d EntryData, amountDecimals int) (*Entry, error) {
	n, err := normalizeEntry(d, amountDecimals)
	if err != nil {
		return nil, err
	}
	return &Entry{s: EntryState{
		ID: vo.NewID[vo.EntryID](), CompanyID: companyID, Data: n, Status: vo.EntryDraft, CreatedBy: createdBy,
	}}, nil
}

func RehydrateEntry(s EntryState) *Entry { return &Entry{s: s} }

func (e *Entry) ID() vo.EntryID          { return e.s.ID }
func (e *Entry) CompanyID() vo.CompanyID { return e.s.CompanyID }
func (e *Entry) Data() EntryData         { return e.s.Data }
func (e *Entry) Status() vo.EntryStatus  { return e.s.Status }
func (e *Entry) State() EntryState       { return e.s }

// Totals คืนยอดรวมเดบิต / เครดิต
func (e *Entry) Totals() (debit, credit vo.Money) {
	debit, credit = decimal.Zero, decimal.Zero
	for _, l := range e.s.Data.Lines {
		debit = debit.Add(l.Debit)
		credit = credit.Add(l.Credit)
	}
	return debit, credit
}

// AccountIDs คืน id บัญชีที่ใช้ (ไม่ซ้ำ ตามลำดับบรรทัด)
func (e *Entry) AccountIDs() []vo.AccountID {
	seen := map[vo.AccountID]bool{}
	var out []vo.AccountID
	for _, l := range e.s.Data.Lines {
		if !seen[l.AccountID] {
			seen[l.AccountID] = true
			out = append(out, l.AccountID)
		}
	}
	return out
}

func (e *Entry) Update(d EntryData, amountDecimals int) error {
	if e.s.Status != vo.EntryDraft {
		return domain.ErrEntryNotDraft
	}
	n, err := normalizeEntry(d, amountDecimals)
	if err != nil {
		return err
	}
	e.s.Data = n
	return nil
}

func (e *Entry) EnsureDeletable() error {
	if e.s.Status != vo.EntryDraft {
		return domain.ErrEntryNotDraft
	}
	return nil
}

// EnsurePostable ตรวจกฎความสมบูรณ์ของรายการ (ไม่รวมกฎที่ต้องใช้ข้อมูลภายนอก เช่น งวดบัญชี / บัญชี active)
func (e *Entry) EnsurePostable() error {
	if e.s.Status != vo.EntryDraft {
		return domain.ErrEntryNotDraft
	}
	debit, credit := e.Totals()
	if len(e.s.Data.Lines) < 2 || debit.IsZero() {
		return domain.ErrEntryIncomplete
	}
	if !debit.Equal(credit) {
		return domain.ErrUnbalanced
	}
	return nil
}

func (e *Entry) Post(p Posting) error {
	if err := e.EnsurePostable(); err != nil {
		return err
	}
	e.s.Status = vo.EntryPosted
	e.s.Posting = &p
	return nil
}

// Reversal สร้างรายการกลับ (draft) ที่สลับเดบิต / เครดิตของทุกบรรทัด — ต้นฉบับต้อง posted
// use case ผ่านรายการกลับแล้วเรียก MarkReversed กับต้นฉบับใน transaction เดียวกัน
func (e *Entry) Reversal(date time.Time, description string, by vo.UserID) (*Entry, error) {
	if e.s.Status != vo.EntryPosted || e.s.Posting == nil {
		return nil, domain.ErrEntryNotPosted
	}
	if description == "" {
		description = "Reversal of " + e.s.Posting.EntryNo
	}
	lines := make([]LineData, len(e.s.Data.Lines))
	for i, l := range e.s.Data.Lines {
		l.Debit, l.Credit = l.Credit, l.Debit
		lines[i] = l
	}
	// จำนวนเงินเท่ากับต้นฉบับซึ่งผ่านการตรวจทศนิยมแล้ว (ทศนิยมของบริษัทเพิ่มได้อย่างเดียว) จึงไม่ตรวจซ้ำ
	r, err := NewEntry(e.s.CompanyID, by, EntryData{
		JournalID: e.s.Data.JournalID, Date: date, Description: description,
		Reference: e.s.Posting.EntryNo, Lines: lines,
		// กลับรายการงวดปรับปรุง ณ วันเดียวกัน = อยู่ในงวดปรับปรุงด้วย
		AdjustingPeriod: e.s.Data.AdjustingPeriod && date.Equal(e.s.Data.Date),
	}, numeric.MaxScale)
	if err != nil {
		return nil, err
	}
	id := e.s.ID
	r.s.ReversalOf = &id
	r.s.Closing = e.s.Closing
	return r, nil
}

// NewAutoReversal สร้างรายการกลับอัตโนมัติ (draft) ของรายการปรับปรุงที่ผ่านแล้ว ณ วันแรกของงวดถัดไป
// ต้นฉบับยังเป็น posted (ไม่ใช่ "ถูกกลับรายการ") — ผูกกันสองทางด้วย ReversalOf / AutoReversalID
func (e *Entry) NewAutoReversal(date time.Time, by vo.UserID) (*Entry, error) {
	if e.s.Status != vo.EntryPosted || e.s.Posting == nil || !e.s.Data.AutoReverse {
		return nil, domain.ErrEntryNotPosted
	}
	lines := make([]LineData, len(e.s.Data.Lines))
	for i, l := range e.s.Data.Lines {
		l.Debit, l.Credit = l.Credit, l.Debit
		lines[i] = l
	}
	r, err := NewEntry(e.s.CompanyID, by, EntryData{
		JournalID: e.s.Data.JournalID, Date: date, Description: "Auto reversal of " + e.s.Posting.EntryNo,
		Reference: e.s.Posting.EntryNo, Lines: lines,
	}, numeric.MaxScale)
	if err != nil {
		return nil, err
	}
	id := e.s.ID
	r.s.ReversalOf, r.s.AutoReversal = &id, true
	rid := r.s.ID
	e.s.AutoReversalID = &rid
	return r, nil
}

// AutoReversalOf คืน id รายการกลับอัตโนมัติ (nil = ไม่มี)
func (e *Entry) AutoReversalOf() *vo.EntryID { return e.s.AutoReversalID }

// MarkClosing: รายการปิดบัญชีสิ้นปี (ระบบสร้าง) — งบกำไรขาดทุนไม่นับรายการนี้ และลงในงวดที่ปิดแล้วได้
func (e *Entry) MarkClosing() { e.s.Closing = true }

func (e *Entry) IsClosing() bool { return e.s.Closing }

func (e *Entry) MarkReversed(by vo.EntryID) error {
	if e.s.Status != vo.EntryPosted {
		return domain.ErrEntryNotPosted
	}
	e.s.Status = vo.EntryReversed
	e.s.ReversedBy = &by
	return nil
}

func normalizeEntry(d EntryData, amountDecimals int) (EntryData, error) {
	var err error
	if !vo.IsDateOnly(d.Date) {
		return EntryData{}, domain.ErrInvalidDate
	}
	out := EntryData{JournalID: d.JournalID, Date: d.Date, AdjustingPeriod: d.AdjustingPeriod, Template: d.Template, AutoReverse: d.AutoReverse}
	if d.Template == "" && d.AutoReverse {
		return EntryData{}, domain.ErrInvalidTemplate // กลับรายการอัตโนมัติใช้กับรายการปรับปรุงเท่านั้น
	}
	if out.Description, err = vo.OptionalText(d.Description, 500); err != nil {
		return EntryData{}, err
	}
	if out.Reference, err = vo.OptionalText(d.Reference, 100); err != nil {
		return EntryData{}, err
	}
	if len(d.Lines) == 0 || len(d.Lines) > MaxLines {
		return EntryData{}, fmt.Errorf("%w: an entry must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	out.Lines = make([]LineData, len(d.Lines))
	for i, l := range d.Lines {
		n := i + 1
		if l.Debit.IsNegative() || l.Credit.IsNegative() || !numeric.InRange(l.Debit) || !numeric.InRange(l.Credit) {
			return EntryData{}, fmt.Errorf("%w: line %d amount must be zero or positive and below 10^%d", domain.ErrInvalidLine, n, numeric.MaxIntegerDigits)
		}
		if l.Debit.IsPositive() == l.Credit.IsPositive() {
			return EntryData{}, fmt.Errorf("%w: line %d must have either a debit or a credit amount", domain.ErrInvalidLine, n)
		}
		if !numeric.FitsScale(l.Debit, amountDecimals) || !numeric.FitsScale(l.Credit, amountDecimals) {
			return EntryData{}, fmt.Errorf("%w: line %d amount has more than %d decimal places", domain.ErrInvalidLine, n, amountDecimals)
		}
		if l.Description, err = vo.OptionalText(l.Description, 500); err != nil {
			return EntryData{}, fmt.Errorf("%w: line %d description is too long", domain.ErrInvalidLine, n)
		}
		out.Lines[i] = l
	}
	return out, nil
}
