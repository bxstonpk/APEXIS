// Package ports คือ interface ที่ application ต้องการจากโลกภายนอก
// implementation อยู่ใน infrastructure (repository อยู่ใน ports/repositories)
package ports

import (
	"context"
	"time"

	"apexis/backend/internal/modules/identity/domain/events"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

// TransactionManager: repository ทุกตัวที่ถูกเรียกภายใน fn จะใช้ transaction เดียวกันผ่าน ctx
// ถ้า fn คืน error จะ rollback ทั้งหมด
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
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
