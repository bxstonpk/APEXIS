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

func parseEnum[T ~string](raw string, allowed []T) (T, error) {
	if slices.Contains(allowed, T(raw)) {
		return T(raw), nil
	}
	return "", domain.ErrInvalidStatus
}
