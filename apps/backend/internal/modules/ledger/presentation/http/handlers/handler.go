// Package handlers คือ HTTP handler ของ ledger — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/usecases"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/modules/ledger/presentation/http/requests"
	"apexis/backend/internal/platform/httpx"
)

// CurrentUser คืน user id ของ request ที่ผ่าน Authenticate แล้ว (มาจาก identity module)
type CurrentUser func(ctx context.Context) (uuid.UUID, bool)

type Handler struct {
	uc          *usecases.UseCases
	currentUser CurrentUser
}

func NewHandler(uc *usecases.UseCases, currentUser CurrentUser) *Handler {
	return &Handler{uc: uc, currentUser: currentUser}
}

var errUnauthenticated = errors.New("unauthenticated")

var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	// 400
	{Err: requests.ErrValidation, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCode, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidText, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidStatus, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAccountType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAmount, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidLine, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidDate, Status: http.StatusBadRequest},
	// 401 / 403
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	// 404
	{Err: domain.ErrAccountNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrJournalNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrEntryNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrDimensionNotFound, Status: http.StatusNotFound},
	// 409
	{Err: domain.ErrCodeExists, Status: http.StatusConflict},
	{Err: domain.ErrInUse, Status: http.StatusConflict},
	// 422
	{Err: domain.ErrCompanyInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrHierarchyCycle, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrParentNotHeader, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrParentTypeMismatch, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrAccountHasChildren, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrAccountNotPostable, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrJournalInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrEntryNotDraft, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrEntryNotPosted, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrEntryIncomplete, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrUnbalanced, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrNoFiscalPeriod, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrRangeCrossesYear, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrPeriodClosed, Status: http.StatusUnprocessableEntity},
}, requests.ErrValidation, domain.ErrInvalidAmount, domain.ErrInvalidLine,
	domain.ErrAccountNotFound, domain.ErrDimensionNotFound, domain.ErrAccountNotPostable)

// serve: อ่าน actor (user + {companyID}) -> fn -> เขียน response
// fn คืน body ที่ห่อ envelope แล้ว (one / httpx.ToList) หรือ nil สำหรับ 204
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, status int, fn func(a dto.Actor) (any, error)) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := pathID[vo.CompanyID](r, "companyID")
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	body, err := fn(dto.Actor{UserID: vo.UserID(userID), CompanyID: companyID})
	switch {
	case err != nil:
		errorMapper.Write(w, r, err)
	case status == http.StatusNoContent:
		w.WriteHeader(status)
	default:
		httpx.WriteJSON(w, status, body)
	}
}

// one ห่อ resource เดียวด้วย {"data": ...}
func one[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return httpx.Data[T]{Data: v}, nil
}

func pathID[T ~[16]byte](r *http.Request, name string) (T, error) {
	return vo.ParseID[T](r.PathValue(name))
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	err := httpx.DecodeJSON(w, r, &v)
	return v, err
}
