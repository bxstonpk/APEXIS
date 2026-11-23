package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/finance/domain/errors"
	"apexis/backend/internal/modules/finance/domain/services"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

var day = time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func eq(t *testing.T, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(d(want)) {
		t.Fatalf("%s = %s, want %s", what, got, want)
	}
}

func ptr[T any](v T) *T { return &v }

func TestWithholdingOnPreVatAmount(t *testing.T) {
	// ใบแจ้งหนี้ 1,000 + VAT 70 = 1,070; จ่ายครบ หัก 3% ของ 1,000 = 30
	base, tax := services.Withholding(d("1070"), d("1000"), d("1070"), d("3"), 2)
	eq(t, "full base", base, "1000")
	eq(t, "full tax", tax, "30")
	// จ่ายครึ่งหนึ่ง 535 -> ฐาน 500, หัก 15
	base, tax = services.Withholding(d("535"), d("1000"), d("1070"), d("3"), 2)
	eq(t, "half base", base, "500")
	eq(t, "half tax", tax, "15")
	// จ่ายเงินอื่น: ฐาน = ยอดจ่าย
	base, tax = services.WithholdingOnAmount(d("2500"), d("3"), 2)
	eq(t, "other base", base, "2500")
	eq(t, "other tax", tax, "75")
}

func TestSupplierPaymentRulesAndJournalOneLinePerAccount(t *testing.T) {
	inv1, inv2 := vo.NewID[vo.InvoiceID](), vo.NewID[vo.InvoiceID]()
	cash, bank1, bank2 := vo.NewID[vo.CashAccountID](), vo.NewID[vo.CashAccountID](), vo.NewID[vo.CashAccountID]()
	ap, wht, cashGL, bankGL := vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID]()
	lines := []PaymentLine{
		{InvoiceID: &inv1, Amount: d("6000"), Wht: d("100")},
		{InvoiceID: &inv2, Amount: d("4100"), Wht: d("0")},
	}
	party := Party{Name: "V", TaxID: "0105551234567", TaxType: vo.TaxJuristic}
	state := func(methods ...Method) PaymentState {
		return PaymentState{Kind: vo.PaymentSupplier, Party: party, Date: day, Lines: lines, Methods: methods}
	}
	// ยอดช่องทางจ่ายต้องเท่ากับ 10,100 - 100 = 10,000
	if _, err := NewPayment(state(Method{CashAccountID: cash, GLAccountID: cashGL, Amount: d("10100")}), 2); !errors.Is(err, domain.ErrInvalidPayment) {
		t.Fatalf("methods not matching net: %v", err)
	}
	// เงินสด 2,000 + ธนาคาร 2 บัญชีที่ผูกผังบัญชีเดียวกัน 3,000 + 5,000 -> บรรทัดธนาคารเหลือ 1 บรรทัด
	p, err := NewPayment(state(
		Method{CashAccountID: cash, GLAccountID: cashGL, Amount: d("2000")},
		Method{CashAccountID: bank1, GLAccountID: bankGL, Amount: d("3000"), Reference: "CHQ-001"},
		Method{CashAccountID: bank2, GLAccountID: bankGL, Amount: d("5000")},
	), 2)
	if err != nil {
		t.Fatal(err)
	}
	j := p.Journal(ap, wht)
	if len(j) != 4 || j[0].AccountID != ap || !j[0].Debit.Equal(d("10100")) || j[2].AccountID != bankGL || !j[2].Credit.Equal(d("8000")) ||
		j[3].AccountID != wht || !j[3].Credit.Equal(d("100")) {
		t.Fatalf("journal = %+v", j)
	}
	lines = []PaymentLine{lines[0], lines[0]}
	if _, err := NewPayment(state(Method{CashAccountID: cash, GLAccountID: cashGL, Amount: d("11900")}), 2); !errors.Is(err, domain.ErrInvalidPayment) {
		t.Fatalf("duplicate invoice: %v", err)
	}
}

