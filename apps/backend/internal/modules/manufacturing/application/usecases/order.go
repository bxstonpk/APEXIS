package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/application/ports"
	"apexis/backend/internal/modules/manufacturing/application/ports/repositories"
	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	"apexis/backend/internal/modules/manufacturing/domain/services"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// OrderUseCases คือใบสั่งผลิต: สร้าง / แก้ (draft) -> ปล่อยผลิต -> เบิก / คืนวัตถุดิบ -> ปิดงาน (รับสินค้าสำเร็จรูป) / ยกเลิก
//
// การลงบัญชี (เฉพาะปีบัญชีที่ inventory กำหนดเป็นระบบต่อเนื่อง; สมุดรายวัน + งานระหว่างผลิตจากค่าตั้ง):
//   - เบิกวัตถุดิบ: Dr งานระหว่างผลิต / Cr สินค้าคงเหลือของวัตถุดิบแต่ละตัว (ต้นทุน FIFO)
//   - คืนวัตถุดิบ: Dr สินค้าคงเหลือของวัตถุดิบ / Cr งานระหว่างผลิต (ต้นทุนเฉลี่ยที่เบิกสุทธิ)
//   - ปิดงาน: Dr สินค้าคงเหลือของสินค้าสำเร็จรูป (ต้นทุนรวม) / Cr งานระหว่างผลิต (วัตถุดิบสุทธิ)
//     / Cr บัญชีปันเข้างานของค่าแรง / ค่าใช้จ่ายมาตรฐาน — เท่ากับ Dr WIP / Cr ปันเข้างาน แล้ว Dr สินค้า / Cr WIP
//     รวมเป็นรายการเดียว (งานระหว่างผลิตของใบสั่งผลิตที่ปิดแล้วเป็นศูนย์)
//   - ยกเลิก: ยกเลิกเอกสารสต็อก + กลับรายการบัญชีทุกการเคลื่อนไหว (ล่าสุดก่อน) ในวันที่เดิมของแต่ละรายการ
type OrderUseCases struct {
	base
	orders repositories.OrderRepository
	boms   repositories.BomRepository
}

// List: 1 query (หน้า + จำนวนทั้งหมด, JOIN สูตรของ module นี้) + สินค้า 1 + คลัง 1
func (uc *OrderUseCases) List(ctx context.Context, a dto.Actor, f dto.OrderFilter) (listing.Result[dto.OrderRow], error) {
	return read(ctx, uc.base, a, vo.PermOrderRead, func() (listing.Result[dto.OrderRow], error) {
		res, err := uc.orders.List(ctx, a.CompanyID, f)
		if err != nil {
			return res, err
		}
		items := make([]*dto.Ref, len(res.Items))
		warehouses := make([]*dto.Ref, len(res.Items))
		for i := range res.Items {
			items[i], warehouses[i] = &res.Items[i].Item, &res.Items[i].Warehouse
		}
		if err := uc.itemRefs(ctx, a.CompanyID, items); err != nil {
			return res, err
		}
		return res, uc.warehouseRefs(ctx, a.CompanyID, warehouses)
	})
}

func (uc *OrderUseCases) Get(ctx context.Context, a dto.Actor, id vo.OrderID) (dto.Order, error) {
	return read(ctx, uc.base, a, vo.PermOrderRead, func() (dto.Order, error) {
		o, err := uc.orders.FindByID(ctx, a.CompanyID, id, false)
		if err != nil {
			return dto.Order{}, err
		}
		s, err := uc.org.CompanySettings(ctx, a.CompanyID)
		if err != nil {
			return dto.Order{}, err
		}
		return uc.view(ctx, o, s.AmountDecimals)
	})
}

