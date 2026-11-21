package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/presentation/http/requests"
	"apexis/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/members?q=&status=&role_id=&sort=&page=&page_size=
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	filter, err := requests.ParseMemberFilter(r.URL.Query())
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.uc.Members.List(r.Context(), dto.ListMembersInput{ActorID: userID, CompanyID: companyID, Filter: filter})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, responses.ToList(result, responses.FromMember))
}

// POST /api/v1/companies/{companyID}/members
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.AddMember
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput(userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.uc.Members.Add(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromMembership(m))
}

// PATCH /api/v1/companies/{companyID}/members/{userID} — {"status": "active" | "suspended"}
func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	actorID, companyID, target, err := actorCompanyAndMember(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.UpdateMember
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput(actorID, companyID, target)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.uc.Members.ChangeStatus(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMembership(m))
}

// PUT /api/v1/companies/{companyID}/members/{userID}/roles — แทนที่ role ทั้งชุดของ member
func (h *Handler) AssignMemberRoles(w http.ResponseWriter, r *http.Request) {
	actorID, companyID, target, err := actorCompanyAndMember(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.AssignRoles
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput(actorID, companyID, target)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.uc.Members.AssignRoles(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMembership(m))
}

// GET /api/v1/companies/{companyID}/members/{userID} — สมาชิกหนึ่งคน (หน้าแก้ไขสมาชิก)
func (h *Handler) GetMember(w http.ResponseWriter, r *http.Request) {
	actorID, companyID, target, err := actorCompanyAndMember(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.uc.Members.Get(r.Context(), actorID, companyID, target)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMember(m))
}
