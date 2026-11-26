// Package repositories คือสัญญา (port) ที่ application ของ ledger ต้องการจาก persistence
// implementation อยู่ใน infrastructure/persistence/gorm/repositories
//
// กติกา:
//   - ทุก method รับ CompanyID (tenant isolation) และไม่เห็น record ที่ถูก soft delete
//   - FindByID คืน entity สำหรับฝั่งเขียน, Get / List คืน read DTO (JOIN ชื่อของ record ที่อ้างถึง)
//   - Find* / Get คืน domain.Err*NotFound เมื่อไม่พบ, Create / Update คืน domain.ErrCodeExists เมื่อชน unique
//   - ห้ามเปิด transaction เอง — ใช้ transaction ที่ use case เปิดไว้ใน ctx
package repositories

import (
	"context"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/entities"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type AccountRepository interface {
	Create(ctx context.Context, a *entities.Account) error
	// CreateMany เขียนหลายบัญชี (เรียง parent ก่อน child) — ต้องอยู่ใน transaction
	CreateMany(ctx context.Context, as []*entities.Account) error
	// HasAny: บริษัทมีบัญชีอยู่แล้วหรือไม่
	HasAny(ctx context.Context, companyID vo.CompanyID) (bool, error)
	// PostableIDsByCode คืน id ของบัญชีที่ลงรายการได้ (active, ไม่ใช่บัญชีคุม) ตามรหัส — รหัสที่ไม่พบไม่อยู่ในผลลัพธ์; query เดียว
	PostableIDsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error)
	// RefsByIDs คืนรหัส / ชื่อบัญชีตาม id (รวมที่ลบแล้ว — เอกสารเดิมยังแสดงชื่อได้); query เดียว
	RefsByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]dto.CodeName, error)
	Update(ctx context.Context, a *entities.Account) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) error // soft delete
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (*entities.Account, error)
	// FindByIDs คืนเฉพาะบัญชีที่พบ (ไม่รวมที่ถูกลบ) — query เดียว
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]*entities.Account, error)
	// Usage: มีบัญชีย่อย / ถูกใช้ในบรรทัดรายการแล้วหรือไม่ — query เดียว
	Usage(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (entities.AccountUsage, error)
	// Ancestry คืน [id, parent, grandparent, ...] ด้วย recursive query เดียว — ว่าง = ไม่พบ id
	Ancestry(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) ([]vo.AccountID, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.AccountID) (dto.Account, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.AccountFilter) (listing.Result[dto.Account], error)
	// Tree คืนทุกบัญชีของบริษัทเรียงตามรหัส (หน้าผังบัญชีแบบต้นไม้)
	Tree(ctx context.Context, companyID vo.CompanyID) ([]dto.Account, error)
	// Chart คืนผังบัญชีทั้งหมดของบริษัท (ใช้สร้างรายงาน)
	Chart(ctx context.Context, companyID vo.CompanyID) ([]services.AccountNode, error)
}

type JournalRepository interface {
	Create(ctx context.Context, j *entities.Journal) error
	Update(ctx context.Context, j *entities.Journal) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) error // soft delete
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (*entities.Journal, error)
	HasEntries(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error)
	// ExistingCodes คืนรหัสสมุดที่มีอยู่แล้วจาก codes — query เดียว
	ExistingCodes(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]bool, error)
	// ActiveIDsByCode คืน id ของสมุดที่ active ตามรหัส — query เดียว
	ActiveIDsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error)
	// RefsByIDs คืนรหัส / ชื่อสมุดตาม id (รวมที่ลบแล้ว); query เดียว
	RefsByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.JournalID) (map[vo.JournalID]dto.CodeName, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (dto.Journal, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.JournalFilter) (listing.Result[dto.Journal], error)
	// NextNumber คืนเลขลำดับถัดไปของสมุด + ปีบัญชี (เริ่ม 1) — ต้องอยู่ใน transaction
	// แถวลำดับถูก lock จน commit จึงไม่มีเลขซ้ำ / ขาดตอนเมื่อผ่านรายการพร้อมกัน
	NextNumber(ctx context.Context, companyID vo.CompanyID, journalID vo.JournalID, periodKey string) (int64, error)
}

type EntryRepository interface {
	// Create / Update เขียนหัวรายการ + บรรทัด (หลายตาราง) — ต้องอยู่ใน transaction
	Create(ctx context.Context, e *entities.Entry) error
	Update(ctx context.Context, e *entities.Entry) error
	// UpdateStatus เขียนเฉพาะสถานะ / ข้อมูลการผ่านรายการ / การกลับรายการ ของหัวรายการ
	UpdateStatus(ctx context.Context, e *entities.Entry) error
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) error // ลบจริง (เฉพาะ draft)
	// FindForUpdate โหลดรายการพร้อม lock แถวหัวรายการ (SELECT ... FOR UPDATE) กันการผ่าน / กลับรายการซ้อนกัน
	FindForUpdate(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) (*entities.Entry, error)
	Get(ctx context.Context, companyID vo.CompanyID, id vo.EntryID) (dto.Entry, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.EntryFilter) (listing.Result[dto.EntrySummary], error)
}

// Bucket คือช่วงวันที่ [From, To] (From nil = ตั้งแต่ต้น) สำหรับรวมยอด
type Bucket struct {
	From *time.Time
	To   time.Time
}

// Sum คือยอดรวมเดบิต / เครดิตของบัญชีในหนึ่ง bucket
type Sum struct {
	Debit  vo.Money
	Credit vo.Money
}

func (s Sum) Net() vo.Money { return s.Debit.Sub(s.Credit) }

type ReportRepository interface {
	// SumByAccount รวมยอดของรายการที่มีผล (posted / reversed) ต่อบัญชี แยกตาม bucket — query เดียว
	// accountID != nil = เฉพาะบัญชีนั้น; ผลลัพธ์ map[บัญชี][]Sum เรียงตาม buckets
	SumByAccount(ctx context.Context, companyID vo.CompanyID, accountID *vo.AccountID, buckets []Bucket) (map[vo.AccountID][]Sum, error)
	// AccountLines คือรายการเคลื่อนไหวของบัญชีในช่วงวันที่ พร้อมยอดสะสม (ไม่รวมยอดยกมา) — แบ่งหน้าใน DB
	AccountLines(ctx context.Context, companyID vo.CompanyID, accountID vo.AccountID, start, end time.Time, page listing.Page) ([]dto.LedgerLine, int64, error)
	// EntryActivity: จำนวนรายการร่าง / ที่มีผล (posted / reversed) ที่ลงวันที่ในช่วง — query เดียว (COUNT FILTER)
	EntryActivity(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (drafts, posted int64, err error)
	// RecentEntries: รายการที่มีผลล่าสุด (วันที่ / เลขที่ล่าสุดก่อน)
	RecentEntries(ctx context.Context, companyID vo.CompanyID, limit int) ([]dto.RecentEntry, error)
}
