package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

var (
	company = vo.NewID[vo.CompanyID]()
	user    = vo.NewID[vo.UserID]()
	item    = vo.NewID[vo.ItemID]()
	day     = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d       = vo.MustDecimal
	dec     = Decimals{Quantity: 2, Amount: 2}
)

func eq(t *testing.T, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(d(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

func terms(t *testing.T) Terms {
	t.Helper()
	v, err := DefaultSettings().Terms(false)
	if err != nil {
		t.Fatal(err)
	}
	return Terms{Vat: v, PaymentTermDays: 30}
}

func TestVatTermsFollowCompanySettings(t *testing.T) {
	s := DefaultSettings()
	s.VatRegistered = false
	if _, err := s.Terms(true); !errors.Is(err, domain.ErrVatNotApplicable) {
		t.Fatalf("inclusive price when not VAT registered: %v", err)
	}
	if v, _ := s.Terms(false); !v.Rate.IsZero() {
		t.Fatalf("not registered rate = %s", v.Rate)
	}
}

func TestQuotationConvertRules(t *testing.T) {
	q, err := NewQuotation(company, user, QuotationData{CustomerID: vo.NewID[vo.CustomerID](), Date: day, ValidUntil: day.AddDate(0, 0, 7),
		Lines: []LineData{{ItemID: item, Quantity: d("2"), UnitPrice: d("100")}}}, terms(t), dec)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "quotation gross", q.State().Priced.Totals.Gross, "214")
	if err := q.Convert(vo.NewID[vo.OrderID](), day); !errors.Is(err, domain.ErrQuotationState) {
		t.Fatalf("convert draft: %v", err)
	}
	_ = q.Send("QT2026-000001", 2026)
	if err := q.Convert(vo.NewID[vo.OrderID](), day.AddDate(0, 0, 8)); !errors.Is(err, domain.ErrQuotationExpired) {
		t.Fatalf("convert expired: %v", err)
	}
	if err := q.Convert(vo.NewID[vo.OrderID](), day.AddDate(0, 0, 7)); err != nil || q.Status() != vo.QuotationAccepted {
		t.Fatalf("convert on last valid day: %v", err)
	}
	if err := q.Convert(vo.NewID[vo.OrderID](), day); !errors.Is(err, domain.ErrQuotationUsed) {
		t.Fatalf("convert twice: %v", err)
	}
}

func TestOrderDeliverInvoiceBothStyles(t *testing.T) {
	o, err := NewOrder(company, user, OrderData{CustomerID: vo.NewID[vo.CustomerID](), Date: day,
		Lines: []LineData{{ItemID: item, Quantity: d("10"), UnitPrice: d("100")}}}, terms(t), dec)
	if err != nil {
		t.Fatal(err)
	}
	_ = o.Approve(Approval{OrderNo: "SO2026-000001", FiscalYear: 2026, By: user})
	eq(t, "open amount", o.OpenAmount(2), "1070")
	// แยกใบ: ส่ง 4 -> ออกบิลได้ไม่เกิน 4
	if err := o.Deliver([]Shipment{{LineNo: 1, Quantity: d("4")}}, 2, day); err != nil {
		t.Fatal(err)
	}
	if err := o.Invoice([]Shipment{{LineNo: 1, Quantity: d("5")}}, 2, false, day); !errors.Is(err, domain.ErrOverInvoice) {
		t.Fatalf("invoice more than delivered: %v", err)
	}
	if err := o.Invoice([]Shipment{{LineNo: 1, Quantity: d("4")}}, 2, false, day); err != nil {
		t.Fatal(err)
	}
	// รวมใบ: ส่ง + ออกบิล 6 ที่เหลือ -> ปิดอัตโนมัติ
	if err := o.Invoice([]Shipment{{LineNo: 1, Quantity: d("7")}}, 2, true, day); !errors.Is(err, domain.ErrOverDelivery) {
		t.Fatalf("combined more than left: %v", err)
	}
	if err := o.Invoice([]Shipment{{LineNo: 1, Quantity: d("6")}}, 2, true, day); err != nil || o.Status() != vo.OrderClosed {
		t.Fatalf("combined rest: %v %s", err, o.Status())
	}
	eq(t, "open amount after invoicing", o.OpenAmount(2), "0")
	if err := o.Undeliver([]Shipment{{LineNo: 1, Quantity: d("4")}}); !errors.Is(err, domain.ErrDeliveryInvoiced) {
		t.Fatalf("undeliver invoiced: %v", err)
	}
	if err := o.Uninvoice([]Shipment{{LineNo: 1, Quantity: d("6")}}, true); err != nil || o.Status() != vo.OrderApproved {
		t.Fatalf("uninvoice combined should reopen: %v %s", err, o.Status())
	}
}

func TestInvoiceJournalWithDeliveryOneLinePerAccount(t *testing.T) {
	ar, vat, cogs, inv, rev := vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID]()
	tr := terms(t)
	// 2 บรรทัดใช้บัญชีรายได้ / สินค้าคงเหลือเดียวกัน -> รวมเป็นบรรทัดเดียวต่อบัญชี
	line := func(qty, price, cost string) InvoiceLine {
		return InvoiceLine{ItemID: item, Quantity: d(qty), UnitPrice: d(price), Amounts: services.LineAmounts(d(qty), d(price), tr.Vat, 2),
			RevenueAccountID: rev, CostLine: CostLine{Stock: true, Cost: d(cost), InventoryAccountID: &inv}}
	}
	i, err := NewInvoice(InvoiceState{Kind: vo.KindInvoice, Date: day, Vat: tr.Vat, Stock: Stock{System: vo.Perpetual},
		Lines: []InvoiceLine{line("1", "1000", "600"), line("2", "500", "700")}}, 30)
	if err != nil {
		t.Fatal(err)
	}
	j := i.Journal(JournalAccounts{AR: ar, OutputVat: vat, Cogs: cogs})
	got := map[vo.AccountID]string{}
	for _, l := range j {
		got[l.AccountID] = l.Debit.String() + "/" + l.Credit.String()
	}
	want := map[vo.AccountID]string{ar: "2140/0", rev: "0/2000", vat: "0/140", cogs: "1300/0", inv: "0/1300"}
	if len(j) != len(want) {
		t.Fatalf("journal = %+v", j)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %s, want %s", k, got[k], v)
		}
	}
	if !i.State().DueDate.Equal(day.AddDate(0, 0, 30)) {
		t.Fatalf("due date = %v", i.State().DueDate)
	}
}
