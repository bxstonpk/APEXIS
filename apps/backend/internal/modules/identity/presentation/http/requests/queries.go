package requests

import (
	"net/url"

	"apexis/backend/internal/modules/identity/application/dto"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// การค้นหาทุกหน้าใช้ GET + URL query เท่านั้น
// frontend เก็บ query เดียวกันไว้ใน URL ของหน้า แล้วส่งต่อให้ API ตรงๆ
// เพื่อให้กด Back / refresh / bookmark แล้วได้หน้าการค้นหาเดิม
//
// parameter ที่ไม่รู้จักถูกเพิกเฉย, ค่าที่ไม่ถูกต้องของ parameter ที่รู้จัก -> 400

// ParseMemberFilter อ่าน
//
//	GET /api/v1/companies/{companyID}/members?q=&status=active,suspended&role_id=&sort=-joined_at&page=1&page_size=20
func ParseMemberFilter(v url.Values) (dto.MemberFilter, error) {
	var (
		f   dto.MemberFilter
		err error
	)
	if f.Search, err = listing.ParseSearch(v); err != nil {
		return f, err
	}
	if f.Statuses, err = listing.ParseList(v, "status", vo.ParseMembershipStatus); err != nil {
		return f, err
	}
	if f.RoleID, err = listing.ParseOptional(v, "role_id", vo.ParseRoleID); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, dto.MemberSortKeys, dto.DefaultMemberSort); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}

// ParseRoleFilter อ่าน
//
//	GET /api/v1/companies/{companyID}/roles?q=&is_system=true|false&sort=code&page=1&page_size=20
func ParseRoleFilter(v url.Values) (dto.RoleFilter, error) {
	var (
		f   dto.RoleFilter
		err error
	)
	if f.Search, err = listing.ParseSearch(v); err != nil {
		return f, err
	}
	if f.IsSystem, err = listing.ParseOptional(v, "is_system", listing.ParseBool); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, dto.RoleSortKeys, dto.DefaultRoleSort); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}
