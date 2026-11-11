package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/access — role และ permission ของตัวเองในบริษัท
func (h *Handler) MyAccess(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	access, err := h.uc.Access.GetMine(r.Context(), userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromAccess(access))
}
