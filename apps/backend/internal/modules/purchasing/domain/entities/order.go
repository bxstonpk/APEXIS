package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// MaxLines คือจำนวนบรรทัดสูงสุดต่อเอกสาร
const MaxLines = 500

type OrderLineData struct {
	ItemID      vo.ItemID
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money // ราคาต่อหน่วยตามเอกสาร (รวมหรือไม่รวม VAT ตาม PriceIncludesVat)
}

type OrderData struct {
	// Kind: goods = ใบสั่งซื้อ (สินค้า), service = ใบสั่งจ้าง (บริการ) — กำหนดตอนสร้าง แก้ไม่ได้; use case ตรวจชนิดสินค้าทุกบรรทัด
	Kind             vo.PurchaseKind
	SupplierID       vo.SupplierID
	SupplierBranchNo string // สาขาผู้ขาย (ว่าง = สาขาหลัก) — use case ตรวจกับทะเบียนผู้ขายแล้ว; ใบแจ้งหนี้ใช้สาขานี้
	Date             time.Time
	DeliveryDate     *time.Time
	Reference        string
	Description      string
	PriceIncludesVat bool // เลือกได้เฉพาะผู้ขายที่จด VAT
	Lines            []OrderLineData
}

// Terms คือเงื่อนไขที่ใบสั่งซื้อเก็บจากผู้ขาย / ค่าตั้ง ณ ตอนสร้าง — เอกสารที่ตามมาใช้ค่านี้ ไม่กลับไปดูผู้ขาย
type Terms struct {
	Vat             services.VatTerms
	PaymentTermDays int
}

// Progress คือความคืบหน้าของบรรทัด: รับแล้ว / แจ้งหนี้แล้ว และมูลค่าที่ลง GR/IR / ล้าง GR/IR แล้ว
type Progress struct {
	Received      vo.Quantity
	Invoiced      vo.Quantity
	ReceivedValue vo.Money
	ClearedValue  vo.Money
}

func zeroProgress() Progress {
	return Progress{Received: decimal.Zero, Invoiced: decimal.Zero, ReceivedValue: decimal.Zero, ClearedValue: decimal.Zero}
}

type Approval struct {
	OrderNo    string // เช่น PO2026-000001 — ออกตอนอนุมัติ เรียงไม่ขาดตอนต่อปีบัญชี
	FiscalYear int
	By         vo.UserID
	At         time.Time
}

// Closure: By = nil คือปิดอัตโนมัติ (รับครบและแจ้งหนี้ครบ)
type Closure struct {
	By *vo.UserID
	At time.Time
}

type Cancellation struct {
	By vo.UserID
	At time.Time
}

type OrderState struct {
	ID           vo.OrderID
	CompanyID    vo.CompanyID
	Data         OrderData
	Terms        Terms
	Amounts      []services.Amounts // ยอดตามบรรทัด (ปัดตามทศนิยมของบริษัท)
	Totals       services.Amounts
	Progress     []Progress // ตามบรรทัด
	Status       vo.OrderStatus
	Approval     *Approval
	Closure      *Closure
	Cancellation *Cancellation
	CreatedBy    vo.UserID
}

// Order คือใบสั่งซื้อ
//
// กฎ:
//   - VAT ตามผู้ขาย ณ ตอนสร้าง: จด VAT = อัตราที่ตั้ง (เลือกได้ว่าราคารวม / แยก VAT), ไม่จด = 0%
//   - draft แก้ / ลบได้; อนุมัติแล้วรับสินค้าได้ไม่เกินจำนวนสั่ง, แจ้งหนี้ได้ไม่เกินจำนวนที่รับ
//   - รับครบและแจ้งหนี้ครบ = ปิดอัตโนมัติ; ปิดเอง = หยุดรับ (ยังแจ้งหนี้ของที่รับแล้วได้)
//   - ยกเลิกได้เมื่อยังไม่มีการรับ / แจ้งหนี้
type Order struct{ s OrderState }

// SupplierTerms คือเงื่อนไขจากผู้ขาย ณ ตอนนี้ (vatRate จากค่าตั้งจัดซื้อ)
func SupplierTerms(s *Supplier, vatRate decimal.Decimal, inclusive bool) (Terms, error) {
	d := s.Data()
	if !d.VatRegistered {
		if inclusive {
			return Terms{}, domain.ErrVatNotApplicable
		}
		return Terms{Vat: services.VatTerms{Rate: decimal.Zero}, PaymentTermDays: d.PaymentTermDays}, nil
	}
	return Terms{Vat: services.VatTerms{Registered: true, Rate: vatRate, Inclusive: inclusive}, PaymentTermDays: d.PaymentTermDays}, nil
}

