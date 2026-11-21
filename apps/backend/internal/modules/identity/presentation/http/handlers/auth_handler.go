package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/identity/presentation/http/requests"
	"apexis/backend/internal/modules/identity/presentation/http/responses"
)

// POST /api/v1/users — สมัครสมาชิก (สร้าง user)
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req requests.Register
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput()
	if err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.uc.Auth.Register(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromUser(user))
}

// POST /api/v1/auth/tokens — ล็อกอิน (สร้าง access token)
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req requests.Login
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	in, err := req.ToInput()
	if err != nil {
		writeError(w, r, err)
		return
	}
	token, err := h.uc.Auth.Login(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, responses.FromAuthToken(token))
}

// GET /api/v1/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := actor(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.uc.Auth.GetMe(r.Context(), userID)
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
	in, err := req.ToInput(userID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.uc.Auth.ChangePassword(r.Context(), in); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
