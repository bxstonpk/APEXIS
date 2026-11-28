package entities

import (
	"errors"
	"testing"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// serviceInvoice: ใบแจ้งหนี้บริการ 1,000 + VAT 7% = 1,070 (ภาษียังไม่ถึงกำหนด)
func serviceInvoice(t *testing.T, deposits ...DepositUse) *Invoice {
	t.Helper()
	tr := terms(t)
	rev := vo.NewID[vo.AccountID]()
	i, err := NewInvoice(InvoiceState{Kind: vo.KindServiceInvoice, Date: day, Vat: tr.Vat, Deposits: deposits,
		Lines: []InvoiceLine{{ItemID: item, Service: true, Quantity: d("1"), UnitPrice: d("1000"),
			Amounts: services.LineAmounts(d("1"), d("1000"), tr.Vat, 2), RevenueAccountID: rev}}}, 30)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestServiceInvoiceRejectsGoodsLine(t *testing.T) {
	tr := terms(t)
	_, err := NewInvoice(InvoiceState{Kind: vo.KindServiceInvoice, Date: day, Vat: tr.Vat,
		Lines: []InvoiceLine{{ItemID: item, Service: false, Quantity: d("1"), UnitPrice: d("10"), Amounts: services.LineAmounts(d("1"), d("10"), tr.Vat, 2)}}}, 0)
	if !errors.Is(err, domain.ErrServiceInvoiceGoodsLine) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceInvoiceJournalUsesUndueVat(t *testing.T) {
	i := serviceInvoice(t)
	ar, out, undue := vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID]()
	got := map[vo.AccountID]string{}
	for _, l := range i.Journal(JournalAccounts{AR: ar, OutputVat: out, UndueVat: undue}) {
		got[l.AccountID] = l.Debit.String() + "/" + l.Credit.String()
	}
	if got[ar] != "1070/0" || got[undue] != "0/70" || got[out] != "" {
		t.Fatalf("journal = %v", got)
	}
	if s := i.State(); !s.Deferral.Vat.Equal(d("70")) || !s.Deferral.Base.Equal(d("1000")) {
		t.Fatalf("deferral = %+v", s.Deferral)
	}
}

// รับ 3 ครั้ง (ไม่ลงตัว) — ผลรวมภาษีที่ถึงกำหนด = ภาษีทั้งใบพอดี, ครั้งสุดท้ายรับส่วนที่เหลือ
func TestRecognizeProRataThenRemainder(t *testing.T) {
	i := serviceInvoice(t)
	sumVat, sumBase := d("0"), d("0")
	for n, amt := range []string{"333.33", "333.33", "403.34"} {
		base, vat := i.Recognize(d(amt), 2)
		if err := i.Receive(i.State().CustomerID, d(amt)); err != nil {
			t.Fatal(err)
		}
		sumVat, sumBase = sumVat.Add(vat), sumBase.Add(base)
		if n == 0 && (!vat.Equal(d("21.81")) || !base.Equal(d("311.52"))) {
			t.Fatalf("first = %s / %s", base, vat)
		}
	}
	if !sumVat.Equal(d("70")) || !sumBase.Equal(d("1000")) || i.VatDue() {
		t.Fatalf("sum = %s / %s, due %v", sumBase, sumVat, i.VatDue())
	}
	// ยกเลิกใบรับเงินใบสุดท้าย -> ภาษีส่วนนั้นกลับมาค้าง
	i.Unreceive(d("403.34"))
	i.Unrecognize(d("376.96"), d("26.38"))
	if !i.VatDue() || !i.State().Deferral.RemainingVat().Equal(d("26.38")) {
		t.Fatalf("after unrecognize remaining = %s", i.State().Deferral.RemainingVat())
	}
}

func TestTaxInvoiceRecognizesAllAndBlocksReceipts(t *testing.T) {
	i := serviceInvoice(t)
	base, vat := i.Recognize(d("535"), 2) // รับครึ่งหนึ่งก่อน
	_ = i.Receive(i.State().CustomerID, d("535"))
	if !vat.Equal(d("35")) || !base.Equal(d("500")) {
		t.Fatalf("half = %s / %s", base, vat)
	}
	b2, v2, err := i.IssueTaxInvoice("TX1", day)
	if err != nil || !v2.Equal(d("35")) || !b2.Equal(d("500")) {
		t.Fatalf("tax invoice = %s / %s %v", b2, v2, err)
	}
	if _, _, err := i.IssueTaxInvoice("TX2", day); !errors.Is(err, domain.ErrTaxInvoiceIssued) {
		t.Fatalf("second tax invoice: %v", err)
	}
	if b, v := i.Recognize(d("535"), 2); !v.IsZero() || !b.IsZero() {
		t.Fatalf("after tax invoice receipt recognized %s", v)
	}
	if i.CreditVatKind() != vo.VatOutput {
		t.Fatal("credit after tax invoice must reduce output vat")
	}
}

func TestCreditUndueBounded(t *testing.T) {
	i := serviceInvoice(t)
	if i.CreditVatKind() != vo.VatUndue {
		t.Fatal("credit before tax point must be undue")
	}
	if err := i.CreditUndue(d("100"), d("7")); err != nil {
		t.Fatal(err)
	}
	if err := i.CreditUndue(d("1000"), d("70")); !errors.Is(err, domain.ErrOverCredit) {
		t.Fatalf("over credit: %v", err)
	}
	if rem := i.State().Deferral.RemainingVat(); !rem.Equal(d("63")) {
		t.Fatalf("remaining = %s", rem)
	}
}

func deposit(t *testing.T) *Invoice {
	t.Helper()
	tr := terms(t)
	dep, err := NewInvoice(InvoiceState{Kind: vo.KindDeposit, Date: day, Vat: tr.Vat, Totals: services.LineAmounts(d("1"), d("300"), tr.Vat, 2)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return dep
}

func TestDepositMustBePaidBeforeApply(t *testing.T) {
	dep := deposit(t)
	cust := dep.State().CustomerID
	if _, err := dep.ApplyDeposit(cust, d("100"), 2); !errors.Is(err, domain.ErrDepositNotPaid) {
		t.Fatalf("unpaid deposit: %v", err)
	}
	if err := dep.Receive(cust, d("321")); err != nil {
		t.Fatal(err)
	}
	v1, err := dep.ApplyDeposit(cust, d("100"), 2)
	if err != nil || !v1.Equal(d("7")) {
		t.Fatalf("apply 100 = %s %v", v1, err)
	}
	if _, err := dep.ApplyDeposit(cust, d("201"), 2); !errors.Is(err, domain.ErrDepositOverApplied) {
		t.Fatalf("over apply: %v", err)
	}
	v2, _ := dep.ApplyDeposit(cust, d("200"), 2)
	if !v1.Add(v2).Equal(d("21")) || !dep.DepositRemaining().IsZero() {
		t.Fatalf("vat %s + %s, remaining %s", v1, v2, dep.DepositRemaining())
	}
	if err := dep.Cancel(vo.NewID[vo.UserID](), day); err == nil {
		t.Fatal("applied deposit must not cancel")
	}
}

func TestInvoiceWithDepositJournal(t *testing.T) {
	tr := terms(t)
	ar, out, depAcc, rev := vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID](), vo.NewID[vo.AccountID]()
	i, err := NewInvoice(InvoiceState{Kind: vo.KindInvoice, Date: day, Vat: tr.Vat,
		Deposits: []DepositUse{{DepositID: vo.NewID[vo.InvoiceID](), Amount: d("300"), Vat: d("21")}},
		Lines:    []InvoiceLine{{ItemID: item, Quantity: d("1"), UnitPrice: d("1000"), Amounts: services.LineAmounts(d("1"), d("1000"), tr.Vat, 2), RevenueAccountID: rev}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[vo.AccountID]string{}
	for _, l := range i.Journal(JournalAccounts{AR: ar, OutputVat: out, Deposit: depAcc}) {
		got[l.AccountID] = l.Debit.String() + "/" + l.Credit.String()
	}
	if got[ar] != "749/0" || got[depAcc] != "300/0" || got[rev] != "0/1000" || got[out] != "0/49" {
		t.Fatalf("journal = %v", got)
	}
	if !i.Outstanding().Equal(d("749")) || !i.Receivable().Equal(d("749")) {
		t.Fatalf("outstanding = %s", i.Outstanding())
	}
	if err := i.Cancel(vo.NewID[vo.UserID](), day); err != nil {
		t.Fatalf("cancel with only deposits deducted: %v", err)
	}
}