// Decimals คือจำนวนทศนิยมที่บริษัทกำหนด
type Decimals struct{ Quantity, Amount int }

func NewOrder(companyID vo.CompanyID, by vo.UserID, d OrderData, terms Terms, dec Decimals) (*Order, error) {
	o := &Order{s: OrderState{ID: vo.NewID[vo.OrderID](), CompanyID: companyID, Status: vo.OrderDraft, CreatedBy: by}}
	if err := o.set(d, terms, dec); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Order) set(d OrderData, terms Terms, dec Decimals) error {
	n, err := normalizeOrder(d, dec.Quantity)
	if err != nil {
		return err
	}
	o.s.Data, o.s.Terms, o.s.Progress = n, terms, progressFor(n)
	o.s.Amounts, o.s.Totals = make([]services.Amounts, len(n.Lines)), services.ZeroAmounts()
	for i, l := range n.Lines {
		o.s.Amounts[i] = services.LineAmounts(l.Quantity, l.UnitPrice, terms.Vat, dec.Amount)
		o.s.Totals = o.s.Totals.Add(o.s.Amounts[i])
	}
	return nil
}

func RehydrateOrder(s OrderState) *Order { return &Order{s: s} }

func (o *Order) ID() vo.OrderID         { return o.s.ID }
func (o *Order) Data() OrderData        { return o.s.Data }
func (o *Order) Terms() Terms           { return o.s.Terms }
func (o *Order) Status() vo.OrderStatus { return o.s.Status }
func (o *Order) State() OrderState      { return o.s }

func (o *Order) ItemIDs() []vo.ItemID {
	seen := map[vo.ItemID]bool{}
	var out []vo.ItemID
	for _, l := range o.s.Data.Lines {
		if !seen[l.ItemID] {
			seen[l.ItemID] = true
			out = append(out, l.ItemID)
		}
	}
	return out
}

// Update แทนที่ draft ทั้งก้อน — terms คือเงื่อนไขจากผู้ขายของข้อมูลใหม่
func (o *Order) Update(d OrderData, terms Terms, dec Decimals) error {
	if o.s.Status != vo.OrderDraft {
		return domain.ErrOrderNotDraft
	}
	d.Kind = o.s.Data.Kind
	return o.set(d, terms, dec)
}

func (o *Order) Kind() vo.PurchaseKind { return o.s.Data.Kind }

// CheckLineKind: สายสินค้ารับเฉพาะสินค้า (stock / ไม่นับสต็อก), สายบริการรับเฉพาะสินค้าชนิดบริการ
// (จุดเกิดภาษีซื้อของสินค้ากับบริการต่างกัน จึงแยกเอกสารคนละใบ)
func CheckLineKind(kind vo.PurchaseKind, service bool, code string) error {
	switch {
	case kind == vo.KindService && !service:
		return fmt.Errorf("%w: %s is not a service item", domain.ErrServiceDocumentGoodsLine, code)
	case kind != vo.KindService && service:
		return fmt.Errorf("%w: %s is a service item", domain.ErrGoodsDocumentServiceLine, code)
	}
	return nil
}

func (o *Order) EnsureDraft() error {
	if o.s.Status != vo.OrderDraft {
		return domain.ErrOrderNotDraft
	}
	return nil
}

func (o *Order) Approve(a Approval) error {
	if err := o.EnsureDraft(); err != nil {
		return err
	}
	o.s.Status, o.s.Approval = vo.OrderApproved, &a
	return nil
}

// Close: ปิดเอง (หยุดรับสินค้า) — ของที่รับแล้วยังแจ้งหนี้ได้
func (o *Order) Close(by vo.UserID, at time.Time) error {
	if o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	o.s.Status, o.s.Closure = vo.OrderClosed, &Closure{By: &by, At: at}
	return nil
}

// Cancel: draft หรืออนุมัติแล้วแต่ยังไม่มีการรับ / แจ้งหนี้
func (o *Order) Cancel(by vo.UserID, at time.Time) error {
	if o.s.Status != vo.OrderDraft && o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	for _, p := range o.s.Progress {
		if !p.Received.IsZero() || !p.Invoiced.IsZero() {
			return domain.ErrOrderHasActivity
		}
	}
	o.s.Status, o.s.Cancellation = vo.OrderCancelled, &Cancellation{By: by, At: at}
	return nil
}

// Line คืนบรรทัดที่ n (เริ่ม 1)
func (o *Order) Line(n int) (OrderLineData, Progress, error) {
	if n < 1 || n > len(o.s.Data.Lines) {
		return OrderLineData{}, Progress{}, fmt.Errorf("%w: purchase order has no line %d", domain.ErrInvalidLine, n)
	}
	return o.s.Data.Lines[n-1], o.s.Progress[n-1], nil
}

