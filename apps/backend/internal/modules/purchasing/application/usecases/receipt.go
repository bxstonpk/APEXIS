package usecases

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ReceiptUseCases: ใบรับสินค้าตามใบสั่งซื้อ / ใบตรวจรับงานตามใบสั่งจ้าง (รับบางส่วน / ตามงวดได้)
//
// ใบตรวจรับงานไม่ลงบัญชีและไม่มีสต็อก — บันทึกเฉพาะจำนวนที่ตรวจรับต่อบรรทัดใบสั่งจ้าง (ซื้อบริการได้ไม่เกินที่ตรวจรับ)
// ค่าใช้จ่ายรับรู้ที่ซื้อบริการ ซึ่งมีใบแจ้งหนี้ของผู้ขายเป็นหลักฐาน (ไม่ตั้งค้างจ่ายสองครั้ง)
type ReceiptUseCases struct {
	base
	receipts repositories.ReceiptRepository
	orders   repositories.OrderRepository
}

func (uc *ReceiptUseCases) Get(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	kind := kindOf(in.Kind)
	return read(ctx, uc.base, in.Actor, kind.Perms().ReceiptRead, func() (dto.Receipt, error) {
		out, err := uc.get(ctx, in.CompanyID, in.ID)
		if err == nil && out.Kind != string(kind) {
			return dto.Receipt{}, domain.ErrReceiptNotFound
		}
		return out, err
	})
}

