package valueobjects

// Permission ของ pettycash module (รูปแบบเดียวกับ identity: "<resource>:<action>")
type Permission string

const (
	PermFundRead          Permission = "petty_cash.fund:read"
	PermFundManage        Permission = "petty_cash.fund:manage" // กองทุน + ค่าตั้ง
	PermVoucherRead       Permission = "petty_cash.voucher:read"
	PermVoucherManage     Permission = "petty_cash.voucher:manage"  // สร้าง / แก้ / ยกเลิกร่าง
	PermVoucherApprove    Permission = "petty_cash.voucher:approve" // อนุมัติ / ยกเลิกที่อนุมัติแล้ว
	PermReplenishmentRead Permission = "petty_cash.replenishment:read"
	PermReplenishmentPost Permission = "petty_cash.replenishment:post" // ขอตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย / ยกเลิกคำขอ
	PermReportRead        Permission = "petty_cash.report:read"
)

func (p Permission) String() string { return string(p) }

// AllPermissions คือสิทธิ์ทั้งหมดของ pettycash (ลงทะเบียนกับ platform/permissions ผ่าน Module.Permissions)
var AllPermissions = []Permission{
	PermFundRead,
	PermFundManage,
	PermVoucherRead,
	PermVoucherManage,
	PermVoucherApprove,
	PermReplenishmentRead,
	PermReplenishmentPost,
	PermReportRead,
}
