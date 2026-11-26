package sales_test

import (
	"os"
	"strings"
	"testing"

	"apexis/backend/internal/modules/sales"
	"apexis/backend/internal/platform/database"
)

// เลขผู้เสียภาษี 1 เลข = ลูกค้า 1 รหัส, สาขาเพิ่มเติม, เลือกสาขาในใบสั่งขาย, ผู้ซื้อ ณ วันออกใบกำกับภาษี, รวมรหัสซ้ำตอน migrate
func TestCustomerBranches(t *testing.T) {
	a, owner := setup(t)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "br", "name": "BR", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for code, typ := range map[string]string{"1130": "asset", "2140": "liability", "4110": "revenue"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	sj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "SJ", "name": "สมุดขาย"}), 201, "journal").id()
	a.expect(a.do("PUT", base+"/sales-settings", owner, obj{"vat_registered": true, "vat_rate": "7", "journal_id": sj, "ar_account_id": acc["1130"],
		"output_vat_account_id": acc["2140"]}), 200, "settings")
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "บริการ", "type": "service", "uom_id": pcs, "revenue_account_id": acc["4110"]}), 201, "service").id()

	const tax = "0105551234567"
	bangna := obj{"house_no": "9", "street": "บางนา-ตราด", "sub_district": "บางนา", "district": "บางนา", "province": "กรุงเทพมหานคร", "postal_code": "10260"}
	cust := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C1", "name": "บริษัท หนึ่ง จำกัด", "tax_id": tax, "branch_no": "00000",
		"tax_type": "juristic", "address_detail": obj{"house_no": "1", "province": "นนทบุรี"},
		"branches": []obj{{"branch_no": "00001", "name": "สาขาบางนา", "address_detail": bangna}}}), 201, "customer with branch").data()
	custID := cust["id"].(string)

	// เลขซ้ำ = 409 พร้อมรหัสเดิม; เลขสาขาซ้ำ (รวมสาขาหลัก) = 400
	dup := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C2", "name": "x", "tax_id": tax, "tax_type": "juristic"}), 409, "duplicate tax id")
	if e := dup.body["error"].(obj); e["code"] != "tax_id_already_belongs_to_another_customer" || !strings.Contains(e["message"].(string), "C1") {
		t.Fatalf("duplicate tax id error = %v", e)
	}
	a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C3", "name": "x", "tax_id": "0105559999999", "branch_no": "00000", "tax_type": "juristic",
		"branches": []obj{{"branch_no": "00000"}}}), 400, "branch duplicates main")

	got := a.expect(a.do("GET", base+"/customers/"+custID, owner, nil), 200, "customer get").data()
	if bs := objs(got["branches"]); len(bs) != 1 || bs[0]["branch_no"] != "00001" || !strings.Contains(bs[0]["address"].(string), "แขวงบางนา") {
		t.Fatalf("branches = %v", got["branches"])
	}

	// ใบสั่งขายเลือกสาขา 00001 -> ใบกำกับภาษีเก็บชื่อ / เลข / สาขา / ที่อยู่สาขานั้น
	a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": custID, "customer_branch_no": "00009", "date": "2026-03-02",
		"lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": "1000"}}}), 422, "unknown branch")
	order := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": custID, "customer_branch_no": "00001", "date": "2026-03-02",
		"lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": "1000"}}}), 201, "order").data()
	if order["customer_branch_no"] != "00001" {
		t.Fatalf("order branch = %v", order["customer_branch_no"])
	}
	orderID := order["id"].(string)
	a.expect(a.do("PATCH", base+"/sales-orders/"+orderID, owner, obj{"status": "approved"}), 200, "approve")
	inv := a.expect(a.do("POST", base+"/sales-invoices", owner, obj{"order_id": orderID, "date": "2026-03-03",
		"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice").data()
	bill := inv["bill_to"].(obj)
	if bill["name"] != "บริษัท หนึ่ง จำกัด" || bill["tax_id"] != tax || bill["branch_no"] != "00001" || !strings.Contains(bill["address"].(string), "บางนา-ตราด") {
		t.Fatalf("bill_to = %v", bill)
	}
	// แก้ทะเบียนลูกค้าภายหลังไม่กระทบใบที่ออกแล้ว
	a.expect(a.do("PUT", base+"/customers/"+custID, owner, obj{"code": "C1", "name": "บริษัท หนึ่ง (ชื่อใหม่) จำกัด", "tax_id": tax, "branch_no": "00000",
		"tax_type": "juristic", "status": "active", "branches": []obj{{"branch_no": "00001", "name": "สาขาบางนา", "address_detail": bangna}}}), 200, "rename customer")
	again := a.expect(a.do("GET", base+"/sales-invoices/"+inv["id"].(string), owner, nil), 200, "invoice get").data()
	if again["bill_to"].(obj)["name"] != "บริษัท หนึ่ง จำกัด" {
		t.Fatalf("bill_to changed after customer edit: %v", again["bill_to"])
	}
	vat := a.expect(a.do("GET", base+"/reports?book=output-vat&start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "output vat").data()
	if r := objs(vat["rows"]); len(r) != 1 || r[0]["branch_no"] != "00001" {
		t.Fatalf("output vat rows = %v", vat["rows"])
	}

	// ---------- ข้อมูลเก่าที่มีรหัสซ้ำ (ก่อนมี unique index): migrate รวมเป็นรหัสเดียว ----------
	db, err := database.OpenPostgres(os.Getenv("IDENTITY_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`DROP INDEX uq_sal_customers_tax_id`)
	mustExec(`INSERT INTO sales_customers (id, company_id, code, name, tax_id, branch_no, address, payment_term_days, tax_type, status, created_at, updated_at)
		SELECT gen_random_uuid(), company_id, 'C1DUP', name, tax_id, '00002', 'ที่อยู่สาขา 2', payment_term_days, tax_type, status, now(), now()
		FROM sales_customers WHERE id = ?`, custID)
	var dupID string
	if err := db.Raw(`SELECT id FROM sales_customers WHERE code = 'C1DUP'`).Scan(&dupID).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(`UPDATE sales_orders SET customer_id = ?, customer_branch_no = '' WHERE id = ?`, dupID, orderID)

	merges, err := sales.Migrate(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 1 || merges[0].From.String() != dupID || merges[0].To.String() != custID {
		t.Fatalf("merges = %+v", merges)
	}
	var moved struct {
		CustomerID       string
		CustomerBranchNo string
	}
	if err := db.Raw(`SELECT customer_id, customer_branch_no FROM sales_orders WHERE id = ?`, orderID).Scan(&moved).Error; err != nil {
		t.Fatal(err)
	}
	if moved.CustomerID != custID || moved.CustomerBranchNo != "00002" {
		t.Fatalf("order after merge = %+v", moved)
	}
	got = a.expect(a.do("GET", base+"/customers/"+custID, owner, nil), 200, "customer after merge").data()
	if bs := objs(got["branches"]); len(bs) != 2 || bs[1]["branch_no"] != "00002" || bs[1]["address"] != "ที่อยู่สาขา 2" {
		t.Fatalf("branches after merge = %v", got["branches"])
	}
	a.expect(a.do("GET", base+"/customers/"+dupID, owner, nil), 404, "merged duplicate removed")
	a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C4", "name": "x", "tax_id": tax, "tax_type": "juristic"}), 409, "index back")
}
