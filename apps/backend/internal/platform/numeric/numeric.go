// Package numeric คือกติกาตัวเลขทศนิยมที่ทุก module ใช้ร่วมกัน (จำนวนเงิน / จำนวนสินค้า)
//
//   - ใช้ decimal ฐาน 10 (github.com/shopspring/decimal) ไม่ใช้ float
//   - ฐานข้อมูลเก็บ NUMERIC(24,6): จำนวนเต็มไม่เกิน 18 หลัก ทศนิยมไม่เกิน 6 ตำแหน่ง
//   - จำนวนทศนิยมที่ "ยอมรับ" ไม่ fix ไว้ในโค้ด — บริษัทกำหนดเอง (organization: amount_decimals / quantity_decimals)
package numeric

import (
	"regexp"

	"github.com/shopspring/decimal"
)

const (
	// MaxScale คือทศนิยมสูงสุดที่ระบบเก็บได้ (บริษัทตั้งได้ 0..MaxScale)
	MaxScale = 6
	// MaxIntegerDigits คือจำนวนหลักของส่วนจำนวนเต็มสูงสุด
	MaxIntegerDigits = 18
	// SQLType คือชนิดคอลัมน์ของจำนวนเงิน / จำนวนสินค้า
	SQLType = "numeric(24,6)"
)

var pattern = regexp.MustCompile(`^-?[0-9]{1,18}(\.[0-9]{1,6})?$`)

// Parse รับเลขฐาน 10 เช่น "1234", "1234.5", "-0.25" — ไม่รับคอมมา / exponent / ช่องว่าง
// และไม่เกินขอบเขตที่ฐานข้อมูลเก็บได้ (ok = false เมื่อรูปแบบผิด)
func Parse(s string) (decimal.Decimal, bool) {
	if !pattern.MatchString(s) {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

// FitsScale: d มีทศนิยมไม่เกิน scale ตำแหน่ง (เลขศูนย์ท้ายไม่นับ เช่น 1.50 ผ่าน scale 1)
func FitsScale(d decimal.Decimal, scale int) bool {
	return d.Equal(d.Truncate(int32(scale)))
}

var limit = decimal.New(1, MaxIntegerDigits) // 10^18

// InRange: |d| < 10^18 (คอลัมน์ NUMERIC(24,6) เก็บได้)
func InRange(d decimal.Decimal) bool { return d.Abs().LessThan(limit) }
