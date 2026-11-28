// Package gormpersistence คือ adapter ของ pettycash กับ gorm (transaction / migrate)
package gormpersistence

import (
	"context"

	"gorm.io/gorm"

	"apexis/backend/internal/modules/pettycash/application/ports"
	"apexis/backend/internal/modules/pettycash/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
)

var _ ports.TransactionManager = (*TransactionManager)(nil)

// TransactionManager เปิด transaction ให้ use case แล้วส่ง tx ผ่าน ctx (ใช้ร่วมกับ module อื่นได้ — เช่นใบจ่ายของการเงิน)
type TransactionManager struct{ db *gorm.DB }

func NewTransactionManager(db *gorm.DB) *TransactionManager { return &TransactionManager{db: db} }

func (tm *TransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return database.WithinTransaction(ctx, tm.db, fn)
}

// Migrate สร้างตารางของ pettycash
func Migrate(db *gorm.DB) error { return models.AutoMigrate(db) }
