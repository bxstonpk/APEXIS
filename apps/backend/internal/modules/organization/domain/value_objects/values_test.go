package valueobjects

import (
	"errors"
	"testing"
	"time"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

func TestNewCode(t *testing.T) {
	c, err := NewCode(" sales-01 ")
	if err != nil || c != "SALES-01" {
		t.Fatalf("got %q %v", c, err)
	}
	for _, bad := range []string{"", "-X", "มีไทย", "A B", "ABCDEFGHIJKLMNOPQRSTU"} {
		if _, err := NewCode(bad); !errors.Is(err, domain.ErrInvalidCode) {
			t.Errorf("%q: got %v", bad, err)
		}
	}
}

func TestTaxBranchNo(t *testing.T) {
	hq, err := NewTaxBranchNo("00000")
	if err != nil || !hq.IsHeadOffice() {
		t.Fatalf("hq = %q %v", hq, err)
	}
	if b, _ := NewTaxBranchNo("00001"); b.IsHeadOffice() {
		t.Fatal("00001 is not head office")
	}
	for _, bad := range []string{"0", "000000", "0000a", ""} {
		if _, err := NewTaxBranchNo(bad); !errors.Is(err, domain.ErrInvalidTaxBranchNo) {
			t.Errorf("%q: got %v", bad, err)
		}
	}
}

func TestNewAddress(t *testing.T) {
	a, err := NewAddress(Address{Line: " 99 ถ.สุขุมวิท ", Province: "กรุงเทพมหานคร", PostalCode: "10110"})
	if err != nil || a.Country != "TH" || a.Line != "99 ถ.สุขุมวิท" {
		t.Fatalf("got %+v %v", a, err)
	}
	if _, err := NewAddress(Address{PostalCode: "1011"}); !errors.Is(err, domain.ErrInvalidPostalCode) {
		t.Fatalf("TH postal code must be 5 digits: %v", err)
	}
	if _, err := NewAddress(Address{PostalCode: "SW1A 1AA", Country: "gb"}); err != nil {
		t.Fatalf("foreign postal code: %v", err)
	}
	if _, err := NewAddress(Address{Country: "THA"}); !errors.Is(err, domain.ErrInvalidCountry) {
		t.Fatalf("country: %v", err)
	}
}

func TestEnsureDateRange(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	end := d("2026-12-31")
	if err := EnsureDateRange(d("2026-01-01"), &end); err != nil {
		t.Fatal(err)
	}
	before := d("2025-12-31")
	if err := EnsureDateRange(d("2026-01-01"), &before); !errors.Is(err, domain.ErrInvalidDateRange) {
		t.Fatalf("got %v", err)
	}
	withTime := d("2026-01-01").Add(time.Hour)
	if err := EnsureDateRange(withTime, nil); !errors.Is(err, domain.ErrInvalidDateRange) {
		t.Fatalf("time of day must be rejected: %v", err)
	}
}
