// Package handlers คือ HTTP handler ของ inventory — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/usecases"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/modules/inventory/presentation/http/requests"
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

var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	// 400
	{Err: requests.ErrValidation, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCode, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidText, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidStatus, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidItemType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidDocType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidQuantity, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidLine, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidDate, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidSystem, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidYear, Status: http.StatusBadRequest},
	// 401 / 403
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	// 404
	{Err: domain.ErrUomNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrCategoryNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrItemNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrWarehouseNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrDocumentNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrBranchNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrClosingNotFound, Status: http.StatusNotFound},
	// 409
	{Err: domain.ErrCodeExists, Status: http.StatusConflict},
	{Err: domain.ErrInUse, Status: http.StatusConflict},
	{Err: domain.ErrClosingExists, Status: http.StatusConflict},
	{Err: domain.ErrDocumentNoUsed, Status: http.StatusConflict},
	// 422
	{Err: domain.ErrCompanyInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrItemNotStocked, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrWarehouseInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrSameWarehouse, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrDocumentNotDraft, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrDocumentNotPosted, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrInsufficientStock, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrStockAlreadyUsed, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrNoFiscalPeriod, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrPeriodClosed, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrItemHasStock, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrAccountInvalid, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrJournalInvalid, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrSystemLocked, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrNotPeriodic, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrSettingsMissing, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrInventoryClosed, Status: http.StatusUnprocessableEntity},
	// กฎของ module อื่น (เช่น ledger ปฏิเสธรายการปรับปรุงสิ้นงวด) — ข้อความมาจาก domain error ของ module นั้น
	{Err: integration.ErrRejected, Status: http.StatusUnprocessableEntity},
}, requests.ErrValidation, domain.ErrInvalidQuantity, domain.ErrInvalidLine, domain.ErrItemNotFound,
	domain.ErrItemNotStocked, domain.ErrInsufficientStock, domain.ErrStockAlreadyUsed,
	domain.ErrAccountInvalid, domain.ErrSettingsMissing, integration.ErrRejected)

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

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	err := httpx.DecodeJSON(w, r, &v)
	return v, err
}

// decodeData อ่าน body แล้วแปลงเป็น entities.*Data
func decodeData[R interface{ ToData(bool) (D, error) }, D any](w http.ResponseWriter, r *http.Request, isCreate bool) (D, error) {
	req, err := decode[R](w, r)
	if err != nil {
		var zero D
		return zero, err
	}
	return req.ToData(isCreate)
}
