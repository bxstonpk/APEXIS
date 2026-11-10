// Package repositories กำหนดสัญญา (port) ที่ domain ต้องการ
// implementation อยู่ใน infrastructure/persistence/gorm/repositories
//
// แยกเป็น package ย่อยแทน domain/repository.go เพราะต้อง import entities
// และ entities import domain (errors) — ถ้าอยู่ใน package domain จะเกิด import cycle
//
// กติกา:
//   - ทุก method ที่เกี่ยวกับข้อมูลของบริษัทต้องรับ CompanyID (tenant isolation)
//   - Find* คืน domain.Err*NotFound เมื่อไม่พบ
//   - Create คืน domain.Err*Exists เมื่อชน unique
package repositories

import (
	"context"

	"account/backend/internal/modules/identity/domain/entities"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type UserRepository interface {
	Create(ctx context.Context, u *entities.User) error
	Update(ctx context.Context, u *entities.User) error
	FindByID(ctx context.Context, id vo.UserID) (*entities.User, error)
	FindByEmail(ctx context.Context, email vo.Email) (*entities.User, error)
	FindByIDs(ctx context.Context, ids []vo.UserID) ([]*entities.User, error)
}

type CompanyRepository interface {
	Create(ctx context.Context, c *entities.Company) error
	Update(ctx context.Context, c *entities.Company) error
	FindByID(ctx context.Context, id vo.CompanyID) (*entities.Company, error)
	FindByCode(ctx context.Context, code string) (*entities.Company, error)
	FindByIDs(ctx context.Context, ids []vo.CompanyID) ([]*entities.Company, error)
}

type RoleRepository interface {
	Create(ctx context.Context, r *entities.Role) error
	Update(ctx context.Context, r *entities.Role) error
	// Delete คืน domain.ErrRoleInUse ถ้ายังมี member ถือ role นี้อยู่
	Delete(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) error
	FindByID(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) (*entities.Role, error)
	// FindByIDs คืนเฉพาะ role ของ companyID (id ที่ไม่พบจะไม่อยู่ในผลลัพธ์)
	FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error)
	ListByCompany(ctx context.Context, companyID vo.CompanyID) ([]*entities.Role, error)
}

type MembershipRepository interface {
	Create(ctx context.Context, m *entities.Membership) error
	Update(ctx context.Context, m *entities.Membership) error
	Find(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) (*entities.Membership, error)
	ListByUser(ctx context.Context, userID vo.UserID) ([]*entities.Membership, error)
	ListByCompany(ctx context.Context, companyID vo.CompanyID) ([]*entities.Membership, error)
}
