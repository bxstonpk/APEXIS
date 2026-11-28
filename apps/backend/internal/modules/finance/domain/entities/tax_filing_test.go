package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

func TestTaxFiling(t *testing.T) {
	m, err := vo.ParseTaxMonth("2026-11")
	if err != nil || m.Start() != time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) || m.End() != time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC) ||
		m.Prev().String() != "2026-10" {
		t.Fatalf("month = %v %v", m, err)
	}
	for _, raw := range []string{"", "2026-13", "26-11", "2026/11"} {
		if _, err := vo.ParseTaxMonth(raw); !errors.Is(err, domain.ErrInvalidMonth) {
			t.Fatalf("month %q = %v", raw, err)
		}
	}
	at := time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC)
	d := func(form vo.TaxForm, amount, carry string) entities.TaxFilingData {
		return entities.TaxFilingData{Form: form, Month: m, FiledDate: at, Amount: decimal.RequireFromString(amount), CarryForward: decimal.RequireFromString(carry)}
	}
	cases := []struct {
		name string
		data entities.TaxFilingData
		ok   bool
	}{
		{"pnd53", d(vo.FormPnd53, "320", "0"), true},
		{"pp30 overpaid", d(vo.FormPp30, "0", "70"), true},
		{"pnd carry forward", d(vo.FormPnd3, "60", "1"), false},
		{"pp30 both", d(vo.FormPp30, "10", "70"), false},
		{"negative", d(vo.FormPnd53, "-1", "0"), false},
		{"too many decimals", d(vo.FormPnd53, "1.001", "0"), false},
	}
	for _, c := range cases {
		_, err := entities.NewTaxFiling(vo.CompanyID{}, c.data, vo.UserID{}, at, 2)
		if (err == nil) != c.ok {
			t.Fatalf("%s: err = %v", c.name, err)
		}
	}
	early := d(vo.FormPnd53, "1", "0")
	early.FiledDate = time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if _, err := entities.NewTaxFiling(vo.CompanyID{}, early, vo.UserID{}, at, 2); !errors.Is(err, domain.ErrInvalidTaxFiling) {
		t.Fatalf("filed before month = %v", err)
	}
	f, _ := entities.NewTaxFiling(vo.CompanyID{}, d(vo.FormPnd53, "300", "0"), vo.UserID{}, at, 2)
	if err := f.Cancel(vo.UserID{}, at); err != nil || f.Cancel(vo.UserID{}, at) == nil || f.State().Status != vo.FilingCancelled {
		t.Fatalf("cancel = %v", f.State())
	}
}
