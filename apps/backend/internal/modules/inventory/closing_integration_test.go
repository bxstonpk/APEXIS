package inventory_test

import (
	"strings"
	"testing"
)

// ระบบ periodic: ตั้งระบบต่อปีบัญชี, บัญชีตามหมวด, ปรับปรุงสินค้าคงเหลือสิ้นงวดผ่าน ledger, ยกเลิก (กลับรายการ)
func TestPeriodicClosing(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "fiscal year")

	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	inv := account("1150", "asset")
	cogs := account("5100", "expense")
	purchase := account("5110", "expense")
	header := a.expect(a.do("POST", base+"/accounts", owner, obj{"code": "1000", "name": "h", "type": "asset", "postable": false}), 201, "header").id()
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "JV", "name": "JV"}), 201, "journal").id()

	// ---------- บัญชีของหมวด / สินค้า ตรวจกับ ledger ----------
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "BAD", "name": "x", "inventory_account_id": header}), 422, "category with header account")
	cat := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "RAW", "name": "วัตถุดิบ", "inventory_account_id": inv, "purchase_account_id": purchase}), 201, "category")
	if cat.data()["inventory_account_id"] != inv || cat.data()["purchase_account_id"] != purchase {
		t.Fatalf("category = %v", cat.data())
	}
	item := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "A", "uom_id": pcs, "category_id": cat.id()}), 201, "item").id()
	a.expect(a.do("POST", base+"/items", owner, obj{"code": "B", "name": "B", "uom_id": pcs, "purchase_account_id": "0195f0a0-0000-7000-8000-000000000000"}), 422, "item with unknown account")

	// ---------- ค่าตั้ง ----------
	a.expect(a.do("PUT", base+"/inventory-settings", owner, obj{"cogs_account_id": cogs, "journal_id": "0195f0a0-0000-7000-8000-000000000000"}), 422, "unknown journal")
	s := a.expect(a.do("PUT", base+"/inventory-settings", owner, obj{"cogs_account_id": cogs, "journal_id": jv}), 200, "settings").data()
	if s["cogs_account_id"] != cogs || s["journal_id"] != jv || len(s["accounting_systems"].([]any)) != 0 {
		t.Fatalf("settings = %v", s)
	}
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026 (default = periodic)")
	a.expect(a.do("POST", base+"/inventory-closings", owner, obj{"date": "2026-01-31"}), 422, "closing in perpetual year")
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "fifo"}), 400, "invalid system")
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "periodic"}), 200, "periodic from 2026")

	doc := func(typ, date string, line obj) obj {
		return obj{"type": typ, "date": date, "warehouse_id": wh(a, base, owner), "status": "posted", "lines": []obj{line}}
	}
	post := func(body obj, status int, what string) {
		t.Helper()
		a.expect(a.do("POST", base+"/stock-documents", owner, body), status, what)
	}
	post(doc("receipt", "2026-01-05", obj{"item_id": item, "quantity": "10", "unit_cost": "10"}), 201, "receipt 10 @ 10")
	post(doc("issue", "2026-01-20", obj{"item_id": item, "quantity": "4"}), 201, "issue 4 (FIFO 40)")
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 422, "change system of year with postings")
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2027", owner, obj{"system": "perpetual"}), 200, "perpetual from 2027")

	// ---------- ปิดงวด ม.ค.: สินค้าคงเหลือ 60, ledger 0 -> Dr สินค้าคงเหลือ 60 / Cr ต้นทุนขาย 60 ----------
	closeAt := func(date string, status int, what string) resp {
		t.Helper()
		return a.expect(a.do("POST", base+"/inventory-closings", owner, obj{"date": date}), status, what)
	}
	jan := closeAt("2026-01-15", 201, "close Jan")
	if d := jan.data(); d["period_end"] != "2026-01-31" || d["adjustment"] != "60" || !strings.HasPrefix(d["entry_no"].(string), "JV2026-") || d["status"] != "posted" {
		t.Fatalf("Jan closing = %v", d)
	}
	closeAt("2026-01-31", 409, "close Jan twice")
	post(doc("receipt", "2026-01-25", obj{"item_id": item, "quantity": "1", "unit_cost": "1"}), 422, "receipt dated in closed inventory period")

	// ---------- ก.พ.: รับ 5 @ 20 -> 160; ledger มี 60 จากงวดก่อน -> ปรับเพิ่ม 100 ----------
	post(doc("receipt", "2026-02-03", obj{"item_id": item, "quantity": "5", "unit_cost": "20"}), 201, "receipt Feb")
	feb := closeAt("2026-02-28", 201, "close Feb")
	if feb.data()["adjustment"] != "100" {
		t.Fatalf("Feb closing = %v", feb.data())
	}
	cancel := func(id string, status int, what string) resp {
		t.Helper()
		return a.expect(a.do("PATCH", base+"/inventory-closings/"+id, owner, obj{"status": "cancelled"}), status, what)
	}
	cancel(jan.id(), 422, "cancel Jan while Feb is closed")
	if c := cancel(feb.id(), 200, "cancel Feb").data(); c["status"] != "cancelled" || c["cancelled_by"] == nil {
		t.Fatalf("cancelled Feb = %v", c)
	}
	cancel(feb.id(), 404, "cancel Feb twice")
	// กลับรายการแล้ว ledger กลับเป็น 60 -> ปิดใหม่ได้ส่วนต่าง 100 เท่าเดิม
	if again := closeAt("2026-02-01", 201, "close Feb again"); again.data()["adjustment"] != "100" {
		t.Fatalf("Feb closing again = %v", again.data())
	}
	if r := a.expect(a.do("GET", base+"/inventory-closings?fiscal_year=2026&status=posted", owner, nil), 200, "list closings"); r.total() != 2 {
		t.Fatalf("posted closings = %d", r.total())
	}
	if r := a.expect(a.do("GET", base+"/inventory-settings", owner, nil), 200, "settings").data(); len(r["accounting_systems"].([]any)) != 2 {
		t.Fatalf("systems = %v", r["accounting_systems"])
	}
	a.expect(a.do("GET", base+"/inventory-closings?status=open", owner, nil), 400, "bad closing status filter")
}

var warehouses = map[string]string{}

// wh: คลังเดียวของบริษัทใน test (สร้างครั้งแรกที่เรียก)
func wh(a api, base, token string) string {
	if id, ok := warehouses[base]; ok {
		return id
	}
	id := a.expect(a.do("POST", base+"/warehouses", token, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	warehouses[base] = id
	return id
}
