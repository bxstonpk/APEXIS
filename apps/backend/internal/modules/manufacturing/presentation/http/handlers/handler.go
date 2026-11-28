// Package handlers คือ HTTP handler ของ manufacturing — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	"apexis/backend/internal/modules/manufacturing/application/usecases"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/modules/manufacturing/presentation/http/requests"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/integration"
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

const (
	badRequest = http.StatusBadRequest
	notFound   = http.StatusNotFound
	conflict   = http.StatusConflict
	rejected   = http.StatusUnprocessableEntity
)

var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	{Err: requests.ErrValidation, Status: badRequest},
	{Err: domain.ErrInvalidID, Status: badRequest},
	{Err: domain.ErrInvalidCode, Status: badRequest},
	{Err: domain.ErrInvalidName, Status: badRequest},
	{Err: domain.ErrInvalidText, Status: badRequest},
	{Err: domain.ErrInvalidStatus, Status: badRequest},
	{Err: domain.ErrInvalidDate, Status: badRequest},
	{Err: domain.ErrInvalidQuantity, Status: badRequest},
	{Err: domain.ErrInvalidLine, Status: badRequest},
	{Err: domain.ErrInvalidReason, Status: badRequest},
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrBomNotFound, Status: notFound},
	{Err: domain.ErrOrderNotFound, Status: notFound},
	{Err: domain.ErrItemNotFound, Status: notFound},
	{Err: domain.ErrWarehouseNotFound, Status: notFound},
	{Err: domain.ErrCodeExists, Status: conflict},
	{Err: domain.ErrDefaultBomExists, Status: conflict},
	{Err: domain.ErrInUse, Status: conflict},
	{Err: domain.ErrDocumentNoUsed, Status: conflict},
	{Err: domain.ErrNotManufacturing, Status: rejected},
	{Err: domain.ErrCompanyInactive, Status: rejected},
	{Err: domain.ErrItemNotStock, Status: rejected},
	{Err: domain.ErrItemInactive, Status: rejected},
	{Err: domain.ErrWarehouseInactive, Status: rejected},
	{Err: domain.ErrBomInactive, Status: rejected},
	{Err: domain.ErrBomItemMismatch, Status: rejected},
	{Err: domain.ErrOrderNotDraft, Status: rejected},
	{Err: domain.ErrOrderNotReleased, Status: rejected},
	{Err: domain.ErrAlreadyCancelled, Status: rejected},
	{Err: domain.ErrMaterialNotOnOrder, Status: rejected},
	{Err: domain.ErrOverReturn, Status: rejected},
	{Err: domain.ErrNoFiscalPeriod, Status: rejected},
	{Err: domain.ErrPeriodClosed, Status: rejected},
	{Err: domain.ErrSettingsMissing, Status: rejected},
	{Err: domain.ErrAccountInvalid, Status: rejected},
	{Err: domain.ErrJournalInvalid, Status: rejected},
	{Err: domain.ErrAccountMissing, Status: rejected},
	{Err: domain.ErrDateBeforeOrder, Status: rejected},
	// กฎของ inventory / ledger (สต็อกไม่พอ, งวด, บัญชี) — ข้อความมาจาก domain error ของ module นั้น
	{Err: integration.ErrRejected, Status: rejected},
}, requests.ErrValidation, domain.ErrInvalidQuantity, domain.ErrInvalidLine, domain.ErrItemNotFound, domain.ErrItemNotStock,
	domain.ErrItemInactive, domain.ErrMaterialNotOnOrder, domain.ErrOverReturn, domain.ErrSettingsMissing, domain.ErrAccountMissing,
	domain.ErrInvalidDate, integration.ErrRejected)

// serve: อ่าน actor (user + {companyID}) -> fn -> เขียน response (fn คืน body ที่ห่อ envelope แล้ว หรือ nil สำหรับ 204)
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

func one[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return httpx.Data[T]{Data: v}, nil
}

func pathID[T ~[16]byte](r *http.Request, name string) (T, error) {
	return vo.ParseID[T](r.PathValue(name))
}
