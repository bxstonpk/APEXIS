package handlers

import (
	"net/http"

	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	"apexis/backend/internal/modules/identity/presentation/http/requests"
	"apexis/backend/internal/platform/httpx"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return httpx.DecodeJSON(w, r, dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) { httpx.WriteJSON(w, status, v) }

func writeData[T any](w http.ResponseWriter, status int, data T) { httpx.WriteData(w, status, data) }

func writeError(w http.ResponseWriter, r *http.Request, err error) { errorMapper.Write(w, r, err) }

// errorMapper แปลง domain error เป็น HTTP status
var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	// 400
	{Err: requests.ErrValidation, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidMemberStatus, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidUserID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidEmail, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidPassword, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCompanyID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidRoleID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidRoleCode, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidRoleName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidPermission, Status: http.StatusBadRequest},
	// 401
	{Err: domain.ErrUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrInvalidCredentials, Status: http.StatusUnauthorized},
	// 403
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrUserInactive, Status: http.StatusForbidden},
	{Err: domain.ErrMembershipInactive, Status: http.StatusForbidden},
	// 404
	{Err: domain.ErrUserNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrRoleNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrMembershipNotFound, Status: http.StatusNotFound},
	// 409
	{Err: domain.ErrEmailAlreadyExists, Status: http.StatusConflict},
	{Err: domain.ErrRoleCodeExists, Status: http.StatusConflict},
	{Err: domain.ErrMembershipExists, Status: http.StatusConflict},
	{Err: domain.ErrRoleInUse, Status: http.StatusConflict},
	// 422
	{Err: domain.ErrSystemRoleImmutable, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrRoleCompanyMismatch, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrCannotModifySelf, Status: http.StatusUnprocessableEntity},
}, requests.ErrValidation)

func pathCompanyID(r *http.Request) (vo.CompanyID, error) {
	return vo.ParseCompanyID(r.PathValue("companyID"))
}

func pathRoleID(r *http.Request) (vo.RoleID, error) { return vo.ParseRoleID(r.PathValue("roleID")) }

func pathUserID(r *http.Request) (vo.UserID, error) { return vo.ParseUserID(r.PathValue("userID")) }
