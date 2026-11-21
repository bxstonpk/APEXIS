package usecases

import (
	"context"
	"fmt"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	"apexis/backend/internal/modules/inventory/domain/services"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numbering"
)

// DocumentUseCases จัดการเอกสารเคลื่อนไหวสต็อก: draft -> posted -> cancelled
type DocumentUseCases struct {
	base
	docs       repositories.DocumentRepository
	items      repositories.ItemRepository
	warehouses repositories.WarehouseRepository
	stock      repositories.StockRepository
	closings   repositories.ClosingRepository
}

func (uc *DocumentUseCases) Get(ctx context.Context, in dto.ByID[vo.DocumentID]) (dto.Document, error) {
	return read(ctx, uc.base, in.Actor, vo.PermDocumentRead, func() (dto.Document, error) {
		return uc.docs.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *DocumentUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.DocumentSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermDocumentRead, func() (listing.Result[dto.DocumentSummary], error) {
		return uc.docs.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create สร้าง draft — ถ้า in.Post ผ่านเอกสารต่อทันทีใน transaction เดียว (ต้องมีสิทธิ์ post ด้วย)
func (uc *DocumentUseCases) Create(ctx context.Context, in dto.CreateDocument[entities.DocumentData]) (dto.Document, error) {
	var d *entities.Document
	err := uc.writeWith(ctx, in.Actor, vo.PermDocumentManage, func(ctx context.Context, s ports.CompanySettings) error {
		var err error
		if d, err = entities.NewDocument(in.CompanyID, in.UserID, in.Data, s.QuantityDecimals); err != nil {
			return err
		}
		if err := uc.checkRefs(ctx, in.CompanyID, d, in.Post); err != nil {
			return err
		}
		if err := uc.docs.Create(ctx, d); err != nil {
			return err
		}
		if !in.Post {
			return nil
		}
		if err := uc.authorize(ctx, in.Actor, vo.PermDocumentPost); err != nil {
			return err
		}
		return uc.post(ctx, in.Actor, s, d)
	})
	if err != nil {
		return dto.Document{}, err
	}
	return uc.docs.Get(ctx, in.CompanyID, d.ID())
}

// Update แทนที่ draft ทั้งก้อน
func (uc *DocumentUseCases) Update(ctx context.Context, in dto.Update[vo.DocumentID, entities.DocumentData]) (dto.Document, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermDocumentManage, func(ctx context.Context, s ports.CompanySettings) error {
		d, err := uc.docs.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := d.Update(in.Data, s.QuantityDecimals); err != nil {
			return err
		}
		if err := uc.checkRefs(ctx, in.CompanyID, d, false); err != nil {
			return err
		}
		return uc.docs.Update(ctx, d)
	})
	if err != nil {
		return dto.Document{}, err
	}
	return uc.docs.Get(ctx, in.CompanyID, in.ID)
}

// Delete ลบ draft (ลบจริง — ยังไม่มีผลต่อสต็อก)
func (uc *DocumentUseCases) Delete(ctx context.Context, in dto.ByID[vo.DocumentID]) error {
	return uc.write(ctx, in.Actor, vo.PermDocumentManage, func(ctx context.Context) error {
		d, err := uc.docs.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := d.EnsureDraft(); err != nil {
			return err
		}
		return uc.docs.Delete(ctx, in.CompanyID, in.ID)
	})
}

// Post ผ่าน draft: lock เอกสาร -> ตรวจสินค้า / คลัง -> งวดต้องเปิด -> lock ชั้นต้นทุน -> FIFO -> เขียนผล -> ออกเลขที่
func (uc *DocumentUseCases) Post(ctx context.Context, in dto.ByID[vo.DocumentID]) (dto.Document, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermDocumentPost, func(ctx context.Context, s ports.CompanySettings) error {
		d, err := uc.docs.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := d.EnsureDraft(); err != nil {
			return err
		}
		if err := uc.checkRefs(ctx, in.CompanyID, d, true); err != nil {
			return err
		}
		return uc.post(ctx, in.Actor, s, d)
	})
	if err != nil {
		return dto.Document{}, err
	}
	return uc.docs.Get(ctx, in.CompanyID, in.ID)
}

// Cancel ยกเลิกเอกสารที่ผ่านแล้ว: คืนชั้นที่ถูกตัด, ปิดชั้นที่เอกสารสร้าง (ต้องยังไม่ถูกใช้),
// บันทึกการเคลื่อนไหวกลับรายการที่วันที่เดิมของเอกสาร — งวดของวันที่นั้นต้องยังเปิด
func (uc *DocumentUseCases) Cancel(ctx context.Context, in dto.ByID[vo.DocumentID]) (dto.Document, error) {
	err := uc.write(ctx, in.Actor, vo.PermDocumentPost, func(ctx context.Context) error {
		return uc.cancel(ctx, in.Actor, in.ID)
	})
	if err != nil {
		return dto.Document{}, err
	}
	return uc.docs.Get(ctx, in.CompanyID, in.ID)
}

func (uc *DocumentUseCases) cancel(ctx context.Context, a dto.Actor, id vo.DocumentID) error {
	d, err := uc.docs.FindForUpdate(ctx, a.CompanyID, id)
	if err != nil {
		return err
	}
	if d.Status() != vo.DocPosted {
		return domain.ErrDocumentNotPosted
	}
	if _, err := uc.openPeriod(ctx, a.CompanyID, d); err != nil {
		return err
	}
	input, err := uc.stock.LoadForCancel(ctx, a.CompanyID, id)
	if err != nil {
		return err
	}
	plan, err := services.PlanCancel(input)
	if err != nil {
		return err
	}
	if err := uc.stock.ApplyCancel(ctx, a.CompanyID, plan); err != nil {
		return err
	}
	if err := d.Cancel(entities.Cancellation{By: a.UserID, At: uc.now()}); err != nil {
		return err
	}
	return uc.docs.UpdateStatus(ctx, d)
}

// post คำนวณ FIFO และเขียนผลทั้งหมด (เอกสารต้องถูกบันทึกแล้ว — การเคลื่อนไหวอ้างถึงเอกสาร)
func (uc *DocumentUseCases) post(ctx context.Context, a dto.Actor, s ports.CompanySettings, d *entities.Document) error {
	period, err := uc.openPeriod(ctx, a.CompanyID, d)
	if err != nil {
		return err
	}
	data := d.Data()
	var keys []services.StockKey
	seen := map[services.StockKey]bool{}
	for _, l := range data.Lines {
		k := services.StockKey{ItemID: l.ItemID, WarehouseID: data.WarehouseID}
		if !l.Inbound(data.Type) && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	var layers []*services.Layer
	if len(keys) > 0 {
		if layers, err = uc.stock.LockLayers(ctx, a.CompanyID, keys, data.Date); err != nil {
			return err
		}
	}
	plan, err := services.PlanPosting(d, layers, s.AmountDecimals)
	if err != nil {
		return err
	}
	if err := uc.stock.ApplyPosting(ctx, a.CompanyID, plan); err != nil {
		return err
	}
	format, err := uc.org.DocumentFormat(ctx, a.CompanyID, data.Type.NumberingKey())
	if err != nil {
		return err
	}
	nc := numbering.Context{Date: data.Date, FiscalYear: period.FiscalYear}
	n, err := uc.docs.NextNumber(ctx, a.CompanyID, data.Type, format.PeriodKey(nc))
	if err != nil {
		return err
	}
	err = d.Post(entities.Posting{
		DocumentNo: format.Render(nc, n),
		PeriodID:   period.ID, FiscalYear: period.FiscalYear, By: a.UserID, At: uc.now(),
	}, plan.LineValues)
	if err != nil {
		return err
	}
	return uc.docs.UpdateStatus(ctx, d)
}

func (uc *DocumentUseCases) openPeriod(ctx context.Context, companyID vo.CompanyID, d *entities.Document) (ports.Period, error) {
	p, err := uc.org.PeriodAt(ctx, companyID, d.Data().Date)
	if err != nil {
		return ports.Period{}, err
	}
	if !p.Open {
		return ports.Period{}, domain.ErrPeriodClosed
	}
	// ปรับปรุงสินค้าคงเหลือสิ้นงวด (periodic) ใช้มูลค่าสะสมถึงวันสิ้นงวด — เอกสารย้อนหลังจะทำให้ยอดนั้นผิด
	closed, err := uc.closings.PostedOnOrAfter(ctx, companyID, d.Data().Date)
	if err != nil {
		return ports.Period{}, err
	}
	if closed {
		return ports.Period{}, domain.ErrInventoryClosed
	}
	return p, nil
}

// checkRefs: คลังและสินค้าต้องมีอยู่, สินค้าต้องเป็นชนิด stock (forPosting: คลัง / สินค้าต้อง active ด้วย)
// — query ละครั้งไม่ว่าเอกสารจะมีกี่บรรทัด
func (uc *DocumentUseCases) checkRefs(ctx context.Context, companyID vo.CompanyID, d *entities.Document, forPosting bool) error {
	whIDs := d.WarehouseIDs()
	whs, err := uc.warehouses.FindByIDs(ctx, companyID, whIDs)
	if err != nil {
		return err
	}
	if len(whs) != len(whIDs) {
		return domain.ErrWarehouseNotFound
	}
	if forPosting {
		for _, w := range whs {
			if !w.IsActive() {
				return domain.ErrWarehouseInactive
			}
		}
	}
	items, err := uc.items.FindByIDs(ctx, companyID, d.ItemIDs())
	if err != nil {
		return err
	}
	byID := make(map[vo.ItemID]*entities.Item, len(items))
	for _, i := range items {
		byID[i.ID()] = i
	}
	for n, l := range d.Data().Lines {
		i, ok := byID[l.ItemID]
		switch {
		case !ok:
			return fmt.Errorf("%w: line %d", domain.ErrItemNotFound, n+1)
		case i.Data().Type != vo.ItemStock || (forPosting && !i.IsStockable()):
			return fmt.Errorf("%w: line %d item %s", domain.ErrItemNotStocked, n+1, i.Data().Code)
		}
	}
	return nil
}
