package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/usecases"
	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/modules/inventory/presentation/http/requests"
	"apexis/backend/internal/modules/inventory/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GetSettings: GET /inventory-settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		s, err := h.uc.Settings.Get(r.Context(), a)
		return one(responses.FromSettings(s), err)
	})
}

// UpdateSettings: PUT /inventory-settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Settings](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToSettings()
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Settings.Update(r.Context(), dto.Create[entities.Settings]{Actor: a, Data: data})
		return one(responses.FromSettings(s), err)
	})
}

// SetAccountingSystem: PUT /inventory-settings/accounting-systems/{fiscalYear} — {"system":"periodic"}
func (h *Handler) SetAccountingSystem(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		year, err := requests.ParseYear(r.PathValue("fiscalYear"))
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.AccountingSystem](w, r)
		if err != nil {
			return nil, err
		}
		system, err := req.Parse()
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Settings.SetSystem(r.Context(), usecases.SetSystemInput{Actor: a, FromYear: year, System: system})
		return one(responses.FromSettings(s), err)
	})
}

// ListClosings: GET /inventory-closings
func (h *Handler) ListClosings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseClosingFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Closings.List(r.Context(), dto.List[dto.ClosingFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromClosing), err
	})
}

// GetClosing: GET /inventory-closings/{closingID}
func (h *Handler) GetClosing(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ClosingID](r, "closingID")
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Closings.Get(r.Context(), dto.ByID[vo.ClosingID]{Actor: a, ID: id})
		return one(responses.FromClosing(c), err)
	})
}

// CreateClosing: POST /inventory-closings — {"date":"2026-01-31"} ปรับปรุงสินค้าคงเหลือของงวดที่ date อยู่
func (h *Handler) CreateClosing(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Closing](w, r)
		if err != nil {
			return nil, err
		}
		date, err := req.ParseDate()
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Closings.Create(r.Context(), usecases.CreateClosingInput{Actor: a, Date: date})
		return one(responses.FromClosing(c), err)
	})
}

// UpdateClosingStatus: PATCH /inventory-closings/{closingID} — {"status":"cancelled"} กลับรายการบัญชี
func (h *Handler) UpdateClosingStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ClosingID](r, "closingID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.ClosingStatus](w, r)
		if err != nil {
			return nil, err
		}
		if req.Status != "cancelled" {
			return nil, requests.Invalid("status must be cancelled")
		}
		c, err := h.uc.Closings.Cancel(r.Context(), dto.ByID[vo.ClosingID]{Actor: a, ID: id})
		return one(responses.FromClosing(c), err)
	})
}
