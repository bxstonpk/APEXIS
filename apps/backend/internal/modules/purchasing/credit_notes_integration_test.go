package purchasing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ใบลดหนี้ผู้ขาย: ส่งคืนสินค้า (perpetual, ต้นทุน FIFO ≠ ราคาใบแจ้งหนี้ -> ผลต่างราคา), ลดราคา, ลดหนี้เกิน,
// เลขที่ซ้ำ, ยกเลิกใบแจ้งหนี้ที่มีใบลดหนี้, ยกเลิกใบลดหนี้ (คืนสต็อก / ยอดค้าง), รายงานภาษีซื้อ, อายุเจ้าหนี้, Module.InputVat, periodic
func TestSupplierCreditNotes(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2027, "start_date": "2027-01-01"}), 201, "FY2027")

	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	invAcc, vatAcc := account("1150", "asset"), account("1160", "asset")
	apAcc, grirAcc := account("2100", "liability"), account("2110", "liability")
	purchaseAcc, varianceAcc, returnsAcc := account("5110", "expense"), account("5120", "expense"), account("5130", "expense")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "สมุดรายวันซื้อ"}), 201, "journal").id()

	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "PCS", "name": "ชิ้น"}), 201, "uom").id()
	raw := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "RAW", "name": "วัตถุดิบ", "inventory_account_id": invAcc, "purchase_account_id": purchaseAcc}), 201, "category").id()
	itemA := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "สินค้า A", "uom_id": pcs, "category_id": raw}), 201, "item A").id()
	svc := a.expect(a.do("POST", base+"/items", owner, obj{"code": "SVC", "name": "ค่าบริการ", "type": "service", "uom_id": pcs, "purchase_account_id": purchaseAcc}), 201, "service").id()
	wh := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2026", owner, obj{"system": "perpetual"}), 200, "perpetual 2026")
	a.expect(a.do("PUT", base+"/inventory-settings/accounting-systems/2027", owner, obj{"system": "periodic"}), 200, "periodic 2027")

	s := a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv, "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "price_variance_account_id": varianceAcc, "purchase_returns_account_id": returnsAcc}), 200, "settings").data()
	if s["purchase_returns_account_id"] != returnsAcc {
		t.Fatalf("settings = %v", s)
	}
	sup := a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": "V", "name": "ผู้ขาย V", "tax_id": "0105551234567", "payment_term_days": 30, "vat_registered": true, "tax_type": "juristic"}), 201, "supplier").id()

	// ใบสั่งซื้อ -> รับ A 10 @100 + บริการ -> ใบแจ้งหนี้ A @110 (ผลต่าง 100) + บริการ 500: ก่อน VAT 1600, VAT 112, รวม 1712
	line := func(item, qty, price string) obj { return obj{"item_id": item, "quantity": qty, "unit_price": price} }
	qty := func(n int, q string) obj { return obj{"order_line_no": n, "quantity": q} }
	purchase := func(date, invNo, price string) (string, obj) {
		t.Helper()
		po := a.expect(a.do("POST", base+"/purchase-orders", owner, obj{"supplier_id": sup, "date": date, "lines": []obj{line(itemA, "10", "100"), line(svc, "1", "500")}}), 201, "PO").id()
		a.expect(a.do("PATCH", base+"/purchase-orders/"+po, owner, obj{"status": "approved"}), 200, "approve")
		a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": po, "date": date, "warehouse_id": wh, "lines": []obj{qty(1, "10"), qty(2, "1")}}), 201, "receipt")
		inv := a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"order_id": po, "supplier_invoice_no": invNo, "date": date,
			"lines": []obj{{"order_line_no": 1, "quantity": "10", "unit_price": price}, qty(2, "1")}}), 201, "invoice").data()
		return inv["id"].(string), inv
	}
	invID, inv := purchase("2026-02-10", "INV-1", "110")
	if l := objs(inv["lines"]); l[0]["line_no"] != float64(1) || l[1]["line_no"] != float64(2) || l[0]["returned_quantity"] != "0" ||
		inv["totals"].(obj)["gross"] != "1712" || inv["credited_amount"] != "0" {
		t.Fatalf("invoice = %v", inv)
	}

	cn := func(no, kind, date string, warehouse any, lines ...obj) obj {
		return obj{"invoice_id": invID, "supplier_credit_note_no": no, "date": date, "kind": kind, "reason": "สินค้าชำรุด",
			"warehouse_id": warehouse, "lines": lines}
	}
	ret := func(n int, q string) obj { return obj{"invoice_line_no": n, "quantity": q} }
	allow := func(n int, amt string) obj { return obj{"invoice_line_no": n, "amount": amt} }

	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "return", "2026-02-15", nil, ret(1, "3"))), 422, "return stock without warehouse")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "return", "2026-02-15", wh, ret(1, "11"))), 422, "return more than invoiced")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "return", "2026-02-15", wh, allow(1, "10"))), 400, "amount on return")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "return", "2026-02-01", wh, ret(1, "3"))), 422, "before invoice date")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "swap", "2026-02-15", wh, ret(1, "3"))), 400, "bad kind")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, obj{"invoice_id": invID, "supplier_credit_note_no": "CN-1", "date": "2026-02-15",
		"kind": "return", "warehouse_id": wh, "lines": []obj{ret(1, "3")}}), 400, "reason missing")

	// ส่งคืน A 3 @110: ก่อน VAT 330, VAT 23.1 — ต้นทุน FIFO 300 -> Cr สินค้าคงเหลือ 300 + ผลต่างราคา 30
	c1 := a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "return", "2026-02-15", wh, ret(1, "3"))), 201, "credit note return").data()
	l1 := objs(c1["lines"])[0]
	if c1["document_no"] != "PCN2026-000001" || c1["kind"] != "return" || c1["totals"].(obj)["gross"] != "353.1" || c1["accounting_system"] != "perpetual" ||
		c1["stock_document_no"] == "" || c1["entry_no"] == "" || c1["invoice"].(obj)["supplier_invoice_no"] != "INV-1" ||
		l1["line_no"] != float64(1) || l1["invoice_line_no"] != float64(1) || l1["cost"] != "300" || l1["price_variance"] != "30" ||
		l1["inventory_account_id"] != invAcc || l1["account_id"] != varianceAcc || l1["item"].(obj)["code"] != "A" {
		t.Fatalf("credit note 1 = %v", c1)
	}
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "allowance", "2026-02-16", nil, allow(2, "10"))), 409, "duplicate supplier credit note no")

	// ลดราคาบริการ 100: ก่อน VAT 100, VAT 7 -> Cr ส่งคืนสินค้าและส่วนลดรับ
	c2 := a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-2", "allowance", "2026-02-16", nil, allow(2, "100"))), 201, "credit note allowance").data()
	if l := objs(c2["lines"])[0]; c2["totals"].(obj)["gross"] != "107" || l["account_id"] != returnsAcc || l["quantity"] != "0" || l["stock"] != false || c2["stock_document_no"] != "" {
		t.Fatalf("credit note 2 = %v", c2)
	}
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-3", "allowance", "2026-02-16", nil, allow(2, "401"))), 422, "allowance above line left (400)")
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-3", "return", "2026-02-16", wh, ret(1, "8"))), 422, "return above qty left (7)")

	got := a.expect(a.do("GET", base+"/supplier-invoices/"+invID, owner, nil), 200, "invoice after credits").data()
	gl := objs(got["lines"])
	if gl[0]["returned_quantity"] != "3" || gl[0]["credited_amount"] != "330" || gl[1]["credited_amount"] != "100" ||
		got["credited_amount"] != "460.1" || got["paid_amount"] != "0" || got["outstanding"] != "1251.9" {
		t.Fatalf("invoice after credits = %v", got)
	}
	if r := a.expect(a.do("GET", base+"/supplier-credit-notes?invoice_id="+invID+"&status=posted", owner, nil), 200, "list"); r.total() != 2 {
		t.Fatalf("credit notes listed = %d", r.total())
	}
	if ag := a.expect(a.do("GET", base+"/reports?book=ap-aging&end_date=2026-02-28", owner, nil), 200, "aging").data(); ag["totals"].(obj)["total"] != "1251.9" {
		t.Fatalf("aging = %v", ag)
	}
	if v := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-02-28", owner, nil), 200, "stock").data(); v["total_value"] != "700" {
		t.Fatalf("stock after return = %v", v["total_value"])
	}
	tb := a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "trial balance").data()
	closing := map[string]string{}
	for _, r := range objs(tb["rows"]) {
		closing[r["account"].(obj)["code"].(string)] = r["closing_balance"].(string)
	}
	for code, want := range map[string]string{"1150": "700", "1160": "81.9", "2100": "-1251.9", "2110": "0", "5110": "500", "5120": "70", "5130": "-100"} {
		if closing[code] != want {
			t.Errorf("TB %s = %q, want %s (all = %v)", code, closing[code], want, closing)
		}
	}

	// รายงานภาษีซื้อ: ใบแจ้งหนี้ + ใบลดหนี้ติดลบ
	vat := a.expect(a.do("GET", base+"/reports?book=input-vat&start_date=2026-02-01&end_date=2026-02-28", owner, nil), 200, "input vat").data()
	rows := objs(vat["rows"])
	if len(rows) != 3 || vat["total_base"] != "1170" || vat["total_tax"] != "81.9" || rows[0]["kind"] != "invoice" || rows[0]["tax_invoice_no"] != "INV-1" ||
		rows[1]["kind"] != "credit_note" || rows[1]["tax"] != "-23.1" || rows[1]["reference_no"] != "INV-1" || rows[1]["tax_invoice_no"] != "CN-1" ||
		rows[0]["tax_id"] != "0105551234567" || rows[0]["branch_no"] != "00000" || rows[0]["supplier"].(obj)["name"] != "ผู้ขาย V" {
		t.Fatalf("input vat = %v", vat)
	}
	a.expect(a.do("GET", base+"/reports?book=input-vat&end_date=2026-02-28", owner, nil), 400, "input vat without start_date")

	// ยกเลิก: ใบแจ้งหนี้ที่มีใบลดหนี้ยกเลิกไม่ได้; ยกเลิกใบลดหนี้คืนสต็อก + ยอดค้าง
	cancel := func(path string, status int, what string) obj {
		t.Helper()
		return a.expect(a.do("PATCH", base+path, owner, obj{"status": "cancelled"}), status, what).body
	}
	if b := cancel("/supplier-invoices/"+invID, 422, "cancel invoice with credits"); b["error"].(obj)["message"] != "invoice has credit notes (cancel the credit notes first)" {
		t.Fatalf("cancel invoice error = %v", b)
	}
	cancel("/supplier-credit-notes/"+c1["id"].(string), 200, "cancel CN-1")
	cancel("/supplier-credit-notes/"+c1["id"].(string), 422, "cancel CN-1 twice")
	if v := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-02-28", owner, nil), 200, "stock after cancel").data(); v["total_value"] != "1000" {
		t.Fatalf("stock after cancel = %v", v["total_value"])
	}
	if got := a.expect(a.do("GET", base+"/supplier-invoices/"+invID, owner, nil), 200, "invoice after cancel").data(); got["outstanding"] != "1605" || got["credited_amount"] != "107" {
		t.Fatalf("invoice after cancel = %v", got)
	}
	// เลขที่ใบลดหนี้ที่ยกเลิกแล้วใช้ใหม่ได้
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-1", "allowance", "2026-02-20", nil, allow(1, "10"))), 201, "reuse cancelled number")

	// Module.InputVat (finance ใช้ทำ ภ.พ.30): ใบแจ้งหนี้ 1600 / 112 - CN-2 100 / 7 - CN-1 ใหม่ 10 / 0.7
	totals, err := a.pur.InputVat(context.Background(), uuid.MustParse(company), date("2026-02-01"), date("2026-02-28"))
	if err != nil || totals.Base.String() != "1490" || totals.Tax.String() != "104.3" || totals.Documents != 3 {
		t.Fatalf("InputVat = %+v, %v", totals, err)
	}
	if empty, err := a.pur.InputVat(context.Background(), uuid.MustParse(company), date("2026-03-01"), date("2026-03-31")); err != nil || !empty.Base.IsZero() || empty.Documents != 0 {
		t.Fatalf("InputVat March = %+v, %v", empty, err)
	}

	// ---------- periodic (2027): ส่งคืนสินค้า stock -> เบิกออก (จำนวน) แต่ Cr ส่งคืนสินค้า ไม่ใช่สินค้าคงเหลือ ----------
	invID, _ = purchase("2027-01-10", "INV-2", "100")
	c3 := a.expect(a.do("POST", base+"/supplier-credit-notes", owner, cn("CN-9", "return", "2027-01-12", wh, ret(1, "2"))), 201, "periodic return").data()
	l3 := objs(c3["lines"])[0]
	if c3["accounting_system"] != "periodic" || c3["stock_document_no"] == "" || c3["document_no"] != "PCN2027-000001" ||
		l3["account_id"] != returnsAcc || l3["inventory_account_id"] != nil || l3["price_variance"] != "0" || c3["totals"].(obj)["gross"] != "214" {
		t.Fatalf("periodic credit note = %v", c3)
	}
}

func date(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}
