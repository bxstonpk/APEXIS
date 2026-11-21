// Package services คือกฎคำนวณของ sales ที่ไม่ผูกกับ entity ใด (pure function)
package services

import (
	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

var hundred = decimal.NewFromInt(100)

// VatTerms คือเงื่อนไข VAT ที่ใบเสนอราคา / ใบสั่งขายเก็บไว้ตอนสร้าง — เอกสารที่ตามมาใช้ค่านี้เสมอ
//
//	Registered: บริษัทจด VAT; Rate: อัตรา VAT (%) — ไม่จด = 0
//	Inclusive: ราคาในเอกสารรวม VAT แล้ว (เฉพาะบริษัทที่จด VAT)
type VatTerms struct {
	Registered bool
	Rate       decimal.Decimal
	Inclusive  bool
}

type Amounts struct {
	Net   vo.Money // ก่อน VAT (รายได้)
	Vat   vo.Money // ภาษีขาย
	Gross vo.Money // รวม VAT (ลูกหนี้)
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

type JournalLine struct {
	AccountID vo.AccountID
	Debit     vo.Money
	Credit    vo.Money
}

// Journal รวมยอดสุทธิต่อบัญชี (เดบิต +, เครดิต -) — 1 ผังบัญชี = 1 บรรทัด ตามลำดับที่เพิ่มครั้งแรก
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
