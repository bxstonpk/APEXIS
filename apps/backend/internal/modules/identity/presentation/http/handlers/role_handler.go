package handlers

import (
	"net/http"

	"account/backend/internal/modules/identity/application/commands"
	"account/backend/internal/modules/identity/presentation/http/requests"
	"account/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/roles
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	roles, err := h.queries.ListRoles(r.Context(), userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromRoles(roles))
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
	role, err := h.commands.CreateRole(r.Context(), req.ToCommand(userID, companyID))
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
	role, err := h.commands.UpdateRole(r.Context(), req.ToCommand(userID, companyID, roleID))
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
	cmd := commands.DeleteRole{ActorID: userID, CompanyID: companyID, RoleID: roleID}
	if err := h.commands.DeleteRole(r.Context(), cmd); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
