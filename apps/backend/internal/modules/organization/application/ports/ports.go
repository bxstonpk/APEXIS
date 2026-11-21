// Package ports คือ interface ที่ application ของ organization ต้องการจากโลกภายนอก
// implementation อยู่ใน infrastructure (repository อยู่ใน ports/repositories)
package ports

import (
	"context"

	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// TransactionManager: repository ทุกตัวที่ถูกเรียกภายใน fn จะใช้ transaction เดียวกันผ่าน ctx
// ถ้า fn คืน error จะ rollback ทั้งหมด
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Membership คือสิ่งที่ organization ต้องการจาก identity เกี่ยวกับสมาชิกของบริษัท
type Membership interface {
	// ProvisionOwner ให้ userID เป็น owner (สิทธิ์ทุกอย่าง) ของบริษัทที่เพิ่งสร้าง
	// ต้องเข้าร่วม transaction ใน ctx; คืน domain.ErrForbidden ถ้าผู้ใช้ไม่ active
	ProvisionOwner(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) error
	// CompanyIDsOf คืน id ของบริษัทที่ user เป็นสมาชิก (active)
	CompanyIDsOf(ctx context.Context, userID vo.UserID) ([]vo.CompanyID, error)
}

// Authorizer ตรวจสิทธิ์ของ user ในบริษัท (implementation เรียก identity module)
// คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์ — เข้าร่วม transaction ใน ctx ได้
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}
