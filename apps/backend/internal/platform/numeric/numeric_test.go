package numeric

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestParse(t *testing.T) {
	for _, s := range []string{"0", "1", "1.5", "-12.30", "123456789012345678", "0.000001", "999999999999999999.999999"} {
		if _, ok := Parse(s); !ok {
			t.Errorf("Parse(%q) should succeed", s)
		}
	}
	for _, s := range []string{"", "-", "1.", ".5", "1.2345678", "1,000", "1e3", " 1", "1234567890123456789", "abc"} {
		if _, ok := Parse(s); ok {
			t.Errorf("Parse(%q) should fail", s)
		}
	}
}

func TestFitsScale(t *testing.T) {
	d := decimal.RequireFromString("1.250")
	if !FitsScale(d, 2) || FitsScale(d, 1) || !FitsScale(decimal.RequireFromString("100"), 0) {
		t.Fatal("FitsScale")
	}
}

func TestInRange(t *testing.T) {
	if !InRange(decimal.RequireFromString("-999999999999999999.999999")) || InRange(decimal.New(1, 18)) {
		t.Fatal("InRange")
	}
}