// Create สร้างใบสั่งผลิต (draft) จากสูตร — ออกเลขที่ตามรูปแบบของบริษัททันที (ปีบัญชีของวันที่เอกสาร)
func (uc *OrderUseCases) Create(ctx context.Context, in dto.CreateOrderInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, dec entities.Decimals) (*entities.Order, error) {
		bom, err := uc.prepare(ctx, in.CompanyID, in.Data, nil)
		if err != nil {
			return nil, err
		}
		p, err := uc.period(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return nil, err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocProductionOrder, in.Data.Date, p.FiscalYear)
		if err != nil {
			return nil, err
		}
		o, err := entities.NewOrder(in.CompanyID, no, in.Data, bom, in.UserID, uc.now(), dec)
		if err != nil {
			return nil, err
		}
		return o, uc.orders.Create(ctx, o)
	})
}

// Update แก้ใบสั่งผลิต draft — materials = nil: คำนวณปริมาณวัตถุดิบจากสูตรใหม่ (เลขที่คงเดิม)
func (uc *OrderUseCases) Update(ctx context.Context, in dto.UpdateOrderInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, dec entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		if o.Status() != vo.OrderDraft {
			return nil, domain.ErrOrderNotDraft
		}
		bom, err := uc.prepare(ctx, in.CompanyID, in.Data, in.Materials)
		if err != nil {
			return nil, err
		}
		if err := o.UpdateDraft(in.Data, bom, in.Materials, dec); err != nil {
			return nil, err
		}
		return o, uc.orders.Save(ctx, o)
	})
}

// prepare: สูตร (ต้องเป็นของสินค้านี้และใช้งานอยู่), สินค้า + วัตถุดิบเป็นชนิด stock ที่ใช้งานอยู่, คลังใช้งานอยู่
// query: สูตร 1 + สินค้า 1 + คลัง 1
func (uc *OrderUseCases) prepare(ctx context.Context, companyID vo.CompanyID, d entities.OrderData, materials []entities.MaterialInput) (entities.BomSnapshot, error) {
	b, err := uc.boms.FindByID(ctx, companyID, d.BomID)
	if err != nil {
		return entities.BomSnapshot{}, err
	}
	snap := entities.SnapshotOf(b)
	if snap.ItemID != d.ItemID {
		return snap, domain.ErrBomItemMismatch
	}
	if !snap.Active {
		return snap, domain.ErrBomInactive
	}
	ids := []vo.ItemID{d.ItemID}
	if materials == nil {
		ids = append(ids, b.ComponentItemIDs()...)
	} else {
		for _, m := range materials {
			ids = append(ids, m.ItemID)
		}
	}
	if _, err := uc.stockItems(ctx, companyID, ids); err != nil {
		return snap, err
	}
	return snap, uc.activeWarehouse(ctx, companyID, d.WarehouseID)
}

// Release ปล่อยผลิต — เริ่มเบิกวัตถุดิบได้
func (uc *OrderUseCases) Release(ctx context.Context, in dto.OrderAction) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, _ entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		if err := o.Release(); err != nil {
			return nil, err
		}
		return o, uc.orders.Save(ctx, o)
	})
}

