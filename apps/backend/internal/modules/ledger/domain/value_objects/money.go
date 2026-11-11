package valueobjects

import (
	"fmt"
	"strconv"
	"strings"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
)

// Money คือจำนวนเงินในสกุลเงินหลักของบริษัท เก็บเป็นหน่วยย่อย (1/100 เช่น สตางค์) แบบ int64
// ไม่ใช้ float เพื่อไม่ให้ปัดเศษผิด — ตรงกับ NUMERIC(18,2) ในฐานข้อมูล
//
// ภายนอก (JSON / DB) ใช้รูปแบบข้อความทศนิยม 2 ตำแหน่ง เช่น "1234.50"
type Money int64

// MaxMoney = 9,999,999,999,999,999.99 (ขอบเขตของ NUMERIC(18,2))
const MaxMoney Money = 999_999_999_999_999_999

// ParseMoney รับ "1234", "1234.5", "1234.50", "-12.30" — ไม่รับคอมมา, exponent หรือทศนิยมเกิน 2 ตำแหน่ง
func ParseMoney(raw string) (Money, error) {
	s := strings.TrimSpace(raw)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || (hasDot && frac == "") || !digits(whole) || !digits(frac) || len(whole) > 16 {
		return 0, fmt.Errorf("%w: %q must be a decimal number with at most 2 decimal places", domain.ErrInvalidAmount, raw)
	}
	frac += strings.Repeat("0", 2-len(frac))
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil || Money(n) > MaxMoney {
		return 0, fmt.Errorf("%w: %q is out of range", domain.ErrInvalidAmount, raw)
	}
	if neg {
		n = -n
	}
	return Money(n), nil
}

func digits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// String คืนรูปแบบ "1234.50" / "-0.50"
func (m Money) String() string {
	sign := ""
	n := int64(m)
	if n < 0 {
		sign, n = "-", -n
	}
	return fmt.Sprintf("%s%d.%02d", sign, n/100, n%100)
}

func (m Money) IsZero() bool     { return m == 0 }
func (m Money) IsPositive() bool { return m > 0 }
