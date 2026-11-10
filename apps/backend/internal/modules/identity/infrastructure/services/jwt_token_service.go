package services

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"account/backend/internal/modules/identity/application/ports"
	"account/backend/internal/modules/identity/domain"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

var _ ports.TokenService = (*JWTTokenService)(nil)

// JWTTokenService ออก access token (HS256) ที่มีแค่ user id
// token ไม่ผูกกับบริษัท — สิทธิ์ต่อบริษัท resolve จาก DB ทุก request
// ทำให้เปลี่ยน role/ระงับพนักงานแล้วมีผลทันทีโดยไม่ต้องรอ token หมดอายุ
type JWTTokenService struct {
	secret []byte
	ttl    time.Duration
	issuer string
	now    func() time.Time
}

func NewJWTTokenService(secret []byte, ttl time.Duration, issuer string) (*JWTTokenService, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &JWTTokenService{secret: secret, ttl: ttl, issuer: issuer, now: time.Now}, nil
}

func (s *JWTTokenService) Issue(userID vo.UserID) (string, time.Time, error) {
	now := s.now()
	exp := now.Add(s.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    s.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
		ID:        uuid.NewString(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return token, exp, err
}

func (s *JWTTokenService) Verify(token string) (vo.UserID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return vo.UserID{}, domain.ErrUnauthenticated
	}
	id, err := vo.ParseUserID(claims.Subject)
	if err != nil {
		return vo.UserID{}, domain.ErrUnauthenticated
	}
	return id, nil
}
