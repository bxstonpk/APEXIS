package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
	"apexis/backend/internal/modules/organization/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// POST /api/v1/companies — ผู้สร้างเป็น owner ของบริษัท
func (h *Handler) CreateCompany(w http.ResponseWriter, r *http.Request) {
	h.serveUser(w, r, http.StatusCreated, func(userID vo.UserID) (any, error) {
		req, err := decode[requests.Company](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Companies.Create(r.Context(), dto.CreateCompanyInput{UserID: userID, Data: data})
		return one(responses.FromCompany(c), err)
	})
}

// GET /api/v1/me/companies?q=&status=&sort=name|code&page=&page_size= — บริษัทที่ตัวเองเป็นสมาชิก
func (h *Handler) ListMyCompanies(w http.ResponseWriter, r *http.Request) {
	h.serveUser(w, r, http.StatusOK, func(userID vo.UserID) (any, error) {
		f, err := requests.ParseCompanyFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Companies.ListMine(r.Context(), dto.ListMyCompaniesInput{UserID: userID, Filter: f})
		return httpx.ToList(res, responses.FromCompanySummary), err
	})
}

// GET /api/v1/companies/{companyID}
func (h *Handler) GetCompany(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		c, err := h.uc.Companies.Get(r.Context(), a)
		return one(responses.FromCompany(c), err)
	})
}

// PUT /api/v1/companies/{companyID} — แทนที่ทั้งก้อน (ยกเว้น code), ใช้เปิด / ปิดใช้งานบริษัทด้วย status
func (h *Handler) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Company](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Companies.Update(r.Context(), dto.UpdateCompanyInput{Actor: a, Data: data})
		return one(responses.FromCompany(c), err)
	})
}
