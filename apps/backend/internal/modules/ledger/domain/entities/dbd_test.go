package entities

import (
	"testing"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// งบตามแบบกรมพัฒนาธุรกิจการค้าต้องกระทบยอดกับงบทดลองได้เสมอ สำหรับผังมาตรฐานทุกประเภทธุรกิจ:
// ทุกบัญชีย่อยมียอด (ผลรวมทั้งผัง = 0 เหมือนงบทดลอง) -> สินทรัพย์ = หนี้สิน + ส่วนของผู้ถือหุ้น และกำไรสุทธิ = -(ยอดรายได้ / ค่าใช้จ่าย)
func TestDBDStatementsReconcileForEveryChart(t *testing.T) {
	for _, bt := range vo.BusinessTypes {
		chart, err := NewDefaultChart(vo.NewID[vo.CompanyID](), bt)
		if err != nil {
			t.Fatal(err)
		}
		nodes := make([]services.AccountNode, len(chart))
		bsNet := map[vo.AccountID]vo.Money{}
		isNet := services.ISColumn{}
		total, pl := decimal.Zero, decimal.Zero
		var last *services.AccountNode
		for i, a := range chart {
			d := a.Data()
			nodes[i] = services.AccountNode{ID: a.ID(), ParentID: d.ParentID, Code: d.Code, Name: d.Name, Type: d.Type, Postable: d.Postable}
			if !d.Postable {
				continue
			}
			v := decimal.NewFromInt(int64(i*37%1000 + 1))
			if i%3 == 0 {
				v = v.Neg()
			}
			total = total.Add(v)
			if d.Type.IsBalanceSheet() {
				bsNet[a.ID()] = v
			} else {
				isNet[a.ID()] = v
				pl = pl.Add(v)
			}
			last = &nodes[i]
		}
		// ปิดยอดให้ผลรวมทั้งผังเป็นศูนย์ด้วยบัญชีงบแสดงฐานะบัญชีสุดท้ายที่พบ
		for i := len(nodes) - 1; i >= 0; i-- {
			if nodes[i].Postable && nodes[i].Type.IsBalanceSheet() {
				bsNet[nodes[i].ID] = bsNet[nodes[i].ID].Sub(total)
				break
			}
		}
		_ = last
		bs := services.DBDBalanceSheet(nodes, []services.BSColumn{{Net: bsNet, CurrentYear: pl.Neg()}})
		if a, le := services.DBDValue(bs, "total_assets", 0), services.DBDValue(bs, "total_liabilities_equity", 0); !a.Equal(le) {
			t.Fatalf("%s: assets %s != liabilities + equity %s", bt, a, le)
		}
		is := services.DBDIncomeStatement(nodes, []services.ISColumn{isNet})
		if np := services.DBDValue(is, "net_profit", 0); !np.Equal(pl.Neg()) {
			t.Fatalf("%s: net profit %s != %s", bt, np, pl.Neg())
		}
	}
}

func TestDBDLineKeyMapping(t *testing.T) {
	cases := map[string]string{
		"1101010100": "cash", "1103020400": "receivables", "1105040000": "inventories", "1206010000": "ppe",
		"2102020100": "payables", "2201010000": "long_borrowings", "3302000000": "retained_unappropriated",
		"4051000000": "revenue_sales", "4061000000": "revenue_construction", "4101000000": "revenue_services", "4201000000": "other_income",
		"5052000000": "cost_of_sales", "5062000000": "cost_of_sales", "5071000000": "cost_of_construction", "5101000000": "cost_of_services",
		"5201000000": "selling_expenses", "5313000000": "admin_expenses", "5401000000": "finance_costs", "5501000000": "income_tax",
	}
	types := map[byte]vo.AccountType{'1': vo.AccountAsset, '2': vo.AccountLiability, '3': vo.AccountEquity, '4': vo.AccountRevenue, '5': vo.AccountExpense}
	for code, want := range cases {
		if got := services.DBDLineKey(services.AccountNode{Code: code, Type: types[code[0]]}); got != want {
			t.Errorf("%s -> %s, want %s", code, got, want)
		}
	}
	// นอกผังมาตรฐาน -> บรรทัด "อื่น" ของหมวด
	if got := services.DBDLineKey(services.AccountNode{Code: "X-1", Type: vo.AccountRevenue}); got != "other_income" {
		t.Errorf("custom revenue -> %s", got)
	}
	if got := services.DBDLineKey(services.AccountNode{Code: "6400000000", Type: vo.AccountSuspense}); got != "other_current_assets" {
		t.Errorf("suspense -> %s", got)
	}
}

func TestClosingLinesZeroProfitAndLoss(t *testing.T) {
	rev, exp, re, sum := vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID]()
	lines, net := ClosingLines([]ClosingBalance{{rev, decimal.NewFromInt(-60000)}, {exp, decimal.NewFromInt(10000)}}, re, &sum, "close")
	if !net.Equal(decimal.NewFromInt(50000)) || len(lines) != 5 {
		t.Fatalf("net %s lines %d", net, len(lines))
	}
	d, c := decimal.Zero, decimal.Zero
	for _, l := range lines {
		d, c = d.Add(l.Debit), c.Add(l.Credit)
	}
	if !d.Equal(c) {
		t.Fatalf("unbalanced %s / %s", d, c)
	}
	if l, _ := ClosingLines(nil, re, nil, "x"); l != nil {
		t.Fatal("no balances -> no lines")
	}
}
