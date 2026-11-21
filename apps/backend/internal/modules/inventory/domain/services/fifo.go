// Package services คือ domain service ของ inventory — ต้นทุน FIFO และผลของเอกสารต่อสต็อก
//
// ทั้งหมดเป็น pure function: repository โหลดชั้นต้นทุน (layer) ที่เกี่ยวข้องพร้อม lock มาให้,
// service คำนวณว่าต้องแก้ / สร้างอะไร, repository เขียนผลกลับใน transaction เดียวกัน
package services

import (
	"bytes"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// StockKey คือสต็อกของสินค้าหนึ่งในคลังหนึ่ง (ยอดและต้นทุน FIFO แยกตาม key นี้)
type StockKey struct {
	ItemID      vo.ItemID
	WarehouseID vo.WarehouseID
}

// Layer คือชั้นต้นทุน FIFO: ของที่รับเข้าครั้งหนึ่ง (จำนวน / มูลค่า) และส่วนที่ยังเหลือ
type Layer struct {
	ID                vo.LayerID
	Key               StockKey
	Date              time.Time
	DocumentID        vo.DocumentID
	LineNo            int
	Quantity          vo.Quantity
	Value             vo.Money
	RemainingQuantity vo.Quantity
	RemainingValue    vo.Money
}

func (l *Layer) untouched() bool {
	return l.RemainingQuantity.Equal(l.Quantity) && l.RemainingValue.Equal(l.Value)
}

// Allocation คือส่วนที่ตัดจากชั้นหนึ่งโดยการเคลื่อนไหวขาออก (ใช้คืนชั้นเมื่อยกเลิกเอกสาร)
type Allocation struct {
	LayerID  vo.LayerID
	Quantity vo.Quantity
	Value    vo.Money
}

// Movement คือการเคลื่อนไหวสต็อก — จำนวน / มูลค่ามีเครื่องหมาย (+ เข้า, - ออก)
type Movement struct {
	ID          vo.MovementID
	Key         StockKey
	Date        time.Time
	DocumentID  vo.DocumentID
	LineNo      int
	Quantity    vo.Quantity
	Value       vo.Money
	Allocations []Allocation // เฉพาะขาออก
}

// PostingPlan คือผลของการผ่านเอกสาร
type PostingPlan struct {
	LineValues    []vo.Money // มูลค่าต้นทุนของแต่ละบรรทัด (ไม่ติดลบ)
	UpdatedLayers []*Layer   // ชั้นเดิมที่ถูกตัด (remaining ใหม่)
	NewLayers     []*Layer
	Movements     []Movement
}

// PlanPosting คำนวณผลของเอกสารต่อสต็อกด้วย FIFO
//
//   - available: ชั้นที่ยังเหลือของทุก key ที่เอกสารตัดออก และลงวันที่ไม่เกินวันที่เอกสาร
//     (repository โหลดพร้อม lock FOR UPDATE) — ตัดจากชั้นเก่าสุดก่อน (วันที่, แล้วลำดับที่สร้าง)
//   - ขาเข้า: มูลค่า = จำนวน × ต้นทุนต่อหน่วย ปัดตามทศนิยมของเงินที่บริษัทกำหนด
//   - ขาออก: ตัดชั้นบางส่วน มูลค่า = มูลค่าคงเหลือของชั้น × สัดส่วน (ปัด); ตัดหมดชั้น = มูลค่าคงเหลือทั้งหมด
//     จึงไม่มีเศษสะสมค้างในชั้น
//   - ไม่พอ = domain.ErrInsufficientStock (ห้ามติดลบ)
func PlanPosting(doc *entities.Document, available []*Layer, amountDecimals int) (PostingPlan, error) {
	d := doc.Data()
	byKey := map[StockKey][]*Layer{}
	for _, l := range available {
		byKey[l.Key] = append(byKey[l.Key], l)
	}
	for k := range byKey {
		slices.SortStableFunc(byKey[k], fifoOrder)
	}

	plan := PostingPlan{LineValues: make([]vo.Money, len(d.Lines))}
	updated := map[vo.LayerID]*Layer{}
	for i, line := range d.Lines {
		no := i + 1
		from := StockKey{ItemID: line.ItemID, WarehouseID: d.WarehouseID}

		if line.Inbound(d.Type) {
			value := line.Quantity.Mul(*line.UnitCost).Round(int32(amountDecimals))
			l := newLayer(doc, no, from, line.Quantity, value)
			byKey[from] = append(byKey[from], l) // บรรทัดถัดไปในเอกสารเดียวกันตัดชั้นนี้ได้
			plan.NewLayers = append(plan.NewLayers, l)
			plan.Movements = append(plan.Movements, movement(doc, no, from, line.Quantity, value, nil))
			plan.LineValues[i] = value
			continue
		}

		qty := line.Quantity.Abs()
		allocs, total, err := consume(byKey[from], qty, amountDecimals, d.Date)
		if err != nil {
			return PostingPlan{}, fmt.Errorf("%w: line %d", err, no)
		}
		for _, a := range allocs {
			for _, l := range byKey[from] {
				if l.ID == a.LayerID && !isNew(plan.NewLayers, l.ID) {
					updated[l.ID] = l
				}
			}
		}
		plan.Movements = append(plan.Movements, movement(doc, no, from, qty.Neg(), total.Neg(), allocs))
		plan.LineValues[i] = total

		if d.Type == vo.DocTransfer {
			// ฝั่งรับ: หนึ่งชั้นต่อหนึ่งชั้นที่ตัดออก — ต้นทุน FIFO เดิมตามไปด้วย
			to := StockKey{ItemID: line.ItemID, WarehouseID: *d.ToWarehouseID}
			for _, a := range allocs {
				plan.NewLayers = append(plan.NewLayers, newLayer(doc, no, to, a.Quantity, a.Value))
			}
			plan.Movements = append(plan.Movements, movement(doc, no, to, qty, total, nil))
		}
	}
	for _, l := range updated {
		plan.UpdatedLayers = append(plan.UpdatedLayers, l)
	}
	slices.SortFunc(plan.UpdatedLayers, func(a, b *Layer) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return plan, nil
}

// consume ตัดจำนวน qty จากชั้นตามลำดับ FIFO (แก้ remaining ของชั้นโดยตรง)
func consume(layers []*Layer, qty vo.Quantity, amountDecimals int, asOf time.Time) ([]Allocation, vo.Money, error) {
	// ตรวจว่าพอก่อนแก้ชั้นใดๆ — ล้มเหลวแล้วชั้นต้องไม่ถูกแตะ
	available := decimal.Zero
	for _, l := range layers {
		if l.RemainingQuantity.IsPositive() && !l.Date.After(asOf) {
			available = available.Add(l.RemainingQuantity)
		}
	}
	if available.LessThan(qty) {
		return nil, decimal.Zero, fmt.Errorf("%w: need %s, available %s", domain.ErrInsufficientStock, qty, available)
	}

	need := qty
	total := decimal.Zero
	var allocs []Allocation
	for _, l := range layers {
		if need.IsZero() {
			break
		}
		if !l.RemainingQuantity.IsPositive() || l.Date.After(asOf) {
			continue
		}
		take := decimal.Min(l.RemainingQuantity, need)
		var value vo.Money
		if take.Equal(l.RemainingQuantity) {
			value = l.RemainingValue
		} else {
			value = l.RemainingValue.Mul(take).Div(l.RemainingQuantity).Round(int32(amountDecimals))
		}
		l.RemainingQuantity = l.RemainingQuantity.Sub(take)
		l.RemainingValue = l.RemainingValue.Sub(value)
		allocs = append(allocs, Allocation{LayerID: l.ID, Quantity: take, Value: value})
		total = total.Add(value)
		need = need.Sub(take)
	}
	return allocs, total, nil
}

// CancelInput คือข้อมูลที่ repository โหลด (พร้อม lock) สำหรับยกเลิกเอกสาร
type CancelInput struct {
	Movements       []Movement            // การเคลื่อนไหวของเอกสาร (ขาออกมี Allocations)
	CreatedLayerIDs []vo.LayerID          // ชั้นที่เอกสารนี้สร้าง
	Layers          map[vo.LayerID]*Layer // ชั้นทั้งหมดที่เกี่ยวข้อง (ที่สร้าง + ที่ถูกตัด)
}

// CancelPlan คือผลของการยกเลิก: ชั้นที่เอกสารสร้างถูกปิด (คงเหลือ 0 — ไม่ลบ เพราะประวัติ allocation อ้างถึง),
// ชั้นที่ถูกตัดได้คืน, และการเคลื่อนไหวกลับรายการ (วันที่เดิมของเอกสาร)
type CancelPlan struct {
	UpdatedLayers []*Layer
	Movements     []Movement
}

// PlanCancel: คืนส่วนที่ตัดไปให้ชั้นเดิมก่อน แล้วตรวจว่าชั้นที่เอกสารนี้สร้างยังไม่ถูกใช้
// (ถ้าเอกสารอื่นตัดไปแล้ว = domain.ErrStockAlreadyUsed — ต้องยกเลิกเอกสารที่ใช้ก่อน หรือทำใบปรับปรุงแทน)
func PlanCancel(in CancelInput) (CancelPlan, error) {
	restored := map[vo.LayerID]*Layer{}
	for _, m := range in.Movements {
		for _, a := range m.Allocations {
			l, ok := in.Layers[a.LayerID]
			if !ok {
				return CancelPlan{}, fmt.Errorf("inventory: layer %v of allocation not loaded", a.LayerID)
			}
			l.RemainingQuantity = l.RemainingQuantity.Add(a.Quantity)
			l.RemainingValue = l.RemainingValue.Add(a.Value)
			restored[l.ID] = l
		}
	}
	plan := CancelPlan{}
	for _, id := range in.CreatedLayerIDs {
		l, ok := in.Layers[id]
		if !ok {
			return CancelPlan{}, fmt.Errorf("inventory: created layer %v not loaded", id)
		}
		if !l.untouched() {
			return CancelPlan{}, fmt.Errorf("%w: line %d", domain.ErrStockAlreadyUsed, l.LineNo)
		}
		l.RemainingQuantity, l.RemainingValue = decimal.Zero, decimal.Zero
		restored[id] = l
	}
	for _, l := range restored {
		plan.UpdatedLayers = append(plan.UpdatedLayers, l)
	}
	slices.SortFunc(plan.UpdatedLayers, func(a, b *Layer) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	for _, m := range in.Movements {
		plan.Movements = append(plan.Movements, Movement{
			ID: vo.NewID[vo.MovementID](), Key: m.Key, Date: m.Date, DocumentID: m.DocumentID, LineNo: m.LineNo,
			Quantity: m.Quantity.Neg(), Value: m.Value.Neg(),
		})
	}
	return plan, nil
}

func fifoOrder(a, b *Layer) int {
	if c := a.Date.Compare(b.Date); c != 0 {
		return c
	}
	return bytes.Compare(a.ID[:], b.ID[:]) // UUIDv7 เรียงตามเวลาที่สร้าง
}

func newLayer(doc *entities.Document, lineNo int, key StockKey, qty vo.Quantity, value vo.Money) *Layer {
	return &Layer{
		ID: vo.NewID[vo.LayerID](), Key: key, Date: doc.Data().Date, DocumentID: doc.ID(), LineNo: lineNo,
		Quantity: qty, Value: value, RemainingQuantity: qty, RemainingValue: value,
	}
}

func movement(doc *entities.Document, lineNo int, key StockKey, qty vo.Quantity, value vo.Money, allocs []Allocation) Movement {
	return Movement{
		ID: vo.NewID[vo.MovementID](), Key: key, Date: doc.Data().Date, DocumentID: doc.ID(), LineNo: lineNo,
		Quantity: qty, Value: value, Allocations: allocs,
	}
}

func isNew(layers []*Layer, id vo.LayerID) bool {
	for _, l := range layers {
		if l.ID == id {
			return true
		}
	}
	return false
}