// Issue เบิกวัตถุดิบ: ใบเบิกของ inventory (FIFO, ห้ามติดลบ) + รายการบัญชี (ระบบต่อเนื่อง) ใน transaction เดียว
func (uc *OrderUseCases) Issue(ctx context.Context, in dto.MovementInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, dec entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		if err := o.CheckLines(in.Date, in.Lines, dec); err != nil {
			return nil, err
		}
		system, items, settings, err := uc.context(ctx, in.CompanyID, in.Date, lineItems(in.Lines))
		if err != nil {
			return nil, err
		}
		stock := make([]ports.StockLine, len(in.Lines))
		for i, l := range in.Lines {
			stock[i] = ports.StockLine{ItemID: l.ItemID, Quantity: l.Quantity}
		}
		desc := fmt.Sprintf("เบิกวัตถุดิบ ใบสั่งผลิต %s", o.DocumentNo())
		doc, err := uc.inventory.PostIssue(ctx, in.CompanyID, in.UserID, o.Data().WarehouseID, in.Date, o.DocumentNo(), desc, stock)
		if err != nil {
			return nil, err
		}
		mv := movement(vo.MoveIssue, in.Date, doc, sum(doc.LineValues))
		if system == vo.Perpetual && mv.Value.IsPositive() {
			if err := settings.RequirePosting(false); err != nil {
				return nil, err
			}
			lines := []ports.JournalLine{{AccountID: *settings.WipAccountID, Debit: mv.Value}}
			credits, err := inventoryLines(in.Lines, doc.LineValues, items, false)
			if err != nil {
				return nil, err
			}
			if err := uc.post(ctx, in.Actor, *settings.JournalID, in.Date, desc, o.DocumentNo(), append(lines, credits...), &mv); err != nil {
				return nil, err
			}
		}
		o.ApplyIssue(in.Lines, doc.LineValues, mv)
		return o, uc.orders.Save(ctx, o)
	})
}

// Return คืนวัตถุดิบเข้าคลังที่ต้นทุนเฉลี่ยของที่เบิกสุทธิในใบสั่งผลิตนี้ (คืนได้ไม่เกินที่เบิกสุทธิ)
func (uc *OrderUseCases) Return(ctx context.Context, in dto.MovementInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, dec entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		if err := o.CheckLines(in.Date, in.Lines, dec); err != nil {
			return nil, err
		}
		values, err := o.ReturnValues(in.Lines, dec.Amount)
		if err != nil {
			return nil, err
		}
		system, items, settings, err := uc.context(ctx, in.CompanyID, in.Date, lineItems(in.Lines))
		if err != nil {
			return nil, err
		}
		var stock []ports.StockLine
		for i, l := range in.Lines {
			for _, s := range services.ReceiptLines(l.Quantity, values[i], dec.Amount) {
				stock = append(stock, ports.StockLine{ItemID: l.ItemID, Quantity: s.Quantity, UnitCost: s.UnitCost})
			}
		}
		desc := fmt.Sprintf("คืนวัตถุดิบ ใบสั่งผลิต %s", o.DocumentNo())
		doc, err := uc.inventory.PostReceipt(ctx, in.CompanyID, in.UserID, o.Data().WarehouseID, in.Date, o.DocumentNo(), desc, stock)
		if err != nil {
			return nil, err
		}
		mv := movement(vo.MoveReturn, in.Date, doc, sum(values))
		if system == vo.Perpetual && mv.Value.IsPositive() {
			if err := settings.RequirePosting(false); err != nil {
				return nil, err
			}
			debits, err := inventoryLines(in.Lines, values, items, true)
			if err != nil {
				return nil, err
			}
			lines := append(debits, ports.JournalLine{AccountID: *settings.WipAccountID, Credit: mv.Value})
			if err := uc.post(ctx, in.Actor, *settings.JournalID, in.Date, desc, o.DocumentNo(), lines, &mv); err != nil {
				return nil, err
			}
		}
		o.ApplyReturn(in.Lines, values, mv)
		return o, uc.orders.Save(ctx, o)
	})
}

