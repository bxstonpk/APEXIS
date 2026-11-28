// Package repositories คือ implementation ของ sales/application/ports/repositories ด้วย gorm
//
// กติกา: ใช้ database.Conn(ctx, db) เข้าร่วม transaction ใน ctx; เขียนหลายตารางใช้ database.TxConn(ctx);
// ค่าจากผู้ใช้ผ่าน placeholder เสมอ; ไม่ JOIN ตารางของ module อื่น (ชื่อสินค้าเติมที่ use case)
package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

func uuidOf[T ~[16]byte](id T) uuid.UUID { return uuid.UUID(id) }

func uuids[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}

func optString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

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

func dateUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dateOpt(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := dateUTC(*t)
	return &d
}

// updateStatus เขียนสถานะ / ผู้ยกเลิกของเอกสารที่ลงบัญชีแล้ว
func updateStatus(ctx context.Context, db *gorm.DB, model any, companyID, id uuid.UUID, status string, by *uuid.UUID, at *time.Time, notFound error) error {
	res := database.Conn(ctx, db).Model(model).Where("company_id = ? AND id = ?", companyID, id).
		Updates(map[string]any{"status": status, "cancelled_by": by, "cancelled_at": at, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}

// SequenceRepository: เลขลำดับถัดไปต่อชนิดเอกสาร + คีย์ช่วงนับ — INSERT ... ON CONFLICT DO UPDATE ... RETURNING
type SequenceRepository struct{}

func NewSequenceRepository() *SequenceRepository { return &SequenceRepository{} }

func (SequenceRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO sales_number_sequences (company_id, doc_type, period_key, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (company_id, doc_type, period_key) DO UPDATE SET last_no = sales_number_sequences.last_no + 1
		RETURNING last_no`, uuidOf(companyID), string(docType), periodKey).Scan(&n).Error
	return n, err
}

func strOrEmpty(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}
