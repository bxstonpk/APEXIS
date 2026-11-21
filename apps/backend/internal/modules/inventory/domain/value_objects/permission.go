package valueobjects

// Permission ของ inventory module (รูปแบบเดียวกับ identity: "<resource>:<action>")
type Permission string

const (
	PermUomRead         Permission = "inventory.uom:read"
	PermUomManage       Permission = "inventory.uom:manage"
	PermCategoryRead    Permission = "inventory.category:read"
	PermCategoryManage  Permission = "inventory.category:manage"
	PermItemRead        Permission = "inventory.item:read"
	PermItemManage      Permission = "inventory.item:manage"
	PermWarehouseRead   Permission = "inventory.warehouse:read"
	PermWarehouseManage Permission = "inventory.warehouse:manage"
	PermDocumentRead    Permission = "inventory.document:read"
	PermDocumentManage  Permission = "inventory.document:manage" // สร้าง / แก้ / ลบ draft
	PermDocumentPost    Permission = "inventory.document:post"   // ผ่านเอกสาร / ยกเลิก
	PermReportRead      Permission = "inventory.report:read"
	PermSettingsRead    Permission = "inventory.settings:read"
	PermSettingsManage  Permission = "inventory.settings:manage" // บัญชี / ระบบบัญชีสินค้า
	PermClosingRead     Permission = "inventory.closing:read"
	PermClosingPost     Permission = "inventory.closing:post" // ปรับปรุงสินค้าคงเหลือสิ้นงวด / ยกเลิก
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ inventory (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermUomRead,
	PermUomManage,
	PermCategoryRead,
	PermCategoryManage,
	PermItemRead,
	PermItemManage,
	PermWarehouseRead,
	PermWarehouseManage,
	PermDocumentRead,
	PermDocumentManage,
	PermDocumentPost,
	PermReportRead,
	PermSettingsRead,
	PermSettingsManage,
	PermClosingRead,
	PermClosingPost,
}
