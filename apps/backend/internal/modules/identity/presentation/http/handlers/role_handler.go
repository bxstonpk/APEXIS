package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/identity/application/dto"
	"apexis/backend/internal/modules/identity/presentation/http/requests"
	"apexis/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/roles?q=&is_system=&sort=&page=&page_size=
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	filter, err := requests.ParseRoleFilter(r.URL.Query())
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.uc.Roles.List(r.Context(), dto.ListRolesInput{ActorID: userID, CompanyID: companyID, Filter: filter})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, responses.ToList(result, responses.FromRole))
}

// POST /api/v1/companies/{companyID}/roles
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.CreateRole
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput(userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	role, err := h.uc.Roles.Create(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromRole(role))
}

// PUT /api/v1/companies/{companyID}/roles/{roleID}
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	roleID, err := pathRoleID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.UpdateRole
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput(userID, companyID, roleID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	role, err := h.uc.Roles.Update(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromRole(role))
}

// DELETE /api/v1/companies/{companyID}/roles/{roleID}
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	roleID, err := pathRoleID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	in := dto.DeleteRoleInput{ActorID: userID, CompanyID: companyID, RoleID: roleID}
	if err := h.uc.Roles.Delete(r.Context(), in); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/companies/{companyID}/roles/{roleID} — role หนึ่งรายการ (หน้าแก้ไขบทบาท)
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	roleID, err := pathRoleID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	role, err := h.uc.Roles.Get(r.Context(), userID, companyID, roleID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromRole(role))
}
