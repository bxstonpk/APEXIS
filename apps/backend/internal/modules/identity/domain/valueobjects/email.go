package valueobjects

import (
	"net/mail"
	"strings"

	"account/backend/internal/modules/identity/domain"
)

// Email ถูก normalize เป็นตัวพิมพ์เล็กและตรวจรูปแบบแล้วเสมอ
type Email string

func NewEmail(raw string) (Email, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > 320 {
		return "", domain.ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return "", domain.ErrInvalidEmail
	}
	return Email(s), nil
}

func (e Email) String() string { return string(e) }