// Complete ปิดงาน: รับสินค้าสำเร็จรูปเข้าคลังที่ต้นทุนจริง (วัตถุดิบสุทธิ + ค่าแรง / ค่าใช้จ่ายมาตรฐานตามจำนวนที่ผลิตได้)
// มูลค่าในคลังเท่ากับต้นทุนรวมทุกสตางค์ (services.ReceiptLines)
func (uc *OrderUseCases) Complete(ctx context.Context, in dto.CompleteInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, dec entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		plan, err := o.PlanCompletion(in.Date, in.Quantity, dec)
		if err != nil {
			return nil, err
		}
		product := o.Data().ItemID
		system, items, settings, err := uc.context(ctx, in.CompanyID, in.Date, []vo.ItemID{product})
		if err != nil {
			return nil, err
		}
		var stock []ports.StockLine
		for _, s := range services.ReceiptLines(plan.Quantity, plan.Total, dec.Amount) {
			stock = append(stock, ports.StockLine{ItemID: product, Quantity: s.Quantity, UnitCost: s.UnitCost})
		}
		desc := fmt.Sprintf("รับสินค้าสำเร็จรูป ใบสั่งผลิต %s", o.DocumentNo())
		doc, err := uc.inventory.PostReceipt(ctx, in.CompanyID, in.UserID, o.Data().WarehouseID, in.Date, o.DocumentNo(), desc, stock)
		if err != nil {
			return nil, err
		}
		mv := movement(vo.MoveReceipt, in.Date, doc, plan.Total)
		if system == vo.Perpetual && plan.Total.IsPositive() {
			lines, err := completionLines(plan, items[product], settings)
			if err != nil {
				return nil, err
			}
			if err := uc.post(ctx, in.Actor, *settings.JournalID, in.Date, desc, o.DocumentNo(), lines, &mv); err != nil {
				return nil, err
			}
		}
		o.Complete(in.Date, plan, mv)
		return o, uc.orders.Save(ctx, o)
	})
}

// Cancel ยกเลิกใบสั่งผลิต: ยกเลิกเอกสารสต็อกและกลับรายการบัญชีทุกการเคลื่อนไหว (ล่าสุดก่อน) — all-or-nothing
// ใบที่ปิดงานแล้วยกเลิกได้เฉพาะเมื่อสินค้าสำเร็จรูปที่รับเข้ายังไม่ถูกใช้ (inventory ปฏิเสธถ้าถูกตัดไปแล้ว)
func (uc *OrderUseCases) Cancel(ctx context.Context, in dto.CancelInput) (dto.Order, error) {
	return uc.mutate(ctx, in.Actor, func(ctx context.Context, _ entities.Decimals) (*entities.Order, error) {
		o, err := uc.orders.FindByID(ctx, in.CompanyID, in.ID, true)
		if err != nil {
			return nil, err
		}
		reason, err := o.CheckCancel(in.Reason)
		if err != nil {
			return nil, err
		}
		desc := fmt.Sprintf("ยกเลิกใบสั่งผลิต %s", o.DocumentNo())
		for _, mv := range o.ActiveMovements() {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, mv.StockDocumentID); err != nil {
				return nil, err
			}
			if mv.EntryID != nil {
				if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *mv.EntryID, mv.Date, desc); err != nil {
					return nil, err
				}
			}
		}
		o.Cancel(reason, uc.now())
		return o, uc.orders.Save(ctx, o)
	})
}

// mutate: เขียนใน transaction แล้วคืนมุมมองของใบสั่งผลิต (อ่านชื่อสินค้า / คลัง / สูตรหลัง commit)
func (uc *OrderUseCases) mutate(ctx context.Context, a dto.Actor, fn func(ctx context.Context, dec entities.Decimals) (*entities.Order, error)) (dto.Order, error) {
	var o *entities.Order
	var amountDecimals int
	err := uc.writeWith(ctx, a, vo.PermOrderManage, func(ctx context.Context, dec entities.Decimals) error {
		var err error
		amountDecimals = dec.Amount
		o, err = fn(ctx, dec)
		return err
	})
	if err != nil {
		return dto.Order{}, err
	}
	return uc.view(ctx, o, amountDecimals)
}

// context: ระบบบัญชีของปีบัญชี + สินค้าพร้อมบัญชี (lock ค่าตั้งของ inventory) + ค่าตั้งการผลิต
// ตรวจงวดเปิดก่อน (inventory / ledger ตรวจซ้ำตอนลงเอกสาร)
func (uc *OrderUseCases) context(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, map[vo.ItemID]ports.Item, entities.Settings, error) {
	if err := uc.openPeriod(ctx, companyID, date); err != nil {
		return "", nil, entities.Settings{}, err
	}
	system, items, err := uc.inventory.Context(ctx, companyID, date, ids)
	if err != nil {
		return "", nil, entities.Settings{}, err
	}
	s, err := uc.settings.Get(ctx, companyID)
	return system, items, s, err
}

