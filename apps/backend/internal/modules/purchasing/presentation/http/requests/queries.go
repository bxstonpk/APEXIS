package requests

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"apexis/backend/internal/modules/purchasing/application/dto"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
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

// GET /suppliers?q=&status=&vat_registered=true|false&sort=code|name&page=&page_size=
func ParseSupplierFilter(v url.Values) (dto.SupplierFilter, error) {
	q := &query{v: v}
	f := dto.SupplierFilter{
		Search:        q.search(),
		Statuses:      parse(q, func() ([]vo.Status, error) { return listing.ParseList(v, "status", vo.ParseStatus) }),
		VatRegistered: parse(q, func() (*bool, error) { return listing.ParseOptional(v, "vat_registered", strconv.ParseBool) }),
		Sort:          q.sort(dto.SupplierSortKeys, dto.DefaultSupplierSort),
		Page:          q.page(),
	}
	return f, q.err
}

// ParseDocumentFilter: GET /purchase-orders | /goods-receipts | /supplier-invoices
// ?q=&status=&supplier_id=&order_id=&item_id=&start_date=&end_date=&sort=&page=&page_size=
func ParseDocumentFilter(v url.Values, orders bool) (dto.DocumentFilter, error) {
	q := &query{v: v}
	statuses := parse(q, func() ([]string, error) {
		if orders {
			s, err := listing.ParseList(v, "status", vo.ParseOrderStatus)
			return strs(s), err
		}
		s, err := listing.ParseList(v, "status", vo.ParseDocStatus)
		return strs(s), err
	})
	keys, def := dto.DocumentSortKeys, dto.DefaultDocumentSort
	if orders {
		keys, def = dto.OrderSortKeys, dto.DefaultOrderSort
	}
	f := dto.DocumentFilter{
		Search: q.search(), Statuses: statuses, SupplierID: optionalIDParam[vo.SupplierID](q, "supplier_id"),
		Sort: q.sort(keys, def), Page: q.page(),
	}
	if orders {
		f.ItemID = optionalIDParam[vo.ItemID](q, "item_id")
	} else {
		f.OrderID = optionalIDParam[vo.OrderID](q, "order_id")
	}
	if q.err == nil {
		f.StartDate, f.EndDate, q.err = listing.ParseDateRange(v, "start_date", "end_date")
	}
	return f, q.err
}

func strs[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

// GET /reports?book=outstanding-purchase-orders&supplier_id=&item_id=&q=&sort=&page=&page_size=
func ParseOutstanding(v url.Values) (dto.OutstandingFilter, error) {
	q := &query{v: v}
	f := dto.OutstandingFilter{
		Search: q.search(), SupplierID: optionalIDParam[vo.SupplierID](q, "supplier_id"), ItemID: optionalIDParam[vo.ItemID](q, "item_id"),
		Sort: q.sort(dto.OutstandingSortKeys, dto.DefaultOutstandingSort), Page: q.page(),
	}
	return f, q.err
}

// GET /reports?book=ap-aging&end_date=2026-12-31&supplier_id=&q=&page=&page_size=
func ParseAging(v url.Values) (dto.AgingFilter, error) {
	q := &query{v: v}
	f := dto.AgingFilter{Search: q.search(), SupplierID: optionalIDParam[vo.SupplierID](q, "supplier_id"), Page: q.page()}
	if q.err == nil {
		raw := v.Get("end_date")
		if raw == "" {
			return f, fmt.Errorf("%w: end_date is required (YYYY-MM-DD)", listing.ErrInvalidQuery)
		}
		d, err := listing.ParseDate(raw)
		if err != nil {
			return f, fmt.Errorf("%w: end_date has an invalid value %q", listing.ErrInvalidQuery, raw)
		}
		f.AsOf = d
	}
	return f, q.err
}

// GET /withholding-tax-types?q=&status=&sort=code|name&page=&page_size=
func ParseWhtTypeFilter(v url.Values) (dto.WhtTypeFilter, error) {
	q := &query{v: v}
	f := dto.WhtTypeFilter{
		Search:   q.search(),
		Statuses: parse(q, func() ([]vo.Status, error) { return listing.ParseList(v, "status", vo.ParseStatus) }),
		Sort:     q.sort([]string{"code", "name"}, []listing.Sort{{Key: "code"}}),
		Page:     q.page(),
	}
	return f, q.err
}

// GET /reports?book=withholding-tax&start_date=&end_date=&form=pnd3|pnd53&supplier_id=&sort=&page=&page_size=
func ParseWithholding(v url.Values) (dto.WithholdingFilter, error) {
	q := &query{v: v}
	f := dto.WithholdingFilter{
		SupplierID: optionalIDParam[vo.SupplierID](q, "supplier_id"),
		Sort:       q.sort(dto.WithholdingSortKeys, dto.DefaultWithholdingSort), Page: q.page(),
	}
	if q.err != nil {
		return f, q.err
	}
	switch form := v.Get("form"); form {
	case "", "pnd3", "pnd53":
		f.Form = form
	default:
		return f, fmt.Errorf("%w: form must be pnd3 or pnd53", listing.ErrInvalidQuery)
	}
	for _, d := range []struct {
		key string
		dst *time.Time
	}{{"start_date", &f.Start}, {"end_date", &f.End}} {
		raw := v.Get(d.key)
		t, err := listing.ParseDate(raw)
		if raw == "" || err != nil {
			return f, fmt.Errorf("%w: %s is required (YYYY-MM-DD)", listing.ErrInvalidQuery, d.key)
		}
		*d.dst = t
	}
	if f.Start.After(f.End) {
		return f, fmt.Errorf("%w: start_date must not be after end_date", listing.ErrInvalidQuery)
	}
	return f, nil
}
