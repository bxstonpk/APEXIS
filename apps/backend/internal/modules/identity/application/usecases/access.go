package usecases

import (
	"context"
	"errors"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/application/ports/repositories"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

// AccessResolver โหลดข้อมูลสิทธิ์จาก repository (1 query) แล้วให้ domain ตัดสิน
// ใช้ทั้งใน use case และ HTTP middleware ของทุก module
type AccessResolver struct {
	access repositories.AccessRepository
}

// Resolve คืนสิทธิ์ของ user ในบริษัท
// ถ้าไม่ได้เป็นสมาชิก หรือไม่มีบริษัทนี้ จะคืน ErrForbidden เหมือนกัน
// เพื่อไม่ให้คนนอกเดาได้ว่ามีบริษัทนี้อยู่หรือไม่
func (r *AccessResolver) Resolve(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (services.AccessContext, error) {
	facts, err := r.access.FindAccessFacts(ctx, userID, companyID)
	if errors.Is(err, domain.ErrMembershipNotFound) {
		return services.AccessContext{}, domain.ErrForbidden
	}
	if err != nil {
		return services.AccessContext{}, err
	}
	return services.ResolveAccess(facts)
}

// Authorize = Resolve + Require(perm)
func (r *AccessResolver) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) (services.AccessContext, error) {
	access, err := r.Resolve(ctx, userID, companyID)
	if err != nil {
		return services.AccessContext{}, err
	}
	if err := access.Require(perm); err != nil {
		return services.AccessContext{}, err
	}
	return access, nil
}

// GetMine คืนสิทธิ์ของฉันในบริษัท (frontend ใช้ซ่อน/แสดงเมนู)
func (r *AccessResolver) GetMine(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (dto.Access, error) {
	a, err := r.Resolve(ctx, userID, companyID)
	if err != nil {
		return dto.Access{}, err
	}
	return dto.FromAccess(a), nil
}
