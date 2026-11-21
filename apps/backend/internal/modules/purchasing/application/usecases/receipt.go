package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ReceiptUseCases: ใบรับสินค้าตามใบสั่งซื้อ (รับบางส่วนได้)
type ReceiptUseCases struct {
	base
	receipts repositories.ReceiptRepository
	orders   repositories.OrderRepository
}

func (uc *ReceiptUseCases) Get(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReceiptRead, func() (dto.Receipt, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *ReceiptUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.ReceiptSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermReceiptRead, func() (listing.Result[dto.ReceiptSummary], error) {
		return uc.receipts.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create รับสินค้าใน transaction เดียว:
// lock ใบสั่งซื้อ -> ตรวจจำนวน -> งวดต้องเปิด -> ระบบบัญชีของปี + บัญชีของสินค้า (inventory)
// -> ใบรับเข้าคลังสำหรับสินค้า stock (FIFO) -> Dr สินค้าคงเหลือ / ซื้อ  Cr GR/IR (ledger) -> บันทึก
func (uc *ReceiptUseCases) Create(ctx context.Context, in dto.Create[dto.ReceiptInput]) (dto.Receipt, error) {
	var id vo.ReceiptID
	err := uc.writeWith(ctx, in.Actor, vo.PermReceiptPost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "grir_account_id"); err != nil {
			return err
		}
		o, err := uc.orders.FindForUpdate(ctx, in.CompanyID, in.Data.OrderID)
		if err != nil {
			return err
		}
		deliveries := make([]entities.Delivery, len(in.Data.Lines))
		for i, l := range in.Data.Lines {
			deliveries[i] = entities.Delivery{LineNo: l.OrderLineNo, Quantity: l.Quantity}
		}
		if err := o.CheckReceive(deliveries, s.QuantityDecimals); err != nil {
			return err
		}
		if !vo.IsDateOnly(in.Data.Date) {
			return domain.ErrInvalidDate
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		ids := make([]vo.ItemID, len(deliveries))
		for i, d := range deliveries {
			l, _, _ := o.Line(d.LineNo)
			ids[i] = l.ItemID
		}
		system, found, err := uc.inventory.PurchaseContext(ctx, in.CompanyID, in.Data.Date, ids)
		if err != nil {
			return err
		}
		if err := items(found, ids, true); err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocReceipt, in.Data.Date, period.FiscalYear)
		if err != nil {
			return err
		}

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
				return fmt.Errorf("%w: %s account of item %s", domain.ErrSettingsMissing, what, item.Code)
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
		state := entities.ReceiptState{
			CompanyID: in.CompanyID, OrderID: o.ID(), SupplierID: o.Data().SupplierID, Date: in.Data.Date,
			Reference: in.Data.Reference, Description: in.Data.Description, System: system, Lines: lines,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}
		// สินค้า stock: ใบรับเข้าของ inventory — ใช้มูลค่าที่ inventory บันทึกเข้าชั้นต้นทุนจริง
		if len(stock) > 0 {
			if in.Data.WarehouseID == nil {
				return domain.ErrWarehouseRequired
			}
			poNo := o.State().Approval.OrderNo
			sr, err := uc.inventory.PostReceipt(ctx, in.CompanyID, in.UserID, *in.Data.WarehouseID, in.Data.Date, no,
				"Goods receipt "+no+" / "+poNo, stock)
			if err != nil {
				return err
			}
			for k, i := range stockIdx {
				lines[i].Value = sr.LineValues[k]
			}
			state.WarehouseID, state.StockDocumentID, state.StockDocumentNo = in.Data.WarehouseID, &sr.DocumentID, sr.DocumentNo
		}
		for i := range deliveries {
			deliveries[i].Value = lines[i].Value
		}
		if err := o.Receive(deliveries, s.QuantityDecimals, uc.now()); err != nil {
			return err
		}
		r, err := entities.NewReceipt(state)
		if err != nil {
			return err
		}
		if err := uc.post(ctx, in.Actor, r, *settings.JournalID, *settings.GRIRAccountID, o.State().Approval.OrderNo); err != nil {
			return err
		}
		id = r.ID()
		if err := uc.receipts.Create(ctx, r); err != nil {
			return err
		}
		return uc.orders.SaveProgress(ctx, o)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
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

// Cancel ยกเลิกใบรับสินค้า: กลับจำนวนในใบสั่งซื้อ (ต้องยังไม่แจ้งหนี้ส่วนนั้น) -> ยกเลิกใบรับเข้าคลัง
// (ของต้องยังไม่ถูกใช้) -> กลับรายการบัญชี ณ วันที่เดิม — งวดต้องยังเปิด
func (uc *ReceiptUseCases) Cancel(ctx context.Context, in dto.ByID[vo.ReceiptID]) (dto.Receipt, error) {
	err := uc.write(ctx, in.Actor, vo.PermReceiptPost, func(ctx context.Context) error {
		r, err := uc.receipts.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
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
