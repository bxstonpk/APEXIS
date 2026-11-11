package models

import (
	"time"

	"github.com/google/uuid"
)

type MembershipModel struct {
	ID        uuid.UUID             `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:uq_identity_memberships_company_user,priority:1"`
	UserID    uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:uq_identity_memberships_company_user,priority:2;index:idx_identity_memberships_user"`
	User      *UserModel            `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT"`
	Status    string                `gorm:"size:20;not null"`
	Roles     []MembershipRoleModel `gorm:"foreignKey:MembershipID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time             `gorm:"not null"`
	UpdatedAt time.Time             `gorm:"not null"`
}

func (MembershipModel) TableName() string { return "identity_memberships" }

type MembershipRoleModel struct {
	MembershipID uuid.UUID  `gorm:"type:uuid;primaryKey"`
	RoleID       uuid.UUID  `gorm:"type:uuid;primaryKey;index:idx_identity_membership_roles_role"`
	Role         *RoleModel `gorm:"foreignKey:RoleID;constraint:OnDelete:RESTRICT"` // ลบ role ที่ยังมีคนใช้ไม่ได้
}

func (MembershipRoleModel) TableName() string { return "identity_membership_roles" }
