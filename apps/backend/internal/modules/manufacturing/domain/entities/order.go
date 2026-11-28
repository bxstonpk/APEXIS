package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	"apexis/backend/internal/modules/manufacturing/domain/services"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// OrderData คือข้อมูลที่ผู้ใช้กำหนดของใบสั่งผลิต
type OrderData struct {
	Date         time.Time
	ItemID       vo.ItemID
	BomID        vo.BomID
	Quantity     vo.Quantity
	WarehouseID  vo.WarehouseID // คลังที่เบิกวัตถุดิบและรับสินค้าสำเร็จรูปเข้า
	PlannedStart *time.Time
	PlannedEnd   *time.Time
	Description  string
}

// BomSnapshot คือสูตร ณ ตอนสร้าง / แก้ใบสั่งผลิต (ใบสั่งผลิตเก็บสำเนา — แก้สูตรภายหลังไม่กระทบ)
type BomSnapshot struct {
	ID             vo.BomID
	ItemID         vo.ItemID
	OutputQuantity vo.Quantity
	Active         bool
	Components     []Component
	Conversions    []Conversion
}

func SnapshotOf(b *Bom) BomSnapshot {
	return BomSnapshot{ID: b.ID(), ItemID: b.Data().ItemID, OutputQuantity: b.Data().OutputQuantity, Active: b.IsActive(),
		Components: b.Components(), Conversions: b.Conversions()}
}

// Material คือวัตถุดิบของใบสั่งผลิต: ปริมาณที่ต้องใช้ + ที่เบิก / คืนแล้ว และต้นทุน (FIFO จาก inventory)
type Material struct {
	LineNo       int
	ItemID       vo.ItemID
	Required     vo.Quantity
	Issued       vo.Quantity
	Returned     vo.Quantity
	IssuedCost   vo.Money
	ReturnedCost vo.Money
}

func (m Material) NetQuantity() vo.Quantity { return m.Issued.Sub(m.Returned) }
func (m Material) NetCost() vo.Money        { return m.IssuedCost.Sub(m.ReturnedCost) }

// MaterialInput: ปริมาณที่ต้องใช้ที่ผู้ใช้แก้เองตอนเป็น draft
type MaterialInput struct {
	ItemID   vo.ItemID
	Required vo.Quantity
}

// OrderConversion คือค่าแรง / ค่าใช้จ่ายการผลิตมาตรฐานต่อชุดที่คัดลอกจากสูตร
type OrderConversion struct {
	LineNo      int
	Description string
	AccountID   *vo.AccountID
	BatchAmount vo.Money
}

// Movement คือการเคลื่อนไหวสต็อกของใบสั่งผลิต (เอกสารสต็อกของ inventory + รายการบัญชีถ้าเป็นระบบต่อเนื่อง)
type Movement struct {
	ID              vo.MovementID
	Kind            vo.MovementKind
	Date            time.Time
	StockDocumentID vo.StockDocumentID
	StockDocumentNo string
	Value           vo.Money
	EntryID         *vo.EntryID
	EntryNo         string
	Cancelled       bool
}

// MovementLine คือบรรทัดเบิก / คืนวัตถุดิบ
type MovementLine struct {
	ItemID   vo.ItemID
	Quantity vo.Quantity
}

// Order คือใบสั่งผลิต
//
// สถานะ: draft -> released (ปล่อยผลิต) -> completed (ปิดงาน); draft / released / completed -> cancelled
//   - draft: แก้ข้อมูลและปริมาณวัตถุดิบได้; ยังไม่มีการเคลื่อนไหวสต็อก
//   - released: เบิก / คืนวัตถุดิบได้หลายครั้ง (เบิกเกินที่กำหนดได้ — รายงานแสดงผลต่าง), คืนได้ไม่เกินที่เบิกสุทธิ
//   - completed: รับสินค้าสำเร็จรูปเข้าคลังที่ต้นทุนจริง = วัตถุดิบสุทธิ + ค่าแรง / ค่าใช้จ่ายมาตรฐานตามจำนวนที่ผลิตได้
//   - cancelled: use case ยกเลิกเอกสารสต็อก / กลับรายการบัญชีทุกการเคลื่อนไหวก่อน (all-or-nothing)
//
// วันที่เบิก / คืน / ปิดงานต้องไม่ก่อนวันที่ใบสั่งผลิต
type Order struct {
	id                vo.OrderID
	companyID         vo.CompanyID
	documentNo        string
	data              OrderData
	outputQuantity    vo.Quantity // จำนวนที่ได้ต่อชุดของสูตร (ใช้ปรับค่าแรง / ค่าใช้จ่ายตามจำนวน)
	status            vo.OrderStatus
	materials         []Material
	conversions       []OrderConversion
	movements         []Movement
	completedQuantity vo.Quantity
	completedAt       *time.Time
	conversionApplied vo.Money
	cancelledAt       *time.Time
	cancelReason      string
	createdBy         vo.UserID
	createdAt         time.Time
}

