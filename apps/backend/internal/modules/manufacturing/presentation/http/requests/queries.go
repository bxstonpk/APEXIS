package requests

import (
	"fmt"
	"net/url"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
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

func optionalIDParam[T ~[16]byte](q *query, key string) *T {
	return parse(q, func() (*T, error) { return listing.ParseOptional(q.v, key, vo.ParseID[T]) })
}

// GET /boms?q=&item_id=&status=&sort=code|name|updated_at&page=&page_size=
func ParseBomFilter(v url.Values) (dto.BomFilter, error) {
	q := &query{v: v}
	f := dto.BomFilter{
		Search: q.search(), ItemID: optionalIDParam[vo.ItemID](q, "item_id"),
		Status: parse(q, func() ([]vo.Status, error) { return listing.ParseList(v, "status", vo.ParseStatus) }),
		Sort:   q.sort(dto.BomSortKeys, dto.DefaultBomSort), Page: q.page(),
	}
	return f, q.err
}

// GET /production-orders?q=&status=&item_id=&from=&to=&sort=date|document_no|created_at&page=&page_size=
func ParseOrderFilter(v url.Values) (dto.OrderFilter, error) {
	q := &query{v: v}
	f := dto.OrderFilter{
		Search: q.search(), ItemID: optionalIDParam[vo.ItemID](q, "item_id"),
		Status: parse(q, func() ([]vo.OrderStatus, error) { return listing.ParseList(v, "status", vo.ParseOrderStatus) }),
		Sort:   q.sort(dto.OrderSortKeys, dto.DefaultOrderSort), Page: q.page(),
	}
	if q.err == nil {
		f.From, f.To, q.err = listing.ParseDateRange(v, "from", "to")
	}
	return f, q.err
}

// GET /reports?book=production-cost|material-usage&start_date=&end_date=&item_id=&page=&page_size=
func ParseCostFilter(v url.Values) (dto.CostFilter, error) {
	q := &query{v: v}
	f := dto.CostFilter{ItemID: optionalIDParam[vo.ItemID](q, "item_id"), Page: q.page()}
	if q.err != nil {
		return f, q.err
	}
	start, end, err := listing.ParseDateRange(v, "start_date", "end_date")
	if err != nil {
		return f, err
	}
	if start == nil || end == nil {
		return f, fmt.Errorf("%w: start_date and end_date are required (YYYY-MM-DD)", listing.ErrInvalidQuery)
	}
	f.Start, f.End = *start, *end
	return f, nil
}

// GET /reports?book=wip&end_date=&page=&page_size=
func ParseWipFilter(v url.Values) (dto.WipFilter, error) {
	q := &query{v: v}
	f := dto.WipFilter{Page: q.page()}
	if q.err != nil {
		return f, q.err
	}
	raw := v.Get("end_date")
	if raw == "" {
		return f, fmt.Errorf("%w: end_date is required (YYYY-MM-DD)", listing.ErrInvalidQuery)
	}
	d, err := listing.ParseDate(raw)
	if err != nil {
		return f, fmt.Errorf("%w: end_date has an invalid value %q", listing.ErrInvalidQuery, raw)
	}
	f.AsOf = d
	return f, nil
}
