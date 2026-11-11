package valueobjects

// Permission ของ ledger module (รูปแบบเดียวกับ identity: "<resource>:<action>")
// ตรวจสิทธิ์ผ่าน identity — role "owner" (สิทธิ์ "*") ได้ทุกข้อ
type Permission string

const (
	PermAccountRead   Permission = "ledger.account:read"
	PermAccountManage Permission = "ledger.account:manage"
	PermJournalRead   Permission = "ledger.journal:read"
	PermJournalManage Permission = "ledger.journal:manage"
	PermEntryRead     Permission = "ledger.entry:read"
	PermEntryManage   Permission = "ledger.entry:manage" // สร้าง / แก้ / ลบ draft
	PermEntryPost     Permission = "ledger.entry:post"   // ผ่านรายการ / กลับรายการ
	PermReportRead    Permission = "ledger.report:read"
)

func (p Permission) String() string { return string(p) }
