package gormpersistence

import (
	"context"

	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/application/ports"
	"apexis/backend/internal/platform/database"
)

var _ ports.TransactionManager = (*TransactionManager)(nil)

// TransactionManager เปิด transaction ให้ use case แล้วส่ง tx ผ่าน ctx
// ctx key อยู่ที่ platform/database จึงใช้ transaction เดียวกับ identity / organization ได้
// (ตรวจสิทธิ์, ตรวจงวดบัญชี + lock งวด อยู่ใน transaction เดียวกับการผ่านรายการ)
type TransactionManager struct{ db *gorm.DB }

func NewTransactionManager(db *gorm.DB) *TransactionManager { return &TransactionManager{db: db} }

func (tm *TransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return database.WithinTransaction(ctx, tm.db, fn)
}
