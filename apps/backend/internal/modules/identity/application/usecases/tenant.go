package usecases

import (
	"context"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

// TenantUseCases คือส่วนของ identity ที่ module เจ้าของบริษัท (organization) เรียกใช้
// ข้อมูลบริษัทอยู่ที่ organization — identity ดูแลเฉพาะ role / สมาชิก / สิทธิ์ของบริษัทนั้น
type TenantUseCases struct{ base }

// ProvisionOwner สร้าง system role "owner" ของบริษัทใหม่ และให้ผู้สร้างเป็นสมาชิกที่ถือ role นั้น
//
// ถูกเรียกภายใน transaction ที่ organization เปิดไว้ตอนสร้างบริษัท (เข้าร่วม transaction เดิมผ่าน ctx)
// จึงไม่ publish event — use case นี้ไม่รู้ว่า transaction ของผู้เรียกจะ commit หรือไม่
func (uc *TenantUseCases) ProvisionOwner(ctx context.Context, in dto.ProvisionOwnerInput) error {
	owner, err := entities.NewOwnerRole(in.CompanyID)
	if err != nil {
		return err
	}
	membership, err := entities.NewMembership(in.OwnerID, in.CompanyID, []*entities.Role{owner})
	if err != nil {
		return err
	}
	return uc.deps.Tx.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := uc.deps.Users.FindByID(ctx, in.OwnerID)
		if err != nil {
			return err
		}
		if !user.IsActive() {
			return domain.ErrUserInactive
		}
		if err := uc.deps.Roles.Create(ctx, owner); err != nil {
			return err
		}
		return uc.deps.Memberships.Create(ctx, membership)
	})
}

// CompanyIDsOf คืน id ของบริษัทที่ user เป็นสมาชิก (active) — organization ใช้แสดง "บริษัทของฉัน"
func (uc *TenantUseCases) CompanyIDsOf(ctx context.Context, userID vo.UserID) ([]vo.CompanyID, error) {
	return uc.deps.Memberships.CompanyIDsOf(ctx, userID)
}
