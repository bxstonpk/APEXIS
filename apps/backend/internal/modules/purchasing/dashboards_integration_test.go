package purchasing_test

import "testing"

// GET /dashboards/purchasing: ยอดซื้อช่วงนี้ / ก่อนหน้า, 12 เดือน, ใบสั่งซื้อค้าง, เจ้าหนี้ตามอายุ + ครบกำหนดใน 7 วัน, ผู้ขาย 5 อันดับ
func TestPurchasingDashboard(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "dash", "name": "DASH", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	account := func(code, typ string) string {
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	vatAcc, apAcc, grirAcc := account("1160", "asset"), account("2100", "liability"), account("2110", "liability")
	purchaseAcc, varianceAcc := account("5110", "expense"), account("5120", "expense")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "สมุดรายวันซื้อ"}), 201, "journal").id()
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv, "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "price_variance_account_id": varianceAcc}), 200, "settings")
	uom := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ค่าบริการ", "type": "service", "uom_id": uom, "purchase_account_id": purchaseAcc}), 201, "service").id()
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "S1", "name": "ผู้ขายหนึ่ง", "payment_term_days": 10, "vat_registered": true, "tax_type": "juristic"}), 201, "supplier").id()

	po := func(date, price string, approve bool) string {
		id := a.expect(a.do("POST", base+"/purchase-orders", owner, obj{"supplier_id": sup, "date": date, "price_includes_vat": false,
			"lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": price}}}), 201, "PO "+date).id()
		if approve {
			a.expect(a.do("PATCH", base+"/purchase-orders/"+id, owner, obj{"status": "approved"}), 200, "approve "+date)
		}
		return id
	}
	receive := func(id, date string) {
		a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": id, "date": date, "lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "receipt "+date)
	}
	invoice := func(id, no, date string) {
		a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": id, "supplier_invoice_no": no, "invoice_date": date, "date": date,
			"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice "+no)
	}
	p1 := po("2026-02-10", "500", true) // ช่วงก่อนหน้า: ครบกำหนด 20 ก.พ. -> เกิน 39 วัน ณ 31 มี.ค.
	receive(p1, "2026-02-10")
	invoice(p1, "INV-1", "2026-02-10")
	p2 := po("2026-03-05", "1000", true) // ครบกำหนด 15 มี.ค. -> เกิน 16 วัน
	receive(p2, "2026-03-05")
	invoice(p2, "INV-2", "2026-03-05")
	p3 := po("2026-03-28", "200", true) // ครบกำหนด 7 เม.ย. -> ยังไม่ถึง แต่ภายใน 7 วัน
	receive(p3, "2026-03-28")
	invoice(p3, "INV-3", "2026-03-28")
	po("2026-03-20", "300", true)                        // ค้างรับ
	receive(po("2026-03-21", "400", true), "2026-03-21") // รับแล้ว ค้างแจ้งหนี้
	po("2026-03-22", "50", false)                        // ร่าง

	a.expect(a.do("GET", base+"/dashboards/purchasing?end_date=2026-03-31", owner, nil), 400, "start_date missing")
	a.expect(a.do("GET", base+"/dashboards/purchasing?start_date=2026-03-01&end_date=2026-03-31", staff, nil), 403, "no permission")
	d := a.expect(a.do("GET", base+"/dashboards/purchasing?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard").data()

	if p := d["purchases"].(obj); p["current"] != "1200" || p["previous"] != "500" || d["invoice_count"].(float64) != 2 {
		t.Fatalf("purchases = %v count = %v", p, d["invoice_count"])
	}
	months := objs(d["monthly"])
	if len(months) != 12 || months[10]["month"] != "2026-02" || months[10]["net"] != "500" || months[11]["net"] != "1200" || months[0]["net"] != "0" {
		t.Fatalf("monthly = %v", months)
	}
	if o := d["orders"].(obj); o["draft_count"].(float64) != 1 || o["to_receive_count"].(float64) != 1 || o["to_invoice_count"].(float64) != 1 {
		t.Fatalf("orders = %v", o)
	}
	ap := d["payables"].(obj)
	aging := ap["aging"].(obj)
	if ap["outstanding"] != "1819" || ap["overdue"] != "1605" || ap["due_7_days"] != "214" ||
		aging["not_due"] != "214" || aging["days_1_30"] != "1070" || aging["days_31_60"] != "535" {
		t.Fatalf("payables = %v", ap)
	}
	if top := objs(d["top_suppliers"]); len(top) != 1 || top[0]["supplier"].(obj)["code"] != "S1" || top[0]["net"] != "1200" {
		t.Fatalf("top suppliers = %v", d["top_suppliers"])
	}
}
