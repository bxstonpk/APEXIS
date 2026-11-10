// Package handlers คือ HTTP handler ของ identity — แปลง request -> command/query -> response
// ไม่มี business logic และไม่เรียก repository ตรง
package handlers

import (
	"net/http"

	"account/backend/internal/modules/identity/application/commands"
	"account/backend/internal/modules/identity/application/queries"
	"account/backend/internal/modules/identity/domain"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

type Handler struct {
	commands *commands.Handler
	queries  *queries.Handler
}

func NewHandler(c *commands.Handler, q *queries.Handler) *Handler {
	return &Handler{commands: c, queries: q}
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
