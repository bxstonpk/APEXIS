// Package services คือกฎคำนวณของ purchasing ที่ไม่ผูกกับ entity ใด (pure function)
package services

import (
	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

// UnitCostScale: ทศนิยมของราคาต่อหน่วยก่อน VAT (เท่ากับที่ inventory เก็บต้นทุนต่อหน่วย)
const UnitCostScale = 6

var hundred = decimal.NewFromInt(100)

// VatTerms คือเงื่อนไข VAT ที่ใบสั่งซื้อเก็บไว้ตอนสร้าง — เอกสารที่ตามมา (รับสินค้า / แจ้งหนี้) ใช้ค่านี้เสมอ
//
//	Rate: อัตรา VAT (%) — ผู้ขายไม่จด VAT = 0
//	Inclusive: ราคาในเอกสารรวม VAT แล้ว (เฉพาะผู้ขายที่จด VAT)
type VatTerms struct {
	Registered bool
	Rate       decimal.Decimal
	Inclusive  bool
}

// UnitCost คือราคาต่อหน่วยก่อน VAT — ต้นทุนสินค้าไม่รวม VAT เสมอ (VAT ซื้อขอคืนได้)
func UnitCost(price vo.Money, t VatTerms) vo.Money {
	if !t.Inclusive || t.Rate.IsZero() {
		return price
	}
	return price.Mul(hundred).Div(hundred.Add(t.Rate)).Round(UnitCostScale)
}

// ReceiptValue: มูลค่ารับสินค้า (ก่อน VAT) = จำนวน × ราคาต่อหน่วยก่อน VAT ปัดตามทศนิยมของบริษัท
// (สูตรเดียวกับที่ inventory คิดมูลค่าชั้นต้นทุน)
func ReceiptValue(qty, price vo.Money, t VatTerms, amountDecimals int) vo.Money {
	return qty.Mul(UnitCost(price, t)).Round(int32(amountDecimals))
}

type Amounts struct {
	Net   vo.Money // ก่อน VAT
	Vat   vo.Money
	Gross vo.Money // รวม VAT (ยอดเจ้าหนี้)
}

func (a Amounts) Add(b Amounts) Amounts {
	return Amounts{Net: a.Net.Add(b.Net), Vat: a.Vat.Add(b.Vat), Gross: a.Gross.Add(b.Gross)}
}

func ZeroAmounts() Amounts { return Amounts{Net: decimal.Zero, Vat: decimal.Zero, Gross: decimal.Zero} }

// LineAmounts: ยอดของบรรทัด (จำนวน × ราคา ปัดตามทศนิยมของบริษัท) แยก VAT ตามเงื่อนไข
//
//	แยก VAT:  net = ยอด, vat = net × rate / 100
//	รวม VAT:  gross = ยอด, vat = gross × rate / (100 + rate), net = gross - vat
func LineAmounts(qty, price vo.Money, t VatTerms, amountDecimals int) Amounts {
	d := int32(amountDecimals)
	base := qty.Mul(price).Round(d)
	switch {
	case t.Rate.IsZero():
		return Amounts{Net: base, Vat: decimal.Zero, Gross: base}
	case t.Inclusive:
		vat := base.Mul(t.Rate).Div(hundred.Add(t.Rate)).Round(d)
		return Amounts{Net: base.Sub(vat), Vat: vat, Gross: base}
	default:
		vat := base.Mul(t.Rate).Div(hundred).Round(d)
		return Amounts{Net: base, Vat: vat, Gross: base.Add(vat)}
	}
}

// ---------- รายการบัญชี ----------

// JournalLine คือบรรทัดรายการบัญชี (ด้านเดียว)
type JournalLine struct {
	AccountID vo.AccountID
	Debit     vo.Money
	Credit    vo.Money
}

// Journal รวมยอดสุทธิต่อบัญชี (เดบิต +, เครดิต -) ตามลำดับที่เพิ่มครั้งแรก
type Journal struct {
	order []vo.AccountID
	net   map[vo.AccountID]vo.Money
}

func NewJournal() *Journal { return &Journal{net: map[vo.AccountID]vo.Money{}} }

func (j *Journal) Debit(acc vo.AccountID, amount vo.Money)  { j.add(acc, amount) }
func (j *Journal) Credit(acc vo.AccountID, amount vo.Money) { j.add(acc, amount.Neg()) }

func (j *Journal) add(acc vo.AccountID, amount vo.Money) {
	if _, ok := j.net[acc]; !ok {
		j.order = append(j.order, acc)
		j.net[acc] = decimal.Zero
	}
	j.net[acc] = j.net[acc].Add(amount)
}

// Lines คืนบรรทัดของบัญชีที่ยอดสุทธิไม่เป็นศูนย์ — ไม่มีบรรทัด = ไม่ต้องลงบัญชี
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
