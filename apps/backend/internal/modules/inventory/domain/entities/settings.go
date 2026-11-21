package entities

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// Settings คือค่าตั้งบัญชีสินค้าของบริษัท
//
//   - Defaults: บัญชีค่าเริ่มต้น (ใช้เมื่อทั้งสินค้าและหมวดไม่ได้ตั้ง)
//   - CogsAccountID / JournalID: ใช้ลงรายการปรับปรุงสินค้าคงเหลือสิ้นงวด (ระบบ periodic)
type Settings struct {
	Defaults      vo.GLAccounts
	CogsAccountID *vo.AccountID
	JournalID     *vo.JournalID
}

// AccountIDs คือบัญชีทั้งหมดที่ตั้งไว้ (ใช้ตรวจกับ ledger)
func (s Settings) AccountIDs() []vo.AccountID {
	out := s.Defaults.IDs()
	if s.CogsAccountID != nil && !slices.Contains(out, *s.CogsAccountID) {
		out = append(out, *s.CogsAccountID)
	}
	return out
}

// SystemChange: ใช้ระบบบัญชี System ตั้งแต่ปีบัญชี FromYear เป็นต้นไป
type SystemChange struct {
	FromYear int
	System   vo.AccountingSystem
}

// NewSystemChange: เปลี่ยนระบบได้เฉพาะปีบัญชีที่ยังไม่มีเอกสารสต็อกที่ผ่านแล้ว
// (เปลี่ยนกลางปีที่มีรายการแล้ว ยอดในบัญชีปีนั้นจะปนกันระหว่างสองวิธี)
func NewSystemChange(fromYear int, system vo.AccountingSystem, yearHasPostings bool) (SystemChange, error) {
	if fromYear < 1900 || fromYear > 9999 {
		return SystemChange{}, domain.ErrInvalidYear
	}
	if _, err := vo.ParseAccountingSystem(string(system)); err != nil {
		return SystemChange{}, err
	}
	if yearHasPostings {
		return SystemChange{}, domain.ErrSystemLocked
	}
	return SystemChange{FromYear: fromYear, System: system}, nil
}

// SystemFor คืนระบบของปีบัญชี year: การตั้งค่าล่าสุดที่ FromYear <= year — ไม่ได้ตั้ง = perpetual
func SystemFor(changes []SystemChange, year int) vo.AccountingSystem {
	system, best := vo.Perpetual, 0
	for _, c := range changes {
		if c.FromYear <= year && c.FromYear > best {
			system, best = c.System, c.FromYear
		}
	}
	return system
}

// ---------- ปรับปรุงสินค้าคงเหลือสิ้นงวด (periodic) ----------

type ClosingStatus string

const (
	ClosingPosted    ClosingStatus = "posted"
	ClosingCancelled ClosingStatus = "cancelled"
)

type ClosingState struct {
	ID          vo.ClosingID
	CompanyID   vo.CompanyID
	PeriodID    vo.FiscalPeriodID
	FiscalYear  int
	PeriodEnd   time.Time
	EntryID     *vo.EntryID // nil = ไม่มีส่วนต่างต้องปรับ
	EntryNo     string
	Adjustment  vo.Money // สินค้าคงเหลือเพิ่ม (+) / ลด (-) จากรายการปรับปรุง
	Status      ClosingStatus
	CreatedBy   vo.UserID
	CancelledBy *vo.UserID
	CancelledAt *time.Time
}

// PeriodClosing คือการปรับปรุงสินค้าคงเหลือสิ้นงวดหนึ่งครั้ง — งวดหนึ่งมีที่ posted ได้ครั้งเดียว
// ทำใหม่ต้องยกเลิกของเดิม (กลับรายการบัญชี) ก่อน
type PeriodClosing struct{ s ClosingState }

func NewPeriodClosing(companyID vo.CompanyID, periodID vo.FiscalPeriodID, fiscalYear int, periodEnd time.Time, by vo.UserID) *PeriodClosing {
	return &PeriodClosing{s: ClosingState{
		ID: vo.NewID[vo.ClosingID](), CompanyID: companyID, PeriodID: periodID, FiscalYear: fiscalYear, PeriodEnd: periodEnd,
		Status: ClosingPosted, CreatedBy: by, Adjustment: decimal.Zero,
	}}
}

func RehydratePeriodClosing(s ClosingState) *PeriodClosing { return &PeriodClosing{s: s} }

func (c *PeriodClosing) ID() vo.ClosingID    { return c.s.ID }
func (c *PeriodClosing) State() ClosingState { return c.s }

// SetEntry: รายการบัญชีที่ลงไว้ และยอดที่สินค้าคงเหลือเปลี่ยน
func (c *PeriodClosing) SetEntry(id vo.EntryID, no string, adjustment vo.Money) {
	c.s.EntryID, c.s.EntryNo, c.s.Adjustment = &id, no, adjustment
}

func (c *PeriodClosing) Cancel(by vo.UserID, at time.Time) error {
	if c.s.Status != ClosingPosted {
		return domain.ErrClosingNotFound
	}
	c.s.Status, c.s.CancelledBy, c.s.CancelledAt = ClosingCancelled, &by, &at
	return nil
}
