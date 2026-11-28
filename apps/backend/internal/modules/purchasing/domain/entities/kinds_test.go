package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

func TestCheckLineKind(t *testing.T) {
	if err := entities.CheckLineKind(vo.KindGoods, true, "SVC"); !errors.Is(err, domain.ErrGoodsDocumentServiceLine) {
		t.Fatalf("service on goods document = %v", err)
	}
	if err := entities.CheckLineKind(vo.KindService, false, "A"); !errors.Is(err, domain.ErrServiceDocumentGoodsLine) {
		t.Fatalf("goods on service document = %v", err)
	}
	if entities.CheckLineKind(vo.KindGoods, false, "A") != nil || entities.CheckLineKind(vo.KindService, true, "SVC") != nil {
		t.Fatal("matching kinds must pass")
	}
	if vo.KindService.OrderDoc() != vo.DocWorkOrder || vo.KindService.ReceiptDoc() != vo.DocServiceAcceptance || vo.KindService.InvoiceDoc() != vo.DocServiceInvoice ||
		vo.KindGoods.OrderDoc() != vo.DocOrder || vo.KindOrDefault("") != vo.KindGoods {
		t.Fatal("document types per kind")
	}
}

func serviceInvoice(t *testing.T, taxNo string) *entities.Invoice {
	t.Helper()
	d := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	s := entities.InvoiceState{Kind: vo.KindService, SupplierInvoiceNo: "S-1", InvoiceDate: d, Date: d, TaxInvoiceNo: taxNo,
		Lines: []entities.InvoiceLine{{Amounts: services.Amounts{Net: decimal.NewFromInt(4000), Vat: decimal.NewFromInt(280), Gross: decimal.NewFromInt(4280)}}}}
	if taxNo != "" {
		s.TaxInvoiceDate = &d
	}
	inv, err := entities.NewInvoice(s, 30)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

// ซื้อบริการ: ไม่มีใบกำกับ = ภาษีซื้อยังไม่ถึงกำหนด; ตั้งต้นตามสัดส่วนที่จ่าย; ใบสุดท้ายใช้ส่วนที่เหลือ; ใบกำกับมาพร้อมใบแจ้งหนี้ = ใช้สิทธิทันที
func TestServiceInvoiceVatRecognition(t *testing.T) {
	inv := serviceInvoice(t, "")
	if inv.State().TaxInvoiceReceived || inv.VatAccountKey() != "undue_input_vat_account_id" || inv.VatStatus() != entities.VatStatusUndue {
		t.Fatalf("undue invoice = %+v", inv.State())
	}
	if v := inv.DefaultRecognition(decimal.Zero, 2); !v.Equal(decimal.NewFromInt(280)) {
		t.Fatalf("unpaid suggestion = %s", v)
	}
	if v := inv.DefaultRecognition(decimal.NewFromInt(2140), 2); !v.Equal(decimal.NewFromInt(140)) {
		t.Fatalf("half-paid suggestion = %s", v)
	}
	base, err := inv.RecordTaxInvoice(decimal.NewFromInt(140), 2)
	if err != nil || !base.Equal(decimal.NewFromInt(2000)) || inv.VatStatus() != entities.VatStatusPartial {
		t.Fatalf("record 140 = %s %v %s", base, err, inv.VatStatus())
	}
	if _, err := inv.RecordTaxInvoice(decimal.NewFromInt(141), 2); !errors.Is(err, domain.ErrOverRecognition) {
		t.Fatalf("over recognition = %v", err)
	}
	// จ่ายครบแล้ว: ใบกำกับเดิมครอบคลุมยอดจ่ายครึ่งแรก -> ตั้งต้นส่วนที่เหลือ 140
	if v := inv.DefaultRecognition(decimal.NewFromInt(4280), 2); !v.Equal(decimal.NewFromInt(140)) {
		t.Fatalf("remaining suggestion = %s", v)
	}
	base, _ = inv.RecordTaxInvoice(decimal.NewFromInt(140), 2)
	if !base.Equal(decimal.NewFromInt(2000)) || inv.VatStatus() != entities.VatStatusClaimed || !inv.RemainingUndueVat().IsZero() {
		t.Fatalf("claimed = %s %s", base, inv.VatStatus())
	}
	if err := inv.Cancel(inv.State().Posting.By, time.Now()); !errors.Is(err, domain.ErrInvoiceHasTaxInvoices) {
		t.Fatalf("cancel with tax invoices = %v", err)
	}

	withTx := serviceInvoice(t, "TX-1")
	if !withTx.State().TaxInvoiceReceived || withTx.VatAccountKey() != "input_vat_account_id" || withTx.VatStatus() != entities.VatStatusClaimed {
		t.Fatalf("invoice with tax invoice = %+v", withTx.State())
	}

	// ใบลดหนี้ของใบที่ยังรอใบกำกับ: ลดภาษีซื้อยังไม่ถึงกำหนด
	pending := serviceInvoice(t, "")
	undue, err := pending.CreditUndue(decimal.NewFromInt(200), decimal.NewFromInt(14))
	if err != nil || !undue || !pending.RemainingUndueVat().Equal(decimal.NewFromInt(266)) {
		t.Fatalf("credit undue = %v %v %s", undue, err, pending.RemainingUndueVat())
	}
}
