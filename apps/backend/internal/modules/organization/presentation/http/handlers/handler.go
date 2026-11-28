// Package handlers คือ HTTP handler ของ organization — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/usecases"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
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
	{Err: domain.ErrInvalidStatus, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidBusinessType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAddress, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidPostalCode, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCountry, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidTaxBranchNo, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidTaxID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidEmail, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidPhone, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidWebsite, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidCurrency, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidFiscalMonth, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidDecimals, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidDateRange, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidFiscalYear, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidFiscalStart, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidNumbering, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidPaper, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidTemplateDocType, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidTemplateHTML, Status: http.StatusBadRequest},
	{Err: domain.ErrTemplateDisallowed, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAssetName, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidAssetType, Status: http.StatusBadRequest},
	{Err: domain.ErrAssetTooLarge, Status: http.StatusBadRequest},
	// 401 / 403
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	// 404
	{Err: domain.ErrCompanyNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrBranchNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrBusinessUnitNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrDepartmentNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrCostCenterNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrProjectSiteNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrFiscalYearNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrFiscalPeriodNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrDocumentTypeNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrTemplateNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrAssetNotFound, Status: http.StatusNotFound},
	// 409
	{Err: domain.ErrCodeExists, Status: http.StatusConflict},
	{Err: domain.ErrBranchExists, Status: http.StatusConflict},
	{Err: domain.ErrInUse, Status: http.StatusConflict},
	{Err: domain.ErrFiscalYearExists, Status: http.StatusConflict},
	{Err: domain.ErrFiscalYearOverlaps, Status: http.StatusConflict},
	{Err: domain.ErrAssetNameExists, Status: http.StatusConflict},
	// 422
	{Err: domain.ErrCompanyInactive, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrDecimalsDecrease, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrHierarchyCycle, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrHeadOfficeImmutable, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrProjectEndDateRequired, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrPeriodCloseOrder, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrPeriodReopenOrder, Status: http.StatusUnprocessableEntity},
	{Err: domain.ErrTemplateInactive, Status: http.StatusUnprocessableEntity},
}, requests.ErrValidation, domain.ErrInvalidNumbering, domain.ErrTemplateDisallowed)

// serve: อ่าน actor (user + {companyID}) -> fn -> เขียน response (route ใต้ /companies/{companyID})
// fn คืน body ที่ห่อ envelope แล้ว (one / httpx.ToList) หรือ nil สำหรับ 204
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, status int, fn func(a dto.Actor) (any, error)) {
	h.serveUser(w, r, status, func(userID vo.UserID) (any, error) {
		companyID, err := pathID[vo.CompanyID](r, "companyID")
		if err != nil {
			return nil, err
		}
		return fn(dto.Actor{UserID: userID, CompanyID: companyID})
	})
}

// serveUser: เหมือน serve แต่ใช้แค่ user (route ที่ยังไม่อยู่ใต้บริษัท เช่น POST /companies, GET /me/companies)
func (h *Handler) serveUser(w http.ResponseWriter, r *http.Request, status int, fn func(userID vo.UserID) (any, error)) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	body, err := fn(vo.UserID(userID))
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
