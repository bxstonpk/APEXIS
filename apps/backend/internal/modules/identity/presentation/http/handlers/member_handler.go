package handlers

import (
	"context"
	"net/http"

	"account/backend/internal/modules/identity/application/commands"
	"account/backend/internal/modules/identity/presentation/http/requests"
	"account/backend/internal/modules/identity/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/members
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	members, err := h.queries.ListMembers(r.Context(), userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMembers(members))
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
	cmd, err := req.ToCommand(userID, companyID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.commands.AddMember(r.Context(), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromMembership(m))
}

// PUT /api/v1/companies/{companyID}/members/{userID}/roles
func (h *Handler) AssignMemberRoles(w http.ResponseWriter, r *http.Request) {
	actorID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	target, err := pathUserID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.AssignRoles
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	cmd, err := req.ToCommand(actorID, companyID, target)
	if err != nil {
		writeError(w, r, err)
		return
	}
	m, err := h.commands.AssignMemberRoles(r.Context(), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromMembership(m))
}

// POST /api/v1/companies/{companyID}/members/{userID}/suspend
func (h *Handler) SuspendMember(w http.ResponseWriter, r *http.Request) {
	h.changeMemberStatus(w, r, h.commands.SuspendMember)
}

// POST /api/v1/companies/{companyID}/members/{userID}/activate
func (h *Handler) ActivateMember(w http.ResponseWriter, r *http.Request) {
	h.changeMemberStatus(w, r, h.commands.ActivateMember)
}

func (h *Handler) changeMemberStatus(w http.ResponseWriter, r *http.Request, run func(ctx context.Context, cmd commands.ChangeMemberStatus) error) {
	actorID, companyID, err := actorAndCompany(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	target, err := pathUserID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := run(r.Context(), commands.ChangeMemberStatus{ActorID: actorID, CompanyID: companyID, UserID: target}); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
