// Package repositories คือ implementation ของ pettycash/application/ports/repositories ด้วย gorm
//
// กติกา:
//   - ทุก method ใช้ database.Conn(ctx, db) เพื่อเข้าร่วม transaction ที่ use case (หรือใบจ่ายของการเงิน) เปิดไว้ใน ctx
//   - การเขียนหลายตาราง / หลายแถวใช้ database.TxConn(ctx) — repository ไม่เปิด transaction เอง
//   - ชื่อตาราง / column ใน SQL มาจากค่าคงที่ในโค้ดเท่านั้น ค่าจากผู้ใช้ส่งผ่าน placeholder (?) เสมอ
//   - ข้อมูลของ module อื่น (ชื่อบัญชี / ผู้ใช้) ไม่ JOIN ข้าม module — use case เติมผ่าน public API ของ module นั้น
package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	gp "apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm/models"
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

func strs[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func optDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := dateOnly(*t)
	return &d
}

// ---------- ลำดับเลขที่ ----------

type SequenceRepository struct{}

func NewSequenceRepository() *SequenceRepository { return &SequenceRepository{} }

// NextNumber — statement เดียว: INSERT ... ON CONFLICT DO UPDATE ... RETURNING (lock แถวลำดับจน commit)
func (SequenceRepository) NextNumber(ctx context.Context, companyID vo.CompanyID, docType vo.DocType, periodKey string) (int64, error) {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	err = tx.Raw(`
		INSERT INTO petty_cash_number_sequences (company_id, doc_type, period_key, last_no) VALUES (?, ?, ?, 1)
		ON CONFLICT (company_id, doc_type, period_key) DO UPDATE SET last_no = petty_cash_number_sequences.last_no + 1
		RETURNING last_no`, uuidOf(companyID), string(docType), periodKey).Scan(&n).Error
	return n, err
}

// ---------- ค่าตั้ง ----------

type SettingsRepository struct{ db *gorm.DB }

func NewSettingsRepository(db *gorm.DB) *SettingsRepository { return &SettingsRepository{db: db} }

func (r *SettingsRepository) Get(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) {
	var m models.SettingsModel
	err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Take(&m).Error
	if database.IsNotFound(err) {
		return entities.Settings{}, nil
	}
	if err != nil {
		return entities.Settings{}, err
	}
	return gp.SettingsToEntity(m), nil
}

// Save — UPSERT statement เดียว
func (r *SettingsRepository) Save(ctx context.Context, companyID vo.CompanyID, s entities.Settings) error {
	m := gp.SettingsToModel(companyID, s)
	m.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"input_vat_account_id", "updated_at"}),
	}).Create(&m).Error
}
