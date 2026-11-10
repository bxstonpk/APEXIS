// Package ports คือ interface ที่ application ต้องการจากโลกภายนอก
// (นอกเหนือจาก repository ที่ domain กำหนด) — implementation อยู่ใน infrastructure
package ports

import (
	"context"
	"time"

	"account/backend/internal/modules/identity/domain/events"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

// TxManager: repository ทุกตัวที่ถูกเรียกภายใน fn จะใช้ transaction เดียวกันผ่าน ctx
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	// Compare คืน nil ถ้ารหัสผ่านตรงกับ hash
	Compare(hash, plain string) error
}

type TokenService interface {
	Issue(userID vo.UserID) (token string, expiresAt time.Time, err error)
	// Verify คืน domain.ErrUnauthenticated ถ้า token ไม่ถูกต้อง/หมดอายุ
	Verify(token string) (vo.UserID, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, evts ...events.Event)
}

type Clock func() time.Time