// Delivery คือจำนวนและมูลค่า (ก่อน VAT) ที่รับ / แจ้งหนี้ของบรรทัดหนึ่ง
type Delivery struct {
	LineNo   int
	Quantity vo.Quantity
	Value    vo.Money // รับสินค้า: มูลค่าที่ลง GR/IR; แจ้งหนี้: มูลค่าที่ล้าง GR/IR
}

// CheckReceive: ใบสั่งซื้อต้องอนุมัติแล้ว (ยังไม่ปิด), จำนวน > 0, ไม่เกินที่เหลือให้รับ, บรรทัดไม่ซ้ำ
func (o *Order) CheckReceive(lines []Delivery, quantityDecimals int) error {
	if o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	return o.checkLines(lines, quantityDecimals, func(l OrderLineData, p Progress) vo.Quantity { return l.Quantity.Sub(p.Received) }, domain.ErrOverReceipt)
}

// Receive บันทึกการรับ (หลัง CheckReceive) — มูลค่าคือที่ลง GR/IR จริง
func (o *Order) Receive(lines []Delivery, quantityDecimals int, at time.Time) error {
	if err := o.CheckReceive(lines, quantityDecimals); err != nil {
		return err
	}
	for _, d := range lines {
		p := &o.s.Progress[d.LineNo-1]
		p.Received, p.ReceivedValue = p.Received.Add(d.Quantity), p.ReceivedValue.Add(d.Value)
	}
	o.autoClose(at)
	return nil
}

// Unreceive กลับการรับ (ยกเลิกใบรับสินค้า) — จำนวนที่แจ้งหนี้แล้วต้องไม่เกินที่รับหลังกลับ
func (o *Order) Unreceive(lines []Delivery) error {
	for _, d := range lines {
		_, p, err := o.Line(d.LineNo)
		if err != nil {
			return err
		}
		if p.Received.Sub(d.Quantity).LessThan(p.Invoiced) {
			return fmt.Errorf("%w: line %d", domain.ErrReceiptInvoiced, d.LineNo)
		}
	}
	for _, d := range lines {
		p := &o.s.Progress[d.LineNo-1]
		p.Received, p.ReceivedValue = p.Received.Sub(d.Quantity), p.ReceivedValue.Sub(d.Value)
	}
	o.reopen()
	return nil
}

// Invoice บันทึกการแจ้งหนี้ — คืนมูลค่าที่ล้าง GR/IR ต่อบรรทัด:
// แจ้งหนี้ครบจำนวนที่รับ = ล้างส่วนที่เหลือทั้งหมด (GR/IR ของบรรทัดเป็นศูนย์พอดี), ไม่ครบ = จำนวน × ราคาก่อน VAT
func (o *Order) Invoice(lines []Delivery, quantityDecimals, amountDecimals int, at time.Time) ([]vo.Money, error) {
	if o.s.Status != vo.OrderApproved && o.s.Status != vo.OrderClosed {
		return nil, domain.ErrOrderNotOpen
	}
	err := o.checkLines(lines, quantityDecimals, func(_ OrderLineData, p Progress) vo.Quantity { return p.Received.Sub(p.Invoiced) }, domain.ErrOverInvoice)
	if err != nil {
		return nil, err
	}
	cleared := make([]vo.Money, len(lines))
	for i, d := range lines {
		l, p := o.s.Data.Lines[d.LineNo-1], &o.s.Progress[d.LineNo-1]
		switch {
		case o.s.Data.Kind == vo.KindService:
			cleared[i] = decimal.Zero // ใบตรวจรับงานไม่ลงบัญชี — ไม่มี GR/IR ให้ล้าง (ค่าใช้จ่ายรับรู้ที่ซื้อบริการ)
		case p.Invoiced.Add(d.Quantity).Equal(p.Received):
			cleared[i] = p.ReceivedValue.Sub(p.ClearedValue)
		default:
			cleared[i] = services.ReceiptValue(d.Quantity, l.UnitPrice, o.s.Terms.Vat, amountDecimals)
		}
		p.Invoiced, p.ClearedValue = p.Invoiced.Add(d.Quantity), p.ClearedValue.Add(cleared[i])
	}
	o.autoClose(at)
	return cleared, nil
}

// Uninvoice กลับการแจ้งหนี้ (ยกเลิกใบแจ้งหนี้)
func (o *Order) Uninvoice(lines []Delivery) error {
	for _, d := range lines {
		if _, _, err := o.Line(d.LineNo); err != nil {
			return err
		}
	}
	for _, d := range lines {
		p := &o.s.Progress[d.LineNo-1]
		p.Invoiced, p.ClearedValue = p.Invoiced.Sub(d.Quantity), p.ClearedValue.Sub(d.Value)
	}
	o.reopen()
	return nil
}

