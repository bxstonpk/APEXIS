package gormpersistence

import (
	"gorm.io/gorm"

	"apexis/backend/internal/modules/manufacturing/infrastructure/persistence/gorm/models"
)

// Migrate สร้างตารางของ manufacturing
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
