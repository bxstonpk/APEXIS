package handlers

import (
	"net/http"

	"account/backend/internal/modules/identity/presentation/http/requests"
	"account/backend/internal/modules/identity/presentation/http/responses"
)

// POST /api/v1/auth/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req requests.Register
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.commands.RegisterUser(r.Context(), req.ToCommand())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromUser(user))
}

// POST /api/v1/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req requests.Login
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	token, err := h.commands.Login(r.Context(), req.ToCommand())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromAuthToken(token))
}

// GET /api/v1/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := actor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.queries.GetMe(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, responses.FromUser(user))
}

// PUT /api/v1/me/password
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := actor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var req requests.ChangePassword
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.commands.ChangePassword(r.Context(), req.ToCommand(userID)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
