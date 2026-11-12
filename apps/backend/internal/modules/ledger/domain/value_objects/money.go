package valueobjects

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/platform/numeric"
)

// Money คือจำนวนเงินในสกุลเงินหลักของบริษัท — decimal ฐาน 10 (ไม่ใช้ float จึงไม่ปัดเศษผิด)
//
// จำนวนทศนิยมที่ยอมรับไม่ fix ไว้ในโค้ด: บริษัทกำหนดที่ organization (amount_decimals, 0-6)
// ฐานข้อมูลเก็บ NUMERIC(24,6) — ภายนอก (JSON) ใช้ข้อความทศนิยม เช่น "1234.5"
type Money = decimal.Decimal

// ParseMoney รับ "1234", "1234.5", "-12.30" — ไม่รับคอมมา / exponent; ไม่เกิน 18 หลัก + ทศนิยม 6 ตำแหน่ง
// (ตรวจจำนวนทศนิยมตามที่บริษัทกำหนดแยกต่างหากใน entity)
func ParseMoney(raw string) (Money, error) {
	d, ok := numeric.Parse(strings.TrimSpace(raw))
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: %q must be a decimal number (up to %d integer digits and %d decimal places)",
			domain.ErrInvalidAmount, raw, numeric.MaxIntegerDigits, numeric.MaxScale)
	}
	return d, nil
}

// MustMoney ใช้ใน test / ค่าคงที่เท่านั้น
func MustMoney(s string) Money { return decimal.RequireFromString(s) }
