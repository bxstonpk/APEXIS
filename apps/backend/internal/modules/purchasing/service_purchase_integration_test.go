package purchasing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/platform/integration"
)

// สายจ้างบริการ: ใบสั่งจ้าง -> ตรวจรับงาน (บางส่วน, ไม่ลงบัญชี) -> ซื้อบริการ (ภาษีซื้อยังไม่ถึงกำหนด) -> จ่าย (finance)
// -> บันทึกใบกำกับภาษีที่ได้รับ (ตั้งต้นตามสัดส่วนที่จ่าย / ส่วนที่เหลือ) -> ยกเลิกใบกำกับ, ใบกำกับมาพร้อมใบแจ้งหนี้,
// ซื้อตรง (บริการ / สินค้า), ใบลดหนี้ของภาษีที่ยังไม่ถึงกำหนด, รายงานภาษีซื้อ / ยังไม่ถึงกำหนด, Module.InputVat, สิทธิ์
func TestServicePurchaseFlow(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "svc", "name": "SVC", "business_type": "service"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")

	account := func(code, typ string) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/accounts", owner, obj{"code": code, "name": code, "type": typ}), 201, "account "+code).id()
	}
	invAcc, vatAcc, undueAcc := account("1150", "asset"), account("1160", "asset"), account("1170", "asset")
	apAcc, grirAcc := account("2100", "liability"), account("2110", "liability")
	purchaseAcc, expenseAcc, consultAcc := account("5110", "expense"), account("5200", "expense"), account("5210", "expense")
	jv := a.expect(a.do("POST", base+"/journals", owner, obj{"code": "PJ", "name": "สมุดรายวันซื้อ"}), 201, "journal").id()
	a.expect(a.do("PUT", base+"/purchasing-settings", owner, obj{"vat_rate": "7", "journal_id": jv, "grir_account_id": grirAcc, "ap_account_id": apAcc,
		"input_vat_account_id": vatAcc, "undue_input_vat_account_id": undueAcc, "service_expense_account_id": expenseAcc}), 200, "settings")

	pcs := a.expect(a.do("POST", base+"/uoms", owner, obj{"code": "HR", "name": "ชั่วโมง"}), 201, "uom").id()
	raw := a.expect(a.do("POST", base+"/item-categories", owner, obj{"code": "RAW", "name": "วัสดุ", "inventory_account_id": invAcc, "purchase_account_id": purchaseAcc}), 201, "category").id()
	itemA := a.expect(a.do("POST", base+"/items", owner, obj{"code": "A", "name": "วัสดุ A", "uom_id": pcs, "category_id": raw}), 201, "goods").id()
	cons := a.expect(a.do("POST", base+"/items", owner, obj{"code": "CONS", "name": "ที่ปรึกษา", "type": "service", "uom_id": pcs, "purchase_account_id": purchaseAcc}), 201, "service").id()
	wh := a.expect(a.do("POST", base+"/warehouses", owner, obj{"code": "WH", "name": "คลัง"}), 201, "warehouse").id()
	supplier := func(code, taxID string, vat bool) string {
		t.Helper()
		return a.expect(a.do("POST", base+"/suppliers", owner, obj{"code": code, "name": code, "tax_id": taxID, "payment_term_days": 30, "vat_registered": vat, "tax_type": "juristic"}), 201, "supplier "+code).id()
	}
	vatSup, plainSup := supplier("V", "0105551234567", true), supplier("N", "0105557654321", false)
	line := func(item, qty, price string) obj { return obj{"item_id": item, "quantity": qty, "unit_price": price} }
	qty := func(n int, q string) obj { return obj{"order_line_no": n, "quantity": q} }
	pay := func(no, invoiceID string, amount string) {
		t.Helper()
		_, err := a.pur.SettlePayables(context.Background(), uuid.MustParse(company), uuid.MustParse(vatSup),
			integration.SettlementDoc{DocumentID: uuid.New(), DocumentNo: no, Date: time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)},
			[]integration.Settlement{{InvoiceID: uuid.MustParse(invoiceID), Amount: decimal.RequireFromString(amount)}})
		if err != nil {
			t.Fatalf("pay %s: %v", no, err)
		}
	}

	// ---------- ใบสั่งจ้าง: บริการเท่านั้น, เลขที่ WO ----------
	if e := a.expect(a.do("POST", base+"/work-orders", owner, obj{"supplier_id": vatSup, "date": "2026-02-01", "lines": []obj{line(itemA, "1", "1")}}), 422, "goods on WO"); e.errorCode() != "service_document_goods_line" {
		t.Fatalf("goods on WO = %v", e.errorCode())
	}
	wo := a.expect(a.do("POST", base+"/work-orders", owner, obj{"supplier_id": vatSup, "date": "2026-02-01", "description": "วางระบบบัญชี",
		"lines": []obj{line(cons, "10", "1000")}}), 201, "WO").data()
	woID := wo["id"].(string)
	if wo["kind"] != "service" || wo["totals"].(obj)["gross"] != "10700" {
		t.Fatalf("WO = %v", wo)
	}
	a.expect(a.do("GET", base+"/purchase-orders/"+woID, owner, nil), 404, "WO through purchase-orders")
	if no := a.expect(a.do("PATCH", base+"/work-orders/"+woID, owner, obj{"status": "approved"}), 200, "approve WO").data()["order_no"]; no != "WO2026-000001" {
		t.Fatalf("WO no = %v", no)
	}
	if r := a.expect(a.do("GET", base+"/purchase-orders", owner, nil), 200, "PO list"); r.total() != 0 {
		t.Fatalf("PO list should not include work orders, total = %d", r.total())
	}

	// ---------- ตรวจรับงาน 4 ชม.: ไม่มีบัญชี / สต็อก ----------
	sa := a.expect(a.do("POST", base+"/service-acceptances", owner, obj{"order_id": woID, "date": "2026-02-05", "lines": []obj{qty(1, "4")}}), 201, "acceptance").data()
	if sa["document_no"] != "SA2026-000001" || sa["entry_no"] != "" || sa["stock_document_no"] != "" || sa["kind"] != "service" {
		t.Fatalf("acceptance = %v", sa)
	}
	a.expect(a.do("POST", base+"/goods-receipts", owner, obj{"order_id": woID, "date": "2026-02-05", "lines": []obj{qty(1, "1")}}), 404, "goods receipt of WO")

	// ---------- ซื้อบริการ 4 ชม.: Dr ค่าใช้จ่าย 4000 + ภาษีซื้อยังไม่ถึงกำหนด 280 / Cr เจ้าหนี้ 4280 ----------
	si := func(no string, extra obj, lines ...obj) obj {
		b := obj{"order_id": woID, "supplier_invoice_no": no, "date": "2026-02-10", "lines": lines}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	a.expect(a.do("POST", base+"/supplier-service-invoices", owner, si("S-1", nil, qty(1, "5"))), 422, "invoice more than accepted")
	inv := a.expect(a.do("POST", base+"/supplier-service-invoices", owner, si("S-1", nil, qty(1, "4"))), 201, "service invoice").data()
	invID := inv["id"].(string)
	if inv["document_no"] != "SI2026-000001" || inv["vat_status"] != "undue" || inv["vat_undue"] != "280" || inv["vat_claimed"] != "0" ||
		inv["tax_invoice_received"] != false || objs(inv["lines"])[0]["price_variance_account_id"] != expenseAcc || inv["vat_suggested"] != "280" {
		t.Fatalf("service invoice = %v", inv)
	}
	a.expect(a.do("GET", base+"/supplier-invoices/"+invID, owner, nil), 404, "service invoice through supplier-invoices")
	tb := func() map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, r := range objs(a.expect(a.do("GET", base+"/reports?book=trial-balance&start_date=2026-01-01&end_date=2026-12-31", owner, nil), 200, "TB").data()["rows"]) {
			out[r["account"].(obj)["code"].(string)] = r["closing_balance"].(string)
		}
		return out
	}
	if c := tb(); c["5200"] != "4000" || c["1170"] != "280" || c["2100"] != "-4280" || c["1160"] != "" && c["1160"] != "0" {
		t.Fatalf("TB after SI = %v", c)
	}
	inputVat := func() obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=input-vat&start_date=2026-02-01&end_date=2026-02-28", owner, nil), 200, "input vat").data()
	}
	undue := func() obj {
		t.Helper()
		return a.expect(a.do("GET", base+"/reports?book=undue-input-vat&end_date=2026-02-28", owner, nil), 200, "undue input vat").data()
	}
	if iv := inputVat(); len(objs(iv["rows"])) != 0 {
		t.Fatalf("input vat before tax invoice = %v", iv)
	}
	if u := undue(); u["totals"].(obj)["vat_undue"] != "280" {
		t.Fatalf("undue = %v", u)
	}

	// ---------- จ่ายครึ่ง -> ตั้งต้น 140 -> บันทึกใบกำกับ TX-001 ----------
	pay("PV-1", invID, "2140")
	if g := a.expect(a.do("GET", base+"/supplier-service-invoices/"+invID, owner, nil), 200, "SI after pay").data(); g["vat_suggested"] != "140" {
		t.Fatalf("suggested after half payment = %v", g["vat_suggested"])
	}
	a.expect(a.do("PATCH", base+"/supplier-service-invoices/"+invID, owner, obj{"status": "cancelled"}), 422, "cancel paid SI")
	txPath := base + "/supplier-service-invoices/" + invID + "/tax-invoices"
	a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-001", "tax_invoice_date": "2026-02-20", "vat": "300"}), 422, "over recognition")
	r1 := a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-001", "tax_invoice_date": "2026-02-20"}), 201, "record TX-001").data()
	txs := objs(r1["tax_invoices"])
	if r1["vat_status"] != "partial" || r1["vat_claimed"] != "140" || r1["vat_undue"] != "140" || len(txs) != 1 || txs[0]["vat"] != "140" ||
		txs[0]["base"] != "2000" || txs[0]["entry_no"] == "" {
		t.Fatalf("after TX-001 = %v", r1)
	}
	a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-001", "tax_invoice_date": "2026-02-21", "vat": "10"}), 409, "duplicate tax invoice no")
	if iv := inputVat(); len(objs(iv["rows"])) != 1 || objs(iv["rows"])[0]["kind"] != "tax_invoice" || objs(iv["rows"])[0]["tax_invoice_no"] != "TX-001" {
		t.Fatalf("input vat after TX-001 = %v", iv)
	}

	// ยกเลิกใบกำกับ -> กลับเป็นยังไม่ถึงกำหนดทั้งหมด -> บันทึกใหม่ (เลขเดิมใช้ได้หลังยกเลิก)
	c1 := a.expect(a.do("PATCH", txPath+"/"+txs[0]["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel TX-001").data()
	if c1["vat_undue"] != "280" || c1["vat_status"] != "undue" {
		t.Fatalf("after cancel TX = %v", c1)
	}
	a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-001", "tax_invoice_date": "2026-02-20"}), 201, "record TX-001 again")

	// จ่ายส่วนที่เหลือ -> ตั้งต้นที่เหลือ 140 -> ครบ
	pay("PV-2", invID, "2140")
	r2 := a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-002", "tax_invoice_date": "2026-02-25"}), 201, "record TX-002").data()
	if r2["vat_status"] != "claimed" || r2["vat_undue"] != "0" || r2["vat_claimed"] != "280" || r2["vat_suggested"] != "0" {
		t.Fatalf("after TX-002 = %v", r2)
	}
	a.expect(a.do("POST", txPath, owner, obj{"tax_invoice_no": "TX-009", "tax_invoice_date": "2026-02-25"}), 422, "nothing left to claim")
	if c := tb(); c["1170"] != "0" || c["1160"] != "280" {
		t.Fatalf("TB after claims = %v", c)
	}

	// ---------- ใบกำกับมาพร้อมใบแจ้งหนี้: ใช้สิทธิทันที ----------
	a.expect(a.do("POST", base+"/service-acceptances", owner, obj{"order_id": woID, "date": "2026-02-12", "lines": []obj{qty(1, "6")}}), 201, "accept rest")
	withTx := a.expect(a.do("POST", base+"/supplier-service-invoices", owner, si("S-2", obj{"tax_invoice_no": "TX-003", "tax_invoice_date": "2026-02-12"},
		obj{"order_line_no": 1, "quantity": "6", "account_id": consultAcc})), 201, "SI with tax invoice").data()
	if withTx["vat_status"] != "claimed" || withTx["vat_claimed"] != "420" || withTx["tax_invoice_no"] != "TX-003" ||
		objs(withTx["lines"])[0]["price_variance_account_id"] != consultAcc {
		t.Fatalf("SI with TX = %v", withTx)
	}
	if st := a.expect(a.do("GET", base+"/work-orders/"+woID, owner, nil), 200, "WO after all").data()["status"]; st != "closed" {
		t.Fatalf("WO should auto-close, status = %v", st)
	}

	// ---------- ซื้อตรงไม่ผ่านใบสั่ง ----------
	direct := a.expect(a.do("POST", base+"/supplier-service-invoices", owner, obj{"supplier_id": plainSup, "supplier_invoice_no": "N-1", "date": "2026-02-14",
		"lines": []obj{line(cons, "2", "500")}}), 201, "direct service (non-VAT)").data()
	if direct["vat_status"] != "none" || direct["totals"].(obj)["gross"] != "1000" || direct["order"].(obj)["order_no"] != "WO2026-000002" {
		t.Fatalf("direct SI = %v", direct)
	}
	a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"supplier_id": vatSup, "supplier_invoice_no": "G-1", "date": "2026-02-15",
		"lines": []obj{line(cons, "1", "100")}, "warehouse_id": wh}), 422, "service item in direct goods purchase")
	goods := a.expect(a.do("POST", base+"/supplier-invoices", owner, obj{"supplier_id": vatSup, "supplier_invoice_no": "G-1", "date": "2026-02-15",
		"lines": []obj{line(itemA, "5", "100")}, "warehouse_id": wh}), 201, "direct goods").data()
	if goods["kind"] != "goods" || goods["totals"].(obj)["vat"] != "35" || goods["vat_status"] != "claimed" {
		t.Fatalf("direct goods = %v", goods)
	}
	if v := a.expect(a.do("GET", base+"/reports?book=stock-on-hand&end_date=2026-02-28", owner, nil), 200, "stock").data(); v["total_value"] != "500" {
		t.Fatalf("stock after direct goods = %v", v["total_value"])
	}

	// ---------- ใบลดหนี้ของซื้อบริการที่ยังรอใบกำกับ: ลดภาษีซื้อยังไม่ถึงกำหนด ----------
	pending := a.expect(a.do("POST", base+"/supplier-service-invoices", owner, obj{"supplier_id": vatSup, "supplier_invoice_no": "S-3", "date": "2026-02-16",
		"lines": []obj{line(cons, "1", "1000")}}), 201, "pending SI").data()
	pendingID := pending["id"].(string)
	a.expect(a.do("POST", base+"/supplier-credit-notes", owner, obj{"invoice_id": pendingID, "supplier_credit_note_no": "SCN-1", "date": "2026-02-17",
		"kind": "return", "reason": "x", "lines": []obj{{"invoice_line_no": 1, "quantity": "1"}}}), 422, "return on service invoice")
	cn := a.expect(a.do("POST", base+"/supplier-credit-notes", owner, obj{"invoice_id": pendingID, "supplier_credit_note_no": "SCN-1", "date": "2026-02-17",
		"kind": "allowance", "reason": "ลดค่าบริการ", "lines": []obj{{"invoice_line_no": 1, "amount": "200"}}}), 201, "allowance on undue").data()
	if cn["vat_kind"] != "undue" || cn["invoice_kind"] != "service" || cn["totals"].(obj)["vat"] != "14" {
		t.Fatalf("credit note = %v", cn)
	}
	if p := a.expect(a.do("GET", base+"/supplier-service-invoices/"+pendingID, owner, nil), 200, "pending after CN").data(); p["vat_undue"] != "56" {
		t.Fatalf("pending undue after CN = %v", p["vat_undue"])
	}
	if u := undue(); u["totals"].(obj)["vat_undue"] != "56" || len(objs(u["rows"])) != 1 {
		t.Fatalf("undue after CN = %v", u)
	}

	// ---------- รายงานภาษีซื้อ: TX-001 140 + TX-002 140 + TX-003 420 + G-1 35 = 735 ----------
	iv := inputVat()
	if iv["total_tax"] != "735" || iv["total_base"] != "10500" || len(objs(iv["rows"])) != 4 {
		t.Fatalf("input vat = %v", iv)
	}
	kinds := map[string]int{}
	for _, r := range objs(iv["rows"]) {
		kinds[r["kind"].(string)]++
	}
	if kinds["tax_invoice"] != 2 || kinds["service_invoice_with_tx"] != 1 || kinds["invoice"] != 1 {
		t.Fatalf("input vat kinds = %v", kinds)
	}
	totals, err := a.pur.InputVat(context.Background(), uuid.MustParse(company), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC))
	if err != nil || totals.Tax.String() != "735" || totals.Base.String() != "10500" {
		t.Fatalf("Module.InputVat = %v %v", totals, err)
	}
	if r := a.expect(a.do("GET", base+"/reports?book=outstanding-work-orders", owner, nil), 200, "outstanding WOs").data(); len(objs(r["rows"])) != 0 {
		t.Fatalf("outstanding work orders = %v", r)
	}

	// ---------- สิทธิ์: สิทธิ์สายสินค้าไม่ครอบคลุมสายบริการ ----------
	role := a.expect(a.do("POST", base+"/roles", owner, obj{"code": "buyer", "name": "Buyer", "permissions": []string{"purchasing.order:read", "purchasing.invoice:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, obj{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")
	a.expect(a.do("GET", base+"/purchase-orders", staff, nil), 200, "buyer reads POs")
	a.expect(a.do("GET", base+"/work-orders", staff, nil), 403, "buyer reads WOs")
	a.expect(a.do("GET", base+"/supplier-service-invoices", staff, nil), 403, "buyer reads SIs")
	a.expect(a.do("POST", txPath, staff, obj{"tax_invoice_no": "TX-X", "tax_invoice_date": "2026-02-25"}), 403, "buyer records TX")
}
