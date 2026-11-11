package valueobjects

import (
	"errors"
	"testing"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
)

func TestParseMoney(t *testing.T) {
	ok := map[string]Money{
		"0": 0, "1": 100, "1.5": 150, "1.05": 105, "1234.50": 123450, "-12.30": -1230,
		"9999999999999999.99": MaxMoney,
	}
	for in, want := range ok {
		got, err := ParseMoney(in)
		if err != nil || got != want {
			t.Errorf("ParseMoney(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "-", "1.", ".5", "1.234", "1,000", "1e3", "abc", "10000000000000000"} {
		if _, err := ParseMoney(in); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("ParseMoney(%q) err = %v, want ErrInvalidAmount", in, err)
		}
	}
}

func TestMoneyString(t *testing.T) {
	for m, want := range map[Money]string{0: "0.00", 5: "0.05", 150: "1.50", -50: "-0.50", 123450: "1234.50"} {
		if got := m.String(); got != want {
			t.Errorf("Money(%d).String() = %q, want %q", int64(m), got, want)
		}
	}
}
