package models

import "gorm.io/gorm"

// AutoMigrate สร้าง/อัปเดตตารางของ identity
// สำหรับ production แนะนำเปลี่ยนเป็น versioned migration (เช่น goose / golang-migrate)
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&UserModel{},
		&RoleModel{},
		&RolePermissionModel{},
		&MembershipModel{},
		&MembershipRoleModel{},
	)
}
