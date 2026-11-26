package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

func TestCreditAmounts(t *testing.T) {
	d := vo.MustDecimal
	terms := services.VatTerms{Registered: true, Rate: d("7")}
	inv := RehydrateInvoice(InvoiceState{Vat: terms, Lines: []InvoiceLine{
		{Quantity: d("5"), UnitPrice: d("100"), Amounts: services.LineAmounts(d("5"), d("100"), terms, 2)},
	}})
	none := Credited{Returned: decimal.Zero, Net: decimal.Zero}

	a, err := inv.CreditAmounts(vo.CreditReturn, 1, d("2"), d("100"), decimal.Zero, none, 2)
	if err != nil || a.Net.String() != "200" || a.Vat.String() != "14" || a.Gross.String() != "214" {
		t.Fatalf("return 2 = %+v, %v", a, err)
	}
	cases := []struct {
		name          string
		kind          vo.CreditNoteKind
		line          int
		qty, price, x string
		done          Credited
		want          error
	}{
		{"unknown line", vo.CreditReturn, 2, "1", "100", "0", none, domain.ErrInvalidLine},
		{"over return", vo.CreditReturn, 1, "4", "100", "0", Credited{Returned: d("2"), Net: d("200")}, domain.ErrOverReturn},
		{"zero qty", vo.CreditReturn, 1, "0", "100", "0", none, domain.ErrOverReturn},
		{"price above invoice", vo.CreditReturn, 1, "1", "101", "0", none, domain.ErrCreditPrice},
		{"allowance zero", vo.CreditAllowance, 1, "0", "0", "0", none, domain.ErrInvalidAmount},
		{"allowance above net", vo.CreditAllowance, 1, "0", "0", "301", Credited{Returned: decimal.Zero, Net: d("200")}, domain.ErrOverCredit},
		{"price cut after return exceeds net", vo.CreditReturn, 1, "3", "100", "0", Credited{Returned: decimal.Zero, Net: d("300")}, domain.ErrOverCredit},
	}
	for _, c := range cases {
		if _, err := inv.CreditAmounts(c.kind, c.line, d(c.qty), d(c.price), d(c.x), c.done, 2); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if a, err := inv.CreditAmounts(vo.CreditAllowance, 1, decimal.Zero, decimal.Zero, d("50"), none, 2); err != nil || a.Gross.String() != "53.5" {
		t.Fatalf("allowance 50 = %+v, %v", a, err)
	}
}

func TestNewCreditNote(t *testing.T) {
	d := vo.MustDecimal
	day := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	line := CreditNoteLine{InvoiceLineNo: 1, Amounts: services.Amounts{Net: d("100"), Vat: d("7"), Gross: d("107")}}
	ok := CreditNoteState{Date: day("2026-03-10"), Kind: vo.CreditAllowance, Reason: "ลดราคา", Lines: []CreditNoteLine{line}}
	c, err := NewCreditNote(ok, day("2026-03-07"))
	if err != nil || c.State().Totals.Gross.String() != "107" || c.State().Status != vo.DocPosted {
		t.Fatalf("credit note = %+v, %v", c, err)
	}
	if _, err := NewCreditNote(ok, day("2026-03-11")); !errors.Is(err, domain.ErrCreditBeforeInvoice) {
		t.Errorf("before invoice: %v", err)
	}
	noReason := ok
	noReason.Reason = "  "
	if _, err := NewCreditNote(noReason, day("2026-03-07")); !errors.Is(err, domain.ErrReasonRequired) {
		t.Errorf("reason: %v", err)
	}
	noLines := ok
	noLines.Lines = nil
	if _, err := NewCreditNote(noLines, day("2026-03-07")); !errors.Is(err, domain.ErrInvalidLine) {
		t.Errorf("lines: %v", err)
	}
	if err := c.Cancel(vo.UserID{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := c.Cancel(vo.UserID{}, time.Now()); !errors.Is(err, domain.ErrAlreadyCancelled) {
		t.Errorf("cancel twice: %v", err)
	}
}
