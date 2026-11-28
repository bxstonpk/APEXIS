package dto

import (
	"time"

	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

// ---------- แบบยื่นภาษีรายเดือน (ภ.ง.ด.3 / 53 / ภ.พ.30) ----------

// WhtFormLine: 1 บรรทัดใบจ่ายที่หักภาษีของแบบ ภ.ง.ด. — Seq = ลำดับที่ในใบแนบ (1 ใบจ่าย = 1 ราย = 1 หนังสือรับรอง 50 ทวิ)
type WhtFormLine struct {
	Seq           int
	PaymentID     string
	DocumentNo    string
	CertificateNo string
	Date          time.Time
	TaxID         string
	BranchNo      string
	Name          string
	Address       string
	IncomeType    string // ประเภทเงินได้ตาม 50 ทวิ (ประเภทภาษีหัก ณ ที่จ่าย ณ วันนี้ — ไม่มี = other)
	WhtTypeName   string
	Rate          vo.Money
	Base          vo.Money
	Wht           vo.Money
}

// WhtFormSummary: ใบแนบ / แบบยื่น — Payees = จำนวนราย (ใบจ่าย), Documents = จำนวนแผ่นใบแนบไม่นับ (ใช้จำนวนบรรทัด)
type WhtFormSummary struct {
	Payees int
	Lines  int
	Base   vo.Money
	Wht    vo.Money
}

type WhtForm struct {
	Form    vo.TaxForm
	Month   vo.TaxMonth
	Summary WhtFormSummary
	Lines   []WhtFormLine
	Filing  *TaxFiling // nil = ยังไม่ยื่น / ไม่มีสิทธิ์ดูทะเบียนการยื่น
}

// WhtMonthTotal: ยอดภาษีหัก ณ ที่จ่ายของเดือน / แบบ (GROUP BY ใน DB)
type WhtMonthTotal struct {
	Month  vo.TaxMonth
	Form   vo.TaxForm
	Payees int
	Base   vo.Money
	Wht    vo.Money
}

// VatAmounts: ยอดรวมฐาน / ภาษี / จำนวนเอกสาร
type VatAmounts struct {
	Base      vo.Money
	Tax       vo.Money
	Documents int
}

// Pp30: ภ.พ.30 ของเดือน — Net = ภาษีขาย - ภาษีซื้อ - ภาษีชำระเกินยกมา; Payable / Overpaid แยกตามเครื่องหมาย
type Pp30 struct {
	Month       vo.TaxMonth
	Sales       VatAmounts // ยอดขายที่ต้องเสียภาษี + ภาษีขาย
	ZeroRated   vo.Money   // ยอดขายอัตรา 0 % (ยังไม่แยกในระบบ = 0)
	Exempt      vo.Money   // ยอดขายที่ได้รับยกเว้น (ยังไม่แยกในระบบ = 0)
	Purchases   VatAmounts // ยอดซื้อที่มีสิทธิ์นำภาษีซื้อมาหัก + ภาษีซื้อ
	CarryIn     vo.Money   // ภาษีชำระเกินยกมา (ยกไปของบันทึกการยื่นเดือนก่อน)
	Net         vo.Money
	Payable     vo.Money
	Overpaid    vo.Money
	Filing      *TaxFiling
	PrevFiled   bool // มีบันทึกการยื่นของเดือนก่อน (อ่านภาษีชำระเกินยกมาได้)
	CanSeeFiles bool
}

// VatReportRow: แถวรายงานภาษีขาย / ภาษีซื้อตามแบบกรมสรรพากร
type VatReportRow struct {
	Seq        int
	Date       time.Time
	DocumentNo string
	PartyName  string
	TaxID      string
	BranchNo   string
	Base       vo.Money
	Tax        vo.Money
}

type VatReport struct {
	Kind  string // output = ภาษีขาย, input = ภาษีซื้อ
	Month vo.TaxMonth
	Rows  []VatReportRow
	Base  vo.Money
	Tax   vo.Money
}

// TaxOverviewCell: ยอดของแบบในเดือน + บันทึกการยื่น
type TaxOverviewCell struct {
	Amount   vo.Money // ภ.ง.ด. = ภาษีที่หัก; ภ.พ.30 = ภาษีที่ต้องชำระ (ติดลบ = ชำระเกิน)
	Payees   int      // ภ.ง.ด. เท่านั้น
	Filing   *TaxFiling
	Computed bool // false = เดือนในอนาคต (ไม่คำนวณ)
}

type TaxOverviewMonth struct {
	Month vo.TaxMonth
	Pnd3  TaxOverviewCell
	Pnd53 TaxOverviewCell
	Pp30  TaxOverviewCell
}

type TaxOverview struct {
	Year        int
	Months      []TaxOverviewMonth
	CanSeeFiles bool
}

// ---------- ทะเบียนการยื่น ----------

type TaxFiling struct {
	ID           string
	Form         vo.TaxForm
	Month        vo.TaxMonth
	FiledDate    time.Time
	Amount       vo.Money
	Surcharge    vo.Money
	Penalty      vo.Money
	CarryForward vo.Money
	Total        vo.Money
	Reference    string
	Note         string
	PaymentID    *string
	Status       vo.TaxFilingStatus
	CreatedBy    string
	CreatedAt    time.Time
	CancelledAt  *time.Time
}

// TaxFilingFilter: GET /tax-filings?year=&form=&status=
type TaxFilingFilter struct {
	Year   int
	Form   *vo.TaxForm
	Status *vo.TaxFilingStatus
}

type TaxFilingByID struct {
	Actor
	ID vo.TaxFilingID
}
