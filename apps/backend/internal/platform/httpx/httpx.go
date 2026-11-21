// Package httpx คือ helper HTTP ที่ทุก module ใช้ร่วมกัน:
// อ่าน JSON body, เขียน JSON response (data / meta / error) และแปลง error -> HTTP status
//
// ใช้ net/http ของ standard library เท่านั้น (ไม่ผูกกับ framework)
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"apexis/backend/internal/platform/listing"
)

const MaxBodyBytes = 1 << 20 // 1 MB

var ErrBadJSON = errors.New("invalid json body")

// DecodeJSON อ่าน body เป็น JSON object เดียว ไม่รับ field ที่ไม่รู้จัก และจำกัดขนาด
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", ErrBadJSON, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: unexpected data after json object", ErrBadJSON)
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---------- response envelope ----------

type Data[T any] struct {
	Data T `json:"data"`
}

func WriteData[T any](w http.ResponseWriter, status int, data T) {
	WriteJSON(w, status, Data[T]{Data: data})
}

// List คือรูปแบบของทุก collection: ข้อมูลหนึ่งหน้า + meta สำหรับทำ pagination ที่ frontend
type List[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

type Meta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

func ToList[D any, T any](r listing.Result[D], fn func(D) T) List[T] {
	data := make([]T, len(r.Items))
	for i, item := range r.Items {
		data[i] = fn(item)
	}
	return List[T]{Data: data, Meta: Meta{Page: r.Page.Number, PageSize: r.Page.Size, Total: r.Total}}
}

type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---------- error -> HTTP ----------

type ErrorRule struct {
	Err    error
	Status int
}

// ErrorMapper แปลง error ของ module เป็น HTTP response
// error ที่ไม่อยู่ใน rule จะถูก log และตอบ 500 แบบไม่เปิดเผยรายละเอียด
type ErrorMapper struct {
	rules    []ErrorRule
	detailed map[error]bool
}

// NewErrorMapper: detailed คือ error ที่ wrap รายละเอียดที่ปลอดภัยต่อการแสดง (เช่น field ไหนขาด)
// ErrBadJSON และ listing.ErrInvalidQuery ถูกเพิ่มให้อัตโนมัติ (400 + รายละเอียด)
func NewErrorMapper(rules []ErrorRule, detailed ...error) *ErrorMapper {
	m := &ErrorMapper{
		rules: append([]ErrorRule{
			{Err: ErrBadJSON, Status: http.StatusBadRequest},
			{Err: listing.ErrInvalidQuery, Status: http.StatusBadRequest},
		}, rules...),
		detailed: map[error]bool{ErrBadJSON: true, listing.ErrInvalidQuery: true},
	}
	for _, e := range detailed {
		m.detailed[e] = true
	}
	return m
}

func (m *ErrorMapper) Write(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range m.rules {
		if errors.Is(err, rule.Err) {
			msg := rule.Err.Error()
			if m.detailed[rule.Err] {
				msg = err.Error() // รายละเอียดสร้างจากโค้ดของเราเอง ปลอดภัยต่อการแสดง
			}
			WriteJSON(w, rule.Status, Error{Error: ErrorBody{Code: ErrorCode(rule.Err), Message: msg}})
			return
		}
	}
	slog.ErrorContext(r.Context(), "unhandled error", "method", r.Method, "path", r.URL.Path, "err", err)
	WriteJSON(w, http.StatusInternalServerError, Error{Error: ErrorBody{Code: "internal_error", Message: "internal server error"}})
}

// ErrorCode: "invalid company id" -> "invalid_company_id"
func ErrorCode(err error) string {
	return strings.ReplaceAll(err.Error(), " ", "_")
}
