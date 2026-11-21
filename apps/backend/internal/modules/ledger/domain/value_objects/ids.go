// Package valueobjects คือ value object ของ ledger
package valueobjects

import (
	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
)

// ID แต่ละชนิดเป็น type แยกกัน เพื่อให้ compiler กันการส่ง AccountID ไปแทน JournalID
// ใช้ UUIDv7 (เรียงตามเวลา) เพื่อให้ index ใน DB ทำงานได้ดี
//
// CompanyID / UserID / FiscalPeriodID / มิติวิเคราะห์ (สาขา, แผนก, ศูนย์ต้นทุน, โครงการ)
// เป็น id ของ module อื่น (identity / organization) — ผูกผ่านค่า uuid ไม่ผูกผ่าน type
type (
	CompanyID      uuid.UUID
	UserID         uuid.UUID
	AccountID      uuid.UUID
	JournalID      uuid.UUID
	EntryID        uuid.UUID
	FiscalPeriodID uuid.UUID
	BranchID       uuid.UUID
	DepartmentID   uuid.UUID
	CostCenterID   uuid.UUID
	ProjectSiteID  uuid.UUID
)

type anyID interface {
	~[16]byte
}

// NewID สร้าง id ใหม่ของชนิดใดก็ได้ เช่น vo.NewID[vo.AccountID]()
func NewID[T anyID]() T { return T(uuid.Must(uuid.NewV7())) }

// ParseID แปลง string -> id ชนิดที่ต้องการ (ไม่รับ nil uuid)
func ParseID[T anyID](s string) (T, error) {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return T{}, domain.ErrInvalidID
	}
	return T(id), nil
}

func IDString[T anyID](id T) string { return uuid.UUID(id).String() }

func (id CompanyID) String() string { return IDString(id) }
func (id UserID) String() string    { return IDString(id) }
func (id AccountID) String() string { return IDString(id) }
func (id JournalID) String() string { return IDString(id) }
func (id EntryID) String() string   { return IDString(id) }
