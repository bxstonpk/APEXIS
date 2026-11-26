package inventory_test

import "testing"

// GET /dashboards/inventory: มูลค่าคงเหลือ (รวม / ต่อคลัง), สินค้าต่ำกว่าจุดสั่งซื้อ, เอกสารรับ / จ่ายในช่วง, มูลค่าเข้า / ออกรายเดือน
func TestInventoryDashboard(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "dash", "name": "DASH", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "fiscal year")
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	wh1 := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH1", "name": "คลังหลัก"}), 201, "WH1").id()
	wh2 := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH2", "name": "คลังสาขา"}), 201, "WH2").id()
	item := func(body obj) string {
		return a.expect(a.do("POST", base+"/items", owner, body), 201, "item "+body["code"].(string)).id()
	}
	skuA := item(obj{"code": "A", "name": "สินค้า A", "uom_id": pcs, "reorder_point": "5"})
	skuB := item(obj{"code": "B", "name": "สินค้า B", "uom_id": pcs, "reorder_point": "100"})
	item(obj{"code": "C", "name": "สินค้า C", "uom_id": pcs})
	item(obj{"code": "SVC", "name": "บริการ", "type": "service", "uom_id": pcs})

	doc := func(typ, date, wh, status string, lines ...obj) {
		a.expect(a.do("POST", base+"/stock-documents", owner, obj{"type": typ, "date": date, "warehouse_id": wh, "status": status, "lines": lines}), 201, typ+" "+date)
	}
	doc("receipt", "2026-02-05", wh1, "posted", obj{"item_id": skuA, "quantity": "10", "unit_cost": "10"}) // ก.พ. เข้า 100
	doc("receipt", "2026-03-03", wh2, "posted", obj{"item_id": skuB, "quantity": "20", "unit_cost": "5"})  // มี.ค. เข้า 100
	doc("issue", "2026-03-10", wh1, "posted", obj{"item_id": skuA, "quantity": "7"})                       // มี.ค. ออก 70 (FIFO @10)
	doc("receipt", "2026-03-20", wh1, "", obj{"item_id": skuA, "quantity": "50", "unit_cost": "10"})       // ร่าง — ไม่นับ

	a.expect(a.do("GET", base+"/dashboards/inventory?start_date=2026-03-01", owner, nil), 400, "end_date missing")
	a.expect(a.do("GET", base+"/dashboards/inventory?start_date=2026-03-01&end_date=2026-03-31", staff, nil), 403, "no permission")
	d := a.expect(a.do("GET", base+"/dashboards/inventory?start_date=2026-03-01&end_date=2026-03-31", owner, nil), 200, "dashboard").data()

	if d["stock_value"] != "130" || d["stock_item_count"].(float64) != 3 {
		t.Fatalf("stock value = %v items = %v", d["stock_value"], d["stock_item_count"])
	}
	if w := objs(d["warehouses"]); len(w) != 2 || w[0]["warehouse"].(obj)["code"] != "WH2" || w[0]["value"] != "100" || w[1]["value"] != "30" {
		t.Fatalf("warehouses = %v", d["warehouses"])
	}
	low := objs(d["below_reorder"])
	if d["below_reorder_count"].(float64) != 2 || len(low) != 2 || low[0]["item"].(obj)["code"] != "B" || low[1]["on_hand"] != "3" || low[1]["reorder_point"] != "5" {
		t.Fatalf("below reorder = %v (%v)", low, d["below_reorder_count"])
	}
	if m := d["movements"].(obj); m["receipt_count"].(float64) != 1 || m["issue_count"].(float64) != 1 {
		t.Fatalf("movements = %v", m)
	}
	months := objs(d["monthly"])
	if len(months) != 12 || months[10]["month"] != "2026-02" || months[10]["in_value"] != "100" || months[10]["out_value"] != "0" ||
		months[11]["in_value"] != "100" || months[11]["out_value"] != "70" {
		t.Fatalf("monthly = %v", months)
	}
}
