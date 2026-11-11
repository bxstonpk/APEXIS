// Package handlers คือ HTTP handler ของ identity — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/identity/application/usecases"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

type Handler struct {
	uc *usecases.UseCases
}

func NewHandler(uc *usecases.UseCases) *Handler {
	return &Handler{uc: uc}
}

// actor ดึง user id จาก context (route ต้องผ่าน Authenticate)
func actor(r *http.Request) (vo.UserID, error) {
	id, ok := UserIDFrom(r.Context())
	if !ok {
		return vo.UserID{}, domain.ErrUnauthenticated
	}
	return id, nil
}

// actorAndCompany ใช้กับ route ใต้ /companies/{companyID}
func actorAndCompany(r *http.Request) (vo.UserID, vo.CompanyID, error) {
	a, err := actor(r)
	if err != nil {
		return vo.UserID{}, vo.CompanyID{}, err
	}
	c, err := pathCompanyID(r)
	if err != nil {
		return vo.UserID{}, vo.CompanyID{}, err
	}
	return a, c, nil
}

// actorCompanyAndMember ใช้กับ route ใต้ /companies/{companyID}/members/{userID}
func actorCompanyAndMember(r *http.Request) (vo.UserID, vo.CompanyID, vo.UserID, error) {
	a, c, err := actorAndCompany(r)
	if err != nil {
		return vo.UserID{}, vo.CompanyID{}, vo.UserID{}, err
	}
	target, err := pathUserID(r)
	if err != nil {
		return vo.UserID{}, vo.CompanyID{}, vo.UserID{}, err
	}
	return a, c, target, nil
}
