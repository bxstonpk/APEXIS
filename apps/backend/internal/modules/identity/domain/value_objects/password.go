package valueobjects

import domain "apexis/backend/internal/modules/identity/domain/errors"

const (
	PasswordMinLength = 8
	PasswordMaxLength = 72 // ข้อจำกัดของ bcrypt
)

// ValidatePassword ตรวจ policy ของรหัสผ่าน (plain text, ก่อน hash)
func ValidatePassword(plain string) error {
	if len(plain) < PasswordMinLength || len(plain) > PasswordMaxLength {
		return domain.ErrInvalidPassword
	}
	return nil
}
