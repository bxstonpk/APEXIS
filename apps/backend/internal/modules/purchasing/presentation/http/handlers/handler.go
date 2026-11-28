// Package handlers คือ HTTP handler ของ purchasing — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"apexis/backend/internal/platform/thaiaddress"
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/usecases"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/modules/purchasing/presentation/http/requests"
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
	{Err: domain.ErrInvalidTaxID, Status: badRequest},
	{Err: domain.ErrInvalidBranchNo, Status: badRequest},
	{Err: domain.ErrInvalidTerm, Status: badRequest},
	{Err: domain.ErrInvalidVatRate, Status: badRequest},
	{Err: domain.ErrInvalidLine, Status: badRequest},
	{Err: domain.ErrInvalidTaxType, Status: badRequest},
	{Err: domain.ErrInvalidKind, Status: badRequest},
	{Err: domain.ErrInvalidReason, Status: badRequest},
	{Err: thaiaddress.ErrInvalidAddress, Status: badRequest},
	{Err: domain.ErrDuplicateBranch, Status: badRequest},
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrSupplierNotFound, Status: notFound},
	{Err: domain.ErrOrderNotFound, Status: notFound},
	{Err: domain.ErrReceiptNotFound, Status: notFound},
	{Err: domain.ErrInvoiceNotFound, Status: notFound},
	{Err: domain.ErrItemNotFound, Status: notFound},
	{Err: domain.ErrCreditNoteNotFound, Status: notFound},
	{Err: domain.ErrCodeExists, Status: conflict},
	{Err: domain.ErrInUse, Status: conflict},
	{Err: domain.ErrTaxIDExists, Status: conflict},
	{Err: domain.ErrInvoiceNoExists, Status: conflict},
	{Err: domain.ErrDocumentNoUsed, Status: conflict},
	{Err: domain.ErrCreditNoteNoExists, Status: conflict},
	{Err: domain.ErrCompanyInactive, Status: rejected},
	{Err: domain.ErrSupplierInactive, Status: rejected},
	{Err: domain.ErrItemInactive, Status: rejected},
	{Err: domain.ErrVatNotApplicable, Status: rejected},
	{Err: domain.ErrOrderNotDraft, Status: rejected},
	{Err: domain.ErrOrderNotApproved, Status: rejected},
	{Err: domain.ErrOrderNotOpen, Status: rejected},
	{Err: domain.ErrOrderHasActivity, Status: rejected},
	{Err: domain.ErrOverReceipt, Status: rejected},
	{Err: domain.ErrOverInvoice, Status: rejected},
	{Err: domain.ErrReceiptInvoiced, Status: rejected},
	{Err: domain.ErrAlreadyCancelled, Status: rejected},
	{Err: domain.ErrNoFiscalPeriod, Status: rejected},
	{Err: domain.ErrPeriodClosed, Status: rejected},
	{Err: domain.ErrSettingsMissing, Status: rejected},
	{Err: domain.ErrAccountInvalid, Status: rejected},
	{Err: domain.ErrJournalInvalid, Status: rejected},
	{Err: domain.ErrWarehouseRequired, Status: rejected},
	{Err: domain.ErrInvoicePaid, Status: rejected},
	{Err: domain.ErrOverPayment, Status: rejected},
	{Err: domain.ErrInvoiceNotPayable, Status: rejected},
	{Err: domain.ErrInactiveRef, Status: rejected},
	{Err: domain.ErrBranchNotFound, Status: rejected},
	{Err: domain.ErrInvoiceNotCreditable, Status: rejected},
	{Err: domain.ErrCreditBeforeInvoice, Status: rejected},
	{Err: domain.ErrOverCredit, Status: rejected},
	{Err: domain.ErrInvoiceHasCredits, Status: rejected},
	{Err: domain.ErrInvalidTaxInvoiceVat, Status: badRequest},
	{Err: domain.ErrTaxInvoiceNotFound, Status: notFound},
	{Err: domain.ErrTaxInvoiceNoExists, Status: conflict},
	{Err: domain.ErrNoUndueInputVat, Status: rejected},
	{Err: domain.ErrOverRecognition, Status: rejected},
	{Err: domain.ErrInvoiceHasTaxInvoices, Status: rejected},
	{Err: domain.ErrCreditExceedsUndueVat, Status: rejected},
	{Err: domain.ErrGoodsDocumentServiceLine, Status: rejected},
	{Err: domain.ErrServiceDocumentGoodsLine, Status: rejected},
	{Err: domain.ErrReturnOnServiceInvoice, Status: rejected},
	{Err: domain.ErrNoTaxInvoiceToRecord, Status: rejected},
	// กฎของ inventory / ledger (สต็อก, งวด, บัญชี) — ข้อความมาจาก domain error ของ module นั้น
	{Err: integration.ErrRejected, Status: rejected},
}, requests.ErrValidation, thaiaddress.ErrInvalidAddress, domain.ErrDuplicateBranch, domain.ErrTaxIDExists, domain.ErrBranchNotFound, domain.ErrInvalidLine, domain.ErrItemNotFound, domain.ErrItemInactive, domain.ErrOverReceipt,
	domain.ErrOverInvoice, domain.ErrReceiptInvoiced, domain.ErrSettingsMissing, domain.ErrAccountInvalid, integration.ErrRejected,
	domain.ErrOverPayment,
	domain.ErrInvoiceNotPayable, domain.ErrInactiveRef, domain.ErrOverCredit, domain.ErrOverRecognition, domain.ErrCreditExceedsUndueVat,
	domain.ErrGoodsDocumentServiceLine, domain.ErrServiceDocumentGoodsLine)

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

// cancelled: PATCH ที่รับเฉพาะ {"status":"cancelled"}
func cancelled(w http.ResponseWriter, r *http.Request) error {
	req, err := decode[requests.Status](w, r)
	if err != nil {
		return err
	}
	if req.Status != string(vo.DocCancelled) {
		return requests.Invalid("status must be cancelled")
	}
	return nil
}
