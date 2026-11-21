// Package repositories คือ implementation ของ organization/application/ports/repositories ด้วย gorm
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
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/organization/application/dto"
	gp "apexis/backend/internal/modules/organization/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
)

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

// exists ตรวจว่ามี record (ที่ยังไม่ถูกลบ) ในบริษัท
func exists(ctx context.Context, db *gorm.DB, table string, companyID, id uuid.UUID) (bool, error) {
	var ok bool
	err := database.Conn(ctx, db).
		Raw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE company_id = ? AND id = ? AND deleted_at IS NULL)", companyID, id).
		Scan(&ok).Error
	return ok, err
}

// reference คือ "ตาราง.column" ที่อ้างถึง record อื่น
type reference struct{ table, column string }

// referenced ตรวจว่ามี record (ที่ยังไม่ถูกลบ) อ้างถึง id ในตารางใดตารางหนึ่ง — query เดียว
func referenced(ctx context.Context, db *gorm.DB, id uuid.UUID, refs ...reference) (bool, error) {
	parts := make([]string, len(refs))
	args := make([]any, len(refs))
	for i, r := range refs {
		parts[i] = "EXISTS (SELECT 1 FROM " + r.table + " WHERE " + r.column + " = ? AND deleted_at IS NULL)"
		args[i] = id
	}
	var used bool
	err := database.Conn(ctx, db).Raw("SELECT "+strings.Join(parts, " OR "), args...).Scan(&used).Error
	return used, err
}

// ancestry คืน [id, parent, grandparent, ...] ด้วย recursive CTE เดียว (PostgreSQL / MySQL 8)
// ว่าง = ไม่พบ id ในบริษัท — จำกัดความลึกกันวนไม่รู้จบถ้าข้อมูลเสีย
func ancestry(ctx context.Context, db *gorm.DB, table string, companyID, id uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct{ ID uuid.UUID }
	err := database.Conn(ctx, db).Raw(`
		WITH RECURSIVE chain AS (
			SELECT id, parent_id, 1 AS depth FROM `+table+`
			WHERE id = ? AND company_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT t.id, t.parent_id, c.depth + 1 FROM `+table+` AS t
			JOIN chain AS c ON t.id = c.parent_id
			WHERE t.deleted_at IS NULL AND c.depth < 64
		)
		SELECT id FROM chain ORDER BY depth`, id, companyID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out, nil
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

// first คืนแถวแรกของ query หรือ notFound
func first[R any](q *gorm.DB, notFound error) (R, error) {
	var rows []R
	if err := q.Limit(1).Scan(&rows).Error; err != nil {
		var zero R
		return zero, err
	}
	if len(rows) == 0 {
		var zero R
		return zero, notFound
	}
	return rows[0], nil
}

func ids[R any](rows []R, id func(R) uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = id(r)
	}
	return out
}

// refCols คือ column ของ record ที่อ้างถึง (ได้จาก LEFT JOIN) — ว่างได้
type refCols struct {
	ID   *uuid.UUID
	Code *string
	Name *string
}

func (r refCols) ref() *dto.Ref {
	if r.ID == nil {
		return nil
	}
	return &dto.Ref{ID: r.ID.String(), Code: deref(r.Code), Name: deref(r.Name)}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func dateUTC(t time.Time) time.Time      { return gp.DateUTC(t) }
func datePtr(t *time.Time) *time.Time    { return gp.DatePtrUTC(t) }
func uuidOf[T ~[16]byte](id T) uuid.UUID { return uuid.UUID(id) }
