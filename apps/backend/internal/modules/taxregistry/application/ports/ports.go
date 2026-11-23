// Package ports คือ interface ที่ application ของ taxregistry ต้องการจากภายนอก
package ports

import (
	"context"

	"apexis/backend/internal/modules/taxregistry/domain/entities"
	vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"
)

// VatRegistry คือฐานข้อมูลผู้จด VAT ภายนอก (กรมสรรพากร) — ไม่พบ = slice ว่าง (ไม่ใช่ error), ติดต่อไม่ได้ = ErrUnavailable
type VatRegistry interface {
	Lookup(ctx context.Context, taxID, branchNo string) ([]entities.VatRegistrant, error)
}

// Authorizer: ผ่านเมื่อผู้ใช้มีสิทธิ์อย่างน้อยหนึ่งข้อในบริษัทนั้น
type Authorizer interface {
	AuthorizeAny(ctx context.Context, userID vo.UserID, companyID vo.CompanyID) error
}
