// Package handlers คือ HTTP ของ taxregistry
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/taxregistry/application/dto"
	"apexis/backend/internal/modules/taxregistry/application/usecases"
	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
	vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"
	"apexis/backend/internal/modules/taxregistry/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

type CurrentUser func(ctx context.Context) (uuid.UUID, bool)

type Handler struct {
	lookup      *usecases.Lookup
	currentUser CurrentUser
}

func NewHandler(lookup *usecases.Lookup, currentUser CurrentUser) *Handler {
	return &Handler{lookup: lookup, currentUser: currentUser}
}

var (
	errUnauthenticated = errors.New("unauthenticated")
	errInvalidCompany  = errors.New("invalid company id")
)

// ไม่พบ = 404, บริการสรรพากรติดต่อไม่ได้ = 503 (frontend ให้กรอกเองทั้งสองกรณี); รายละเอียดจากบริการภายนอกไม่ส่งให้ client
var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: errInvalidCompany, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidTaxID, Status: http.StatusBadRequest},
	{Err: domain.ErrInvalidBranchNo, Status: http.StatusBadRequest},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrNotFound, Status: http.StatusNotFound},
	{Err: domain.ErrUnavailable, Status: http.StatusServiceUnavailable},
})

// Lookup: GET /companies/{companyID}/vat-registrants?tax_id=&branch_no=
func (h *Handler) Lookup(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := uuid.Parse(r.PathValue("companyID"))
	if err != nil {
		errorMapper.Write(w, r, errInvalidCompany)
		return
	}
	q := r.URL.Query()
	rows, err := h.lookup.Execute(r.Context(), dto.LookupInput{
		UserID: vo.UserID(userID), CompanyID: vo.CompanyID(companyID), TaxID: q.Get("tax_id"), BranchNo: q.Get("branch_no"),
	})
	if err != nil {
		if errors.Is(err, domain.ErrUnavailable) {
			// log สาเหตุจริงไว้ตรวจสอบ แต่ตอบ client แค่ code
			slog.WarnContext(r.Context(), "vat registry unavailable", "err", err)
		}
		errorMapper.Write(w, r, err)
		return
	}
	out := make([]responses.Registrant, len(rows))
	for i, row := range rows {
		out[i] = responses.FromRegistrant(row)
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func RegisterRoutes(mux *http.ServeMux, h *Handler, authenticate func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/companies/{companyID}/vat-registrants", authenticate(http.HandlerFunc(h.Lookup)))
}
