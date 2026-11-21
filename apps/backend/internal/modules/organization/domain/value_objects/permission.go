package valueobjects

// Permission ของ organization module (รูปแบบเดียวกับ identity: "<resource>:<action>")
// ตรวจสิทธิ์ผ่าน identity — role "owner" (สิทธิ์ "*") ได้ทุกข้อ
type Permission string

const (
	PermCompanyRead       Permission = "organization.company:read"
	PermCompanyManage     Permission = "organization.company:manage"
	PermBranchRead        Permission = "organization.branch:read"
	PermBranchManage      Permission = "organization.branch:manage"
	PermBusinessUnitRead  Permission = "organization.business_unit:read"
	PermBusinessUnitMng   Permission = "organization.business_unit:manage"
	PermDepartmentRead    Permission = "organization.department:read"
	PermDepartmentManage  Permission = "organization.department:manage"
	PermCostCenterRead    Permission = "organization.cost_center:read"
	PermCostCenterManage  Permission = "organization.cost_center:manage"
	PermProjectSiteRead   Permission = "organization.project_site:read"
	PermProjectSiteManage Permission = "organization.project_site:manage"
	PermFiscalPeriodRead  Permission = "organization.fiscal_period:read"
	PermFiscalPeriodMng   Permission = "organization.fiscal_period:manage" // สร้างปีบัญชี
	PermFiscalPeriodClose Permission = "organization.fiscal_period:close"  // ปิด / เปิดงวด
	PermNumberingRead     Permission = "organization.numbering:read"
	PermNumberingManage   Permission = "organization.numbering:manage" // รูปแบบเลขที่เอกสาร
)

func (p Permission) String() string { return string(p) }
