// Package services คือ domain service ของ ledger — กฎที่ไม่ได้อยู่ใน entity เดียว
package services

import (
	"slices"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// EnsureNoCycle: ancestry คือ id ของ parent ใหม่และบรรพบุรุษทั้งหมด (โหลดด้วย recursive query เดียว)
// ถ้า self อยู่ใน ancestry แปลว่า parent ใหม่คือตัวเองหรือลูกหลานของตัวเอง
func EnsureNoCycle[T comparable](self T, ancestry []T) error {
	if slices.Contains(ancestry, self) {
		return domain.ErrHierarchyCycle
	}
	return nil
}

// ---------- งบการเงิน: รวมยอดตามผังบัญชี ----------

// AccountNode คือบัญชีในผัง (ทั้งบัญชีคุมและบัญชีย่อย) ที่ใช้สร้างงบ
type AccountNode struct {
	ID       vo.AccountID
	ParentID *vo.AccountID
	Code     string
	Name     string
	Type     vo.AccountType
	Postable bool
}

// StatementLine คือบรรทัดของงบการเงิน — Amount เป็นยอดตามด้านปกติของหมวด
// (สินทรัพย์ / ค่าใช้จ่าย = เดบิต - เครดิต, ที่เหลือ = เครดิต - เดบิต) รวมยอดของบัญชีย่อยทั้งหมดแล้ว
type StatementLine struct {
	AccountID vo.AccountID
	Code      string
	Name      string
	Level     int // 0 = บนสุด
	IsHeader  bool
	Amount    vo.Money
}

// BuildSection สร้างบรรทัดงบของหมวดหนึ่ง เรียงตามโครงสร้างผังบัญชี (parent ก่อนลูก, เรียงตามรหัส)
//
// net คือยอดเดบิต - เครดิตของแต่ละบัญชีย่อย (จาก query รวมยอดใน DB)
// แสดงเฉพาะบัญชีที่ยอด (รวมลูก) ไม่เป็นศูนย์ — คืนบรรทัดและยอดรวมของหมวด
func BuildSection(accounts []AccountNode, net map[vo.AccountID]vo.Money, t vo.AccountType) ([]StatementLine, vo.Money) {
	children := map[vo.AccountID][]AccountNode{}
	var roots []AccountNode
	inType := map[vo.AccountID]bool{}
	for _, a := range accounts {
		if a.Type == t {
			inType[a.ID] = true
		}
	}
	for _, a := range accounts {
		if a.Type != t {
			continue
		}
		if a.ParentID != nil && inType[*a.ParentID] {
			children[*a.ParentID] = append(children[*a.ParentID], a)
		} else {
			roots = append(roots, a)
		}
	}
	byCode := func(x, y AccountNode) int {
		if x.Code < y.Code {
			return -1
		}
		if x.Code > y.Code {
			return 1
		}
		return 0
	}

	var lines []StatementLine
	var walk func(a AccountNode, level int) vo.Money
	walk = func(a AccountNode, level int) vo.Money {
		idx := len(lines)
		lines = append(lines, StatementLine{AccountID: a.ID, Code: a.Code, Name: a.Name, Level: level, IsHeader: !a.Postable})
		total := net[a.ID]
		if !t.DebitNormal() {
			total = total.Neg()
		}
		kids := children[a.ID]
		slices.SortFunc(kids, byCode)
		for _, c := range kids {
			total = total.Add(walk(c, level+1))
		}
		if total.IsZero() && len(lines) == idx+1 {
			lines = lines[:idx] // ตัดบัญชีที่ยอดเป็นศูนย์และไม่มีลูกที่มียอด
			return decimal.Zero
		}
		lines[idx].Amount = total
		return total
	}
	slices.SortFunc(roots, byCode)
	sum := decimal.Zero
	for _, r := range roots {
		sum = sum.Add(walk(r, 0))
	}
	return lines, sum
}
