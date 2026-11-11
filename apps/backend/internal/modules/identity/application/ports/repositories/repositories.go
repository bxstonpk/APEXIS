// Package repositories คือสัญญา (port) ที่ application ต้องการจาก persistence
// implementation อยู่ใน infrastructure/persistence/gorm/repositories
//
// กติกา:
//   - ทุก method ที่เกี่ยวกับข้อมูลของบริษัทต้องรับ CompanyID (tenant isolation)
//   - Find* คืน domain.Err*NotFound เมื่อไม่พบ
//   - Create คืน domain.Err*Exists เมื่อชน unique
//   - List* ทำ filter / sort / pagination ใน database และคืนเป็น read DTO (ไม่ใช่ entity)
//   - ห้ามเปิด transaction เอง — ใช้ transaction ที่ use case เปิดไว้ใน ctx
package repositories

import (
	"context"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/domain/entities"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type UserRepository interface {
	Create(ctx context.Context, u *entities.User) error
	Update(ctx context.Context, u *entities.User) error
	FindByID(ctx context.Context, id vo.UserID) (*entities.User, error)
	FindByEmail(ctx context.Context, email vo.Email) (*entities.User, error)
}

type RoleRepository interface {
	Create(ctx context.Context, r *entities.Role) error
	Update(ctx context.Context, r *entities.Role) error
	// Delete คืน domain.ErrRoleInUse ถ้ายังมี member ถือ role นี้อยู่
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) (*entities.Role, error)
	// FindByIDs คืนเฉพาะ role ของ companyID (id ที่ไม่พบจะไม่อยู่ในผลลัพธ์)
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.RoleFilter) (listing.Result[dto.Role], error)
}

type MembershipRepository interface {
	Create(ctx context.Context, m *entities.Membership) error
	Update(ctx context.Context, m *entities.Membership) error
	Find(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) (*entities.Membership, error)
	List(ctx context.Context, companyID vo.CompanyID, f dto.MemberFilter) (listing.Result[dto.Member], error)
	// CompanyIDsOf คืน id ของบริษัทที่ user มี membership ที่ active
	CompanyIDsOf(ctx context.Context, userID vo.UserID) ([]vo.CompanyID, error)
}

// AccessRepository โหลดข้อมูลสำหรับตัดสินสิทธิ์ใน query เดียว
// (ถูกเรียกทุก request ที่อยู่ใต้ /companies/{companyID} จึงต้องเร็ว)
type AccessRepository interface {
	// FindAccessFacts คืน domain.ErrMembershipNotFound ถ้า user ไม่ได้เป็นสมาชิกบริษัทนี้
	FindAccessFacts(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) (services.AccessFacts, error)
}
