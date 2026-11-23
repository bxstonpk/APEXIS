// Package services คือกฎคำนวณของ finance ที่ไม่ผูกกับ entity ใด (pure function)
package services

import (
	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

var hundred = decimal.NewFromInt(100)

// JournalLine คือบรรทัดรายการบัญชี (ด้านเดียว)
type JournalLine struct {
	AccountID vo.GLAccountID
	Debit     vo.Money
	Credit    vo.Money
}

// Journal รวมยอดสุทธิต่อบัญชี (เดบิต +, เครดิต -) ตามลำดับที่เพิ่มครั้งแรก — 1 ผังบัญชี = 1 บรรทัด
type Journal struct {
	order []vo.GLAccountID
	net   map[vo.GLAccountID]vo.Money
}

func NewJournal() *Journal { return &Journal{net: map[vo.GLAccountID]vo.Money{}} }

func (j *Journal) Debit(acc vo.GLAccountID, amount vo.Money)  { j.add(acc, amount) }
func (j *Journal) Credit(acc vo.GLAccountID, amount vo.Money) { j.add(acc, amount.Neg()) }

func (j *Journal) add(acc vo.GLAccountID, amount vo.Money) {
	if amount.IsZero() {
		return
	}
	if _, ok := j.net[acc]; !ok {
		j.order = append(j.order, acc)
		j.net[acc] = decimal.Zero
	}
	j.net[acc] = j.net[acc].Add(amount)
}

// Lines คืนบรรทัดของบัญชีที่ยอดสุทธิไม่เป็นศูนย์
func (j *Journal) Lines() []JournalLine {
	var out []JournalLine
	for _, acc := range j.order {
		n := j.net[acc]
		switch {
		case n.IsPositive():
			out = append(out, JournalLine{AccountID: acc, Debit: n, Credit: decimal.Zero})
		case n.IsNegative():
			out = append(out, JournalLine{AccountID: acc, Debit: decimal.Zero, Credit: n.Neg()})
		}
	}
	return out
}

// Withholding: ฐานภาษี = ยอดก่อน VAT ของใบแจ้งหนี้ตามสัดส่วนที่จ่าย (จ่ายครบ = ยอดก่อน VAT ทั้งหมด)
// ภาษี = ฐาน × อัตรา / 100 — ปัดตามทศนิยมของบริษัททั้งสองค่า
func Withholding(paid, invoiceNet, invoiceGross, rate vo.Money, amountDecimals int) (base, tax vo.Money) {
	d := int32(amountDecimals)
	switch {
	case invoiceGross.IsZero():
		base = decimal.Zero
	case paid.Equal(invoiceGross):
		base = invoiceNet
	default:
		base = invoiceNet.Mul(paid).Div(invoiceGross).Round(d)
	}
	return base, base.Mul(rate).Div(hundred).Round(d)
}

// WithholdingOnAmount: จ่ายเงินอื่น (ไม่มีใบแจ้งหนี้ / VAT แยก) — ฐาน = ยอดจ่าย
func WithholdingOnAmount(amount, rate vo.Money, amountDecimals int) (base, tax vo.Money) {
	return amount, amount.Mul(rate).Div(hundred).Round(int32(amountDecimals))
}
