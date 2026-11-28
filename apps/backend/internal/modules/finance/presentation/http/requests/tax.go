package requests

import (
	"net/url"
	"strconv"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

// TaxMonth: ?month=YYYY-MM (บังคับ)
func TaxMonth(v url.Values) (vo.TaxMonth, error) {
	raw := v.Get("month")
	if raw == "" {
		return vo.TaxMonth{}, invalid("month is required (YYYY-MM)")
	}
	return vo.ParseTaxMonth(raw)
}

// WhtForm: ?form=pnd3|pnd53&month=YYYY-MM
func WhtForm(v url.Values) (vo.TaxForm, vo.TaxMonth, error) {
	form, err := vo.ParseTaxForm(v.Get("form"))
	if err != nil {
		return "", vo.TaxMonth{}, err
	}
	if !form.IsWithholding() {
		return "", vo.TaxMonth{}, invalid("form must be pnd3 or pnd53")
	}
	m, err := TaxMonth(v)
	return form, m, err
}

// TaxYear: ?year=YYYY (บังคับ, ปี ค.ศ.)
func TaxYear(v url.Values) (int, error) {
	y, err := strconv.Atoi(v.Get("year"))
	if err != nil || y < 2000 || y > 2200 {
		return 0, invalid("year must be a year (YYYY)")
	}
	return y, nil
}

// ParseTaxFilingFilter: GET /tax-filings?year=&form=&status=
func ParseTaxFilingFilter(v url.Values) (dto.TaxFilingFilter, error) {
	var f dto.TaxFilingFilter
	if v.Get("year") != "" {
		y, err := TaxYear(v)
		if err != nil {
			return f, err
		}
		f.Year = y
	}
	if raw := v.Get("form"); raw != "" {
		form, err := vo.ParseTaxForm(raw)
		if err != nil {
			return f, err
		}
		f.Form = &form
	}
	if raw := v.Get("status"); raw != "" {
		s := vo.TaxFilingStatus(raw)
		if s != vo.FilingFiled && s != vo.FilingCancelled {
			return f, invalid("status must be filed or cancelled")
		}
		f.Status = &s
	}
	return f, nil
}

// TaxFiling: POST /tax-filings — บันทึกการยื่น (ไม่จ่ายเงิน)
type TaxFiling struct {
	Form         string   `json:"form"`
	Month        string   `json:"month"`      // YYYY-MM
	FiledDate    string   `json:"filed_date"` // YYYY-MM-DD
	Amount       *Decimal `json:"amount"`
	Surcharge    *Decimal `json:"surcharge"`
	Penalty      *Decimal `json:"penalty"`
	CarryForward *Decimal `json:"carry_forward"` // ภ.พ.30: ภาษีชำระเกินยกไปเดือนถัดไป
	Reference    string   `json:"reference"`
	Note         string   `json:"note"`
	PaymentID    *string  `json:"payment_id"`
}

func (r TaxFiling) ToData() (entities.TaxFilingData, error) {
	form, err := vo.ParseTaxForm(r.Form)
	if err != nil {
		return entities.TaxFilingData{}, err
	}
	m, err := vo.ParseTaxMonth(r.Month)
	if err != nil {
		return entities.TaxFilingData{}, err
	}
	filed, err := date("filed_date", r.FiledDate)
	if err != nil {
		return entities.TaxFilingData{}, err
	}
	amount, err := r.Amount.required("amount")
	if err != nil {
		return entities.TaxFilingData{}, err
	}
	payment, err := optID[vo.PaymentID]("payment_id", r.PaymentID)
	if err != nil {
		return entities.TaxFilingData{}, err
	}
	return entities.TaxFilingData{Form: form, Month: m, FiledDate: filed, Amount: amount, Surcharge: r.Surcharge.orZero(),
		Penalty: r.Penalty.orZero(), CarryForward: r.CarryForward.orZero(), Reference: r.Reference, Note: r.Note, PaymentID: payment}, nil
}

// CancelFiling: PATCH /tax-filings/{id} {"status":"cancelled"}
type CancelFiling struct {
	Status string `json:"status"`
}

func (r CancelFiling) Validate() error {
	if r.Status != string(vo.FilingCancelled) {
		return invalid(`status can only be changed to "cancelled"`)
	}
	return nil
}
