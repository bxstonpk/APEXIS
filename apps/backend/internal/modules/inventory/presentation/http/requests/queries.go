package requests

import (
	"fmt"
	"net/url"
	"time"

	"apexis/backend/internal/modules/inventory/application/dto"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// การค้นหา / รายงานทุกหน้าใช้ GET + URL query เท่านั้น — frontend เก็บ query เดียวกันไว้ใน URL ของหน้า
// วันที่ YYYY-MM-DD (ค.ศ.); parameter ที่ไม่รู้จักถูกเพิกเฉย, ค่าที่ไม่ถูกต้องของ parameter ที่รู้จัก -> 400

type query struct {
	v   url.Values
	err error
}

func parse[T any](q *query, fn func() (T, error)) T {
	var zero T
	if q.err != nil {
		return zero
	}
	v, err := fn()
	if err != nil {
		q.err = err
		return zero
	}
	return v
}

func (q *query) search() string {
	return parse(q, func() (string, error) { return listing.ParseSearch(q.v) })
}

func (q *query) sort(keys []string, def []listing.Sort) []listing.Sort {
	return parse(q, func() ([]listing.Sort, error) { return listing.ParseSort(q.v, keys, def) })
}

func (q *query) page() listing.Page {
	return parse(q, func() (listing.Page, error) { return listing.ParsePage(q.v) })
}

func (q *query) statuses() []vo.Status {
	return parse(q, func() ([]vo.Status, error) { return listing.ParseList(q.v, "status", vo.ParseStatus) })
}

func optionalIDParam[T ~[16]byte](q *query, key string) *T {
	return parse(q, func() (*T, error) { return listing.ParseOptional(q.v, key, vo.ParseID[T]) })
}

func (q *query) requiredDate(key string) time.Time {
	return parse(q, func() (time.Time, error) {
		raw := q.v.Get(key)
		if raw == "" {
			return time.Time{}, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, key)
		}
		t, err := listing.ParseDate(raw)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %s has an invalid value %q", listing.ErrInvalidQuery, key, raw)
		}
		return t, nil
	})
}

// GET /uoms, /item-categories, /warehouses ?q=&status=&branch_id= (เฉพาะ warehouses)&sort=code|name&page=&page_size=
func ParseMasterFilter(v url.Values, withBranch bool) (dto.MasterFilter, error) {
	q := &query{v: v}
	f := dto.MasterFilter{
		Search: q.search(), Statuses: q.statuses(),
		Sort: q.sort(dto.MasterSortKeys, dto.DefaultMasterSort), Page: q.page(),
	}
	if withBranch {
		f.BranchID = optionalIDParam[vo.BranchID](q, "branch_id")
	}
	return f, q.err
}

// GET /items?q=&type=stock,service&status=&category_id=&uom_id=&sort=code|name|type&page=&page_size=
func ParseItemFilter(v url.Values) (dto.ItemFilter, error) {
	q := &query{v: v}
	f := dto.ItemFilter{
		Search:     q.search(),
		Types:      parse(q, func() ([]vo.ItemType, error) { return listing.ParseList(v, "type", vo.ParseItemType) }),
		Statuses:   q.statuses(),
		CategoryID: optionalIDParam[vo.CategoryID](q, "category_id"),
		UomID:      optionalIDParam[vo.UomID](q, "uom_id"),
		Sort:       q.sort(dto.ItemSortKeys, dto.DefaultItemSort), Page: q.page(),
	}
	return f, q.err
}

// GET /stock-documents?q=&type=&status=&warehouse_id=&item_id=&start_date=&end_date=&sort=-date&page=&page_size=
func ParseDocumentFilter(v url.Values) (dto.DocumentFilter, error) {
	q := &query{v: v}
	f := dto.DocumentFilter{
		Search:      q.search(),
		Types:       parse(q, func() ([]vo.DocumentType, error) { return listing.ParseList(v, "type", vo.ParseDocumentType) }),
		Statuses:    parse(q, func() ([]vo.DocumentStatus, error) { return listing.ParseList(v, "status", vo.ParseDocumentStatus) }),
		WarehouseID: optionalIDParam[vo.WarehouseID](q, "warehouse_id"),
		ItemID:      optionalIDParam[vo.ItemID](q, "item_id"),
		Sort:        q.sort(dto.DocumentSortKeys, dto.DefaultDocumentSort), Page: q.page(),
	}
	if q.err == nil {
		f.StartDate, f.EndDate, q.err = listing.ParseDateRange(v, "start_date", "end_date")
	}
	return f, q.err
}

// GET /reports?book=stock-on-hand&end_date=2026-12-31&warehouse_id=&item_id=&category_id=&q=&sort=&page=&page_size=
// (start_date ไม่ใช้ — ยอดคงเหลือ ณ end_date)
func ParseStockOnHand(v url.Values) (dto.StockOnHandFilter, error) {
	q := &query{v: v}
	f := dto.StockOnHandFilter{
		AsOf:        q.requiredDate("end_date"),
		Search:      q.search(),
		WarehouseID: optionalIDParam[vo.WarehouseID](q, "warehouse_id"),
		ItemID:      optionalIDParam[vo.ItemID](q, "item_id"),
		CategoryID:  optionalIDParam[vo.CategoryID](q, "category_id"),
		Sort:        q.sort(dto.StockOnHandSortKeys, dto.DefaultStockOnHandSort),
		Page:        q.page(),
	}
	return f, q.err
}

// GET /reports?book=stock-card&item_id=...&warehouse_id=&start_date=&end_date=&page=&page_size=
func ParseStockCard(v url.Values) (dto.StockCardFilter, error) {
	q := &query{v: v}
	item := optionalIDParam[vo.ItemID](q, "item_id")
	if q.err == nil && item == nil {
		q.err = fmt.Errorf("%w: item_id is required for book=stock-card", listing.ErrInvalidQuery)
	}
	f := dto.StockCardFilter{WarehouseID: optionalIDParam[vo.WarehouseID](q, "warehouse_id")}
	f.Start, f.End = q.requiredDate("start_date"), q.requiredDate("end_date")
	if q.err == nil && f.Start.After(f.End) {
		q.err = fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	f.Page = q.page()
	if item != nil {
		f.ItemID = *item
	}
	return f, q.err
}
