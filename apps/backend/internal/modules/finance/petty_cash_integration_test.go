package finance_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/platform/integration"
)

// TestPettyCash: กองทุนเงินสดย่อย -> ตั้งวงเงิน (การเงินจ่าย) -> ใบสำคัญจ่าย (มี / ไม่มี VAT) -> อนุมัติ (เงินไม่พอ = 422)
// -> ขอเบิกชดเชย -> การเงินจ่าย (Dr ค่าใช้จ่าย + ภาษีซื้อ / Cr ธนาคาร) -> ภาษีซื้อ / ภ.พ.30 -> ยกเลิกใบจ่าย = คำขอกลับมาเปิด
func TestPettyCash(t *testing.T) {
	a := setupApp(t, []integration.CompanyCreatedHook{})
	owner := a.user("owner@example.com")
	outsider := a.user("outsider@example.com")
	me := a.expect(a.do("GET", "/api/v1/me", owner, nil), 200, "me").data()["id"].(string)
	company := a.expect(a.do("POST", "/api/v1/companies", owner, obj{"code": "pc", "name": "Petty", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + company
	a.expect(a.do("POST", base+"/fiscal-years", owner, obj{"year": 2026, "start_date": "2026-01-01"}), 201, "FY2026")
	acc := map[string]string{}
	for _, x := range objs(a.expect(a.do("GET", base+"/accounts/tree", owner, nil), 200, "tree").body["data"]) {
		acc[x["code"].(string)] = x["id"].(string)
	}
	bank := a.expect(a.do("POST", base+"/cash-accounts", owner, obj{"code": "BANK", "name": "ธนาคาร", "type": "bank", "bank_name": "KBANK",
		"account_no": "111-1-11111-1", "gl_account_id": acc["1101030100"]}), 201, "bank").id()
	dec := func(v any) decimal.Decimal { return decimal.RequireFromString(v.(string)) }

	// ---------- สิทธิ์ / ค่าตั้ง ----------
	a.expect(a.do("GET", base+"/petty-cash-funds", outsider, nil), 403, "outsider")
	s := a.expect(a.do("GET", base+"/petty-cash-settings", owner, nil), 200, "settings").data()
	if s["input_vat_account"].(obj)["code"] != "1107090000" {
		t.Fatalf("provisioned input vat = %v", s)
	}

	// ---------- กองทุน ----------
	a.expect(a.do("POST", base+"/petty-cash-funds", owner, obj{"code": "PC-HO", "name": "เงินสดย่อย", "custodian_user_id": company, "imprest_amount": "20000"}), 422, "custodian not member")
	fund := a.expect(a.do("POST", base+"/petty-cash-funds", owner, obj{"code": "pc-ho", "name": "เงินสดย่อยสำนักงาน", "custodian_user_id": me, "imprest_amount": "20000"}), 201, "fund").data()
	if fund["code"] != "PC-HO" || fund["gl_account"].(obj)["code"] != "1101010200" || fund["funded"] != false || fund["custodian"].(obj)["name"] != "owner@example.com" {
		t.Fatalf("fund = %v", fund)
	}
	a.expect(a.do("POST", base+"/petty-cash-funds", owner, obj{"code": "PC-HO", "name": "x", "custodian_user_id": me, "imprest_amount": "1"}), 409, "duplicate code")
	fundID := fund["id"].(string)

	voucher := func(amount string, vat bool, what string, status int) resp {
		body := obj{"fund_id": fundID, "date": "2026-11-05", "payee": obj{"name": "ร้านค้า"}, "description": what,
			"lines": []obj{{"account_id": acc["5309050000"], "description": what, "amount": amount}}}
		if vat {
			body["vat_rate"] = "7"
			body["tax_invoice"] = obj{"no": "TI-" + amount, "date": "2026-11-05"}
			body["payee"] = obj{"name": "บริษัท เครื่องเขียน จำกัด", "tax_id": "0105555000001", "branch_no": "00000"}
		}
		return a.expect(a.do("POST", base+"/petty-cash-vouchers", owner, body), status, what)
	}
	postage := voucher("1000", false, "ค่าไปรษณีย์", 201).data()
	if postage["document_no"] != "PCV2026-000001" || postage["status"] != "draft" {
		t.Fatalf("voucher = %v", postage)
	}
	a.expect(a.do("POST", base+"/petty-cash-vouchers/"+postage["id"].(string)+"/approve", owner, nil), 422, "approve before funded")

	// ---------- ตั้งวงเงิน -> การเงินจ่าย ----------
	est := a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fundID, "kind": "establish", "date": "2026-11-01"}), 201, "establish").data()
	if est["amount"] != "20000" || est["status"] != "requested" || est["document_no"] != "PCR2026-000001" {
		t.Fatalf("establish = %v", est)
	}
	a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fundID, "kind": "establish", "date": "2026-11-01"}), 422, "second open establish")
	open := objs(a.expect(a.do("GET", base+"/payments/petty-cash-requests", owner, nil), 200, "open requests").body["data"])
	if len(open) != 1 || open[0]["kind"] != "establish" || open[0]["fund_code"] != "PC-HO" {
		t.Fatalf("open = %v", open)
	}
	pay := func(requestID, amount, date string) obj {
		return a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "petty_cash", "petty_cash_request_id": requestID, "date": date,
			"methods": []obj{{"cash_account_id": bank, "amount": amount}}}), 201, "pay "+requestID).data()
	}
	a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "petty_cash", "petty_cash_request_id": est["id"], "date": "2026-11-06",
		"methods": []obj{{"cash_account_id": bank, "amount": "19999"}}}), 400, "methods != amount")
	p1 := pay(est["id"].(string), "20000", "2026-11-02")
	if l := objs(p1["lines"]); len(l) != 1 || *strPtr(l[0]["account_id"]) != acc["1101010200"] || l[0]["amount"] != "20000" || p1["kind"] != "petty_cash" {
		t.Fatalf("establish payment = %v", p1)
	}
	fund = a.expect(a.do("GET", base+"/petty-cash-funds/"+fundID, owner, nil), 200, "fund").data()
	if fund["funded"] != true || !dec(fund["cash_on_hand"]).Equal(decimal.NewFromInt(20000)) {
		t.Fatalf("funded fund = %v", fund)
	}

	// ---------- ใบสำคัญจ่าย ----------
	voucher("2000", true, "เครื่องเขียน (ไม่มีใบกำกับ)", 201)
	bad := a.do("POST", base+"/petty-cash-vouchers", owner, obj{"fund_id": fundID, "date": "2026-11-05", "payee": obj{"name": "x"}, "vat_rate": "7",
		"lines": []obj{{"account_id": acc["5309050000"], "amount": "100"}}})
	a.expect(bad, 422, "vat without tax invoice")
	supplies := voucher("2000", true, "เครื่องเขียน", 201).data()
	if tt := supplies["totals"].(obj); tt["vat"] != "140" || tt["gross"] != "2140" {
		t.Fatalf("vat voucher totals = %v", tt)
	}
	for _, v := range []obj{postage, supplies} {
		a.expect(a.do("POST", base+"/petty-cash-vouchers/"+v["id"].(string)+"/approve", owner, nil), 200, "approve "+v["document_no"].(string))
	}
	big := voucher("17000", false, "เกินเงินคงเหลือ", 201).data()
	a.expect(a.do("POST", base+"/petty-cash-vouchers/"+big["id"].(string)+"/approve", owner, nil), 422, "insufficient cash")
	fund = a.expect(a.do("GET", base+"/petty-cash-funds/"+fundID, owner, nil), 200, "fund").data()
	if !dec(fund["cash_on_hand"]).Equal(decimal.NewFromInt(16860)) || !dec(fund["pending_replenishment"]).Equal(decimal.NewFromInt(3140)) {
		t.Fatalf("after vouchers = %v", fund)
	}

	// ---------- เบิกชดเชย -> การเงินจ่าย ----------
	a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fundID, "kind": "replenish", "date": "2026-11-20",
		"voucher_ids": []string{big["id"].(string)}}), 422, "draft voucher in request")
	req := a.expect(a.do("POST", base+"/petty-cash-replenishments", owner, obj{"fund_id": fundID, "kind": "replenish", "date": "2026-11-20",
		"voucher_ids": []string{postage["id"].(string), supplies["id"].(string)}}), 201, "replenish").data()
	if req["amount"] != "3140" || len(objs(req["vouchers"])) != 2 {
		t.Fatalf("request = %v", req)
	}
	a.expect(a.do("POST", base+"/payments", owner, obj{"kind": "petty_cash", "petty_cash_request_id": req["id"], "date": "2026-11-19",
		"methods": []obj{{"cash_account_id": bank, "amount": "3140"}}}), 422, "payment before request")
	p2 := pay(req["id"].(string), "3140", "2026-11-21")
	lines := map[string]string{}
	for _, l := range objs(p2["lines"]) {
		lines[*strPtr(l["account_id"])] = l["amount"].(string)
	}
	if lines[acc["5309050000"]] != "3000" || lines[acc["1107090000"]] != "140" || len(lines) != 2 {
		t.Fatalf("replenish payment lines = %v", lines)
	}
	fund = a.expect(a.do("GET", base+"/petty-cash-funds/"+fundID, owner, nil), 200, "fund").data()
	if !dec(fund["cash_on_hand"]).Equal(decimal.NewFromInt(20000)) || !dec(fund["pending_replenishment"]).IsZero() {
		t.Fatalf("after replenish = %v", fund)
	}
	if v := a.expect(a.do("GET", base+"/petty-cash-vouchers/"+supplies["id"].(string), owner, nil), 200, "voucher").data(); v["status"] != "replenished" {
		t.Fatalf("voucher status = %v", v["status"])
	}
	a.expect(a.do("POST", base+"/petty-cash-vouchers/"+supplies["id"].(string)+"/cancel", owner, nil), 422, "cancel replenished")

	// ---------- ภาษีซื้อ / ภ.พ.30 ----------
	iv := a.expect(a.do("GET", base+"/reports?book=petty-cash-input-vat&start_date=2026-11-01&end_date=2026-11-30", owner, nil), 200, "input vat").data()
	if rows := objs(iv["rows"]); len(rows) != 1 || rows[0]["vat"] != "140" || rows[0]["tax_invoice_no"] != "TI-2000" || iv["totals"].(obj)["base"] != "2000" {
		t.Fatalf("input vat = %v", iv)
	}
	sum := a.expect(a.do("GET", base+"/reports?book=vat-summary&start_date=2026-11-01&end_date=2026-11-30", owner, nil), 200, "vat summary").data()
	if in := sum["input"].(obj); in["tax"] != "140" || int(in["documents"].(float64)) != 1 {
		t.Fatalf("vat summary input = %v", in)
	}
	reg := a.expect(a.do("GET", base+"/reports?book=petty-cash-voucher-register&start_date=2026-11-01&end_date=2026-11-30", owner, nil), 200, "register").data()
	if reg["totals"].(obj)["gross"] != "22280" { // 1,000 + 2,140 + ร่าง 2,140 + ร่าง 17,000 (ไม่รวมที่ยกเลิก)
		t.Fatalf("register totals = %v", reg["totals"])
	}

	// ---------- ยกเลิกใบจ่าย = คำขอกลับมาเปิด, ใบสำคัญจ่ายกลับเป็นอนุมัติ ----------
	a.expect(a.do("PATCH", base+"/payments/"+p2["id"].(string), owner, obj{"status": "cancelled"}), 200, "cancel payment")
	req = a.expect(a.do("GET", base+"/petty-cash-replenishments/"+req["id"].(string), owner, nil), 200, "request").data()
	if req["status"] != "requested" || req["payment"] != nil {
		t.Fatalf("request after cancel = %v", req)
	}
	fund = a.expect(a.do("GET", base+"/petty-cash-funds/"+fundID, owner, nil), 200, "fund").data()
	if !dec(fund["cash_on_hand"]).Equal(decimal.NewFromInt(16860)) {
		t.Fatalf("after payment cancel = %v", fund)
	}
	sum = a.expect(a.do("GET", base+"/reports?book=vat-summary&start_date=2026-11-01&end_date=2026-11-30", owner, nil), 200, "vat summary").data()
	if sum["input"].(obj)["tax"] != "0" {
		t.Fatalf("vat summary after cancel = %v", sum["input"])
	}
	// ยกเลิกคำขอ -> ใบสำคัญจ่ายว่างให้ขอใหม่ได้
	a.expect(a.do("POST", base+"/petty-cash-replenishments/"+req["id"].(string)+"/cancel", owner, nil), 200, "cancel request")
	free := a.expect(a.do("GET", base+"/petty-cash-vouchers?unrequested=true", owner, nil), 200, "unrequested")
	if free.total() != 2 {
		t.Fatalf("unrequested = %d", free.total())
	}
}

func strPtr(v any) *string {
	s, _ := v.(string)
	return &s
}