func (uc *ReceiptUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.ReceiptSummary], error) {
	in.Filter.Kind = kindOf(in.Filter.Kind)
	return read(ctx, uc.base, in.Actor, in.Filter.Kind.Perms().ReceiptRead, func() (listing.Result[dto.ReceiptSummary], error) {
		return uc.receipts.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create รับสินค้า / ตรวจรับงานใน transaction เดียว
//
//	สินค้า: lock ใบสั่งซื้อ -> ตรวจจำนวน -> งวดต้องเปิด -> ระบบบัญชีของปี + บัญชีของสินค้า (inventory)
//	        -> ใบรับเข้าคลังสำหรับสินค้า stock (FIFO) -> Dr สินค้าคงเหลือ / ซื้อ  Cr GR/IR (ledger) -> บันทึก
//	บริการ: lock ใบสั่งจ้าง -> ตรวจจำนวน -> งวดต้องเปิด -> บันทึกจำนวนที่ตรวจรับ (ไม่มีบัญชี / สต็อก)
func (uc *ReceiptUseCases) Create(ctx context.Context, in dto.Create[dto.ReceiptInput]) (dto.Receipt, error) {
	kind := kindOf(in.Data.Kind)
	var id vo.ReceiptID
	err := uc.writeWith(ctx, in.Actor, kind.Perms().ReceiptPost, func(ctx context.Context, s ports.CompanySettings) error {
		r, err := uc.create(ctx, in.Actor, s, in.Data)
		if err != nil {
			return err
		}
		id = r.ID()
		return nil
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// create: ภายใน transaction ที่ตรวจสิทธิ์แล้ว (ใช้ร่วมกับซื้อตรงไม่ผ่านใบสั่ง)
func (uc *ReceiptUseCases) create(ctx context.Context, a dto.Actor, s ports.CompanySettings, in dto.ReceiptInput) (*entities.Receipt, error) {
	kind := kindOf(in.Kind)
	settings, err := uc.settings.Get(ctx, a.CompanyID)
	if err != nil {
		return nil, err
	}
	if kind == vo.KindGoods {
		if err := settings.Require("journal_id", "grir_account_id"); err != nil {
			return nil, err
		}
	}
	o, err := uc.orders.FindForUpdate(ctx, a.CompanyID, in.OrderID)
	if err != nil {
		return nil, err
	}
	if o.Kind() != kind {
		return nil, domain.ErrOrderNotFound
	}
	deliveries := make([]entities.Delivery, len(in.Lines))
	for i, l := range in.Lines {
		deliveries[i] = entities.Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity}
	}
	if err := o.CheckReceive(deliveries, s.QuantityDecimals); err != nil {
		return nil, err
	}
	if !vo.IsDateOnly(in.Date) {
		return nil, domain.ErrInvalidDate
	}
	period, err := uc.openPeriod(ctx, a.CompanyID, in.Date)
	if err != nil {
		return nil, err
	}
	ids := make([]vo.ItemID, len(deliveries))
	for i, d := range deliveries {
		l, _, _ := o.Line(d.LineNo)
		ids[i] = l.ItemID
	}
	no, err := uc.number(ctx, a.CompanyID, kind.ReceiptDoc(), in.Date, period.FiscalYear)
	if err != nil {
		return nil, err
	}
	poNo := o.State().Approval.OrderNo
	state := entities.ReceiptState{
		CompanyID: a.CompanyID, Kind: kind, OrderID: o.ID(), SupplierID: o.Data().SupplierID, Date: in.Date,
		Reference: in.Reference, Description: in.Description, System: vo.Periodic,
		Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: a.UserID, At: uc.now()},
	}

	if kind == vo.KindService {
		found, err := uc.inventory.Items(ctx, a.CompanyID, ids)
		if err != nil {
			return nil, err
		}
		if err := items(found, ids, false); err != nil {
			return nil, err
		}
		state.Lines = make([]entities.ReceiptLine, len(deliveries))
		for i, d := range deliveries {
			ol, _, _ := o.Line(d.LineNo)
			state.Lines[i] = entities.ReceiptLine{
				OrderLineNo: d.LineNo, ItemID: ol.ItemID, Quantity: d.Quantity,
				UnitCost: services.UnitCost(ol.UnitPrice, o.Terms().Vat), Value: decimal.Zero,
			}
			deliveries[i].Value = decimal.Zero
		}
		if err := o.Receive(deliveries, s.QuantityDecimals, uc.now()); err != nil {
			return nil, err
		}
		r, err := entities.NewReceipt(state)
		if err != nil {
			return nil, err
		}
		if err := uc.receipts.Create(ctx, r); err != nil {
			return nil, err
		}
		return r, uc.orders.SaveProgress(ctx, o)
	}

	system, found, err := uc.inventory.PurchaseContext(ctx, a.CompanyID, in.Date, ids)
	if err != nil {
		return nil, err
	}
	if err := items(found, ids, true); err != nil {
		return nil, err
	}
	state.System = system

	// บรรทัด: มูลค่าก่อน VAT + บัญชีเดบิตตามระบบบัญชี
	vat := o.Terms().Vat
	lines := make([]entities.ReceiptLine, len(deliveries))
	var stock []ports.StockLine
	var stockIdx []int
	for i, d := range deliveries {
		ol, _, _ := o.Line(d.LineNo)
		item := found[ol.ItemID]
		acc, what := item.PurchaseAccountID, "purchase"
		if system == vo.Perpetual && item.Stock {
			acc, what = item.InventoryAccountID, "inventory"
		}
		if acc == nil {
			return nil, fmt.Errorf("%w: %s account of item %s", domain.ErrSettingsMissing, what, item.Code)
		}
		cost := services.UnitCost(ol.UnitPrice, vat)
		lines[i] = entities.ReceiptLine{
			OrderLineNo: d.LineNo, ItemID: ol.ItemID, Stock: item.Stock, Quantity: d.Quantity, UnitCost: cost,
			Value: services.ReceiptValue(d.Quantity, ol.UnitPrice, vat, s.AmountDecimals), AccountID: *acc,
		}
		if item.Stock {
			stock = append(stock, ports.StockLine{ItemID: ol.ItemID, Quantity: d.Quantity, UnitCost: cost})
			stockIdx = append(stockIdx, i)
		}
	}
	// สินค้า stock: ใบรับเข้าของ inventory — ใช้มูลค่าที่ inventory บันทึกเข้าชั้นต้นทุนจริง
	if len(stock) > 0 {
		if in.WarehouseID == nil {
			return nil, domain.ErrWarehouseRequired
		}
		sr, err := uc.inventory.PostReceipt(ctx, a.CompanyID, a.UserID, *in.WarehouseID, in.Date, no,
			"Goods receipt "+no+" / "+poNo, stock)
		if err != nil {
			return nil, err
		}
		for k, i := range stockIdx {
			lines[i].Value = sr.LineValues[k]
		}
		state.WarehouseID, state.StockDocumentID, state.StockDocumentNo = in.WarehouseID, &sr.DocumentID, sr.DocumentNo
	}
	state.Lines = lines
	for i := range deliveries {
		deliveries[i].Value = lines[i].Value
	}
	if err := o.Receive(deliveries, s.QuantityDecimals, uc.now()); err != nil {
		return nil, err
	}
	r, err := entities.NewReceipt(state)
	if err != nil {
		return nil, err
	}
	if err := uc.post(ctx, a, r, *settings.JournalID, *settings.GRIRAccountID, poNo); err != nil {
		return nil, err
	}
	if err := uc.receipts.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, uc.orders.SaveProgress(ctx, o)
}

func (uc *ReceiptUseCases) post(ctx context.Context, a dto.Actor, r *entities.Receipt, journal vo.JournalID, grir vo.AccountID, poNo string) error {
	lines := r.Journal(grir)
	if len(lines) == 0 {
		return nil // มูลค่าเป็นศูนย์ (เช่น ของแถม) — ไม่มีรายการบัญชี
	}
	s := r.State()
	entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
		CompanyID: a.CompanyID, UserID: a.UserID, JournalID: journal, Date: s.Date,
		Description: "Goods receipt " + s.Posting.DocumentNo + " / " + poNo, Reference: s.Posting.DocumentNo, Lines: lines,
	})
	if err != nil {
		return err
	}
	r.SetEntry(entryID, entryNo)
	return nil
}

// Cancel ยกเลิกใบรับ / ใบตรวจรับงาน: กลับจำนวนในใบสั่ง (ต้องยังไม่แจ้งหนี้ส่วนนั้น) -> ยกเลิกใบรับเข้าคลัง
// (ของต้องยังไม่ถูกใช้) -> กลับรายการบัญชี ณ วันที่เดิม — งวดต้องยังเปิด
func (uc *ReceiptUseCases) Cancel(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	kind := kindOf(in.Kind)
	err := uc.write(ctx, in.Actor, kind.Perms().ReceiptPost, func(ctx context.Context) error {
		r, err := uc.receipts.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if kindOf(r.State().Kind) != kind {
			return domain.ErrReceiptNotFound
		}
		if err := r.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := r.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, s.OrderID)
		if err != nil {
			return err
		}
		if err := o.Unreceive(r.Deliveries()); err != nil {
			return err
		}
		if s.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.StockDocumentID); err != nil {
				return err
			}
		}
		if s.Posting.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *s.Posting.EntryID, s.Date, "Cancel goods receipt "+s.Posting.DocumentNo); err != nil {
				return err
			}
		}
		if err := uc.receipts.UpdateStatus(ctx, r); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *ReceiptUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.ReceiptID) (dto.Receipt, error) {
	out, err := uc.receipts.Get(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	refs := make([]*dto.Ref, len(out.Lines))
	for i := range out.Lines {
		refs[i] = &out.Lines[i].Item
	}
	return out, uc.itemRefs(ctx, companyID, refs)
}
