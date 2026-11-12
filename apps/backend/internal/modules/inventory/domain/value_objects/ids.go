// Package valueobjects คือ value object ของ inventory
package valueobjects

import (
	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
)

// ID แต่ละชนิดเป็น type แยกกัน (UUIDv7 เรียงตามเวลา)
// CompanyID / UserID / FiscalPeriodID / BranchID เป็น id ของ module อื่น — ผูกผ่านค่า uuid
type (
	CompanyID      uuid.UUID
	UserID         uuid.UUID
	FiscalPeriodID uuid.UUID
	BranchID       uuid.UUID
	UomID          uuid.UUID
	CategoryID     uuid.UUID
	ItemID         uuid.UUID
	WarehouseID    uuid.UUID
	DocumentID     uuid.UUID
	LayerID        uuid.UUID
	MovementID     uuid.UUID
)

type anyID interface {
	~[16]byte
}

func NewID[T anyID]() T { return T(uuid.Must(uuid.NewV7())) }

func ParseID[T anyID](s string) (T, error) {
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		return T{}, domain.ErrInvalidID
	}
	return T(id), nil
}

func IDString[T anyID](id T) string { return uuid.UUID(id).String() }

func (id CompanyID) String() string   { return IDString(id) }
func (id UserID) String() string      { return IDString(id) }
func (id UomID) String() string       { return IDString(id) }
func (id CategoryID) String() string  { return IDString(id) }
func (id ItemID) String() string      { return IDString(id) }
func (id WarehouseID) String() string { return IDString(id) }
func (id DocumentID) String() string  { return IDString(id) }
