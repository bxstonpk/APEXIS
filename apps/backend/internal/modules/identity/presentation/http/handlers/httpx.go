package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"account/backend/internal/modules/identity/domain"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
	"account/backend/internal/modules/identity/presentation/http/responses"
)

const maxBodyBytes = 1 << 20 // 1 MB

var errBadJSON = errors.New("invalid json body")

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", errBadJSON, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: unexpected data after json object", errBadJSON)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeData[T any](w http.ResponseWriter, status int, data T) {
	writeJSON(w, status, responses.Data[T]{Data: data})
}

// errorStatus แปลง domain error เป็น HTTP status
var errorStatus = []struct {
	err    error
	status int
}{
	// 400
	{errBadJSON, http.StatusBadRequest},
	{domain.ErrInvalidUserID, http.StatusBadRequest},
	{domain.ErrInvalidName, http.StatusBadRequest},
	{domain.ErrInvalidEmail, http.StatusBadRequest},
	{domain.ErrInvalidPassword, http.StatusBadRequest},
	{domain.ErrInvalidCompanyID, http.StatusBadRequest},
	{domain.ErrInvalidCompanyCode, http.StatusBadRequest},
	{domain.ErrInvalidCompanyName, http.StatusBadRequest},
	{domain.ErrInvalidTaxID, http.StatusBadRequest},
	{domain.ErrInvalidRoleID, http.StatusBadRequest},
	{domain.ErrInvalidRoleCode, http.StatusBadRequest},
	{domain.ErrInvalidRoleName, http.StatusBadRequest},
	{domain.ErrInvalidPermission, http.StatusBadRequest},
	// 401
	{domain.ErrUnauthenticated, http.StatusUnauthorized},
	{domain.ErrInvalidCredentials, http.StatusUnauthorized},
	// 403
	{domain.ErrForbidden, http.StatusForbidden},
	{domain.ErrUserInactive, http.StatusForbidden},
	{domain.ErrCompanyInactive, http.StatusForbidden},
	{domain.ErrMembershipInactive, http.StatusForbidden},
	// 404
	{domain.ErrUserNotFound, http.StatusNotFound},
	{domain.ErrCompanyNotFound, http.StatusNotFound},
	{domain.ErrRoleNotFound, http.StatusNotFound},
	{domain.ErrMembershipNotFound, http.StatusNotFound},
	// 409
	{domain.ErrEmailAlreadyExists, http.StatusConflict},
	{domain.ErrCompanyCodeExists, http.StatusConflict},
	{domain.ErrRoleCodeExists, http.StatusConflict},
	{domain.ErrMembershipExists, http.StatusConflict},
	{domain.ErrRoleInUse, http.StatusConflict},
	// 422
	{domain.ErrSystemRoleImmutable, http.StatusUnprocessableEntity},
	{domain.ErrRoleCompanyMismatch, http.StatusUnprocessableEntity},
	{domain.ErrCannotModifySelf, http.StatusUnprocessableEntity},
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	for _, e := range errorStatus {
		if errors.Is(err, e.err) {
			msg := e.err.Error()
			if e.err == errBadJSON {
				msg = err.Error()
			}
			writeJSON(w, e.status, responses.Error{Error: responses.ErrorBody{Code: errorCode(e.err), Message: msg}})
			return
		}
	}
	slog.ErrorContext(r.Context(), "unhandled error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeJSON(w, http.StatusInternalServerError, responses.Error{Error: responses.ErrorBody{Code: "internal_error", Message: "internal server error"}})
}

// "invalid company id" -> "invalid_company_id"
func errorCode(err error) string {
	return strings.ReplaceAll(err.Error(), " ", "_")
}

func pathCompanyID(r *http.Request) (vo.CompanyID, error) {
	return vo.ParseCompanyID(r.PathValue("companyID"))
}

func pathRoleID(r *http.Request) (vo.RoleID, error) { return vo.ParseRoleID(r.PathValue("roleID")) }

func pathUserID(r *http.Request) (vo.UserID, error) { return vo.ParseUserID(r.PathValue("userID")) }
