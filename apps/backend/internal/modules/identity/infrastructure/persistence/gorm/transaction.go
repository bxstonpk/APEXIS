package gormpersistence

import (
	"context"

	"gorm.io/gorm"

	"apexis/backend/internal/modules/identity/application/ports"
	"apexis/backend/internal/platform/database"
)

var _ ports.TransactionManager = (*TransactionManager)(nil)

// TransactionManager เปิด transaction ให้ use case แล้วส่ง tx ผ่าน ctx
// repository ทุกตัวได้ tx เดียวกันผ่าน database.Conn / database.TxConn
//
// ctx key อยู่ที่ platform/database เพื่อให้ repository ของ module อื่น
// เข้าร่วม transaction เดียวกันได้เมื่อ business operation ข้าม module
type TransactionManager struct{ db *gorm.DB }

func NewTransactionManager(db *gorm.DB) *TransactionManager { return &TransactionManager{db: db} }

func (tm *TransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return database.WithinTransaction(ctx, tm.db, fn)
}
