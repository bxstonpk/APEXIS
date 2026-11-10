package models

import (
	"time"

	"github.com/google/uuid"
)

type RoleModel struct {
	ID          uuid.UUID             `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID             `gorm:"type:uuid;not null;uniqueIndex:uq_identity_roles_company_code,priority:1"`
	Company     *CompanyModel         `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code        string                `gorm:"size:50;not null;uniqueIndex:uq_identity_roles_company_code,priority:2"`
	Name        string                `gorm:"size:100;not null"`
	Description string                `gorm:"size:500;not null;default:''"`
	IsSystem    bool                  `gorm:"not null"`
	Permissions []RolePermissionModel `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE"`
	CreatedAt   time.Time             `gorm:"not null"`
	UpdatedAt   time.Time             `gorm:"not null"`
}

func (RoleModel) TableName() string { return "identity_roles" }

type RolePermissionModel struct {
	RoleID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Permission string    `gorm:"size:150;primaryKey"`
}

func (RolePermissionModel) TableName() string { return "identity_role_permissions" }
