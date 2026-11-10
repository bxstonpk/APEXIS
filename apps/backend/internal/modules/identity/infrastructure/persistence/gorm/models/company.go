package models

import (
	"time"

	"github.com/google/uuid"
)

type CompanyModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code      string    `gorm:"size:20;not null;uniqueIndex:uq_identity_companies_code"`
	Name      string    `gorm:"size:200;not null"`
	TaxID     string    `gorm:"size:20;not null;default:''"`
	Status    string    `gorm:"size:20;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (CompanyModel) TableName() string { return "identity_companies" }
