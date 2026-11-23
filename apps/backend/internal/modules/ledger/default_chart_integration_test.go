package ledger_test

import (
	"slices"
	"testing"

	"apexis/backend/internal/modules/ledger"
)

// ผังบัญชีมาตรฐาน: สร้างได้ครั้งเดียวเมื่อยังไม่มีบัญชี, ต้นไม้เรียงตามรหัส, บัญชีพักไม่เข้างบกำไรขาดทุน
func TestDefaultChart(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "fiscal year")

	// สมุด GEN มีอยู่แล้ว = ข้าม สร้างเฉพาะ CRJ / CDJ
	a.expect(a.do("POST", base+"/journals", owner, obj{"code": "GEN", "name": "สมุดของฉัน"}), 201, "existing GEN")

	res := a.expect(a.do("POST", base+"/accounts/defaults", owner, nil), 201, "create defaults").data()
	if res["accounts"] != float64(262) || res["journals"] != float64(2) {
		t.Fatalf("defaults = %v", res)
	}
	a.expect(a.do("POST", base+"/accounts/defaults", owner, nil), 409, "defaults twice")

	tree := a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree").list()
	codes := field(tree, "code")
	if len(tree) != 262 || !slices.IsSorted(codes) || codes[0] != "1000000000" {
		t.Fatalf("tree: %d accounts, first %v, sorted %v", len(tree), codes[0], slices.IsSorted(codes))
	}
	byCode := map[string]obj{}
	for _, acc := range tree {
		byCode[acc["code"].(string)] = acc
	}
	pending := byCode["6400000000"]
	if pending["type"] != "suspense" || pending["postable"] != true || pending["parent"].(obj)["code"] != "6000000000" || pending["description"] == "" {
		t.Fatalf("6400000000 = %v", pending)
	}
	if root := byCode["1000000000"]; root["parent"] != nil || root["postable"] != false || root["name_en"] != "Assets" {
		t.Fatalf("root = %v", root)
	}
	if got := field(a.expect(a.do("GET", base+"/journals?sort=code", owner, nil), 200, "journals").list(), "name"); !slices.Equal(got, []string{"สมุดรายวันจ่าย", "สมุดรายวันรับ", "สมุดของฉัน"}) {
		t.Fatalf("journals = %v", got)
	}

	// เงินเข้าธนาคารแต่ยังไม่รู้ว่าของใคร -> พักรับ แล้วโอนออกเมื่อทราบที่มา
	var bank, revenue string
	for _, acc := range tree {
		if acc["postable"] != true {
			continue
		}
		switch acc["type"] {
		case "asset":
			if bank == "" {
				bank = acc["id"].(string)
			}
		case "revenue":
			if revenue == "" {
				revenue = acc["id"].(string)
			}
		}
	}
	journal := a.expect(a.do("GET", base+"/journals?q=CRJ", owner, nil), 200, "CRJ").list()[0]["id"].(string)
	post := func(date string, lines ...obj) {
		t.Helper()
		a.expect(a.do("POST", base+"/journal-entries", owner, obj{"journal_id": journal, "date": date, "status": "posted", "lines": lines}), 201, "entry "+date)
	}
	sheet := func() obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=balance-sheet&end_date=2026-12-31", owner, nil), 200, "balance sheet").data()
	}

	post("2026-03-01", ln(bank, "1000", nil), ln(pending["id"].(string), nil, "1000"))
	bs := sheet()
	if bs["total_assets"] != "1000" || bs["total_suspense"] != "-1000" || bs["total_liabilities_and_equity"] != "0" || len(objs(bs["suspense"])) == 0 {
		t.Fatalf("balance sheet with pending receipt = %v", bs)
	}
	if is := a.expect(a.do("GET", base+"/reports?book=income-statement&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "income").data(); is["net_income"] != "0" ||
		is["total_suspense"] != "-1000" || len(objs(is["suspense"])) == 0 {
		t.Fatalf("suspense must be shown separately, not in net income: %v", is)
	}

	post("2026-03-05", ln(pending["id"].(string), "1000", nil), ln(revenue, nil, "1000"))
	bs = sheet()
	if bs["total_suspense"] != "0" || bs["total_assets"] != "1000" || bs["total_liabilities_and_equity"] != "1000" {
		t.Fatalf("balance sheet after clearing = %v", bs)
	}

	// บริษัทที่ตั้งผังเองแล้วใช้ผังมาตรฐานไม่ได้
	other := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "beta", "name": "Beta"}), 201, "company B").id()
	a.expect(a.do("POST", "/api/v1/companies/"+other+"/accounts", owner, obj{"code": "1000", "name": "สินทรัพย์", "type": "asset", "description": "หมวดสินทรัพย์"}), 201, "own account")
	a.expect(a.do("POST", "/api/v1/companies/"+other+"/accounts/defaults", owner, nil), 409, "defaults on existing chart")
	if d := a.expect(a.do("GET", "/api/v1/companies/"+other+"/accounts/tree", owner, nil), 200, "tree B").list(); len(d) != 1 || d[0]["description"] != "หมวดสินทรัพย์" {
		t.Fatalf("tree B = %v", d)
	}
}

// บริษัทที่สร้างผังมาตรฐานก่อนเพิ่มบัญชีสินค้าคงเหลือ / ต้นทุนขาย ได้รับบัญชีใหม่ตอน migrate — ครั้งเดียว ไม่ทับบัญชีที่ลบเอง
func TestDefaultChartTopUp(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/accounts/defaults", owner, nil), 201, "create defaults")
	count := func() int {
		t.Helper()
		return len(a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree").list())
	}
	// จำลองผังรุ่นเดิม (245 บัญชี): ลบรหัสชุดใหม่ออกจริง ลูกก่อนพ่อ
	newer := "code ~ '^(110505|110506|110507|110508|110509|110504|2102030|405|505)'"
	if err := a.db.Exec("DELETE FROM ledger_accounts WHERE " + newer + " AND postable").Error; err != nil {
		t.Fatal(err)
	}
	if err := a.db.Exec("DELETE FROM ledger_accounts WHERE " + newer).Error; err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 245 {
		t.Fatalf("old chart = %d", n)
	}
	if err := ledger.Migrate(a.db); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 262 {
		t.Fatalf("after top-up = %d", n)
	}
	tree := a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree").list()
	for _, acc := range tree {
		if acc["code"] == "5052000000" && (acc["parent"].(obj)["code"] != "5050000000" || acc["type"] != "expense" || acc["postable"] != true) {
			t.Fatalf("5052000000 = %v", acc)
		}
	}
	// ผู้ใช้ลบบัญชีใหม่ทิ้ง -> migrate รอบถัดไปไม่สร้างกลับ
	var id string
	for _, acc := range tree {
		if acc["code"] == "5056000000" {
			id = acc["id"].(string)
		}
	}
	a.expect(a.do("DELETE", base+"/accounts/"+id, owner, nil), 204, "delete new account")
	if err := ledger.Migrate(a.db); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 261 {
		t.Fatalf("after delete + migrate = %d", n)
	}
}
