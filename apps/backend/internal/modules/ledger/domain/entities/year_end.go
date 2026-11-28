package entities

import (
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// Settings คือค่าตั้งของ ledger ต่อบริษัท (ใช้กับการปิดบัญชีสิ้นปี)
//
//   - RetainedEarningsAccountID: กำไรสะสม-ยังไม่ได้จัดสรร (3302000000 ในผังมาตรฐาน) — ปลายทางของกำไร (ขาดทุน) ประจำปี
//   - IncomeSummaryAccountID: กำไร (ขาดทุน) สุทธิประจำงวด (3303000000) — ไม่บังคับ ถ้าตั้งไว้รายการปิดบัญชีผ่านบัญชีนี้ 2 ขั้น
//   - ClosingJournalID: สมุดที่ใช้ลงรายการปิดบัญชี (GEN)
type Settings struct {
	RetainedEarningsAccountID *vo.AccountID
	IncomeSummaryAccountID    *vo.AccountID
	ClosingJournalID          *vo.JournalID
}

// Ready: มีค่าที่จำเป็นสำหรับปิดบัญชีครบ
func (s Settings) Ready() bool { return s.RetainedEarningsAccountID != nil && s.ClosingJournalID != nil }

// รหัสตั้งต้นในผังบัญชีมาตรฐาน (ทุกประเภทธุรกิจ)
const (
	DefaultRetainedEarningsCode = "3302000000"
	DefaultIncomeSummaryCode    = "3303000000"
	DefaultClosingJournalCode   = "GEN"
)

// YearEndStatus คือสถานะการปิดบัญชีสิ้นปีของปีบัญชีหนึ่ง
type YearEndStatus string

const (
	YearClosed   YearEndStatus = "closed"
	YearReopened YearEndStatus = "reopened"
)

// YearEndState คือข้อมูลการปิดบัญชีสิ้นปี (1 แถวต่อบริษัทต่อปี — ปิดซ้ำหลังเปิดใหม่ใช้แถวเดิม)
type YearEndState struct {
	ID             vo.YearEndClosingID
	CompanyID      vo.CompanyID
	FiscalYear     int
	StartDate      time.Time
	EndDate        time.Time
	Status         YearEndStatus
	NetIncome      vo.Money     // กำไร (ขาดทุน) สุทธิที่ปิดเข้ากำไรสะสม — ขาดทุน = ติดลบ
	EntryID        *vo.EntryID  // รายการปิดบัญชี (nil = ปีที่ไม่มีรายได้ / ค่าใช้จ่าย)
	ReversalID     *vo.EntryID  // รายการกลับตอนเปิดปีใหม่ (ครั้งล่าสุด)
	ClosedBy       vo.UserID
	ClosedAt       time.Time
	ReopenedBy     *vo.UserID
	ReopenedAt     *time.Time
}

// YearEnd คือการปิดบัญชีสิ้นปี
//
// กฎ (ตรวจที่ use case ซึ่งต้องใช้ข้อมูลภายนอก): ทุกงวดของปีปิดแล้ว, บัญชีพักเป็นศูนย์, ค่าตั้งครบ
// กฎในตัว: ปิดซ้ำไม่ได้, เปิดได้เฉพาะปีที่ปิดอยู่
type YearEnd struct{ s YearEndState }

func NewYearEnd(companyID vo.CompanyID, year int, start, end time.Time) *YearEnd {
	return &YearEnd{s: YearEndState{ID: vo.NewID[vo.YearEndClosingID](), CompanyID: companyID, FiscalYear: year, StartDate: start, EndDate: end, Status: YearReopened}}
}

func RehydrateYearEnd(s YearEndState) *YearEnd { return &YearEnd{s: s} }

func (y *YearEnd) State() YearEndState { return y.s }
func (y *YearEnd) IsClosed() bool      { return y.s.Status == YearClosed }

// Close บันทึกผลการปิดบัญชี (entryID = nil เมื่อไม่มีรายการ)
func (y *YearEnd) Close(net vo.Money, entryID *vo.EntryID, by vo.UserID, at time.Time) error {
	if y.IsClosed() {
		return domain.ErrYearAlreadyClosed
	}
	y.s.Status, y.s.NetIncome, y.s.EntryID, y.s.ReversalID = YearClosed, net, entryID, nil
	y.s.ClosedBy, y.s.ClosedAt, y.s.ReopenedBy, y.s.ReopenedAt = by, at, nil, nil
	return nil
}

// Reopen เปิดปีที่ปิดแล้ว (use case กลับรายการปิดบัญชีแล้วส่ง id รายการกลับมา)
func (y *YearEnd) Reopen(reversalID *vo.EntryID, by vo.UserID, at time.Time) error {
	if !y.IsClosed() {
		return domain.ErrYearNotClosed
	}
	y.s.Status, y.s.ReversalID, y.s.ReopenedBy, y.s.ReopenedAt = YearReopened, reversalID, &by, &at
	return nil
}

// SetReversal: บันทึกรายการกลับของการเปิดปี (หลังผ่านรายการกลับแล้ว)
func (y *YearEnd) SetReversal(id vo.EntryID) { y.s.ReversalID = &id }

// ClosingBalance คือยอดสุทธิ (เดบิต - เครดิต) ของบัญชีรายได้ / ค่าใช้จ่ายในปีที่ปิด
type ClosingBalance struct {
	AccountID vo.AccountID
	Net       vo.Money
}

// ClosingLines สร้างบรรทัดรายการปิดบัญชี:
//
//	ทุกบัญชีรายได้ / ค่าใช้จ่ายที่มียอด -> กลับด้านให้เป็นศูนย์
//	ส่วนต่าง (กำไร = เครดิต, ขาดทุน = เดบิต) -> กำไรสะสม
//	ถ้ามีบัญชีสรุปผลกำไรขาดทุน: ปิดเข้าบัญชีสรุปก่อน แล้วโอนบัญชีสรุปเข้ากำไรสะสม (บัญชีสรุปเป็นศูนย์ในรายการเดียวกัน)
//
// net = กำไรสุทธิ (เครดิตเป็นบวก); lines ว่าง = ไม่มีอะไรต้องปิด
func ClosingLines(balances []ClosingBalance, retained vo.AccountID, summary *vo.AccountID, description string) (lines []LineData, net vo.Money) {
	net = decimal.Zero
	for _, b := range balances {
		if b.Net.IsZero() {
			continue
		}
		l := LineData{AccountID: b.AccountID, Description: description, Debit: decimal.Zero, Credit: decimal.Zero}
		if b.Net.IsPositive() {
			l.Credit = b.Net
		} else {
			l.Debit = b.Net.Neg()
		}
		lines = append(lines, l)
		net = net.Sub(b.Net)
	}
	if len(lines) == 0 {
		return nil, decimal.Zero
	}
	target := retained
	if summary != nil {
		target = *summary
	}
	if !net.IsZero() {
		lines = append(lines, balancing(target, net, description))
		if summary != nil {
			// โอนบัญชีสรุปผลกำไรขาดทุนเข้ากำไรสะสม
			lines = append(lines, balancing(*summary, net.Neg(), description), balancing(retained, net, description))
		}
	}
	return lines, net
}

// balancing: amount > 0 = เครดิต (กำไรเข้าส่วนของเจ้าของ), < 0 = เดบิต
func balancing(account vo.AccountID, amount vo.Money, description string) LineData {
	l := LineData{AccountID: account, Description: description, Debit: decimal.Zero, Credit: decimal.Zero}
	if amount.IsPositive() {
		l.Credit = amount
	} else {
		l.Debit = amount.Neg()
	}
	return l
}