// OrderState คือข้อมูลทั้งหมดของใบสั่งผลิต (repository ใช้ Rehydrate / mapper ใช้อ่าน)
type OrderState struct {
	ID                vo.OrderID
	CompanyID         vo.CompanyID
	DocumentNo        string
	Data              OrderData
	OutputQuantity    vo.Quantity
	Status            vo.OrderStatus
	Materials         []Material
	Conversions       []OrderConversion
	Movements         []Movement
	CompletedQuantity vo.Quantity
	CompletedAt       *time.Time
	ConversionApplied vo.Money
	CancelledAt       *time.Time
	CancelReason      string
	CreatedBy         vo.UserID
	CreatedAt         time.Time
}

func RehydrateOrder(s OrderState) *Order {
	return &Order{id: s.ID, companyID: s.CompanyID, documentNo: s.DocumentNo, data: s.Data, outputQuantity: s.OutputQuantity,
		status: s.Status, materials: s.Materials, conversions: s.Conversions, movements: s.Movements,
		completedQuantity: s.CompletedQuantity, completedAt: s.CompletedAt, conversionApplied: s.ConversionApplied,
		cancelledAt: s.CancelledAt, cancelReason: s.CancelReason, createdBy: s.CreatedBy, createdAt: s.CreatedAt}
}

func (o *Order) State() OrderState {
	return OrderState{ID: o.id, CompanyID: o.companyID, DocumentNo: o.documentNo, Data: o.data, OutputQuantity: o.outputQuantity,
		Status: o.status, Materials: o.materials, Conversions: o.conversions, Movements: o.movements,
		CompletedQuantity: o.completedQuantity, CompletedAt: o.completedAt, ConversionApplied: o.conversionApplied,
		CancelledAt: o.cancelledAt, CancelReason: o.cancelReason, CreatedBy: o.createdBy, CreatedAt: o.createdAt}
}

func (o *Order) ID() vo.OrderID                 { return o.id }
func (o *Order) CompanyID() vo.CompanyID        { return o.companyID }
func (o *Order) DocumentNo() string             { return o.documentNo }
func (o *Order) Data() OrderData                { return o.data }
func (o *Order) Status() vo.OrderStatus         { return o.status }
func (o *Order) Materials() []Material          { return o.materials }
func (o *Order) Movements() []Movement          { return o.movements }
func (o *Order) Conversions() []OrderConversion { return o.conversions }

// NewOrder สร้างใบสั่งผลิต (draft) จากสูตร: ปริมาณวัตถุดิบตามสูตร × จำนวนสั่งผลิต (รวม %สูญเสีย)
func NewOrder(companyID vo.CompanyID, documentNo string, d OrderData, bom BomSnapshot, createdBy vo.UserID, now time.Time, dec Decimals) (*Order, error) {
	o := &Order{id: vo.NewID[vo.OrderID](), companyID: companyID, documentNo: documentNo, status: vo.OrderDraft,
		createdBy: createdBy, createdAt: now}
	if err := o.set(d, bom, nil, dec); err != nil {
		return nil, err
	}
	return o, nil
}

// UpdateDraft แก้ใบสั่งผลิตที่เป็น draft — materials = nil: คำนวณปริมาณวัตถุดิบจากสูตรใหม่
func (o *Order) UpdateDraft(d OrderData, bom BomSnapshot, materials []MaterialInput, dec Decimals) error {
	if o.status != vo.OrderDraft {
		return domain.ErrOrderNotDraft
	}
	return o.set(d, bom, materials, dec)
}

