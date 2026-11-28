package valueobjects

import (
	"time"

	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/finance/domain/errors"
)

// TaxFilingID คือรหัสบันทึกการยื่นแบบภาษี
type TaxFilingID uuid.UUID

func (id TaxFilingID) String() string { return uuid.UUID(id).String() }

// TaxForm คือแบบยื่นภาษีรายเดือน
//   - pnd3: ภ.ง.ด.3 (ภาษีหัก ณ ที่จ่ายของบุคคลธรรมดา)
//   - pnd53: ภ.ง.ด.53 (ภาษีหัก ณ ที่จ่ายของนิติบุคคล)
//   - pp30: ภ.พ.30 (ภาษีมูลค่าเพิ่ม)
type TaxForm string

const (
	FormPnd3  TaxForm = "pnd3"
	FormPnd53 TaxForm = "pnd53"
	FormPp30  TaxForm = "pp30"
)

var TaxForms = []TaxForm{FormPnd3, FormPnd53, FormPp30}

func ParseTaxForm(raw string) (TaxForm, error) {
	return parseEnum(raw, TaxForms, domain.ErrInvalidTaxForm)
}

// IsWithholding: ภ.ง.ด.3 / 53
func (f TaxForm) IsWithholding() bool { return f == FormPnd3 || f == FormPnd53 }

// PayeeTaxType: ประเภทผู้มีเงินได้ของแบบ ภ.ง.ด. (ภ.ง.ด.3 = บุคคลธรรมดา, ภ.ง.ด.53 = นิติบุคคล)
func (f TaxForm) PayeeTaxType() TaxType {
	if f == FormPnd3 {
		return TaxIndividual
	}
	return TaxJuristic
}

// TaxMonth คือเดือนภาษี (วันที่ 1 ของเดือน, UTC)
type TaxMonth struct{ start time.Time }

// ParseTaxMonth: "YYYY-MM" (ปี ค.ศ.)
func ParseTaxMonth(raw string) (TaxMonth, error) {
	t, err := time.Parse("2006-01", raw)
	if err != nil || t.Year() < 2000 || t.Year() > 2200 {
		return TaxMonth{}, domain.ErrInvalidMonth
	}
	return TaxMonth{start: t.UTC()}, nil
}

// TaxMonthOf: เดือนภาษีของวันที่
func TaxMonthOf(t time.Time) TaxMonth {
	return TaxMonth{start: time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)}
}

func (m TaxMonth) IsZero() bool     { return m.start.IsZero() }
func (m TaxMonth) Start() time.Time { return m.start }
func (m TaxMonth) End() time.Time   { return m.start.AddDate(0, 1, -1) }
func (m TaxMonth) Prev() TaxMonth   { return TaxMonth{start: m.start.AddDate(0, -1, 0)} }
func (m TaxMonth) String() string   { return m.start.Format("2006-01") }

// TaxFilingStatus: filed = ยื่นแล้ว, cancelled = ยกเลิกบันทึก (ยื่นใหม่ได้)
type TaxFilingStatus string

const (
	FilingFiled     TaxFilingStatus = "filed"
	FilingCancelled TaxFilingStatus = "cancelled"
)
