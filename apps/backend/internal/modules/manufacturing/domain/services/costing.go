// Package services คือกฎการคำนวณของการผลิตที่ไม่ผูกกับ entity ใดตัวหนึ่ง (ปริมาณตามสูตร / ต้นทุน / แบ่งบรรทัดรับเข้า)
package services

import (
	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

var hundred = decimal.NewFromInt(100)

// Requirement คือปริมาณวัตถุดิบที่ต้องใช้ของใบสั่งผลิต:
// ปริมาณต่อชุดตามสูตร × (จำนวนที่สั่งผลิต / จำนวนที่ได้ต่อชุด) × (1 + %สูญเสีย / 100) ปัดตามทศนิยมจำนวนของบริษัท
// (คูณก่อนหารครั้งเดียว เพื่อไม่ให้ปัดเศษกลางทาง)
func Requirement(perBatch, scrapPercent, outputQuantity, orderQuantity vo.Quantity, quantityDecimals int) vo.Quantity {
	if !outputQuantity.IsPositive() {
		return decimal.Zero
	}
	return perBatch.Mul(orderQuantity).Mul(hundred.Add(scrapPercent)).
		Div(outputQuantity.Mul(hundred)).Round(int32(quantityDecimals))
}

// Scaled คือยอดเงินต่อชุดที่ปรับตามจำนวนผลิต: ต่อชุด × (จำนวน / จำนวนที่ได้ต่อชุด) ปัดตามทศนิยมเงินของบริษัท
// ใช้กับค่าแรง / ค่าใช้จ่ายการผลิตมาตรฐาน (ตอนสร้าง = ตามจำนวนสั่งผลิต, ตอนปิดงาน = ตามจำนวนที่ผลิตได้จริง)
func Scaled(perBatch, outputQuantity, quantity vo.Money, amountDecimals int) vo.Money {
	if !outputQuantity.IsPositive() {
		return decimal.Zero
	}
	return perBatch.Mul(quantity).Div(outputQuantity).Round(int32(amountDecimals))
}

// Proportional คือมูลค่าตามสัดส่วนจำนวน (เช่นต้นทุนของวัตถุดิบที่คืน = ต้นทุนเฉลี่ยที่เบิกสุทธิ × จำนวนคืน)
// คืนทั้งหมด (quantity = ofQuantity) ได้มูลค่าเต็มพอดี ไม่มีเศษจากการปัด
func Proportional(value, ofQuantity, quantity vo.Quantity, amountDecimals int) vo.Money {
	if !ofQuantity.IsPositive() {
		return decimal.Zero
	}
	if quantity.Equal(ofQuantity) {
		return value
	}
	return value.Mul(quantity).Div(ofQuantity).Round(int32(amountDecimals))
}

// StockLine คือบรรทัดรับเข้าคลัง (จำนวน + ต้นทุนต่อหน่วยก่อน VAT)
type StockLine struct {
	Quantity vo.Quantity
	UnitCost vo.Money
}

// ReceiptLines แบ่งการรับเข้า (จำนวน quantity มูลค่ารวม value) เป็นบรรทัดของใบรับเข้าคลัง ให้มูลค่ารวมตรงกับ value พอดี
//
// inventory คิดมูลค่าบรรทัด = round(จำนวน × ต้นทุนต่อหน่วย, ทศนิยมเงิน) และต้นทุนต่อหน่วยเก็บได้ไม่เกิน numeric.MaxScale ตำแหน่ง
// ต้นทุนต่อหน่วยที่หารไม่ลงตัวจึงอาจทำให้มูลค่าคลาดจาก value เล็กน้อย (เกิดเมื่อจำนวนมาก) — กรณีนั้นแบ่งเป็น 2 บรรทัด:
// (จำนวน − 1) หน่วยที่ต้นทุนเฉลี่ย และ 1 หน่วยที่รับส่วนที่เหลือ เพื่อให้มูลค่าในคลังเท่ากับต้นทุนที่ลงบัญชีทุกสตางค์
func ReceiptLines(quantity vo.Quantity, value vo.Money, amountDecimals int) []StockLine {
	unit := value.DivRound(quantity, numeric.MaxScale)
	if quantity.Mul(unit).Round(int32(amountDecimals)).Equal(value) || quantity.LessThanOrEqual(decimal.NewFromInt(1)) {
		return []StockLine{{Quantity: quantity, UnitCost: unit}}
	}
	rest := quantity.Sub(decimal.NewFromInt(1))
	restValue := rest.Mul(unit).Round(int32(amountDecimals))
	last := value.Sub(restValue)
	if last.IsNegative() {
		last = decimal.Zero
	}
	return []StockLine{{Quantity: rest, UnitCost: unit}, {Quantity: decimal.NewFromInt(1), UnitCost: last}}
}
