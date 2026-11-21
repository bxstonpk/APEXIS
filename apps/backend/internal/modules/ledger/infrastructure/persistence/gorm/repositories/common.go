// Package repositories คือ implementation ของ ledger/application/ports/repositories ด้วย gorm
//
// กติกา:
//   - ทุก method ใช้ database.Conn(ctx, db) เพื่อเข้าร่วม transaction ที่ use case เปิดไว้ใน ctx
//   - การเขียนหลายตารางใช้ database.TxConn(ctx) ซึ่งบังคับว่าต้องมี transaction — repository ไม่เปิดเอง
//   - query อ่านแบบ raw (Table("... AS x")) ต้องกรอง x.deleted_at IS NULL เอง
//   - ชื่อตาราง / column ใน SQL มาจากค่าคงที่ในโค้ดเท่านั้น ค่าจากผู้ใช้ส่งผ่าน placeholder (?) เสมอ
package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

func uuidOf[T ~[16]byte](id T) uuid.UUID { return uuid.UUID(id) }

// create insert model (ไม่บันทึก association) แปลง unique violation เป็น dup
func create(ctx context.Context, db *gorm.DB, model any, dup error) error {
	if err := database.Conn(ctx, db).Omit(clause.Associations).Create(model).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return dup
		}
		return err
	}
	return nil
}

// update แก้เฉพาะ columns ของแถวในบริษัท (soft-deleted ถูกกรองอัตโนมัติโดย gorm)
func update(ctx context.Context, db *gorm.DB, model any, companyID uuid.UUID, columns []string, dup, notFound error) error {
	res := database.Conn(ctx, db).Model(model).
		Where("company_id = ?", companyID).
		Select(append(columns, "updated_at")).
		Omit(clause.Associations).
		Updates(model)
	if res.Error != nil {
		if database.IsUniqueViolation(res.Error) {
			return dup
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}

// softDelete ตั้ง deleted_at (model ต้องมี gorm.DeletedAt)
func softDelete[M any](ctx context.Context, db *gorm.DB, companyID, id uuid.UUID, notFound error) error {
	res := database.Conn(ctx, db).Where("company_id = ? AND id = ?", companyID, id).Delete(new(M))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}

// findByID โหลด model เต็มแถวสำหรับฝั่งเขียน
func findByID[M any](ctx context.Context, db *gorm.DB, companyID, id uuid.UUID, notFound error) (M, error) {
	var m M
	err := database.Conn(ctx, db).Where("company_id = ? AND id = ?", companyID, id).Take(&m).Error
	if database.IsNotFound(err) {
		return m, notFound
	}
	return m, err
}

// search เพิ่มเงื่อนไขค้นหาข้อความ (ไม่สนตัวพิมพ์) ใน columns ใดก็ได้
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

func optString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}
