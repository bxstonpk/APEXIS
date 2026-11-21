// Package reports คือ endpoint รายงานเดียวของทั้งระบบ: GET /api/v1/companies/{companyID}/reports?book=...
//
// หน้ารายงานของ frontend เป็นหน้าเดียว เลือกรายงานด้วย book — แต่ละ module ลงทะเบียน book ของตัวเอง
// (ledger: trial-balance, balance-sheet, ...; inventory: stock-on-hand, stock-card, ...)
// registry อ่าน book แล้วส่งต่อให้ handler ของ module เจ้าของ (handler ตรวจสิทธิ์ / parameter เอง)
package reports

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
)

type Registry struct {
	books map[string]http.Handler
}

func NewRegistry() *Registry { return &Registry{books: map[string]http.Handler{}} }

// Register ผูกชื่อ book กับ handler — ชื่อซ้ำข้าม module เป็น bug ของโปรแกรม จึง panic ตอนเริ่ม
func (r *Registry) Register(book string, h http.Handler) {
	if _, dup := r.books[book]; dup {
		panic(fmt.Sprintf("reports: book %q registered twice", book))
	}
	r.books[book] = h
}

// Books คืนชื่อ book ทั้งหมด (เรียงตามตัวอักษร)
func (r *Registry) Books() []string {
	out := make([]string, 0, len(r.books))
	for b := range r.books {
		out = append(out, b)
	}
	slices.Sort(out)
	return out
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	book := req.URL.Query().Get("book")
	if h, ok := r.books[book]; ok {
		h.ServeHTTP(w, req)
		return
	}
	msg := fmt.Sprintf("%s: book must be one of %s", listing.ErrInvalidQuery, strings.Join(r.Books(), ", "))
	if book == "" {
		msg = fmt.Sprintf("%s: book is required (%s)", listing.ErrInvalidQuery, strings.Join(r.Books(), ", "))
	}
	httpx.WriteJSON(w, http.StatusBadRequest, httpx.Error{Error: httpx.ErrorBody{Code: httpx.ErrorCode(listing.ErrInvalidQuery), Message: msg}})
}
