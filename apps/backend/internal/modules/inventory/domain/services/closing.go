package services

import (
	"bytes"
	"slices"

	"github.com/shopspring/decimal"

	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// AdjustmentLine คือบรรทัดของรายการปรับปรุงสินค้าคงเหลือสิ้นงวด (ด้านเดียว: เดบิตหรือเครดิต)
type AdjustmentLine struct {
	AccountID vo.AccountID
	Debit     vo.Money
	Credit    vo.Money
}

// PlanClosing คำนวณรายการปรับปรุงสินค้าคงเหลือสิ้นงวด (ระบบ periodic)
//
//   - target: มูลค่าสินค้าคงเหลือปลายงวด (FIFO) ต่อบัญชีสินค้าคงเหลือ
//   - balance: ยอดคงเหลือปัจจุบันของบัญชีเหล่านั้นใน ledger ณ วันสิ้นงวด (เดบิตเป็นบวก)
//
// ปรับบัญชีสินค้าคงเหลือให้เท่ามูลค่าปลายงวด ส่วนต่างรวมลงบัญชีต้นทุนขาย (ด้านตรงข้าม)
// ไม่มีส่วนต่างเลย = คืน nil (ไม่ต้องลงรายการ)
func PlanClosing(target, balance map[vo.AccountID]vo.Money, cogs vo.AccountID) []AdjustmentLine {
	accounts := make([]vo.AccountID, 0, len(target)+len(balance))
	for id := range target {
		accounts = append(accounts, id)
	}
	for id := range balance {
		if _, ok := target[id]; !ok {
			accounts = append(accounts, id)
		}
	}
	slices.SortFunc(accounts, func(a, b vo.AccountID) int { return bytes.Compare(a[:], b[:]) })

	var lines []AdjustmentLine
	net := decimal.Zero // ผลรวมเดบิต - เครดิตของบรรทัดสินค้าคงเหลือ
	for _, id := range accounts {
		diff := target[id].Sub(balance[id])
		switch {
		case diff.IsPositive():
			lines = append(lines, AdjustmentLine{AccountID: id, Debit: diff, Credit: decimal.Zero})
		case diff.IsNegative():
			lines = append(lines, AdjustmentLine{AccountID: id, Debit: decimal.Zero, Credit: diff.Neg()})
		default:
			continue
		}
		net = net.Add(diff)
	}
	switch {
	case net.IsPositive(): // สินค้าคงเหลือเพิ่ม -> ต้นทุนขายลดลง
		lines = append(lines, AdjustmentLine{AccountID: cogs, Debit: decimal.Zero, Credit: net})
	case net.IsNegative():
		lines = append(lines, AdjustmentLine{AccountID: cogs, Debit: net.Neg(), Credit: decimal.Zero})
	}
	return lines
}