func (uc *OrderUseCases) post(ctx context.Context, a dto.Actor, journal vo.JournalID, date time.Time, desc, ref string, lines []ports.JournalLine, mv *entities.Movement) error {
	id, no, err := uc.ledger.Post(ctx, ports.LedgerEntry{CompanyID: a.CompanyID, UserID: a.UserID, JournalID: journal, Date: date,
		Description: desc, Reference: ref, Lines: lines})
	if err != nil {
		return err
	}
	mv.EntryID, mv.EntryNo = &id, no
	return nil
}

// inventoryLines: บรรทัดบัญชีสินค้าคงเหลือของวัตถุดิบ รวมตามบัญชี (debit = คืน, credit = เบิก)
func inventoryLines(lines []entities.MovementLine, values []vo.Money, items map[vo.ItemID]ports.Item, debit bool) ([]ports.JournalLine, error) {
	var order []vo.AccountID
	totals := map[vo.AccountID]vo.Money{}
	for i, l := range lines {
		item := items[l.ItemID]
		if item.InventoryAccountID == nil {
			return nil, fmt.Errorf("%w: %s", domain.ErrAccountMissing, item.Code)
		}
		acc := *item.InventoryAccountID
		if _, ok := totals[acc]; !ok {
			order = append(order, acc)
			totals[acc] = decimal.Zero
		}
		totals[acc] = totals[acc].Add(values[i])
	}
	var out []ports.JournalLine
	for _, acc := range order {
		if v := totals[acc]; v.IsPositive() {
			if debit {
				out = append(out, ports.JournalLine{AccountID: acc, Debit: v})
			} else {
				out = append(out, ports.JournalLine{AccountID: acc, Credit: v})
			}
		}
	}
	return out, nil
}

// completionLines: Dr สินค้าสำเร็จรูป (รวม) / Cr งานระหว่างผลิต (วัตถุดิบสุทธิ) / Cr บัญชีปันเข้างาน (ตามบรรทัด รวมตามบัญชี)
func completionLines(p entities.CompletionPlan, product ports.Item, s entities.Settings) ([]ports.JournalLine, error) {
	needConversion := false
	for _, c := range p.Conversions {
		if c.AccountID == nil {
			needConversion = true
		}
	}
	if err := s.RequirePosting(needConversion); err != nil {
		return nil, err
	}
	if product.InventoryAccountID == nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrAccountMissing, product.Code)
	}
	lines := []ports.JournalLine{{AccountID: *product.InventoryAccountID, Debit: p.Total}}
	if p.Materials.IsPositive() {
		lines = append(lines, ports.JournalLine{AccountID: *s.WipAccountID, Credit: p.Materials})
	}
	var order []vo.AccountID
	totals := map[vo.AccountID]vo.Money{}
	for _, c := range p.Conversions {
		acc := s.ConversionAccountID
		if c.AccountID != nil {
			acc = c.AccountID
		}
		if _, ok := totals[*acc]; !ok {
			order = append(order, *acc)
			totals[*acc] = decimal.Zero
		}
		totals[*acc] = totals[*acc].Add(c.Amount)
	}
	for _, acc := range order {
		lines = append(lines, ports.JournalLine{AccountID: acc, Credit: totals[acc]})
	}
	return lines, nil
}

func movement(kind vo.MovementKind, date time.Time, doc ports.StockDocument, value vo.Money) entities.Movement {
	return entities.Movement{ID: vo.NewID[vo.MovementID](), Kind: kind, Date: date, StockDocumentID: doc.DocumentID,
		StockDocumentNo: doc.DocumentNo, Value: value}
}

