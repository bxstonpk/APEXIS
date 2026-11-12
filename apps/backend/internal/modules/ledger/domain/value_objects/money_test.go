package valueobjects

import (
	"errors"
	"testing"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
)

func TestParseMoney(t *testing.T) {
	ok := map[string]string{"0": "0", "1.5": "1.5", "1234.50": "1234.5", "-12.30": "-12.3", "0.000001": "0.000001"}
	for in, want := range ok {
		got, err := ParseMoney(in)
		if err != nil || got.String() != want {
			t.Errorf("ParseMoney(%q) = %s, %v; want %s", in, got, err, want)
		}
	}
	for _, in := range []string{"", "-", "1.", ".5", "1.2345678", "1,000", "1e3", "abc", "1234567890123456789"} {
		if _, err := ParseMoney(in); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("ParseMoney(%q) err = %v, want ErrInvalidAmount", in, err)
		}
	}
}
