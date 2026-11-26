package entities

import (
	"testing"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

func TestChartForBusinessType(t *testing.T) {
	cases := map[vo.BusinessType]struct {
		n       int
		has     []string
		missing []string
	}{
		vo.BusinessService:       {245, []string{"4101000000", "5100000000", "1105010000", "1105030000", "2103030000"}, []string{"4050000000", "5050000000", "1105040000", "2102030000", "5060000000", "4060000000"}},
		vo.BusinessTrading:       {259, []string{"4051000000", "4052000000", "5051000000", "5054000000", "1105040000", "1105090000", "2102030000"}, []string{"1105050000", "1105060000", "5060000000", "4060000000", "1103040000"}},
		vo.BusinessManufacturing: {273, []string{"1105050000", "1105060000", "1105070000", "5060000000", "5061000000", "5062000000", "5063010000", "5063060000", "5064000000", "4051000000"}, []string{"4060000000", "5070000000", "1105100000"}},
		vo.BusinessConstruction:  {261, []string{"1103040000", "1103050000", "1105100000", "2103040000", "4060000000", "4061000000", "4062000000", "5070000000", "5077000000", "2102030000"}, []string{"4050000000", "5050000000", "1105040000", "5060000000"}},
	}
	company := vo.NewID[vo.CompanyID]()
	for bt, c := range cases {
		chart := ChartFor(bt)
		if len(chart) != c.n {
			t.Errorf("%s: %d accounts, want %d", bt, len(chart), c.n)
		}
		codes := map[string]bool{}
		for _, d := range chart {
			if d.ParentCode != "" && !codes[d.ParentCode] {
				t.Errorf("%s: %s before its parent %s", bt, d.Code, d.ParentCode)
			}
			if codes[d.Code] {
				t.Errorf("%s: duplicate %s", bt, d.Code)
			}
			codes[d.Code] = true
		}
		for _, code := range c.has {
			if !codes[code] {
				t.Errorf("%s: missing %s", bt, code)
			}
		}
		for _, code := range c.missing {
			if codes[code] {
				t.Errorf("%s: unexpected %s", bt, code)
			}
		}
		// ทุกบัญชีผ่านกฎเดียวกับการสร้างทีละบัญชี (รหัส, ประเภทเดียวกับ parent, parent เป็นบัญชีคุม)
		accounts, err := NewDefaultChart(company, bt)
		if err != nil || len(accounts) != c.n {
			t.Errorf("%s: NewDefaultChart = %d, %v", bt, len(accounts), err)
		}
	}
	// ทุกรหัสใน ChartScope ต้องมีอยู่ในผัง (กันพิมพ์รหัสผิด)
	all := map[string]bool{}
	for _, d := range DefaultChart {
		all[d.Code] = true
	}
	for code := range ChartScope {
		if !all[code] {
			t.Errorf("ChartScope has unknown code %s", code)
		}
	}
	if _, err := NewDefaultChart(company, "retail"); err == nil {
		t.Error("unknown business type must fail")
	}
}
