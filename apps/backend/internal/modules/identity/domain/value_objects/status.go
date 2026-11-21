package valueobjects

import domain "apexis/backend/internal/modules/identity/domain/errors"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type MembershipStatus string

const (
	MembershipStatusActive    MembershipStatus = "active"
	MembershipStatusSuspended MembershipStatus = "suspended"
)

// MembershipStatuses คือค่าที่ถูกต้องทั้งหมด (ใช้ตรวจ input และ query filter)
var MembershipStatuses = []MembershipStatus{MembershipStatusActive, MembershipStatusSuspended}

func ParseMembershipStatus(raw string) (MembershipStatus, error) {
	for _, s := range MembershipStatuses {
		if string(s) == raw {
			return s, nil
		}
	}
	return "", domain.ErrInvalidMemberStatus
}
