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
	// สายจ้างบริการ: ใบสั่งจ้าง -> ใบตรวจรับงาน -> ซื้อบริการ (+ บันทึกใบกำกับภาษีที่ได้รับ)
	PermWorkOrderRead          Permission = "purchasing.work_order:read"
	PermWorkOrderManage        Permission = "purchasing.work_order:manage"
	PermWorkOrderApprove       Permission = "purchasing.work_order:approve"
	PermServiceAcceptanceRead  Permission = "purchasing.service_acceptance:read"
	PermServiceAcceptancePost  Permission = "purchasing.service_acceptance:post"
	PermServiceInvoiceRead     Permission = "purchasing.service_invoice:read"
	PermServiceInvoicePost     Permission = "purchasing.service_invoice:post" // ซื้อบริการ / บันทึกใบกำกับภาษีที่ได้รับ / ยกเลิก
	PermReportRead             Permission = "purchasing.report:read"
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ purchasing (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermSettingsRead,
	PermSettingsManage,
	PermSupplierRead,
	PermSupplierManage,
	PermOrderRead,
	PermOrderManage,
	PermOrderApprove,
	PermReceiptRead,
	PermReceiptPost,
	PermInvoiceRead,
	PermInvoicePost,
	PermWorkOrderRead,
	PermWorkOrderManage,
	PermWorkOrderApprove,
	PermServiceAcceptanceRead,
	PermServiceAcceptancePost,
	PermServiceInvoiceRead,
	PermServiceInvoicePost,
	PermReportRead,
}

// KindPerms คือสิทธิ์ของเอกสารแต่ละสาย (สินค้า / บริการ) — เอกสารใช้ตาราง / use case เดียวกัน แยกด้วย kind
type KindPerms struct {
	OrderRead, OrderManage, OrderApprove Permission
	ReceiptRead, ReceiptPost             Permission
	InvoiceRead, InvoicePost             Permission
}

func (k PurchaseKind) Perms() KindPerms {
	if k == KindService {
		return KindPerms{
			OrderRead: PermWorkOrderRead, OrderManage: PermWorkOrderManage, OrderApprove: PermWorkOrderApprove,
			ReceiptRead: PermServiceAcceptanceRead, ReceiptPost: PermServiceAcceptancePost,
			InvoiceRead: PermServiceInvoiceRead, InvoicePost: PermServiceInvoicePost,
		}
	}
	return KindPerms{
		OrderRead: PermOrderRead, OrderManage: PermOrderManage, OrderApprove: PermOrderApprove,
		ReceiptRead: PermReceiptRead, ReceiptPost: PermReceiptPost,
		InvoiceRead: PermInvoiceRead, InvoicePost: PermInvoicePost,
	}
}
