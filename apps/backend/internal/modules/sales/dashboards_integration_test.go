package sales_test

import "testing"

// GET /dashboards/sales: ยอดขายช่วงนี้ / ก่อนหน้า, 12 เดือน, งานค้าง, ลูกหนี้ตามอายุ, ลูกค้า 5 อันดับ
func TestSalesDashboard(t *testing.T) {
	a, owner := setup(t)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "dash", "name": "DASH", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for code, typ := range map[string]string{"1130": "asset", "2140": "liability", "4110": "revenue"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	sj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "SJ", "name": "สมุดขาย"}), 201, "journal").id()
	a.expect(a.do("PUT", base+"/sales-settings", owner, obj{"vat_registered": true, "vat_rate": "7", "journal_id": sj, "ar_account_id": acc["1130"],
		"output_vat_account_id": acc["2140"]}), 200, "settings")
	uom := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "JOB", "name": "งาน"}), 201, "uom").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "บริการ", "type": "service", "uom_id": uom, "revenue_account_id": acc["4110"]}), 201, "service").id()
	cust := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C1", "name": "ลูกค้าหนึ่ง", "tax_type": "juristic", "payment_term_days": 10}), 201, "customer").id()

	order := func(date, price string) string {
		id := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": cust, "date": date,
			"lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": price}}}), 201, "order "+date).id()
		a.expect(a.do("PATCH", base+"/sales-orders/"+id, owner, obj{"status": "approved"}), 200, "approve "+date)
		return id
	}
	invoice := func(orderID, date string) {
		a.expect(a.do("POST", base+"/sales-invoices", owner, obj{"order_id": orderID, "date": date,
			"lines": []obj{{"order_line_no": 1, "quantity": "1"}}}), 201, "invoice "+date)
	}
	invoice(order("2026-02-10", "500"), "2026-02-10")  // ช่วงก่อนหน้า (ครบกำหนด 20 ก.พ. -> เกิน 39 วัน ณ 31 มี.ค.)
	invoice(order("2026-03-05", "1000"), "2026-03-05") // ช่วงนี้ (ครบกำหนด 15 มี.ค. -> เกิน 16 วัน)
	order("2026-03-20", "300")                         // อนุมัติแล้ว ยังไม่ส่ง / ไม่ออกบิล
	a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": cust, "date": "2026-03-21",
		"lines": []obj{{"item_id": svc, "quantity": "1", "unit_price": "50"}}}), 201, "draft order")
	q := a.expect(a.do("POST", base+"/quotations", owner, obj{"customer_id": cust, "date": "2026-03-22", "valid_until": "2026-04-22",
		"lines": []obj{{"item_id": svc, "quantity": "2", "unit_price": "100"}}}), 201, "quotation").id()
	a.expect(a.do("PATCH", base+"/quotations/"+q, owner, obj{"status": "sent"}), 200, "send quotation")

	a.expect(a.do("GET", base+"/dashboards/sales?start_date=2026-03-01", owner, nil), 400, "end_date missing")
	d := a.expect(a.do("GET", base+"/dashboards/sales?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard").data()

	sales := d["sales"].(obj)
	if sales["current"] != "1000" || sales["previous"] != "500" || d["invoice_count"].(float64) != 1 {
		t.Fatalf("sales = %v count = %v", sales, d["invoice_count"])
	}
	// กราฟ = ปีบัญชี 2026 (ม.ค.–ธ.ค.); ปีก่อนยังไม่มียอด = ไม่ส่ง previous_year
	months := objs(d["monthly"])
	if len(months) != 12 || months[0]["month"] != "2026-01" || months[1]["net"] != "500" || months[2]["net"] != "1000" || months[11]["month"] != "2026-12" {
		t.Fatalf("monthly = %v", months)
	}
	if fy := d["fiscal_year"].(obj); fy["year"].(float64) != 2026 || fy["start_date"] != "2026-01-01" || fy["end_date"] != "2026-12-31" {
		t.Fatalf("fiscal_year = %v", fy)
	}
	if d["previous_year"] != nil {
		t.Fatalf("previous_year without data = %v", d["previous_year"])
	}
	if qs := d["quotations"].(obj); qs["open_count"].(float64) != 1 || qs["open_amount"] != "214" {
		t.Fatalf("quotations = %v", qs)
	}
	if o := d["orders"].(obj); o["draft_count"].(float64) != 1 || o["to_deliver_count"] != nil || o["to_invoice_count"].(float64) != 1 {
		t.Fatalf("orders = %v", o)
	}
	ar := d["receivables"].(obj)
	aging := ar["aging"].(obj)
	if ar["outstanding"] != "1605" || ar["overdue"] != "1605" || aging["days_1_30"] != "1070" || aging["days_31_60"] != "535" || aging["not_due"] != "0" {
		t.Fatalf("receivables = %v", ar)
	}
	if top := objs(d["top_customers"]); len(top) != 1 || top[0]["customer"].(obj)["code"] != "C1" || top[0]["net"] != "1000" {
		t.Fatalf("top customers = %v", d["top_customers"])
	}

	// ปีบัญชีก่อนหน้ามียอด = previous_year เดือนต่อเดือนกับ monthly
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2025, "start_date": "2025-01-01"}), 201, "FY2025")
	invoice(order("2025-03-10", "300"), "2025-03-10")
	d = a.expect(a.do("GET", base+"/dashboards/sales?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard with previous year").data()
	prev := objs(d["previous_year"])
	if len(prev) != 12 || prev[0]["month"] != "2025-01" || prev[2]["net"] != "300" || prev[11]["month"] != "2025-12" {
		t.Fatalf("previous_year = %v", prev)
	}
	// ไม่มีปีบัญชีครอบคลุม end_date = 12 เดือนล่าสุด ไม่มีการเทียบ
	d = a.expect(a.do("GET", base+"/dashboards/sales?start_date=2030-01-01&end_date=2030-01-31", owner, nil), 200, "dashboard without fiscal year").data()
	if m := objs(d["monthly"]); d["fiscal_year"] != nil || d["previous_year"] != nil || m[0]["month"] != "2029-02" || m[11]["month"] != "2030-01" {
		t.Fatalf("no fiscal year: fy=%v prev=%v monthly=%v", d["fiscal_year"], d["previous_year"], m)
	}

}
