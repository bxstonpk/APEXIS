// Package valueobjects คือ value object ของ organization
package valueobjects

import (
	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

// ID แต่ละชนิดเป็น type แยกกัน เพื่อให้ compiler กันการส่ง BranchID ไปแทน DepartmentID
// ใช้ UUIDv7 (เรียงตามเวลา) เพื่อให้ index ใน DB ทำงานได้ดี
//
// CompanyID / UserID เป็น id เดียวกับของ identity module (ผูกผ่านค่า uuid ไม่ผูกผ่าน type)
type (
	CompanyID      uuid.UUID
	UserID         uuid.UUID
	BranchID       uuid.UUID
	BusinessUnitID uuid.UUID
	DepartmentID   uuid.UUID
	CostCenterID   uuid.UUID
	ProjectSiteID  uuid.UUID
	FiscalYearID   uuid.UUID
	FiscalPeriodID uuid.UUID
)

type anyID interface {
	~[16]byte
}

// NewID สร้าง id ใหม่ของชนิดใดก็ได้ เช่น vo.NewID[vo.BranchID]()
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

func (id CompanyID) String() string      { return IDString(id) }
func (id UserID) String() string         { return IDString(id) }
func (id BranchID) String() string       { return IDString(id) }
func (id BusinessUnitID) String() string { return IDString(id) }
func (id DepartmentID) String() string   { return IDString(id) }
func (id CostCenterID) String() string   { return IDString(id) }
func (id ProjectSiteID) String() string  { return IDString(id) }
func (id FiscalYearID) String() string   { return IDString(id) }
func (id FiscalPeriodID) String() string { return IDString(id) }
