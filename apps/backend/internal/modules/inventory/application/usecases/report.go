package usecases

import (
	"context"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// ReportUseCases คือรายงานสต็อก — คำนวณจากการเคลื่อนไหว (ผลรวมของการเคลื่อนไหว = ยอดคงเหลือตามชั้น FIFO)
type ReportUseCases struct {
	base
	stock      repositories.StockRepository
	items      repositories.ItemRepository
	warehouses repositories.WarehouseRepository
}

// StockOnHand ยอดคงเหลือ ณ วันที่ ต่อสินค้า × คลัง — 1 query (แบ่งหน้า + มูลค่ารวมด้วย window function)
func (uc *ReportUseCases) StockOnHand(ctx context.Context, in dto.List[dto.StockOnHandFilter]) (dto.StockOnHand, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.StockOnHand{}, err
	}
	rows, total, err := uc.stock.StockOnHand(ctx, in.CompanyID, in.Filter)
	if err != nil {
		return dto.StockOnHand{}, err
	}
	return dto.StockOnHand{AsOf: in.Filter.AsOf, Rows: rows, TotalValue: total}, nil
}

// StockCard การเคลื่อนไหวของสินค้า: ยอดยกมา + รายการพร้อมยอดสะสม (แบ่งหน้าใน DB)
func (uc *ReportUseCases) StockCard(ctx context.Context, in dto.List[dto.StockCardFilter]) (dto.StockCard, error) {
	if err := uc.authorize(ctx, in.Actor, vo.PermReportRead); err != nil {
		return dto.StockCard{}, err
	}
	f := in.Filter
	item, err := uc.items.Get(ctx, in.CompanyID, f.ItemID)
	if err != nil {
		return dto.StockCard{}, err
	}
	out := dto.StockCard{Item: dto.Ref{ID: item.ID, Code: item.Code, Name: item.Name}, UomCode: item.Uom.Code, Start: f.Start, End: f.End}
	if f.WarehouseID != nil {
		w, err := uc.warehouses.Get(ctx, in.CompanyID, *f.WarehouseID)
		if err != nil {
			return dto.StockCard{}, err
		}
		out.Warehouse = &dto.Ref{ID: w.ID, Code: w.Code, Name: w.Name}
	}
	if out.OpeningQuantity, out.OpeningValue, err = uc.stock.StockCardOpening(ctx, in.CompanyID, f); err != nil {
		return dto.StockCard{}, err
	}
	lines, total, err := uc.stock.StockCardLines(ctx, in.CompanyID, f)
	if err != nil {
		return dto.StockCard{}, err
	}
	for i := range lines {
		lines[i].BalanceQuantity = lines[i].BalanceQuantity.Add(out.OpeningQuantity)
		lines[i].BalanceValue = lines[i].BalanceValue.Add(out.OpeningValue)
	}
	out.Lines.Items, out.Lines.Total, out.Lines.Page = lines, total, f.Page
	return out, nil
}
