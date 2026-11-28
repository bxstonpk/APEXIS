package purchasing_test

import (
	"os"
	"strings"
	"testing"

	"apexis/backend/internal/modules/purchasing"
	"apexis/backend/internal/platform/database"
)

// เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส, สาขาเพิ่มเติม, เลือกสาขาในใบสั่งซื้อ, ผู้ขาย ณ วันบันทึกใบแจ้งหนี้, รวมรหัสซ้ำตอน migrate
func TestSupplierBranches(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "br", "name": "BR", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	vatAcc, apAcc, grirAcc, purchaseAcc, varianceAcc := account("1160", "asset"), account("2100", "liability"), account("2110", "liability"),
		account("5110", "expense"), account("5120", "expense")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "สมุดรายวันซื้อ"}), 201, "journal").id()
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv, "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "price_variance_account_id": varianceAcc}), 200, "settings")
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "บริการ", "type": "non_stock", "uom_id": pcs, "purchase_account_id": purchaseAcc}), 201, "service").id()

	const tax = "0105551234567"
	bangna := obj{"house_no": "9", "street": "บางนา-ตราด", "sub_district": "บางนา", "district": "บางนา", "province": "กรุงเทพมหานคร", "postal_code": "10260"}
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "S1", "name": "บริษัท ผู้ขาย จำกัด", "tax_id": tax, "branch_no": "00000",
		"vat_registered": true, "tax_type": "juristic", "address_detail": obj{"house_no": "1", "province": "นนทบุรี"},
		"branches": []obj{{"branch_no": "00001", "name": "สาขาบางนา", "address_detail": bangna}}}), 201, "supplier with branch").data()
	supID := sup["id"].(string)

	// เลขซ้ำ = 409 พร้อมรหัสเดิม; เลขสาขาซ้ำ (รวมสาขาหลัก) = 400
	dup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "S2", "name": "x", "tax_id": tax, "vat_registered": true, "tax_type": "juristic"}), 409, "duplicate tax id")
	if e := dup.body["error"].(obj); e["code"] != "tax_id_already_belongs_to_another_supplier" || !strings.Contains(e["message"].(string), "S1") {
		t.Fatalf("duplicate tax id error = %v", e)
	}
	br := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "S3", "name": "x", "tax_id": "0105559999999", "branch_no": "00000", "vat_registered": true,
		"tax_type": "juristic", "branches": []obj{{"branch_no": "00000"}}}), 400, "branch duplicates main")
	if code := br.body["error"].(obj)["code"]; code != "duplicate_supplier_branch_number" {
		t.Fatalf("duplicate branch code = %v", code)
	}

	got := a.expect(a.do("GET", base+"/suppliers/"+supID, owner, nil), 200, "supplier get").data()
	if bs := objs(got["branches"]); len(bs) != 1 || bs[0]["branch_no"] != "00001" || !strings.Contains(bs[0]["address"].(string), "แขวงบางนา") {
		t.Fatalf("branches = %v", got["branches"])
	}

	// ใบสั่งซื้อเลือกสาขา 00001 -> ใบแจ้งหนี้เก็บชื่อ / เลข / สาขา / ที่อยู่สาขานั้น
	order := func(branch string) obj {
		return obj{"supplier_id": supID, "supplier_branch_no": branch, "date": "2026-03-02", "lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": "1000"}}}
	}
	nf := a.expect(a.do("POST", base+"/purchase-orders", owner, order("00009")), 422, "unknown branch")
	if code := nf.body["error"].(obj)["code"]; code != "supplier_branch_not_found" {
		t.Fatalf("unknown branch code = %v", code)
	}
	po := a.expect(a.do("POST", base+"/purchase-orders", owner, order("00001")), 201, "PO").data()
	if po["supplier_branch_no"] != "00001" {
		t.Fatalf("PO branch = %v", po["supplier_branch_no"])
	}
	main := a.expect(a.do("POST", base+"/purchase-orders", owner, order("")), 201, "PO main branch").data()
	if main["supplier_branch_no"] != "00000" {
		t.Fatalf("PO default branch = %v", main["supplier_branch_no"])
	}
	poID := po["id"].(string)
	a.expect(a.do("PATCH", base+"/purchase-orders/"+poID, owner, obj{"status": "approved"}), 200, "approve")
	a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": poID, "date": "2026-03-03", "lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "receipt")
	inv := a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": poID, "supplier_invoice_no": "TX-1", "date": "2026-03-04",
		"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice").data()
	bill := inv["bill_from"].(obj)
	if bill["name"] != "บริษัท ผู้ขาย จำกัด" || bill["tax_id"] != tax || bill["branch_no"] != "00001" || !strings.Contains(bill["address"].(string), "บางนา-ตราด") {
		t.Fatalf("bill_from = %v", bill)
	}
	// แก้ทะเบียนผู้ขายภายหลังไม่กระทบใบที่บันทึกแล้ว
	a.expect(a.do("PUT", base+"/suppliers/"+supID, owner, obj{"code": "S1", "name": "บริษัท ผู้ขาย (ชื่อใหม่) จำกัด", "tax_id": tax, "branch_no": "00000",
		"vat_registered": true, "tax_type": "juristic", "status": "active",
		"branches": []obj{{"branch_no": "00001", "name": "สาขาบางนา", "address_detail": bangna}}}), 200, "rename supplier")
	again := a.expect(a.do("GET", base+"/supplier-invoices/"+inv["id"].(string), owner, nil), 200, "invoice get").data()
	if again["bill_from"].(obj)["name"] != "บริษัท ผู้ขาย จำกัด" {
		t.Fatalf("bill_from changed after supplier edit: %v", again["bill_from"])
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
	mustExec(`DROP INDEX uq_pur_suppliers_tax_id`)
	mustExec(`INSERT INTO purchasing_suppliers (id, company_id, code, name, tax_id, branch_no, address, payment_term_days, vat_registered, tax_type, status, created_at, updated_at)
		SELECT gen_random_uuid(), company_id, 'S1DUP', name, tax_id, '00002', 'ที่อยู่สาขา 2', payment_term_days, vat_registered, tax_type, status, now(), now()
		FROM purchasing_suppliers WHERE id = ?`, supID)
	var dupID string
	if err := db.Raw(`SELECT id FROM purchasing_suppliers WHERE code = 'S1DUP'`).Scan(&dupID).Error; err != nil {
		t.Fatal(err)
	}
	mainID := main["id"].(string)
	mustExec(`UPDATE purchasing_orders SET supplier_id = ?, supplier_branch_no = '' WHERE id = ?`, dupID, mainID)

	merges, err := purchasing.Migrate(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 1 || merges[0].From.String() != dupID || merges[0].To.String() != supID {
		t.Fatalf("merges = %+v", merges)
	}
	var moved struct {
		SupplierID       string
		SupplierBranchNo string
	}
	if err := db.Raw(`SELECT supplier_id, supplier_branch_no FROM purchasing_orders WHERE id = ?`, mainID).Scan(&moved).Error; err != nil {
		t.Fatal(err)
	}
	if moved.SupplierID != supID || moved.SupplierBranchNo != "00002" {
		t.Fatalf("PO after merge = %+v", moved)
	}
	got = a.expect(a.do("GET", base+"/suppliers/"+supID, owner, nil), 200, "supplier after merge").data()
	if bs := objs(got["branches"]); len(bs) != 2 || bs[1]["branch_no"] != "00002" || bs[1]["address"] != "ที่อยู่สาขา 2" {
		t.Fatalf("branches after merge = %v", got["branches"])
	}
	a.expect(a.do("GET", base+"/suppliers/"+dupID, owner, nil), 404, "merged duplicate removed")
	a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "S4", "name": "x", "tax_id": tax, "vat_registered": true, "tax_type": "juristic"}), 409, "index back")
}
