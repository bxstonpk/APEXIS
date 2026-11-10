package handlers

import (
	"net/http"

	"account/backend/internal/modules/identity/presentation/http/requests"
	"account/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/me/companies — บริษัทที่ user เข้าได้ (หน้าเลือกบริษัทหลังล็อกอิน)
func (h *Handler) MyCompanies(w http.ResponseWriter, r *http.Request) {
	userID, err := actor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	list, err := h.queries.ListMyCompanies(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMyCompanies(list))
}

// POST /api/v1/companies
func (h *Handler) CreateCompany(w http.ResponseWriter, r *http.Request) {
	userID, err := actor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.CreateCompany
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	company, err := h.commands.CreateCompany(r.Context(), req.ToCommand(userID))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromCompany(company))
}

// GET /api/v1/companies/{companyID}
func (h *Handler) GetCompany(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	company, err := h.queries.GetCompany(r.Context(), userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromCompany(company))
}

// PUT /api/v1/companies/{companyID}
func (h *Handler) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.UpdateCompany
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	company, err := h.commands.UpdateCompany(r.Context(), req.ToCommand(userID, companyID))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromCompany(company))
}

// GET /api/v1/companies/{companyID}/access — สิทธิ์ของฉันในบริษัทนี้ (frontend ใช้ซ่อน/แสดงเมนู)
func (h *Handler) MyAccess(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	access, err := h.queries.GetMyAccess(r.Context(), userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromAccess(access))
}
