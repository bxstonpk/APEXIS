package sales_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/sales"
)

// lastSales คือ sales.Module ของ setup ล่าสุด (ทดสอบ public API ที่ finance เรียก)
var lastSales *sales.Module

// ใบลดหนี้: รับคืนสินค้า (คืนสต็อกที่ต้นทุนเดิม, perpetual) / ลดราคา, ห้ามเกินที่เหลือ, ตัดยอดลูกหนี้,
// รายงานภาษีขายเป็นยอดติดลบ, ยกเลิกแล้วคืนยอด / สต็อก, ยกเลิกใบแจ้งหนี้ที่มีใบลดหนี้ไม่ได้
func TestCreditNotes(t *testing.T) {
	a, owner := setup(t)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	acc := map[string]string{}
	for code, typ := range map[string]string{"1101": "asset", "1130": "asset", "1150": "asset", "2140": "liability", "4100": "revenue",
		"4110": "revenue", "4052": "revenue", "5100": "expense"} {
		acc[code] = a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	sj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "SJ", "name": "สมุดขาย"}), 201, "journal").id()
	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	fg := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "FG", "name": "สินค้า", "inventory_account_id": acc["1150"], "revenue_account_id": acc["4100"]}), 201, "category").id()
	itemA := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "สินค้า A", "uom_id": pcs, "category_id": fg}), 201, "item A").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ติดตั้ง", "type": "service", "uom_id": pcs, "revenue_account_id": acc["4110"]}), 201, "service").id()
	wh := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026")
	a.expect(a.do("POST", base+"/stock-documents", owner, obj{"type": "receipt", "date": "2026-03-01", "warehouse_id": wh, "status": "posted",
		"lines": []obj{{"item_id": itemA, "quantity": "10", "unit_cost": "60"}}}), 201, "opening stock")
	settings := a.expect(a.do("PUT", base+"/sales-settings", owner, obj{"vat_registered": true, "vat_rate": "7", "journal_id": sj, "ar_account_id": acc["1130"],
		"output_vat_account_id": acc["2140"], "cogs_account_id": acc["5100"], "sales_returns_account_id": acc["4052"]}), 200, "settings").data()
	if settings["sales_returns_account_id"] != acc["4052"] {
		t.Fatalf("settings = %v", settings)
	}
	cust := a.expect(a.do("POST", base+"/customers", owner, obj{"code": "C", "name": "ลูกค้า C", "tax_id": "0105551234567", "tax_type": "juristic"}), 201, "customer").id()

	// ใบสั่งขาย A 5 @100 + SVC 1 @1,000 -> ส่งของ / ใบกำกับภาษี (ต้นทุน A 300)
	so := a.expect(a.do("POST", base+"/sales-orders", owner, obj{"customer_id": cust, "date": "2026-03-05",
		"lines": []obj{{"item_id": itemA, "quantity": "5", "unit_price": "100"}, {"item_id": svc, "quantity": "1", "unit_price": "1000"}}}), 201, "order").id()
	a.expect(a.do("PATCH", base+"/sales-orders/"+so, owner, obj{"status": "approved"}), 200, "approve")
	iv := a.expect(a.do("POST", base+"/sales-invoices", owner, obj{"order_id": so, "date": "2026-03-07", "with_delivery": true, "warehouse_id": wh,
		"lines": []obj{{"order_line_no": 1, "quantity": "5"}, {"order_line_no": 2, "quantity": "1"}}}), 201, "invoice").data()
	ivID, ivNo := iv["id"].(string), iv["document_no"].(string)
	if l := objs(iv["lines"]); l[0]["line_no"] != float64(1) || l[1]["line_no"] != float64(2) || l[0]["returned_quantity"] != "0" || iv["credited_amount"] != "0" {
		t.Fatalf("invoice lines = %v", iv["lines"])
	}

	entryLines := func(id string) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, l := range objs(a.expect(a.do("GET", base+"/journal-entries/"+id, owner, nil), 200, "entry").data()["lines"]) {
			out[l["account"].(obj)["code"].(string)] = l["debit"].(string) + "/" + l["credit"].(string)
		}
		return out
	}
	credit := func(body obj, status int, what string) obj {
		t.Helper()
		body["invoice_id"] = ivID
		if _, ok := body["date"]; !ok {
			body["date"] = "2026-03-10"
		}
		r := a.expect(a.do("POST", base+"/sales-credit-notes", owner, body), status, what)
		if status != 201 {
			return nil
		}
		return r.data()
	}
	invoice := func() obj {
		return a.expect(a.do("GET", base+"/sales-invoices/"+ivID, owner, nil), 200, "invoice").data()
	}
	book := func(q string) obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book="+q, owner, nil), 200, q).data()
	}

	// ---------- ตรวจรูปแบบ / กฎ ----------
	credit(obj{"kind": "return", "lines": []obj{{"invoice_line_no": 1, "quantity": "1"}}}, 400, "reason required")
	credit(obj{"kind": "swap", "reason": "x", "lines": []obj{{"invoice_line_no": 1, "quantity": "1"}}}, 400, "bad kind")
	credit(obj{"kind": "return", "reason": "ชำรุด", "date": "2026-03-06", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 1, "quantity": "1"}}}, 422, "before invoice date")
	credit(obj{"kind": "return", "reason": "ชำรุด", "lines": []obj{{"invoice_line_no": 1, "quantity": "1"}}}, 422, "warehouse required for stock return")
	credit(obj{"kind": "return", "reason": "ชำรุด", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 1, "quantity": "6"}}}, 422, "return more than invoiced")
	credit(obj{"kind": "return", "reason": "ชำรุด", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 1, "quantity": "1", "unit_price": "150"}}}, 422, "price above invoice")
	credit(obj{"kind": "return", "reason": "ชำรุด", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 9, "quantity": "1"}}}, 400, "unknown invoice line")

	// ---------- รับคืน A 2 ชิ้น: Dr รับคืน 200 + ภาษีขาย 14 / Cr ลูกหนี้ 214; Dr สินค้าคงเหลือ 120 / Cr ต้นทุนขาย 120 ----------
	cn1 := credit(obj{"kind": "return", "reason": "สินค้าชำรุด", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 1, "quantity": "2"}}}, 201, "return")
	if cn1["document_no"] != "CN2026-000001" || cn1["kind"] != "return" || cn1["invoice"].(obj)["document_no"] != ivNo ||
		cn1["totals"].(obj)["gross"] != "214" || cn1["stock_document_no"] == "" || cn1["original_totals"].(obj)["gross"] != "1605" ||
		cn1["correct_totals"].(obj)["gross"] != "1391" || cn1["bill_to"].(obj)["tax_id"] != "0105551234567" {
		t.Fatalf("credit note 1 = %v", cn1)
	}
	if l := objs(cn1["lines"])[0]; l["line_no"] != float64(1) || l["invoice_line_no"] != float64(1) || l["quantity"] != "2" || l["unit_price"] != "100" ||
		l["cost"] != "120" || l["account_id"] != acc["4052"] || l["item"].(obj)["code"] != "A" {
		t.Fatalf("credit note 1 line = %v", l)
	}
	want := map[string]string{"4052": "200/0", "2140": "14/0", "1130": "0/214", "1150": "120/0", "5100": "0/120"}
	if got := entryLines(cn1["entry_id"].(string)); len(got) != len(want) {
		t.Fatalf("credit note 1 entry = %v", got)
	} else {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("credit note 1 %s = %s, want %s", k, got[k], v)
			}
		}
	}
	if v := book("stock-on-hand&end_date=2026-03-31"); v["total_value"] != "420" {
		t.Fatalf("stock after return = %v", v["total_value"])
	}
	if d := invoice(); d["credited_amount"] != "214" || d["received_amount"] != "0" || d["outstanding"] != "1391" ||
		objs(d["lines"])[0]["returned_quantity"] != "2" || objs(d["lines"])[0]["credited_amount"] != "200" {
		t.Fatalf("invoice after return = %v", d)
	}
	credit(obj{"kind": "return", "reason": "ชำรุด", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 1, "quantity": "4"}}}, 422, "return more than left")

	// ---------- ลดราคาค่าติดตั้ง 100 (+VAT 7) ----------
	cn2 := credit(obj{"kind": "allowance", "reason": "ติดตั้งล่าช้า", "lines": []obj{{"invoice_line_no": 2, "amount": "100"}}}, 201, "allowance")
	if cn2["document_no"] != "CN2026-000002" || cn2["totals"].(obj)["gross"] != "107" || cn2["stock_document_no"] != "" ||
		cn2["credited_before"].(obj)["gross"] != "214" || cn2["correct_totals"].(obj)["gross"] != "1284" || objs(cn2["lines"])[0]["quantity"] != "0" {
		t.Fatalf("credit note 2 = %v", cn2)
	}
	credit(obj{"kind": "allowance", "reason": "x", "lines": []obj{{"invoice_line_no": 2, "amount": "950"}}}, 422, "allowance above line net")
	credit(obj{"kind": "allowance", "reason": "x", "warehouse_id": wh, "lines": []obj{{"invoice_line_no": 2, "amount": "10"}}}, 400, "warehouse with allowance")
	a.expect(a.do("PATCH", base+"/sales-invoices/"+ivID, owner, obj{"status": "cancelled"}), 422, "cancel invoice with credit notes")
	if l := a.expect(a.do("GET", base+"/sales-credit-notes?invoice_id="+ivID, owner, nil), 200, "list").body["data"]; len(objs(l)) != 2 {
		t.Fatalf("credit notes of invoice = %v", l)
	}

	// ---------- รายงาน: ภาษีขายติดลบ / อายุลูกหนี้ / ยอดรวมสำหรับ ภ.พ.30 ----------
	vat := book("output-vat&start_date=2026-03-01&end_date=2026-03-31")
	if rows := objs(vat["rows"]); len(rows) != 3 || vat["total_base"] != "1200" || vat["total_tax"] != "84" {
		t.Fatalf("output VAT = %v", vat)
	} else {
		found := false
		for _, r := range rows {
			if r["document_no"] == "CN2026-000001" {
				found = r["kind"] == "credit_note" && r["reference_no"] == ivNo && r["base"] == "-200" && r["tax"] == "-14"
			}
		}
		if !found {
			t.Fatalf("credit note row = %v", rows)
		}
	}
	if r := book("ar-aging&end_date=2026-03-31"); r["totals"].(obj)["total"] != "1284" {
		t.Fatalf("AR aging = %v", r)
	}
	cid, _ := uuid.Parse(company)
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	if tot, err := lastSales.OutputVat(context.Background(), cid, day("2026-03-01"), day("2026-03-31")); err != nil ||
		tot.Base.String() != "1200" || tot.Tax.String() != "84" || tot.Documents != 3 {
		t.Fatalf("Module.OutputVat = %+v, %v", tot, err)
	}
	if tot, err := lastSales.OutputVat(context.Background(), cid, day("2026-04-01"), day("2026-04-30")); err != nil || !tot.Base.IsZero() || tot.Documents != 0 {
		t.Fatalf("Module.OutputVat empty month = %+v, %v", tot, err)
	}

	// ---------- ยกเลิกใบลดหนี้รับคืน: คืนยอดค้าง + ตัดสต็อกที่รับคืนออก + กลับรายการ ----------
	c1 := a.expect(a.do("PATCH", base+"/sales-credit-notes/"+cn1["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel credit note").data()
	if c1["status"] != "cancelled" {
		t.Fatalf("cancelled = %v", c1)
	}
	a.expect(a.do("PATCH", base+"/sales-credit-notes/"+cn1["id"].(string), owner, obj{"status": "cancelled"}), 422, "cancel twice")
	if v := book("stock-on-hand&end_date=2026-03-31"); v["total_value"] != "300" {
		t.Fatalf("stock after cancelling the return = %v", v["total_value"])
	}
	if d := invoice(); d["outstanding"] != "1498" || objs(d["lines"])[0]["returned_quantity"] != "0" || d["credited_amount"] != "107" {
		t.Fatalf("invoice after cancelling the return = %v", d)
	}
	if r := book("output-vat&start_date=2026-03-01&end_date=2026-03-31"); r["total_base"] != "1400" {
		t.Fatalf("output VAT after cancel = %v", r)
	}

	// ---------- ใบลดหนี้เกินยอดค้าง: รับชำระที่เหลือ 1,498 แล้วลดหนี้อีกไม่ได้ ----------
	cash := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "CASH", "name": "เงินสด", "type": "cash", "gl_account_id": acc["1101"]}), 201, "cash").id()
	rj := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "CRJ", "name": "สมุดรายวันรับ"}), 201, "receipt journal").id()
	a.expect(a.do("PUT", base+"/finance-settings", owner, obj{"receipt_journal_id": rj}), 200, "finance settings")
	a.expect(a.do("POST", base+"/receipts", owner, obj{"kind": "customer", "customer_id": cust, "date": "2026-03-20",
		"lines": []obj{{"invoice_id": ivID, "amount": "1498"}}, "methods": []obj{{"cash_account_id": cash, "amount": "1498"}}}), 201, "receipt")
	credit(obj{"kind": "allowance", "reason": "x", "date": "2026-03-21", "lines": []obj{{"invoice_line_no": 1, "amount": "10"}}}, 422, "credit above outstanding")
	if d := invoice(); d["received_amount"] != "1498" || d["credited_amount"] != "107" || d["outstanding"] != "0" {
		t.Fatalf("invoice after receipt = %v", d)
	}
}
