package models

import "gorm.io/gorm"

// PermissionRename คือการแปลงสิทธิ์ของบทบาท: From -> To (Keep = คงสิทธิ์เดิมไว้ด้วย)
type PermissionRename struct {
	From string
	To   []string
	Keep bool
}

// RenamePermissions แปลงสิทธิ์ของทุกบทบาทใน transaction เดียว — ทำซ้ำได้ (บทบาทที่มีสิทธิ์ใหม่อยู่แล้วไม่ซ้ำ)
// ใช้ตอนความรับผิดชอบย้าย module (เช่นรับ / จ่ายเงินย้ายไป finance) เพื่อให้ผู้ใช้เดิมยังทำงานได้
func RenamePermissions(db *gorm.DB, rules []PermissionRename) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rules {
			for _, to := range r.To {
				if err := tx.Exec(`INSERT INTO identity_role_permissions (role_id, permission)
					SELECT role_id, ? FROM identity_role_permissions WHERE permission = ?
					ON CONFLICT DO NOTHING`, to, r.From).Error; err != nil {
					return err
				}
			}
			if !r.Keep {
				if err := tx.Exec(`DELETE FROM identity_role_permissions WHERE permission = ?`, r.From).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
