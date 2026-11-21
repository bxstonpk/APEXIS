// Package ports คือ interface ที่ application ของ ledger ต้องการจากโลกภายนอก
// implementation อยู่ใน infrastructure (repository อยู่ใน ports/repositories)
package ports

import (
	"context"
	"time"

	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// TransactionManager: repository ทุกตัว (รวมถึงที่ module อื่นเรียกผ่าน ctx) ใช้ transaction เดียวกัน
// ถ้า fn คืน error จะ rollback ทั้งหมด
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Authorizer ตรวจสิทธิ์ของ user ในบริษัท (implementation เรียก identity module)
// คืน domain.ErrForbidden เมื่อไม่มีสิทธิ์
type Authorizer interface {
	Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error
}

// Period คืองวดบัญชีของ organization ที่ครอบคลุมวันที่หนึ่ง
type Period struct {
	ID         vo.FiscalPeriodID
	FiscalYear int
	YearStart  time.Time // วันเริ่มปีบัญชี — ยอดรายได้ / ค่าใช้จ่ายเริ่มนับจากวันนี้
	Open       bool
}

// Dimensions คือมิติวิเคราะห์ที่บรรทัดรายการอ้างถึง (id ของ organization)
type Dimensions struct {
	Branches     []vo.BranchID
	Departments  []vo.DepartmentID
	CostCenters  []vo.CostCenterID
	ProjectSites []vo.ProjectSiteID
}

// CompanySettings คือค่าตั้งของบริษัทที่ ledger ใช้ตอนเขียนข้อมูล
type CompanySettings struct {
	Active         bool
	AmountDecimals int
}

// Organization คือสิ่งที่ ledger ต้องการจาก organization module (เข้าร่วม transaction ใน ctx)
type Organization interface {
	// CompanySettings: สถานะ (inactive = อ่านอย่างเดียว) และจำนวนทศนิยมของเงินที่บริษัทกำหนด
	CompanySettings(ctx context.Context, companyID vo.CompanyID) (CompanySettings, error)
	// PeriodAt คืน domain.ErrNoFiscalPeriod ถ้าไม่มีงวดครอบคลุมวันนั้น
	// ภายใน transaction จะ lock แถวงวดจน commit — การปิดงวดพร้อมกันต้องรอรายการที่กำลังผ่าน
	PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (Period, error)
	// MissingDimensions คืนชนิดมิติที่มี id ไม่พบ เช่น ["branch"]
	MissingDimensions(ctx context.Context, companyID vo.CompanyID, d Dimensions) ([]string, error)
}
