// Package listing คือรูปแบบกลางของการค้นหา / กรอง / เรียง / แบ่งหน้า ที่รับมาจาก URL query
//
// ทุกหน้าที่มีการค้นหาต้องใช้ GET + URL query (ไม่ใช้ POST /search)
// เพื่อให้ frontend เก็บเงื่อนไขการค้นหาไว้ใน URL ของหน้า แล้วส่งต่อให้ API ด้วยชื่อ parameter เดียวกัน
// กด Back / refresh / bookmark แล้วได้ผลการค้นหาเดิม
//
//	GET /api/v1/companies/{companyID}/members?q=somchai&status=active&sort=-joined_at&page=2&page_size=50
//
// กติกา parameter:
//   - q          ค้นหาข้อความ (ตัดช่องว่างหัวท้าย, ยาวไม่เกิน MaxSearchLength ตัวอักษร)
//   - sort       คีย์ที่อนุญาต คั่นด้วย comma, ขึ้นต้นด้วย "-" = มากไปน้อย เช่น sort=-joined_at,name
//   - page       เริ่มที่ 1 (ค่าเริ่มต้น 1)
//   - page_size  ค่าเริ่มต้น DefaultPageSize, สูงสุด MaxPageSize
//   - filter อื่น ชื่อ snake_case, หลายค่าคั่นด้วย comma เช่น status=active,suspended
//
// ค่าที่ไม่ถูกต้องของ parameter ที่รู้จัก -> ErrInvalidQuery (HTTP 400)
// parameter ที่ไม่รู้จักจะถูกเพิกเฉย เพื่อให้ URL เก่าที่ bookmark ไว้ยังใช้ได้
//
// package นี้ไม่ผูกกับ HTTP framework หรือ database ใช้ได้ทั้ง presentation และ application
package listing

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	MaxSearchLength = 100
)

// ErrInvalidQuery ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client
var ErrInvalidQuery = errors.New("invalid query parameter")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidQuery}, args...)...)
}

// Page คือการแบ่งหน้าแบบ LIMIT / OFFSET
type Page struct {
	Number int // เริ่มที่ 1
	Size   int
}

func (p Page) Limit() int  { return p.Size }
func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// Sort คือคีย์การเรียงหนึ่งตัว — Key เป็นชื่อสาธารณะ (ไม่ใช่ชื่อ column)
// repository ต้อง map Key -> column ผ่าน whitelist เท่านั้น
type Sort struct {
	Key  string
	Desc bool
}

// Result คือผลลัพธ์หนึ่งหน้า พร้อมจำนวนทั้งหมดที่ตรงเงื่อนไข
type Result[T any] struct {
	Items []T
	Total int64
	Page  Page
}

// ParsePage อ่าน page / page_size
func ParsePage(v url.Values) (Page, error) {
	p := Page{Number: 1, Size: DefaultPageSize}
	if raw := v.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return Page{}, invalid("page must be a positive integer")
		}
		p.Number = n
	}
	if raw := v.Get("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxPageSize {
			return Page{}, invalid("page_size must be between 1 and %d", MaxPageSize)
		}
		p.Size = n
	}
	return p, nil
}

// ParseSort อ่าน sort โดยรับเฉพาะคีย์ใน allowed ถ้าไม่ส่งมาจะใช้ def
func ParseSort(v url.Values, allowed []string, def []Sort) ([]Sort, error) {
	raw := strings.TrimSpace(v.Get("sort"))
	if raw == "" {
		return slices.Clone(def), nil
	}
	parts := strings.Split(raw, ",")
	out := make([]Sort, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		s := Sort{Key: part}
		if strings.HasPrefix(part, "-") {
			s = Sort{Key: part[1:], Desc: true}
		}
		if !slices.Contains(allowed, s.Key) {
			return nil, invalid("sort must be one of %s (prefix with - for descending)", strings.Join(allowed, ", "))
		}
		if slices.ContainsFunc(out, func(o Sort) bool { return o.Key == s.Key }) {
			return nil, invalid("sort key %q is duplicated", s.Key)
		}
		out = append(out, s)
	}
	return out, nil
}

// ParseSearch อ่าน q
func ParseSearch(v url.Values) (string, error) {
	q := strings.TrimSpace(v.Get("q"))
	if utf8.RuneCountInString(q) > MaxSearchLength {
		return "", invalid("q must be at most %d characters", MaxSearchLength)
	}
	return q, nil
}

// ParseList อ่าน filter ที่รับหลายค่าคั่นด้วย comma เช่น status=active,suspended
// แต่ละค่าถูกแปลงด้วย parse; ถ้า parse คืน error จะตอบ ErrInvalidQuery พร้อมชื่อ key
func ParseList[T any](v url.Values, key string, parse func(string) (T, error)) ([]T, error) {
	raw := strings.TrimSpace(v.Get(key))
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]T, 0, len(parts))
	for _, part := range parts {
		val, err := parse(strings.TrimSpace(part))
		if err != nil {
			return nil, invalid("%s has an invalid value %q", key, part)
		}
		out = append(out, val)
	}
	return out, nil
}

// ParseOptional อ่าน filter ค่าเดียว คืน nil ถ้าไม่ได้ส่งมา
func ParseOptional[T any](v url.Values, key string, parse func(string) (T, error)) (*T, error) {
	raw := strings.TrimSpace(v.Get(key))
	if raw == "" {
		return nil, nil
	}
	val, err := parse(raw)
	if err != nil {
		return nil, invalid("%s has an invalid value %q", key, raw)
	}
	return &val, nil
}

// ParseDate รับวันที่ ISO 8601 "YYYY-MM-DD" (ปี ค.ศ.) คืนเวลา 00:00 UTC
// frontend แสดงเป็น พ.ศ. ได้ แต่ใน URL / JSON ต้องส่งเป็น ค.ศ. รูปแบบนี้เสมอ
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, s, time.UTC)
}

// ParseDateRange อ่านช่วงวันที่ (เช่น start_date / end_date) และตรวจว่า from <= to
func ParseDateRange(v url.Values, fromKey, toKey string) (from, to *time.Time, err error) {
	if from, err = ParseOptional(v, fromKey, ParseDate); err != nil {
		return nil, nil, err
	}
	if to, err = ParseOptional(v, toKey, ParseDate); err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, invalid("%s must not be after %s", fromKey, toKey)
	}
	return from, to, nil
}

// ParseBool ใช้กับ ParseOptional: รับเฉพาะ "true" / "false"
func ParseBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, errors.New("not a boolean")
}
