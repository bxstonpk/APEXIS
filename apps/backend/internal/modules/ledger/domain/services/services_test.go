package services

import (
	"testing"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

func TestBuildSectionRollsUpAndHidesZero(t *testing.T) {
	assets := vo.NewID[vo.AccountID]()
	current := vo.NewID[vo.AccountID]()
	cash := vo.NewID[vo.AccountID]()
	bank := vo.NewID[vo.AccountID]()
	unused := vo.NewID[vo.AccountID]()
	sales := vo.NewID[vo.AccountID]()
	accounts := []AccountNode{
		{ID: bank, ParentID: &current, Code: "1102", Type: vo.AccountAsset, Postable: true},
		{ID: assets, Code: "1000", Type: vo.AccountAsset},
		{ID: current, ParentID: &assets, Code: "1100", Type: vo.AccountAsset},
		{ID: cash, ParentID: &current, Code: "1101", Type: vo.AccountAsset, Postable: true},
		{ID: unused, ParentID: &current, Code: "1103", Type: vo.AccountAsset, Postable: true},
		{ID: sales, Code: "4101", Type: vo.AccountRevenue, Postable: true},
	}
	net := map[vo.AccountID]vo.Money{cash: 1000, bank: 500, sales: -1500}

	lines, total := BuildSection(accounts, net, vo.AccountAsset)
	if total != 1500 || len(lines) != 4 {
		t.Fatalf("total=%v lines=%+v", total, lines)
	}
	want := []struct {
		code  string
		level int
		amt   vo.Money
	}{{"1000", 0, 1500}, {"1100", 1, 1500}, {"1101", 2, 1000}, {"1102", 2, 500}}
	for i, w := range want {
		if lines[i].Code != w.code || lines[i].Level != w.level || lines[i].Amount != w.amt {
			t.Errorf("line %d = %+v, want %+v", i, lines[i], w)
		}
	}

	// รายได้แสดงด้านเครดิตเป็นบวก
	if _, rev := BuildSection(accounts, net, vo.AccountRevenue); rev != 1500 {
		t.Fatalf("revenue = %v", rev)
	}
}