func lineItems(lines []entities.MovementLine) []vo.ItemID {
	out := make([]vo.ItemID, len(lines))
	for i, l := range lines {
		out[i] = l.ItemID
	}
	return out
}

func sum(values []vo.Money) vo.Money {
	total := decimal.Zero
	for _, v := range values {
		total = total.Add(v)
	}
	return total
}

// warehouseRefs เติมรหัส / ชื่อคลังจาก inventory — query เดียว
func (uc *OrderUseCases) warehouseRefs(ctx context.Context, companyID vo.CompanyID, refs []*dto.Ref) error {
	var ids []vo.WarehouseID
	seen := map[vo.WarehouseID]bool{}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.WarehouseID](r.ID); err == nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	found, err := uc.inventory.Warehouses(ctx, companyID, ids)
	if err != nil {
		return err
	}
	for _, r := range refs {
		if id, err := vo.ParseID[vo.WarehouseID](r.ID); err == nil {
			if w, ok := found[id]; ok {
				r.Code, r.Name = w.Code, w.Name
			}
		}
	}
	return nil
}

// view: ใบสั่งผลิต + ชื่อสินค้า 1 + คลัง 1 + สูตร 1 query
func (uc *OrderUseCases) view(ctx context.Context, o *entities.Order, amountDecimals int) (dto.Order, error) {
	st := o.State()
	d := st.Data
	materials, conversion, total := o.Costs()
	out := dto.Order{
		OrderRow: dto.OrderRow{ID: vo.IDString(st.ID), DocumentNo: st.DocumentNo, Date: d.Date, Item: ref(d.ItemID), Bom: ref(d.BomID),
			Quantity: d.Quantity, Warehouse: ref(d.WarehouseID), PlannedStart: d.PlannedStart, PlannedEnd: d.PlannedEnd,
			Status: st.Status, CompletedQuantity: st.CompletedQuantity, CompletedAt: st.CompletedAt, TotalCost: total, CreatedAt: st.CreatedAt},
		Description:    d.Description,
		Materials:      make([]dto.OrderMaterial, len(st.Materials)),
		ConversionCost: o.PlannedConversion(amountDecimals),
		Movements:      make([]dto.OrderMovement, len(st.Movements)),
		Costs:          dto.OrderCosts{Materials: materials, Conversion: conversion, Total: total, UnitCost: o.UnitCost()},
		CancelledAt:    st.CancelledAt,
		CancelReason:   st.CancelReason,
		CreatedBy:      vo.IDString(st.CreatedBy),
	}
	refs := []*dto.Ref{&out.Item}
	for i, m := range st.Materials {
		out.Materials[i] = dto.OrderMaterial{LineNo: m.LineNo, Item: ref(m.ItemID), Required: m.Required, Issued: m.Issued,
			Returned: m.Returned, Net: m.NetQuantity(), Cost: m.NetCost()}
	}
	for i := range out.Materials {
		refs = append(refs, &out.Materials[i].Item)
	}
	for i, m := range st.Movements {
		out.Movements[i] = dto.OrderMovement{ID: vo.IDString(m.ID), Kind: m.Kind, Date: m.Date, DocumentID: vo.IDString(m.StockDocumentID),
			DocumentNo: m.StockDocumentNo, Value: m.Value, EntryNo: m.EntryNo, Cancelled: m.Cancelled}
	}
	if err := uc.itemRefs(ctx, st.CompanyID, refs); err != nil {
		return dto.Order{}, err
	}
	if err := uc.warehouseRefs(ctx, st.CompanyID, []*dto.Ref{&out.Warehouse}); err != nil {
		return dto.Order{}, err
	}
	boms, err := uc.boms.Refs(ctx, st.CompanyID, []vo.BomID{d.BomID})
	if err != nil {
		return dto.Order{}, err
	}
	if r, ok := boms[d.BomID]; ok {
		out.Bom = r
	}
	return out, nil
}
