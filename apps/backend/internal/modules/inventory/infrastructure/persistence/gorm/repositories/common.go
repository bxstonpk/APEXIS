// Package repositories คือ implementation ของ inventory/application/ports/repositories ด้วย gorm
//
// กติกา:
//   - ทุก method ใช้ database.Conn(ctx, db) เพื่อเข้าร่วม transaction ที่ use case เปิดไว้ใน ctx
//   - การเขียนหลายตาราง / หลายแถวใช้ database.TxConn(ctx) — repository ไม่เปิด transaction เอง
//   - query แบบ raw (Table("... AS x")) ต้องกรอง x.deleted_at IS NULL เอง
//   - ชื่อตาราง / column ใน SQL มาจากค่าคงที่ในโค้ดเท่านั้น ค่าจากผู้ใช้ส่งผ่าน placeholder (?) เสมอ
package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/inventory/application/dto"
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

func create(ctx context.Context, db *gorm.DB, model any, dup error) error {
	if err := database.Conn(ctx, db).Omit(clause.Associations).Create(model).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return dup
		}
		return err
	}
	return nil
}

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

func findByID[M any](ctx context.Context, db *gorm.DB, companyID, id uuid.UUID, notFound error) (M, error) {
	var m M
	err := database.Conn(ctx, db).Where("company_id = ? AND id = ?", companyID, id).Take(&m).Error
	if database.IsNotFound(err) {
		return m, notFound
	}
	return m, err
}

func findByIDs[M any](ctx context.Context, db *gorm.DB, companyID uuid.UUID, ids []uuid.UUID) ([]M, error) {
	var ms []M
	if len(ids) == 0 {
		return ms, nil
	}
	err := database.Conn(ctx, db).Where("company_id = ? AND id IN ?", companyID, ids).Find(&ms).Error
	return ms, err
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

func optString(u *uuid.UUID) *string {
	if u == nil {
		return nil
	}
	s := u.String()
	return &s
}

// ---------- master แบบรหัส + ชื่อ + สถานะ (หน่วยนับ / หมวด / คลัง) ----------

// masterTable คือ Get / List ที่ตารางรูปแบบเดียวกันใช้ร่วมกัน
type masterTable struct {
	table       string
	notFound    error
	hasBranch   bool
	hasAccounts bool
}

type masterRow struct {
	ID                 uuid.UUID
	Code               string
	Name               string
	BranchID           *uuid.UUID
	PurchaseAccountID  *uuid.UUID
	InventoryAccountID *uuid.UUID
	RevenueAccountID   *uuid.UUID
	Status             string
	Total              int64
}

func (r masterRow) dto() dto.Master {
	return dto.Master{
		ID: r.ID.String(), Code: r.Code, Name: r.Name, BranchID: optString(r.BranchID), Status: r.Status,
		Accounts: accountsDTO(r.PurchaseAccountID, r.InventoryAccountID, r.RevenueAccountID),
	}
}

func accountsDTO(purchase, inventory, revenue *uuid.UUID) dto.Accounts {
	return dto.Accounts{PurchaseAccountID: optString(purchase), InventoryAccountID: optString(inventory), RevenueAccountID: optString(revenue)}
}

func (t masterTable) columns() string {
	c := "x.id, x.code, x.name, x.status"
	if t.hasBranch {
		c += ", x.branch_id"
	}
	if t.hasAccounts {
		c += ", x.purchase_account_id, x.inventory_account_id, x.revenue_account_id"
	}
	return c
}

func (t masterTable) base(ctx context.Context, db *gorm.DB, companyID uuid.UUID) *gorm.DB {
	return database.Conn(ctx, db).Table(t.table+" AS x").Where("x.company_id = ? AND x.deleted_at IS NULL", companyID)
}

func (t masterTable) get(ctx context.Context, db *gorm.DB, companyID, id uuid.UUID) (dto.Master, error) {
	var rows []masterRow
	if err := t.base(ctx, db, companyID).Select(t.columns()).Where("x.id = ?", id).Limit(1).Scan(&rows).Error; err != nil {
		return dto.Master{}, err
	}
	if len(rows) == 0 {
		return dto.Master{}, t.notFound
	}
	return rows[0].dto(), nil
}

func (t masterTable) list(ctx context.Context, db *gorm.DB, companyID uuid.UUID, f dto.MasterFilter) (listing.Result[dto.Master], error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"code": "x.code", "name": "x.name"}, "x.id")
	if err != nil {
		return listing.Result[dto.Master]{}, err
	}
	base := func() *gorm.DB {
		q := t.base(ctx, db, companyID)
		if len(f.Statuses) > 0 {
			q = q.Where("x.status IN ?", f.Statuses)
		}
		if t.hasBranch && f.BranchID != nil {
			q = q.Where("x.branch_id = ?", uuidOf(*f.BranchID))
		}
		return search(q, f.Search, "x.code", "x.name")
	}
	rows, total, err := database.FetchPage(base, t.columns(), order, f.Page, func(r masterRow) int64 { return r.Total })
	return toResult(rows, total, f.Page, masterRow.dto), err
}
