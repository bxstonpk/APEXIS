// Package repositories คือ implementation ของ purchasing/application/ports/repositories ด้วย gorm
//
// กติกา:
//   - ทุก method ใช้ database.Conn(ctx, db) เพื่อเข้าร่วม transaction ที่ use case เปิดไว้ใน ctx
//   - การเขียนหลายตาราง / หลายแถวใช้ database.TxConn(ctx) — repository ไม่เปิด transaction เอง
//   - ชื่อตาราง / column ใน SQL มาจากค่าคงที่ในโค้ดเท่านั้น ค่าจากผู้ใช้ส่งผ่าน placeholder (?) เสมอ
//   - ข้อมูลของ module อื่น (ชื่อสินค้า ฯลฯ) ไม่ JOIN ข้าม module — use case เติมผ่าน public API ของ module นั้น
package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

func uuidOf[T ~[16]byte](id T) uuid.UUID { return uuid.UUID(id) }

func optString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

// exists: EXISTS ของเงื่อนไขใดเงื่อนไขหนึ่ง (OR) — query เดียว
func exists(ctx context.Context, db *gorm.DB, conditions []string, args ...any) (bool, error) {
	parts := make([]string, len(conditions))
	for i, c := range conditions {
		parts[i] = "EXISTS (" + c + ")"
	}
	var ok bool
	err := database.Conn(ctx, db).Raw("SELECT "+strings.Join(parts, " OR "), args...).Scan(&ok).Error
	return ok, err
}

func search(q *gorm.DB, text string, columns ...string) *gorm.DB {
	if text == "" {
		return q
	}
	p := database.LikeContains(text)
	parts := make([]string, len(columns))
	args := make([]any, len(columns))
	for i, c := range columns {
		parts[i] = "LOWER(" + c + ") LIKE ?"
		args[i] = p
	}
	return q.Where("("+strings.Join(parts, " OR ")+")", args...)
}

func toResult[R any, T any](rows []R, total int64, page listing.Page, fn func(R) T) listing.Result[T] {
	items := make([]T, len(rows))
	for i, r := range rows {
		items[i] = fn(r)
	}
	return listing.Result[T]{Items: items, Total: total, Page: page}
}

// ---------- ลำดับเลขที่ ----------

type SequenceRepository struct{}

func NewSequenceRepository() *SequenceRepository { return &SequenceRepository{} }

// NextNumber — statement เดียว: INSERT ... ON CONFLICT DO UPDATE ... RETURNING (lock แถวลำดับจน commit)
func (SequenceRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, prefix string, fiscalYear int) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO purchasing_sequences (company_id, prefix, fiscal_year, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (company_id, prefix, fiscal_year) DO UPDATE SET last_no = purchasing_sequences.last_no + 1
		RETURNING last_no`, uuidOf(companyID), prefix, fiscalYear).Scan(&n).Error
	return n, err
}