func (o *Order) set(d OrderData, bom BomSnapshot, materials []MaterialInput, dec Decimals) error {
	var err error
	if !vo.IsDateOnly(d.Date) {
		return domain.ErrInvalidDate
	}
	for _, p := range []*time.Time{d.PlannedStart, d.PlannedEnd} {
		if p != nil && !vo.IsDateOnly(*p) {
			return domain.ErrInvalidDate
		}
	}
	if d.PlannedStart != nil && d.PlannedEnd != nil && d.PlannedEnd.Before(*d.PlannedStart) {
		return fmt.Errorf("%w: planned_end must not be before planned_start", domain.ErrInvalidDate)
	}
	if !d.Quantity.IsPositive() || !vo.FitsCompany(d.Quantity, dec.Quantity) {
		return fmt.Errorf("%w: quantity must be greater than 0 with at most %d decimal places", domain.ErrInvalidQuantity, dec.Quantity)
	}
	if d.Description, err = vo.OptionalText(d.Description, maxDescriptionLength); err != nil {
		return err
	}
	if bom.ItemID != d.ItemID || bom.ID != d.BomID {
		return domain.ErrBomItemMismatch
	}
	if !bom.Active {
		return domain.ErrBomInactive
	}
	var lines []Material
	if materials == nil {
		lines = make([]Material, len(bom.Components))
		for i, c := range bom.Components {
			lines[i] = newMaterial(i+1, c.ItemID, services.Requirement(c.Quantity, c.ScrapPercent, bom.OutputQuantity, d.Quantity, dec.Quantity))
		}
	} else {
		if len(materials) == 0 || len(materials) > maxBomLines {
			return fmt.Errorf("%w: materials must have 1 to %d lines", domain.ErrInvalidLine, maxBomLines)
		}
		seen := map[vo.ItemID]bool{}
		lines = make([]Material, len(materials))
		for i, m := range materials {
			n := i + 1
			switch {
			case m.ItemID == d.ItemID:
				return fmt.Errorf("%w: material %d is the produced item", domain.ErrInvalidLine, n)
			case seen[m.ItemID]:
				return fmt.Errorf("%w: material %d repeats an item", domain.ErrInvalidLine, n)
			case m.Required.IsNegative() || !vo.FitsCompany(m.Required, dec.Quantity):
				return fmt.Errorf("%w: material %d required_quantity must be 0 or more with at most %d decimal places", domain.ErrInvalidLine, n, dec.Quantity)
			}
			seen[m.ItemID] = true
			lines[i] = newMaterial(n, m.ItemID, m.Required)
		}
	}
	conversions := make([]OrderConversion, len(bom.Conversions))
	for i, c := range bom.Conversions {
		conversions[i] = OrderConversion{LineNo: i + 1, Description: c.Description, AccountID: c.AccountID, BatchAmount: c.Amount}
	}
	o.data, o.outputQuantity, o.materials, o.conversions = d, bom.OutputQuantity, lines, conversions
	return nil
}

func newMaterial(n int, item vo.ItemID, required vo.Quantity) Material {
	return Material{LineNo: n, ItemID: item, Required: required, Issued: decimal.Zero, Returned: decimal.Zero,
		IssuedCost: decimal.Zero, ReturnedCost: decimal.Zero}
}

// MaterialItemIDs คือวัตถุดิบทั้งหมดของใบสั่งผลิต
func (o *Order) MaterialItemIDs() []vo.ItemID {
	out := make([]vo.ItemID, len(o.materials))
	for i, m := range o.materials {
		out[i] = m.ItemID
	}
	return out
}

// PlannedConversion คือค่าแรง / ค่าใช้จ่ายการผลิตมาตรฐานตามจำนวนสั่งผลิต
func (o *Order) PlannedConversion(amountDecimals int) vo.Money {
	total := decimal.Zero
	for _, c := range o.conversions {
		total = total.Add(services.Scaled(c.BatchAmount, o.outputQuantity, o.data.Quantity, amountDecimals))
	}
	return total
}

// Costs คือต้นทุนสะสม: วัตถุดิบสุทธิ (เบิก − คืน), ค่าแรง / ค่าใช้จ่ายที่ปันเข้างานแล้ว (ตอนปิดงาน) และรวม
func (o *Order) Costs() (materials, conversion, total vo.Money) {
	materials = decimal.Zero
	for _, m := range o.materials {
		materials = materials.Add(m.NetCost())
	}
	return materials, o.conversionApplied, materials.Add(o.conversionApplied)
}

