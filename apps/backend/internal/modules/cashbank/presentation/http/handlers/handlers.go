// Package handlers คือ HTTP handler ของ cashbank — ไม่มี business logic
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/cashbank/application/dto"
	"apexis/backend/internal/modules/cashbank/application/usecases"
	domain "apexis/backend/internal/modules/cashbank/domain/errors"
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	"apexis/backend/internal/modules/cashbank/presentation/http/requests"
	"apexis/backend/internal/modules/cashbank/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

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
	{Err: requests.ErrValidation, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCode, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidText, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidStatus, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAccountNo, Status: http.StatusBadRequest},
	{Err: domain.ErrBankDetails, Status: http.StatusBadRequest},
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrCashAccountNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrCodeExists, Status: http.StatusConflict},
	{Err: domain.ErrCompanyInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrAccountInvalid, Status: http.StatusUnprocessableEntity},
}, requests.ErrValidation)

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, status int, fn func(a dto.Actor) (any, error)) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := vo.ParseID[vo.CompanyID](r.PathValue("companyID"))
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

func one[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return httpx.Data[T]{Data: v}, nil
}

func byID(r *http.Request, a dto.Actor) (dto.ByID, error) {
	id, err := vo.ParseID[vo.CashAccountID](r.PathValue("cashAccountID"))
	return dto.ByID{Actor: a, ID: id}, err
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Accounts.List(r.Context(), a, f)
		return httpx.ToList(res, responses.From), err
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID(r, a)
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Get(r.Context(), in)
		return one(responses.From(acc), err)
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.CashAccount
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Create(r.Context(), a, d)
		return one(responses.From(acc), err)
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID(r, a)
		if err != nil {
			return nil, err
		}
		var req requests.CashAccount
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Update(r.Context(), in, d)
		return one(responses.From(acc), err)
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		in, err := byID(r, a)
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Accounts.Delete(r.Context(), in)
	})
}

// RegisterRoutes ผูก route ของ cashbank — ทุก route ต้อง Authenticate, สิทธิ์ตรวจใน use case
func RegisterRoutes(mux *http.ServeMux, h *Handler, authenticate func(http.Handler) http.Handler) {
	const base = "/api/v1/companies/{companyID}/cash-accounts"
	handle := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }
	handle("GET "+base, h.List)
	handle("POST "+base, h.Create)
	handle("GET "+base+"/{cashAccountID}", h.Get)
	handle("PUT "+base+"/{cashAccountID}", h.Update)
	handle("DELETE "+base+"/{cashAccountID}", h.Delete)
}
