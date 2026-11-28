package valueobjects

// Permission ของ manufacturing module (รูปแบบเดียวกับ identity: "<resource>:<action>")
type Permission string

const (
	PermSettingsRead   Permission = "manufacturing.settings:read"
	PermSettingsManage Permission = "manufacturing.settings:manage"
	PermBomRead        Permission = "manufacturing.bom:read"
	PermBomManage      Permission = "manufacturing.bom:manage"
	PermOrderRead      Permission = "manufacturing.order:read"
	PermOrderManage    Permission = "manufacturing.order:manage" // สร้าง / แก้ / ปล่อย / เบิก / คืน / ปิดงาน / ยกเลิก
	PermReportRead     Permission = "manufacturing.report:read"
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ manufacturing (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermSettingsRead,
	PermSettingsManage,
	PermBomRead,
	PermBomManage,
	PermOrderRead,
	PermOrderManage,
	PermReportRead,
}
