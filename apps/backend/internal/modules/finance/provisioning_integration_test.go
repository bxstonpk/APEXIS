package finance_test

import (
	"context"
	"errors"
	"testing"

	"apexis/backend/internal/platform/integration"
)

// สร้างบริษัทตามประเภทธุรกิจ -> ผังบัญชีมาตรฐานของประเภทนั้น + สมุดรายวัน + ค่าตั้งทุก module ใน transaction เดียวกับการสร้างบริษัท
func TestCompanyProvisioningByBusinessType(t *testing.T) {
	a := setupApp(t, []integration.CompanyCreatedHook{})
	owner := a.user("owner@example.com")

	type want struct {
		accounts int
		has      []string // ต้องมี
		hasNot   []string // ต้องไม่มี
		revenue  string   // บัญชีรายได้ค่าเริ่มต้นของค่าตั้งขาย
		cogs     string   // บัญชีปรับปรุงสินค้าสิ้นงวดของค่าตั้งคลัง
		grir     bool     // ค่าตั้งจัดซื้อมีบัญชีรับสินค้ารอใบแจ้งหนี้
	}
	cases := map[string]want{
		"service": {245, []string{"4101000000", "1105010000", "2103030000"}, []string{"4051000000", "5051000000", "1105040000", "5061000000", "4061000000", "2102030000"},
			"4101000000", "5102000000", false},
		"trading": {259, []string{"4051000000", "5052000000", "1105040000", "2102030000"}, []string{"1105060000", "5061000000", "4061000000"},
			"4051000000", "5051000000", true},
		"manufacturing": {274, []string{"5065000000", "1105050000", "1105060000", "5061000000", "5063020000", "5064000000", "4051000000"}, []string{"4061000000", "5071000000"},
			"4051000000", "5061000000", true},
		"construction": {261, []string{"1103040000", "1103050000", "1105100000", "2103040000", "4061000000", "5073000000", "2102030000"}, []string{"4051000000", "5051000000", "1105040000"},
			"4061000000", "5071000000", true},
	}
	for bt, w := range cases {
		company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": bt[:4], "name": bt, "business_type": bt}), 201, "company "+bt).data()
		if company["business_type"] != bt {
			t.Fatalf("%s: business_type = %v", bt, company["business_type"])
		}
		base := "/api/v1/companies/" + company["id"].(string)
		tree := objs(a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree "+bt).body["data"])
		ids := map[string]string{}
		for _, acc := range tree {
			ids[acc["code"].(string)] = acc["id"].(string)
		}
		if len(tree) != w.accounts {
			t.Errorf("%s: accounts = %d, want %d", bt, len(tree), w.accounts)
		}
		for _, c := range w.has {
			if ids[c] == "" {
				t.Errorf("%s: missing %s", bt, c)
			}
		}
		for _, c := range w.hasNot {
			if ids[c] != "" {
				t.Errorf("%s: unexpected %s", bt, c)
			}
		}
		journals := objs(a.expect(a.do("GET", base+"/journals", owner, nil), 200, "journals").body["data"])
		jids := map[string]string{}
		for _, j := range journals {
			jids[j["code"].(string)] = j["id"].(string)
		}
		if len(jids) != 3 {
			t.Errorf("%s: journals = %v", bt, jids)
		}

		ss := a.expect(a.do("GET", base+"/sales-settings", owner, nil), 200, "sales settings").data()
		if ss["revenue_account_id"] != ids[w.revenue] || ss["ar_account_id"] != ids["1103010100"] || ss["output_vat_account_id"] != ids["2102021100"] ||
			ss["journal_id"] != jids["GEN"] || ss["vat_rate"] != "7" {
			t.Errorf("%s: sales settings = %v", bt, ss)
		}
		ps := a.expect(a.do("GET", base+"/purchasing-settings", owner, nil), 200, "purchasing settings").data()
		if ps["ap_account_id"] != ids["2102010000"] || ps["input_vat_account_id"] != ids["1107090000"] || (ps["grir_account_id"] != nil) != w.grir {
			t.Errorf("%s: purchasing settings = %v", bt, ps)
		}
		is := a.expect(a.do("GET", base+"/inventory-settings", owner, nil), 200, "inventory settings").data()
		if is["cogs_account_id"] != ids[w.cogs] || is["journal_id"] != jids["GEN"] {
			t.Errorf("%s: inventory settings = %v", bt, is)
		}
		fs := a.expect(a.do("GET", base+"/finance-settings", owner, nil), 200, "finance settings").data()
		if fs["receipt_journal_id"] != jids["CRJ"] || fs["payment_journal_id"] != jids["CDJ"] || fs["wht_pnd53_account_id"] != ids["2102020800"] {
			t.Errorf("%s: finance settings = %v", bt, fs)
		}
		// ผังสร้างแล้วตอนสร้างบริษัท — สร้างซ้ำไม่ได้
		a.expect(a.do("POST", base+"/accounts/defaults", owner, nil), 409, "defaults again "+bt)
		// ประเภทธุรกิจแก้ไม่ได้ (ส่งมาก็ถูกละเว้น)
		upd := a.expect(a.do("PUT", base, owner, obj{"name": bt + " ltd", "base_currency": "THB", "fiscal_year_start_month": 1,
			"amount_decimals": 2, "quantity_decimals": 2, "status": "active", "business_type": "service"}), 200, "update company").data()
		if upd["business_type"] != bt {
			t.Errorf("%s: business_type changed to %v", bt, upd["business_type"])
		}
	}

	// ไม่ส่ง / ส่งผิด = 400
	a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "none", "name": "none"}), 400, "missing business type")
	a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "bad", "name": "bad", "business_type": "retail"}), 400, "invalid business type")

	// รายการบริษัทของฉันแสดงประเภทธุรกิจ
	mine := objs(a.expect(a.do("GET", "/api/v1/me/companies", owner, nil), 200, "my companies").body["data"])
	for _, c := range mine {
		if c["business_type"] == "" || c["business_type"] == nil {
			t.Fatalf("my companies = %v", mine)
		}
	}
}

// hook ใดล้มเหลว = rollback ทั้งการสร้างบริษัท, ผังบัญชี และค่าตั้งที่ทำไปแล้ว
func TestCompanyProvisioningRollback(t *testing.T) {
	boom := errors.New("boom")
	a := setupApp(t, []integration.CompanyCreatedHook{func(context.Context, integration.NewCompany) error { return boom }})
	owner := a.user("owner@example.com")
	a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 500, "failing hook")
	for _, table := range []string{"organization_companies", "ledger_accounts", "ledger_journals", "sales_settings", "purchasing_settings", "inventory_settings", "finance_settings"} {
		var n int64
		if err := a.db.Table(table).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s has %d rows after rollback", table, n)
		}
	}
}
