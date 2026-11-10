// Package services คือ implementation ของ application/ports ที่ไม่ใช่ persistence
package services

import (
	"golang.org/x/crypto/bcrypt"

	"account/backend/internal/modules/identity/application/ports"
)

var _ ports.PasswordHasher = (*BcryptHasher)(nil)

type BcryptHasher struct{ cost int }

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	return string(b), err
}

func (h *BcryptHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
