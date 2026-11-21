package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// ---------- ใบเสนอราคา ----------

type QuotationData struct {
	CustomerID       vo.CustomerID
	Date             time.Time
	ValidUntil       time.Time
	Reference        string
	Description      string
	PriceIncludesVat bool
	Lines            []LineData
}

type QuotationState struct {
	ID          vo.QuotationID
	CompanyID   vo.CompanyID
	QuotationNo string // ออกตอนส่งให้ลูกค้า (sent)
	FiscalYear  int
	Data        QuotationData
	Terms       Terms
	Priced      Priced
	Status      vo.QuotationStatus
	OrderID     *vo.OrderID // ใบสั่งขายที่แปลงจากใบนี้
	CreatedBy   vo.UserID
}

// Quotation คือใบเสนอราคา: draft (แก้ / ลบได้) -> sent (ออกเลขที่) -> accepted / rejected
// แปลงเป็นใบสั่งขายได้ครั้งเดียว เมื่อส่งแล้ว / ลูกค้ายอมรับ และยังไม่หมดอายุ — ไม่ลงบัญชี ไม่กระทบสต็อก
type Quotation struct{ s QuotationState }

func NewQuotation(companyID vo.CompanyID, by vo.UserID, d QuotationData, terms Terms, dec Decimals) (*Quotation, error) {
	q := &Quotation{s: QuotationState{ID: vo.NewID[vo.QuotationID](), CompanyID: companyID, Status: vo.QuotationDraft, CreatedBy: by}}
	return q, q.set(d, terms, dec)
}

func RehydrateQuotation(s QuotationState) *Quotation { return &Quotation{s: s} }

func (q *Quotation) ID() vo.QuotationID         { return q.s.ID }
func (q *Quotation) State() QuotationState      { return q.s }
func (q *Quotation) ItemIDs() []vo.ItemID       { return itemIDs(q.s.Data.Lines) }
func (q *Quotation) Status() vo.QuotationStatus { return q.s.Status }

func (q *Quotation) set(d QuotationData, terms Terms, dec Decimals) error {
	if !vo.IsDateOnly(d.Date) || !vo.IsDateOnly(d.ValidUntil) || d.ValidUntil.Before(d.Date) {
		return domain.ErrInvalidDate
	}
	var err error
	if d.Reference, d.Description, err = headerText(d.Reference, d.Description); err != nil {
		return err
	}
	p, err := price(d.Lines, terms, dec)
	if err != nil {
		return err
	}
	d.Lines = p.Lines
	q.s.Data, q.s.Terms, q.s.Priced = d, terms, p
	return nil
}

func (q *Quotation) Update(d QuotationData, terms Terms, dec Decimals) error {
	if q.s.Status != vo.QuotationDraft {
		return domain.ErrNotDraft
	}
	return q.set(d, terms, dec)
}

func (q *Quotation) EnsureDraft() error {
	if q.s.Status != vo.QuotationDraft {
		return domain.ErrNotDraft
	}
	return nil
}

// Send ออกเลขที่และส่งให้ลูกค้า
func (q *Quotation) Send(no string, fiscalYear int) error {
	if q.s.Status != vo.QuotationDraft {
		return fmt.Errorf("%w: only a draft can be sent", domain.ErrQuotationState)
	}
	q.s.Status, q.s.QuotationNo, q.s.FiscalYear = vo.QuotationSent, no, fiscalYear
	return nil
}

// Respond: ลูกค้ายอมรับ / ปฏิเสธ (ต้องส่งแล้ว)
func (q *Quotation) Respond(s vo.QuotationStatus) error {
	if q.s.Status != vo.QuotationSent || (s != vo.QuotationAccepted && s != vo.QuotationRejected) {
		return fmt.Errorf("%w: only a sent quotation can be accepted or rejected", domain.ErrQuotationState)
	}
	q.s.Status = s
	return nil
}

