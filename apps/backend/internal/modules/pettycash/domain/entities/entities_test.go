package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

var day = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

func voucherData(fund vo.FundID, rate string, includes bool, amounts ...string) entities.VoucherData {
	lines := make([]entities.VoucherLine, len(amounts))
	for i, a := range amounts {
		lines[i] = entities.VoucherLine{AccountID: vo.NewID[vo.AccountID](), Amount: d(a)}
	}
	return entities.VoucherData{FundID: fund, Date: day, Payee: entities.Payee{Name: "ร้านเครื่องเขียน"}, Lines: lines,
		VatRate: d(rate), PriceIncludesVat: includes}
}

func TestVoucherVatRequiresTaxInvoice(t *testing.T) {
	fund := vo.NewID[vo.FundID]()
	_, err := entities.NewVoucher(vo.NewID[vo.CompanyID](), "PCV1", voucherData(fund, "7", false, "100"), vo.NewID[vo.UserID](), 2)
	if !errors.Is(err, domain.ErrTaxInvoiceRequired) {
		t.Fatalf("want ErrTaxInvoiceRequired, got %v", err)
	}
	data := voucherData(fund, "7", false, "100")
	data.TaxInvoice = &entities.TaxInvoice{No: "IV-01", Date: day}
	v, err := entities.NewVoucher(vo.NewID[vo.CompanyID](), "PCV1", data, vo.NewID[vo.UserID](), 2)
	if err != nil {
		t.Fatal(err)
	}
	if tt := v.State().Totals; !tt.Net.Equal(d("100")) || !tt.Vat.Equal(d("7")) || !tt.Gross.Equal(d("107")) {
		t.Fatalf("totals = %+v", tt)
	}
	// ไม่มี VAT: ใบกำกับภาษีที่ส่งมาถูกละทิ้ง
	nv := voucherData(fund, "0", false, "50")
	nv.TaxInvoice = &entities.TaxInvoice{No: "X", Date: day}
	v2, err := entities.NewVoucher(vo.NewID[vo.CompanyID](), "PCV2", nv, vo.NewID[vo.UserID](), 2)
	if err != nil || v2.State().TaxInvoice != nil {
		t.Fatalf("no-vat voucher: %v %+v", err, v2.State().TaxInvoice)
	}
}

func TestVoucherIncludesVatAllocatesNet(t *testing.T) {
	data := voucherData(vo.NewID[vo.FundID](), "7", true, "100", "200")
	data.TaxInvoice = &entities.TaxInvoice{No: "IV-02", Date: day}
	v, err := entities.NewVoucher(vo.NewID[vo.CompanyID](), "PCV3", data, vo.NewID[vo.UserID](), 2)
	if err != nil {
		t.Fatal(err)
	}
	s := v.State()
	// 300 รวม VAT: VAT = 19.63, ก่อน VAT = 280.37 -> บรรทัด 93.46 + 186.91 (เศษไว้บรรทัดสุดท้าย)
	if !s.Totals.Vat.Equal(d("19.63")) || !s.Totals.Net.Equal(d("280.37")) || !s.Totals.Gross.Equal(d("300")) {
		t.Fatalf("totals = %+v", s.Totals)
	}
	if sum := s.Lines[0].Net.Add(s.Lines[1].Net); !sum.Equal(s.Totals.Net) || !s.Lines[0].Net.Equal(d("93.46")) {
		t.Fatalf("lines = %s + %s", s.Lines[0].Net, s.Lines[1].Net)
	}
}