func (o *Order) checkLines(lines []Delivery, quantityDecimals int, remaining func(OrderLineData, Progress) vo.Quantity, over error) error {
	if len(lines) == 0 || len(lines) > MaxLines {
		return fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	seen := map[int]bool{}
	for _, d := range lines {
		l, p, err := o.Line(d.LineNo)
		if err != nil {
			return err
		}
		switch {
		case seen[d.LineNo]:
			return fmt.Errorf("%w: line %d appears more than once", domain.ErrInvalidLine, d.LineNo)
		case !d.Quantity.IsPositive() || !numeric.InRange(d.Quantity):
			return fmt.Errorf("%w: line %d quantity must be positive", domain.ErrInvalidLine, d.LineNo)
		case !numeric.FitsScale(d.Quantity, quantityDecimals):
			return fmt.Errorf("%w: line %d quantity has more than %d decimal places", domain.ErrInvalidLine, d.LineNo, quantityDecimals)
		}
		if left := remaining(l, p); d.Quantity.GreaterThan(left) {
			return fmt.Errorf("%w: line %d (%s left)", over, d.LineNo, left)
		}
		seen[d.LineNo] = true
	}
	return nil
}

// autoClose: รับครบทุกบรรทัดและแจ้งหนี้ครบ -> ปิดอัตโนมัติ
func (o *Order) autoClose(at time.Time) {
	if o.s.Status != vo.OrderApproved {
		return
	}
	for i, l := range o.s.Data.Lines {
		p := o.s.Progress[i]
		if !p.Received.Equal(l.Quantity) || !p.Invoiced.Equal(p.Received) {
			return
		}
	}
	o.s.Status, o.s.Closure = vo.OrderClosed, &Closure{At: at}
}

// reopen: ปิดอัตโนมัติแล้วมีการยกเลิกเอกสาร -> กลับเป็นอนุมัติ (ปิดเองยังคงปิด)
func (o *Order) reopen() {
	if o.s.Status == vo.OrderClosed && o.s.Closure != nil && o.s.Closure.By == nil {
		o.s.Status, o.s.Closure = vo.OrderApproved, nil
	}
}

func progressFor(d OrderData) []Progress {
	out := make([]Progress, len(d.Lines))
	for i := range out {
		out[i] = zeroProgress()
	}
	return out
}

func normalizeOrder(d OrderData, quantityDecimals int) (OrderData, error) {
	var err error
	if !vo.IsDateOnly(d.Date) || (d.DeliveryDate != nil && (!vo.IsDateOnly(*d.DeliveryDate) || d.DeliveryDate.Before(d.Date))) {
		return OrderData{}, domain.ErrInvalidDate
	}
	out := OrderData{Kind: vo.KindOrDefault(string(d.Kind)), SupplierID: d.SupplierID, SupplierBranchNo: d.SupplierBranchNo, Date: d.Date, PriceIncludesVat: d.PriceIncludesVat}
	if d.DeliveryDate != nil {
		dd := *d.DeliveryDate
		out.DeliveryDate = &dd
	}
	if out.Reference, err = vo.OptionalText(d.Reference, 100); err != nil {
		return OrderData{}, err
	}
	if out.Description, err = vo.OptionalText(d.Description, 500); err != nil {
		return OrderData{}, err
	}
	if len(d.Lines) == 0 || len(d.Lines) > MaxLines {
		return OrderData{}, fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	out.Lines = make([]OrderLineData, len(d.Lines))
	for i, l := range d.Lines {
		bad := func(msg string) error { return fmt.Errorf("%w: line %d %s", domain.ErrInvalidLine, i+1, msg) }
		switch {
		case !l.Quantity.IsPositive() || !numeric.InRange(l.Quantity):
			return OrderData{}, bad("quantity must be positive")
		case !numeric.FitsScale(l.Quantity, quantityDecimals):
			return OrderData{}, bad(fmt.Sprintf("quantity has more than %d decimal places", quantityDecimals))
		case l.UnitPrice.IsNegative() || !numeric.InRange(l.UnitPrice) || !numeric.FitsScale(l.UnitPrice, numeric.MaxScale):
			return OrderData{}, bad(fmt.Sprintf("unit_price must be zero or positive with at most %d decimal places", numeric.MaxScale))
		}
		desc, err := vo.OptionalText(l.Description, 500)
		if err != nil {
			return OrderData{}, bad("description is too long")
		}
		out.Lines[i] = OrderLineData{ItemID: l.ItemID, Description: desc, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
	}
	return out, nil
}