// Convert: แปลงเป็นใบสั่งขาย ณ วันที่ date — ส่งแล้ว / ยอมรับแล้ว, ยังไม่เคยแปลง, ยังไม่หมดอายุ
func (q *Quotation) Convert(orderID vo.OrderID, date time.Time) error {
	switch {
	case q.s.OrderID != nil:
		return domain.ErrQuotationUsed
	case q.s.Status != vo.QuotationSent && q.s.Status != vo.QuotationAccepted:
		return fmt.Errorf("%w: only a sent or accepted quotation can become a sales order", domain.ErrQuotationState)
	case date.After(q.s.Data.ValidUntil):
		return domain.ErrQuotationExpired
	}
	q.s.Status, q.s.OrderID = vo.QuotationAccepted, &orderID
	return nil
}

// ---------- ใบสั่งขาย ----------

type OrderData struct {
	CustomerID       vo.CustomerID
	QuotationID      *vo.QuotationID
	Date             time.Time
	DeliveryDate     *time.Time
	Reference        string
	Description      string
	PriceIncludesVat bool
	Lines            []LineData
}

// Progress: ส่งแล้ว / ออกบิลแล้ว ต่อบรรทัด
type Progress struct {
	Delivered vo.Quantity
	Invoiced  vo.Quantity
}

type Approval struct {
	OrderNo    string
	FiscalYear int
	By         vo.UserID
	At         time.Time
}

// Closure: By = nil คือปิดอัตโนมัติ (ส่งครบ + ออกบิลครบ)
type Closure struct {
	By *vo.UserID
	At time.Time
}

type OrderState struct {
	ID           vo.OrderID
	CompanyID    vo.CompanyID
	Data         OrderData
	Terms        Terms
	Priced       Priced
	Progress     []Progress
	Status       vo.OrderStatus
	Approval     *Approval
	Closure      *Closure
	Cancellation *Cancellation
	CreatedBy    vo.UserID
}

// Order คือใบสั่งขาย
//
// กฎ: VAT ตามค่าตั้งขาย ณ ตอนสร้าง (เก็บในใบ); อนุมัติแล้วส่งของได้ไม่เกินที่สั่ง, ออกบิล (แยกใบ) ได้ไม่เกินที่ส่งแล้ว,
// ส่งพร้อมออกบิล (รวมใบ) ได้ไม่เกินที่เหลือให้ส่ง; ส่งครบ + ออกบิลครบ = ปิดอัตโนมัติ
type Order struct{ s OrderState }

func NewOrder(companyID vo.CompanyID, by vo.UserID, d OrderData, terms Terms, dec Decimals) (*Order, error) {
	o := &Order{s: OrderState{ID: vo.NewID[vo.OrderID](), CompanyID: companyID, Status: vo.OrderDraft, CreatedBy: by}}
	return o, o.set(d, terms, dec)
}

func RehydrateOrder(s OrderState) *Order { return &Order{s: s} }

func (o *Order) ID() vo.OrderID         { return o.s.ID }
func (o *Order) State() OrderState      { return o.s }
func (o *Order) Status() vo.OrderStatus { return o.s.Status }
func (o *Order) ItemIDs() []vo.ItemID   { return itemIDs(o.s.Data.Lines) }

func (o *Order) set(d OrderData, terms Terms, dec Decimals) error {
	if !vo.IsDateOnly(d.Date) || (d.DeliveryDate != nil && (!vo.IsDateOnly(*d.DeliveryDate) || d.DeliveryDate.Before(d.Date))) {
		return domain.ErrInvalidDate
	}
	var err error
	if d.Reference, d.Description, err = headerText(d.Reference, d.Description); err != nil {
		return err
	}
	p, err := price(d.Lines, terms, dec)
	if err != nil {
		return err
	}
	d.Lines = p.Lines
	o.s.Data, o.s.Terms, o.s.Priced = d, terms, p
	o.s.Progress = make([]Progress, len(p.Lines))
	for i := range o.s.Progress {
		o.s.Progress[i] = Progress{Delivered: decimal.Zero, Invoiced: decimal.Zero}
	}
	return nil
}