// UnitCost คือต้นทุนต่อหน่วยของสินค้าที่ผลิตได้ (nil = ยังไม่ปิดงาน)
func (o *Order) UnitCost() *vo.Money {
	if o.completedAt == nil || !o.completedQuantity.IsPositive() {
		return nil
	}
	_, _, total := o.Costs()
	u := total.DivRound(o.completedQuantity, numeric.MaxScale)
	return &u
}

func (o *Order) Completion() (quantity vo.Quantity, at *time.Time) {
	return o.completedQuantity, o.completedAt
}

func (o *Order) Cancellation() (at *time.Time, reason string) { return o.cancelledAt, o.cancelReason }

func (o *Order) Creation() (by vo.UserID, at time.Time) { return o.createdBy, o.createdAt }

// Release ปล่อยผลิต (draft -> released)
func (o *Order) Release() error {
	if o.status != vo.OrderDraft {
		return domain.ErrOrderNotDraft
	}
	o.status = vo.OrderReleased
	return nil
}

// CheckMovement ตรวจการเบิก / คืน / ปิดงาน: ใบสั่งผลิตต้อง released และวันที่ไม่ก่อนวันที่ใบสั่งผลิต
func (o *Order) checkReleased(date time.Time) error {
	if o.status != vo.OrderReleased {
		return domain.ErrOrderNotReleased
	}
	if !vo.IsDateOnly(date) {
		return domain.ErrInvalidDate
	}
	if date.Before(o.data.Date) {
		return domain.ErrDateBeforeOrder
	}
	return nil
}

func (o *Order) material(item vo.ItemID) int {
	for i, m := range o.materials {
		if m.ItemID == item {
			return i
		}
	}
	return -1
}

// CheckLines ตรวจบรรทัดเบิก / คืน: มีอย่างน้อย 1 บรรทัด, วัตถุดิบอยู่ในใบสั่งผลิตและไม่ซ้ำ, จำนวน > 0 ตามทศนิยมของบริษัท
func (o *Order) CheckLines(date time.Time, lines []MovementLine, dec Decimals) error {
	if err := o.checkReleased(date); err != nil {
		return err
	}
	if len(lines) == 0 || len(lines) > maxBomLines {
		return fmt.Errorf("%w: lines must have 1 to %d lines", domain.ErrInvalidLine, maxBomLines)
	}
	seen := map[vo.ItemID]bool{}
	for i, l := range lines {
		n := i + 1
		switch {
		case o.material(l.ItemID) < 0:
			return fmt.Errorf("%w: line %d", domain.ErrMaterialNotOnOrder, n)
		case seen[l.ItemID]:
			return fmt.Errorf("%w: line %d repeats an item", domain.ErrInvalidLine, n)
		case !l.Quantity.IsPositive() || !vo.FitsCompany(l.Quantity, dec.Quantity):
			return fmt.Errorf("%w: line %d quantity must be greater than 0 with at most %d decimal places", domain.ErrInvalidLine, n, dec.Quantity)
		}
		seen[l.ItemID] = true
	}
	return nil
}

// ApplyIssue บันทึกการเบิกวัตถุดิบ (costs = ต้นทุน FIFO ต่อบรรทัดจาก inventory)
func (o *Order) ApplyIssue(lines []MovementLine, costs []vo.Money, mv Movement) {
	for i, l := range lines {
		m := &o.materials[o.material(l.ItemID)]
		m.Issued = m.Issued.Add(l.Quantity)
		m.IssuedCost = m.IssuedCost.Add(costs[i])
	}
	o.movements = append(o.movements, mv)
}

// ReturnValues คือมูลค่าของวัตถุดิบที่คืน: ต้นทุนเฉลี่ยของที่เบิกสุทธิในใบสั่งผลิตนี้ × จำนวนคืน
// คืนได้ไม่เกินจำนวนที่เบิกสุทธิ; คืนหมดได้มูลค่าเท่าต้นทุนสุทธิพอดี (เรียกหลัง CheckLines)
func (o *Order) ReturnValues(lines []MovementLine, amountDecimals int) ([]vo.Money, error) {
	out := make([]vo.Money, len(lines))
	for i, l := range lines {
		m := o.materials[o.material(l.ItemID)]
		if l.Quantity.GreaterThan(m.NetQuantity()) {
			return nil, fmt.Errorf("%w: line %d (net issued %s)", domain.ErrOverReturn, i+1, m.NetQuantity())
		}
		out[i] = services.Proportional(m.NetCost(), m.NetQuantity(), l.Quantity, amountDecimals)
	}
	return out, nil
}

