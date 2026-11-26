package valueobjects

import (
	"slices"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

// Status ของ master data (สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน)
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

var Statuses = []Status{StatusActive, StatusInactive}

func ParseStatus(raw string) (Status, error) { return parseEnum(raw, Statuses) }

// ProjectStatus ของโครงการ / ไซต์งาน
type ProjectStatus string

const (
	ProjectPlanned ProjectStatus = "planned"
	ProjectActive  ProjectStatus = "active"
	ProjectClosed  ProjectStatus = "closed"
)

var ProjectStatuses = []ProjectStatus{ProjectPlanned, ProjectActive, ProjectClosed}

func ParseProjectStatus(raw string) (ProjectStatus, error) { return parseEnum(raw, ProjectStatuses) }

// PeriodStatus ของงวดบัญชี
type PeriodStatus string

const (
	PeriodOpen   PeriodStatus = "open"
	PeriodClosed PeriodStatus = "closed"
)

var PeriodStatuses = []PeriodStatus{PeriodOpen, PeriodClosed}

func ParsePeriodStatus(raw string) (PeriodStatus, error) { return parseEnum(raw, PeriodStatuses) }

// BusinessType คือประเภทธุรกิจของบริษัท — เลือกตอนสร้างบริษัทและแก้ไม่ได้ภายหลัง
// กำหนดผังบัญชีมาตรฐาน / ค่าตั้งเริ่มต้นที่สร้างให้ตอนสร้างบริษัท และแอปที่แสดง
type BusinessType string

const (
	BusinessService       BusinessType = "service"       // ธุรกิจบริการทั่วไป
	BusinessTrading       BusinessType = "trading"       // ธุรกิจซื้อขายสินค้า
	BusinessManufacturing BusinessType = "manufacturing" // ธุรกิจผลิตสินค้า
	BusinessConstruction  BusinessType = "construction"  // ธุรกิจก่อสร้าง
)

var BusinessTypes = []BusinessType{BusinessService, BusinessTrading, BusinessManufacturing, BusinessConstruction}

func ParseBusinessType(raw string) (BusinessType, error) {
	if slices.Contains(BusinessTypes, BusinessType(raw)) {
		return BusinessType(raw), nil
	}
	return "", domain.ErrInvalidBusinessType
}

func parseEnum[T ~string](raw string, allowed []T) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", domain.ErrInvalidStatus
}