func (o *Order) Update(d OrderData, terms Terms, dec Decimals) error {
	if err := o.EnsureDraft(); err != nil {
		return err
	}
	return o.set(d, terms, dec)
}

func (o *Order) EnsureDraft() error {
	if o.s.Status != vo.OrderDraft {
		return domain.ErrNotDraft
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

func (o *Order) Close(by vo.UserID, at time.Time) error {
	if o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	o.s.Status, o.s.Closure = vo.OrderClosed, &Closure{By: &by, At: at}
	return nil
}

func (o *Order) Cancel(by vo.UserID, at time.Time) error {
	if o.s.Status != vo.OrderDraft && o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	for _, p := range o.s.Progress {
		if !p.Delivered.IsZero() || !p.Invoiced.IsZero() {
			return domain.ErrOrderHasActivity
		}
	}
	o.s.Status, o.s.Cancellation = vo.OrderCancelled, &Cancellation{By: by, At: at}
	return nil
}

func (o *Order) Line(n int) (LineData, Progress, error) {
	if n < 1 || n > len(o.s.Data.Lines) {
		return LineData{}, Progress{}, fmt.Errorf("%w: sales order has no line %d", domain.ErrInvalidLine, n)
	}
	return o.s.Data.Lines[n-1], o.s.Progress[n-1], nil
}

// OpenAmount: ยอดรวม VAT ที่ยังไม่ได้ออกบิล (ใช้คำนวณวงเงินเครดิต)
func (o *Order) OpenAmount(amountDecimals int) vo.Money {
	total := decimal.Zero
	for i, l := range o.s.Data.Lines {
		left := l.Quantity.Sub(o.s.Progress[i].Invoiced)
		total = total.Add(o.s.Priced.Amounts[i].Gross.Mul(left).Div(l.Quantity))
	}
	return total.Round(int32(amountDecimals))
}

// Shipment คือจำนวนที่ส่ง / ออกบิลของบรรทัด
type Shipment struct {
	LineNo   int
	Quantity vo.Quantity
}

// CheckDeliver: ส่งของ (แยกใบ หรือรวมใบ) — อนุมัติแล้ว, ไม่เกินที่เหลือให้ส่ง
func (o *Order) CheckDeliver(lines []Shipment, quantityDecimals int) error {
	if o.s.Status != vo.OrderApproved {
		return domain.ErrOrderNotApproved
	}
	return o.checkLines(lines, quantityDecimals, func(l LineData, p Progress) vo.Quantity { return l.Quantity.Sub(p.Delivered) }, domain.ErrOverDelivery)
}

func (o *Order) Deliver(lines []Shipment, quantityDecimals int, at time.Time) error {
	if err := o.CheckDeliver(lines, quantityDecimals); err != nil {
		return err
	}
	for _, s := range lines {
		o.s.Progress[s.LineNo-1].Delivered = o.s.Progress[s.LineNo-1].Delivered.Add(s.Quantity)
	}
	o.autoClose(at)
	return nil
}

// Undeliver: ยกเลิกใบส่งของ — จำนวนที่ออกบิลแล้วต้องไม่เกินที่ส่งหลังกลับ
func (o *Order) Undeliver(lines []Shipment) error {
	for _, s := range lines {
		_, p, err := o.Line(s.LineNo)
		if err != nil {
			return err
		}
		if p.Delivered.Sub(s.Quantity).LessThan(p.Invoiced) {
			return fmt.Errorf("%w: line %d", domain.ErrDeliveryInvoiced, s.LineNo)
		}
	}
	for _, s := range lines {
		o.s.Progress[s.LineNo-1].Delivered = o.s.Progress[s.LineNo-1].Delivered.Sub(s.Quantity)
	}
	o.reopen()
	return nil
}

// CheckInvoice: ออกบิลแยกใบ — อนุมัติแล้วหรือปิดแล้ว, ไม่เกินที่ส่งแล้วยังไม่ออกบิล
func (o *Order) CheckInvoice(lines []Shipment, quantityDecimals int) error {
	if o.s.Status != vo.OrderApproved && o.s.Status != vo.OrderClosed {
		return domain.ErrOrderNotOpen
	}
	return o.checkLines(lines, quantityDecimals, func(_ LineData, p Progress) vo.Quantity { return p.Delivered.Sub(p.Invoiced) }, domain.ErrOverInvoice)
}

// Invoice บันทึกจำนวนที่ออกบิล — withDelivery = ใบส่งของ / ใบกำกับภาษี (ส่งและออกบิลพร้อมกัน)
func (o *Order) Invoice(lines []Shipment, quantityDecimals int, withDelivery bool, at time.Time) error {
	if withDelivery {
		if err := o.CheckDeliver(lines, quantityDecimals); err != nil {
			return err
		}
	} else if err := o.CheckInvoice(lines, quantityDecimals); err != nil {
		return err
	}
	for _, s := range lines {
		p := &o.s.Progress[s.LineNo-1]
		if withDelivery {
			p.Delivered = p.Delivered.Add(s.Quantity)
		}
		p.Invoiced = p.Invoiced.Add(s.Quantity)
	}
	o.autoClose(at)
	return nil
}

// Uninvoice: ยกเลิกใบแจ้งหนี้ (withDelivery = คืนจำนวนส่งด้วย)
func (o *Order) Uninvoice(lines []Shipment, withDelivery bool) error {
	for _, s := range lines {
		if _, _, err := o.Line(s.LineNo); err != nil {
			return err
		}
	}
	for _, s := range lines {
		p := &o.s.Progress[s.LineNo-1]
		p.Invoiced = p.Invoiced.Sub(s.Quantity)
		if withDelivery {
			p.Delivered = p.Delivered.Sub(s.Quantity)
		}
	}
	o.reopen()
	return nil
}

func (o *Order) checkLines(lines []Shipment, quantityDecimals int, remaining func(LineData, Progress) vo.Quantity, over error) error {
	if len(lines) == 0 || len(lines) > MaxLines {
		return fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	seen := map[int]bool{}
	for _, s := range lines {
		l, p, err := o.Line(s.LineNo)
		if err != nil {
			return err
		}
		switch {
		case seen[s.LineNo]:
			return fmt.Errorf("%w: line %d appears more than once", domain.ErrInvalidLine, s.LineNo)
		case !s.Quantity.IsPositive() || !numeric.InRange(s.Quantity):
			return fmt.Errorf("%w: line %d quantity must be positive", domain.ErrInvalidLine, s.LineNo)
		case !numeric.FitsScale(s.Quantity, quantityDecimals):
			return fmt.Errorf("%w: line %d quantity has more than %d decimal places", domain.ErrInvalidLine, s.LineNo, quantityDecimals)
		}
		if left := remaining(l, p); s.Quantity.GreaterThan(left) {
			return fmt.Errorf("%w: line %d (%s left)", over, s.LineNo, left)
		}
		seen[s.LineNo] = true
	}
	return nil
}

func (o *Order) autoClose(at time.Time) {
	if o.s.Status != vo.OrderApproved {
		return
	}
	for i, l := range o.s.Data.Lines {
		p := o.s.Progress[i]
		if !p.Delivered.Equal(l.Quantity) || !p.Invoiced.Equal(p.Delivered) {
			return
		}
	}
	o.s.Status, o.s.Closure = vo.OrderClosed, &Closure{At: at}
}

func (o *Order) reopen() {
	if o.s.Status == vo.OrderClosed && o.s.Closure != nil && o.s.Closure.By == nil {
		o.s.Status, o.s.Closure = vo.OrderApproved, nil
	}
}
