package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

var (
	company = vo.NewID[vo.CompanyID]()
	user    = vo.NewID[vo.UserID]()
	item    = vo.NewID[vo.ItemID]()
	day     = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d       = vo.MustDecimal
)

func supplier(t *testing.T, vat bool) *Supplier {
	t.Helper()
	s, err := NewSupplier(company, SupplierData{Code: "s1", Name: "ผู้ขาย", TaxID: "0105551234567", PaymentTermDays: 30, VatRegistered: vat, TaxType: vo.TaxJuristic, Status: vo.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func eq(t *testing.T, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(d(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

func TestVatTermsFollowSupplierAtCreation(t *testing.T) {
	if _, err := SupplierTerms(supplier(t, false), d("7"), true); !errors.Is(err, domain.ErrVatNotApplicable) {
		t.Fatalf("inclusive price for non-VAT supplier: %v", err)
	}
	none, _ := SupplierTerms(supplier(t, false), d("7"), false)
	if !none.Vat.Rate.IsZero() || none.PaymentTermDays != 30 {
		t.Fatalf("non-VAT terms = %+v", none)
	}
	vat, _ := SupplierTerms(supplier(t, true), d("7"), true)
	if !vat.Vat.Rate.Equal(d("7")) || !vat.Vat.Inclusive {
		t.Fatalf("VAT terms = %+v", vat)
	}
}

func TestLineAmounts(t *testing.T) {
	ex := services.LineAmounts(d("3"), d("100"), services.VatTerms{Rate: d("7")}, 2)
	eq(t, "exclusive net", ex.Net, "300")
	eq(t, "exclusive vat", ex.Vat, "21")
	eq(t, "exclusive gross", ex.Gross, "321")
	in := services.LineAmounts(d("1"), d("107"), services.VatTerms{Rate: d("7"), Inclusive: true}, 2)
	eq(t, "inclusive net", in.Net, "100")
	eq(t, "inclusive vat", in.Vat, "7")
	eq(t, "inclusive unit cost", services.UnitCost(d("107"), services.VatTerms{Rate: d("7"), Inclusive: true}), "100")
	zero := services.LineAmounts(d("2"), d("50"), services.VatTerms{Rate: d("0")}, 0)
	eq(t, "no VAT gross", zero.Gross, "100")
}

func TestOrderReceiveInvoiceAndAutoClose(t *testing.T) {
	terms, _ := SupplierTerms(supplier(t, true), d("7"), false)
	o, err := NewOrder(company, user, OrderData{SupplierID: vo.NewID[vo.SupplierID](), Date: day,
		Lines: []OrderLineData{{ItemID: item, Quantity: d("3"), UnitPrice: d("10.005")}}}, terms, Decimals{Quantity: 2, Amount: 2})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "order gross", o.State().Totals.Gross, "32.12") // 30.02 (3 x 10.005 ปัด) + VAT 2.10
	if err := o.CheckReceive([]Delivery{{LineNo: 1, Quantity: d("1")}}, 2); !errors.Is(err, domain.ErrOrderNotApproved) {
		t.Fatalf("receive draft: %v", err)
	}
	if err := o.Approve(Approval{OrderNo: "PO2026-000001", FiscalYear: 2026, By: user}); err != nil {
		t.Fatal(err)
	}
	// รับ 1 + 1 + 1 (ปัดทีละใบ 10.01 x 3 = 30.03) แล้วแจ้งหนี้ 3 ทีเดียว -> ล้าง GR/IR ตามที่ลงไว้จริง
	for range 3 {
		v := services.ReceiptValue(d("1"), d("10.005"), terms.Vat, 2)
		if err := o.Receive([]Delivery{{LineNo: 1, Quantity: d("1"), Value: v}}, 2, day); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.CheckReceive([]Delivery{{LineNo: 1, Quantity: d("0.01")}}, 2); !errors.Is(err, domain.ErrOverReceipt) {
		t.Fatalf("over receipt: %v", err)
	}
	if _, err := o.Invoice([]Delivery{{LineNo: 1, Quantity: d("4")}}, 2, 2, day); !errors.Is(err, domain.ErrOverInvoice) {
		t.Fatalf("over invoice: %v", err)
	}
	cleared, err := o.Invoice([]Delivery{{LineNo: 1, Quantity: d("3")}}, 2, 2, day)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "cleared", cleared[0], "30.03")
	if o.Status() != vo.OrderClosed || o.State().Closure.By != nil {
		t.Fatalf("should auto-close: %s", o.Status())
	}
	if err := o.Unreceive([]Delivery{{LineNo: 1, Quantity: d("1"), Value: d("10.01")}}); !errors.Is(err, domain.ErrReceiptInvoiced) {
		t.Fatalf("unreceive invoiced qty: %v", err)
	}
	if err := o.Uninvoice([]Delivery{{LineNo: 1, Quantity: d("3"), Value: cleared[0]}}); err != nil || o.Status() != vo.OrderApproved {
		t.Fatalf("uninvoice should reopen: %v %s", err, o.Status())
	}
	if err := o.Cancel(user, day); !errors.Is(err, domain.ErrOrderHasActivity) {
		t.Fatalf("cancel with receipts: %v", err)
	}
}

func TestInvoiceJournalBalances(t *testing.T) {
	grir, vat, ap, variance := vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID]()
	terms := services.VatTerms{Registered: true, Rate: d("7")}
	// รับที่ 100 แต่แจ้งหนี้ 110 -> ผลต่าง 10, VAT 7.70, เจ้าหนี้ 117.70
	inv, err := NewInvoice(InvoiceState{SupplierInvoiceNo: "INV-1", InvoiceDate: day, Date: day, Vat: terms, Lines: []InvoiceLine{{
		OrderLineNo: 1, ItemID: item, Quantity: d("1"), UnitPrice: d("110"),
		Amounts: services.LineAmounts(d("1"), d("110"), terms, 2), Cleared: d("100"), VarianceAccountID: &variance,
	}}}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !inv.State().DueDate.Equal(day.AddDate(0, 0, 30)) {
		t.Fatalf("due date = %v", inv.State().DueDate)
	}
	debit, credit := decimal.Zero, decimal.Zero
	for _, l := range inv.Journal(grir, vat, ap) {
		debit, credit = debit.Add(l.Debit), credit.Add(l.Credit)
	}
	eq(t, "debit", debit, "117.7")
	eq(t, "credit", credit, "117.7")
}

func TestInvoicePayRules(t *testing.T) {
	sup := vo.NewID[vo.SupplierID]()
	inv, _ := NewInvoice(InvoiceState{SupplierID: sup, SupplierInvoiceNo: "X", InvoiceDate: day, Date: day,
		Lines: []InvoiceLine{{Amounts: services.Amounts{Net: d("100"), Vat: d("7"), Gross: d("107")}}}}, 0)
	if err := inv.Pay(vo.NewID[vo.SupplierID](), d("1")); !errors.Is(err, domain.ErrInvoiceNotPayable) {
		t.Fatalf("other supplier: %v", err)
	}
	if err := inv.Pay(sup, d("107.01")); !errors.Is(err, domain.ErrOverPayment) {
		t.Fatalf("over payment: %v", err)
	}
	if err := inv.Pay(sup, d("50")); err != nil || !inv.Outstanding().Equal(d("57")) {
		t.Fatalf("partial payment: %v %s", err, inv.Outstanding())
	}
	if err := inv.Cancel(user, day); !errors.Is(err, domain.ErrInvoicePaid) {
		t.Fatalf("cancel paid invoice: %v", err)
	}
}