// ApplyReturn บันทึกการคืนวัตถุดิบ (values จาก ReturnValues)
func (o *Order) ApplyReturn(lines []MovementLine, values []vo.Money, mv Movement) {
	for i, l := range lines {
		m := &o.materials[o.material(l.ItemID)]
		m.Returned = m.Returned.Add(l.Quantity)
		m.ReturnedCost = m.ReturnedCost.Add(values[i])
	}
	o.movements = append(o.movements, mv)
}

// AppliedConversion คือค่าแรง / ค่าใช้จ่ายมาตรฐานหนึ่งบรรทัดที่ปันเข้างานตอนปิดงาน
type AppliedConversion struct {
	AccountID *vo.AccountID // nil = บัญชีปันเข้างานในค่าตั้ง
	Amount    vo.Money
}

// CompletionPlan คือต้นทุนของการปิดงาน
type CompletionPlan struct {
	Quantity    vo.Quantity
	Materials   vo.Money
	Conversions []AppliedConversion
	Conversion  vo.Money
	Total       vo.Money
}

// PlanCompletion คำนวณต้นทุนปิดงาน: วัตถุดิบสุทธิ + ค่าแรง / ค่าใช้จ่ายต่อชุด × (จำนวนที่ผลิตได้ / จำนวนต่อชุด)
func (o *Order) PlanCompletion(date time.Time, quantity vo.Quantity, dec Decimals) (CompletionPlan, error) {
	if err := o.checkReleased(date); err != nil {
		return CompletionPlan{}, err
	}
	if !quantity.IsPositive() || !vo.FitsCompany(quantity, dec.Quantity) {
		return CompletionPlan{}, fmt.Errorf("%w: completed_quantity must be greater than 0 with at most %d decimal places", domain.ErrInvalidQuantity, dec.Quantity)
	}
	materials, _, _ := o.Costs()
	p := CompletionPlan{Quantity: quantity, Materials: materials, Conversion: decimal.Zero}
	for _, c := range o.conversions {
		amount := services.Scaled(c.BatchAmount, o.outputQuantity, quantity, dec.Amount)
		if amount.IsZero() {
			continue
		}
		p.Conversions = append(p.Conversions, AppliedConversion{AccountID: c.AccountID, Amount: amount})
		p.Conversion = p.Conversion.Add(amount)
	}
	p.Total = p.Materials.Add(p.Conversion)
	return p, nil
}

// Complete ปิดงาน (released -> completed) หลังรับสินค้าสำเร็จรูปเข้าคลังแล้ว
func (o *Order) Complete(date time.Time, p CompletionPlan, mv Movement) {
	d := date
	o.status, o.completedQuantity, o.completedAt, o.conversionApplied = vo.OrderCompleted, p.Quantity, &d, p.Conversion
	o.movements = append(o.movements, mv)
}

// ActiveMovements คือการเคลื่อนไหวที่ยังไม่ยกเลิก เรียงจากล่าสุดไปเก่าสุด (ลำดับที่ต้องยกเลิก)
func (o *Order) ActiveMovements() []Movement {
	var out []Movement
	for i := len(o.movements) - 1; i >= 0; i-- {
		if !o.movements[i].Cancelled {
			out = append(out, o.movements[i])
		}
	}
	return out
}

// CheckCancel ตรวจว่ายกเลิกได้ และเหตุผลถูกต้อง (คืนเหตุผลที่ตัดช่องว่างแล้ว)
func (o *Order) CheckCancel(reason string) (string, error) {
	if o.status == vo.OrderCancelled {
		return "", domain.ErrAlreadyCancelled
	}
	r, err := vo.NewName(reason, maxDescriptionLength)
	if err != nil {
		return "", domain.ErrInvalidReason
	}
	return r, nil
}

// Cancel ยกเลิกใบสั่งผลิต — use case ยกเลิกเอกสารสต็อก / กลับรายการบัญชีของทุกการเคลื่อนไหวแล้ว
func (o *Order) Cancel(reason string, now time.Time) {
	for i := range o.movements {
		o.movements[i].Cancelled = true
	}
	t := now
	o.status, o.cancelledAt, o.cancelReason = vo.OrderCancelled, &t, reason
}