func TestOtherPaymentNeedsPayeeAndTaxIDForWithholding(t *testing.T) {
	cash, cashGL, expense := vo.NewID[vo.CashAccountID](), vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID]()
	state := PaymentState{Kind: vo.PaymentOther, Date: day,
		Lines:   []PaymentLine{{AccountID: &expense, Description: "ค่าบริการ", Amount: d("1000"), WhtRate: d("3"), WhtBase: d("1000"), Wht: d("30")}},
		Methods: []Method{{CashAccountID: cash, GLAccountID: cashGL, Amount: d("970")}}}
	if _, err := NewPayment(state, 2); !errors.Is(err, domain.ErrInvalidPayment) {
		t.Fatalf("payee name required: %v", err)
	}
	state.Party = Party{Name: "นาย ก"}
	if _, err := NewPayment(state, 2); !errors.Is(err, domain.ErrPayeeTaxIDRequired) {
		t.Fatalf("withholding needs tax id: %v", err)
	}
	state.Party = Party{Name: "นาย ก", TaxID: "1101700000000", TaxType: vo.TaxIndividual}
	p, err := NewPayment(state, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Dr ค่าใช้จ่าย 1,000 / Cr เงินสด 970 + Cr ภาษีหัก ณ ที่จ่าย 30
	whtPayable := vo.NewID[vo.GLAccountID]()
	if j := p.Journal(vo.GLAccountID{}, whtPayable); len(j) != 3 || j[0].AccountID != expense || !j[2].Credit.Equal(d("30")) {
		t.Fatalf("journal = %+v", j)
	}
	bad := state
	bad.Lines = []PaymentLine{{InvoiceID: ptr(vo.NewID[vo.InvoiceID]()), Amount: d("1000")}}
	bad.Methods = []Method{{CashAccountID: cash, GLAccountID: cashGL, Amount: d("1000")}}
	if _, err := NewPayment(bad, 2); !errors.Is(err, domain.ErrInvalidPayment) {
		t.Fatalf("invoice line in other payment: %v", err)
	}
}

func TestReceiptRules(t *testing.T) {
	inv, cash, other := vo.NewID[vo.InvoiceID](), vo.NewID[vo.CashAccountID](), vo.NewID[vo.GLAccountID]()
	ar, wht, cashGL := vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID]()
	base := ReceiptState{Kind: vo.ReceiptCustomer, Date: day, Lines: []ReceiptLine{{InvoiceID: &inv, Amount: d("1070"), Wht: d("30")}}}
	base.Methods = []Method{{CashAccountID: cash, GLAccountID: cashGL, Amount: d("1070")}}
	if _, err := NewReceipt(base, 2); !errors.Is(err, domain.ErrInvalidReceipt) {
		t.Fatalf("methods must equal amount - wht: %v", err)
	}
	base.Methods[0].Amount = d("1040")
	r, err := NewReceipt(base, 2)
	if err != nil {
		t.Fatal(err)
	}
	j := r.Journal(ar, wht)
	if len(j) != 3 || j[0].AccountID != cashGL || !j[1].Debit.Equal(d("30")) || j[2].AccountID != ar || !j[2].Credit.Equal(d("1070")) {
		t.Fatalf("journal = %+v", j)
	}
	// รับเงินอื่น: ต้องระบุบัญชีและชื่อผู้จ่าย, ไม่มีภาษีถูกหัก
	o := ReceiptState{Kind: vo.ReceiptOther, Date: day, Party: Party{Name: "ธนาคาร"},
		Lines:   []ReceiptLine{{AccountID: &other, Description: "ดอกเบี้ยรับ", Amount: d("12.5")}},
		Methods: []Method{{CashAccountID: cash, GLAccountID: cashGL, Amount: d("12.5")}}}
	if r, err := NewReceipt(o, 2); err != nil || r.Journal(ar, wht)[1].AccountID != other {
		t.Fatalf("other receipt: %v", err)
	}
	o.Lines[0].Wht = d("1")
	o.Methods[0].Amount = d("11.5")
	if _, err := NewReceipt(o, 2); !errors.Is(err, domain.ErrInvalidReceipt) {
		t.Fatalf("withholding on other receipt: %v", err)
	}
	if _, err := NewReceipt(ReceiptState{Kind: vo.ReceiptOther, Date: day, Lines: o.Lines[:1], Methods: o.Methods}, 2); !errors.Is(err, domain.ErrInvalidReceipt) {
		t.Fatalf("payer name required: %v", err)
	}
}

func TestTransferRulesAndCancel(t *testing.T) {
	a, b := vo.NewID[vo.CashAccountID](), vo.NewID[vo.CashAccountID]()
	aGL, bGL := vo.NewID[vo.GLAccountID](), vo.NewID[vo.GLAccountID]()
	if _, err := NewTransfer(TransferState{Date: day, From: Method{CashAccountID: a, GLAccountID: aGL, Amount: d("100")}, To: Method{CashAccountID: a, GLAccountID: aGL}}, 2); !errors.Is(err, domain.ErrInvalidTransfer) {
		t.Fatalf("same account: %v", err)
	}
	tr, err := NewTransfer(TransferState{Date: day, From: Method{CashAccountID: a, GLAccountID: aGL, Amount: d("500")}, To: Method{CashAccountID: b, GLAccountID: bGL}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if j := tr.Journal(); len(j) != 2 || j[0].AccountID != bGL || !j[0].Debit.Equal(d("500")) || j[1].AccountID != aGL {
		t.Fatalf("journal = %+v", j)
	}
	if err := tr.Cancel(vo.NewID[vo.UserID](), day); err != nil {
		t.Fatal(err)
	}
	if err := tr.Cancel(vo.NewID[vo.UserID](), day); !errors.Is(err, domain.ErrAlreadyCancelled) {
		t.Fatalf("cancel twice: %v", err)
	}
}
