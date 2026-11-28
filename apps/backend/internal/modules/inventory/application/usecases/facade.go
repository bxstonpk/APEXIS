package usecases

import (
	"context"
	"time"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// FacadeUseCases คือ operation ที่ module อื่น (จัดซื้อ / ขาย) เรียกผ่าน inventory.Module เท่านั้น — ไม่มี HTTP route
//
// ไม่ตรวจสิทธิ์ของ inventory (ผู้เรียกตรวจสิทธิ์ของเอกสารตัวเองแล้ว) แต่บังคับกฎของ inventory ครบ
// ทำงานใน transaction ของผู้เรียก (ctx)
type FacadeUseCases struct {
	base
	settings   repositories.SettingsRepository
	items      repositories.ItemRepository
	warehouses repositories.WarehouseRepository
	docs       *DocumentUseCases
}

// Warehouses คืนคลังตาม id (คลังที่ไม่พบจะไม่อยู่ในผลลัพธ์) — query เดียว
func (uc *FacadeUseCases) Warehouses(ctx context.Context, companyID vo.CompanyID, ids []vo.WarehouseID) ([]*entities.Warehouse, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return uc.warehouses.FindByIDs(ctx, companyID, ids)
}

// PurchaseItems คืนสินค้าพร้อมบัญชีที่ใช้จริง (ไม่ผูกกับปีบัญชี — ใช้ตรวจสินค้าตอนสร้าง / อนุมัติใบสั่งซื้อ)
func (uc *FacadeUseCases) PurchaseItems(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) ([]dto.PurchaseItem, error) {
	s, _, err := uc.settings.Get(ctx, companyID, repositories.NoLock)
	if err != nil {
		return nil, err
	}
	return uc.resolve(ctx, companyID, s, ids)
}

func (uc *FacadeUseCases) resolve(ctx context.Context, companyID vo.CompanyID, s entities.Settings, ids []vo.ItemID) ([]dto.PurchaseItem, error) {
	rows, err := uc.items.AccountsOf(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PurchaseItem, len(rows))
	for i, r := range rows {
		out[i] = dto.PurchaseItem{
			ID: r.ID, Code: r.Code, Name: r.Name, Uom: r.Uom, Stock: r.Type == vo.ItemStock, Service: r.Type == vo.ItemService, Active: r.Status == vo.StatusActive,
			Accounts: r.Item.Or(r.Category).Or(s.Defaults),
		}
	}
	return out, nil
}

// PurchaseContext คืนระบบบัญชีของปีบัญชีที่ date อยู่ และบัญชีที่ใช้จริงของสินค้า (สินค้าที่ไม่พบจะไม่อยู่ในผลลัพธ์)
// lock แถวค่าตั้งแบบ FOR SHARE จน commit — ระบบบัญชีเปลี่ยนระหว่างลงบัญชีซื้อไม่ได้
func (uc *FacadeUseCases) PurchaseContext(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, []dto.PurchaseItem, error) {
	var system vo.AccountingSystem
	var out []dto.PurchaseItem
	err := uc.systemWrite(ctx, companyID, func(ctx context.Context, _ ports.CompanySettings) error {
		s, systems, err := uc.settings.Get(ctx, companyID, repositories.ForShare)
		if err != nil {
			return err
		}
		period, err := uc.org.PeriodAt(ctx, companyID, date)
		if err != nil {
			return err
		}
		system = entities.SystemFor(systems, period.FiscalYear)
		out, err = uc.resolve(ctx, companyID, s, ids)
		return err
	})
	return system, out, err
}

// PostReceipt สร้างใบรับสินค้า (receipt) แล้วผ่านทันที — ต้นทุน FIFO เข้าชั้นตามราคาที่ส่งมา
func (uc *FacadeUseCases) PostReceipt(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, data entities.DocumentData) (dto.StockReceipt, error) {
	data.Type = vo.DocReceipt
	return uc.postDocument(ctx, companyID, userID, data)
}

// PostIssue สร้างใบเบิก (issue) แล้วผ่านทันที — ตัดชั้นต้นทุน FIFO, ห้ามสต็อกติดลบ; LineValues = ต้นทุนที่ตัด
func (uc *FacadeUseCases) PostIssue(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, data entities.DocumentData) (dto.StockReceipt, error) {
	data.Type = vo.DocIssue
	return uc.postDocument(ctx, companyID, userID, data)
}

func (uc *FacadeUseCases) postDocument(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, data entities.DocumentData) (dto.StockReceipt, error) {
	var out dto.StockReceipt
	err := uc.systemWrite(ctx, companyID, func(ctx context.Context, s ports.CompanySettings) error {
		d, err := entities.NewDocument(companyID, userID, data, s.QuantityDecimals)
		if err != nil {
			return err
		}
		if err := uc.docs.checkRefs(ctx, companyID, d, true); err != nil {
			return err
		}
		if err := uc.docs.docs.Create(ctx, d); err != nil {
			return err
		}
		if err := uc.docs.post(ctx, dto.Actor{UserID: userID, CompanyID: companyID}, s, d); err != nil {
			return err
		}
		st := d.State()
		out = dto.StockReceipt{DocumentID: st.ID, DocumentNo: st.Posting.DocumentNo, LineValues: make([]vo.Money, len(st.Data.Lines))}
		for i, l := range st.Data.Lines {
			out.LineValues[i] = l.Value
		}
		return nil
	})
	return out, err
}

// CancelDocument ยกเลิกเอกสารสต็อกที่ผ่านแล้ว (เช่น ยกเลิกใบรับสินค้าของจัดซื้อ)
func (uc *FacadeUseCases) CancelDocument(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.DocumentID) error {
	return uc.systemWrite(ctx, companyID, func(ctx context.Context, _ ports.CompanySettings) error {
		return uc.docs.cancel(ctx, dto.Actor{UserID: userID, CompanyID: companyID}, id)
	})
}
