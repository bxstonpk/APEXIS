package valueobjects

// Permission ของ purchasing module (รูปแบบเดียวกับ identity: "<resource>:<action>")
type Permission string

const (
	PermSettingsRead   Permission = "purchasing.settings:read"
	PermSettingsManage Permission = "purchasing.settings:manage"
	PermSupplierRead   Permission = "purchasing.supplier:read"
	PermSupplierManage Permission = "purchasing.supplier:manage"
	PermOrderRead      Permission = "purchasing.order:read"
	PermOrderManage    Permission = "purchasing.order:manage"  // สร้าง / แก้ / ลบ draft, ปิด / ยกเลิก
	PermOrderApprove   Permission = "purchasing.order:approve" // อนุมัติ (ออกเลขที่)
	PermReceiptRead    Permission = "purchasing.receipt:read"
	PermReceiptPost    Permission = "purchasing.receipt:post" // รับสินค้า / ยกเลิก (ลงบัญชี + สต็อก)
	PermInvoiceRead    Permission = "purchasing.invoice:read"
	PermInvoicePost    Permission = "purchasing.invoice:post" // บันทึกใบแจ้งหนี้ / ยกเลิก (ลงบัญชี)
	PermReportRead     Permission = "purchasing.report:read"
)

func (p Permission) String() string { return string(p) }
