package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
	"apexis/backend/internal/modules/organization/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GET /api/v1/companies/{companyID}/document-numberings — ทุกชนิดเอกสาร พร้อมรูปแบบที่ใช้และตัวอย่าง
func (h *Handler) ListNumberings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		list, err := h.uc.Numbering.List(r.Context(), a)
		if err != nil {
			return nil, err
		}
		out := make([]responses.DocumentNumbering, len(list))
		for i, n := range list {
			out[i] = responses.FromDocumentNumbering(n)
		}
		return httpx.Data[[]responses.DocumentNumbering]{Data: out}, nil
	})
}

// PUT /api/v1/companies/{companyID}/document-numberings/{docType}
func (h *Handler) UpdateNumbering(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Numbering](w, r)
		if err != nil {
			return nil, err
		}
		f, err := req.ToFormat()
		if err != nil {
			return nil, err
		}
		n, err := h.uc.Numbering.Update(r.Context(), a, r.PathValue("docType"), f)
		return one(responses.FromDocumentNumbering(n), err)
	})
}