func TestVoucherLifecycleAndRequest(t *testing.T) {
	fund := vo.NewID[vo.FundID]()
	cid := vo.NewID[vo.CompanyID]()
	newV := func(no, amount string) *entities.Voucher {
		v, err := entities.NewVoucher(cid, no, voucherData(fund, "0", false, amount), vo.NewID[vo.UserID](), 2)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	draft := newV("PCV1", "120")
	if _, err := entities.NewReplenishRequest(entities.ReplenishmentState{CompanyID: cid, FundID: fund, Date: day}, []*entities.Voucher{draft}); !errors.Is(err, domain.ErrVoucherNotApproved) {
		t.Fatalf("draft in request: %v", err)
	}
	a1, a2 := newV("PCV2", "120"), newV("PCV3", "80")
	for _, v := range []*entities.Voucher{a1, a2} {
		if err := v.Approve(vo.NewID[vo.UserID](), day); err != nil {
			t.Fatal(err)
		}
	}
	r, err := entities.NewReplenishRequest(entities.ReplenishmentState{CompanyID: cid, FundID: fund, Date: day}, []*entities.Voucher{a1, a2})
	if err != nil {
		t.Fatal(err)
	}
	if !r.State().Amount.Equal(d("200")) || r.State().Kind != vo.RequestReplenish {
		t.Fatalf("request = %+v", r.State())
	}
	// อยู่ในคำขอแล้ว: ยกเลิก / ขอซ้ำไม่ได้
	if err := a1.Cancel(day); !errors.Is(err, domain.ErrVoucherInRequest) {
		t.Fatalf("cancel in request: %v", err)
	}
	if _, err := entities.NewReplenishRequest(entities.ReplenishmentState{CompanyID: cid, FundID: fund, Date: day}, []*entities.Voucher{a1}); !errors.Is(err, domain.ErrVoucherInRequest) {
		t.Fatalf("double request: %v", err)
	}
	// การเงินจ่าย -> ชดเชยแล้ว; เดบิต = ค่าใช้จ่ายตามบัญชี (ไม่มี VAT)
	if err := r.Pay(entities.Payment{Date: day}); err != nil {
		t.Fatal(err)
	}
	debits := r.Debits(vo.NewID[vo.AccountID](), vo.AccountID{}, []*entities.Voucher{a1, a2}, func(s string) string { return s })
	if len(debits) != 2 || !debits[0].Amount.Add(debits[1].Amount).Equal(d("200")) {
		t.Fatalf("debits = %+v", debits)
	}
	if err := r.Cancel(day); !errors.Is(err, domain.ErrRequestPaid) {
		t.Fatalf("cancel paid: %v", err)
	}
	if err := r.Unpay(); err != nil || r.State().Status != vo.RequestRequested {
		t.Fatalf("unpay: %v %s", err, r.State().Status)
	}
}

func TestFundingRequestAndDebits(t *testing.T) {
	if _, err := entities.NewFundingRequest(entities.ReplenishmentState{Kind: vo.RequestIncrease, Date: day, Amount: d("0")}, 2); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("zero increase: %v", err)
	}
	r, err := entities.NewFundingRequest(entities.ReplenishmentState{Kind: vo.RequestEstablish, Date: day, Amount: d("20000")}, 2)
	if err != nil {
		t.Fatal(err)
	}
	gl := vo.NewID[vo.AccountID]()
	debits := r.Debits(gl, vo.AccountID{}, nil, func(s string) string { return s })
	if len(debits) != 1 || debits[0].AccountID != gl || !debits[0].Amount.Equal(d("20000")) {
		t.Fatalf("establish debits = %+v", debits)
	}
}

func TestFundImprestEditableUntilFunded(t *testing.T) {
	f, err := entities.NewFund(entities.FundData{Code: "pc-ho", Name: "เงินสดย่อยสำนักงาน", ImprestAmount: d("20000"), Status: vo.StatusActive}, 2)
	if err != nil || f.Data().Code != "PC-HO" {
		t.Fatalf("new fund: %v %s", err, f.Data().Code)
	}
	upd := f.Data()
	upd.ImprestAmount = d("30000")
	if err := f.Update(upd, true, 2); err != nil || !f.Imprest().Equal(d("20000")) {
		t.Fatalf("funded update changed imprest: %s", f.Imprest())
	}
	if err := f.Update(upd, false, 2); err != nil || !f.Imprest().Equal(d("30000")) {
		t.Fatalf("unfunded update: %v %s", err, f.Imprest())
	}
}
