package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
	"apexis/backend/internal/modules/organization/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GET /api/v1/companies/{companyID}/fiscal-years?year=&sort=&page=&page_size=
func (h *Handler) ListFiscalYears(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseFiscalYearFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Fiscal.ListYears(r.Context(), dto.List[dto.FiscalYearFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromFiscalYear), err
	})
}

// GET /api/v1/companies/{companyID}/fiscal-years/{fiscalYearID} — พร้อมงวดทั้ง 12 งวด
func (h *Handler) GetFiscalYear(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.FiscalYearID](r, "fiscalYearID")
		if err != nil {
			return nil, err
		}
		y, err := h.uc.Fiscal.GetYear(r.Context(), dto.ByID[vo.FiscalYearID]{Actor: a, ID: id})
		return one(responses.FromFiscalYear(y), err)
	})
}

// POST /api/v1/companies/{companyID}/fiscal-years {"year": 2026, "start_date": "2026-01-01"} — สร้าง 12 งวดอัตโนมัติ
func (h *Handler) CreateFiscalYear(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.CreateFiscalYear](w, r)
		if err != nil {
			return nil, err
		}
		year, start, err := req.Parse()
		if err != nil {
			return nil, err
		}
		y, err := h.uc.Fiscal.CreateYear(r.Context(), dto.CreateFiscalYearInput{Actor: a, Year: year, StartDate: start})
		return one(responses.FromFiscalYear(y), err)
	})
}

// GET /api/v1/companies/{companyID}/fiscal-periods?fiscal_year=&status=&date=&sort=&page=&page_size=
func (h *Handler) ListFiscalPeriods(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseFiscalPeriodFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Fiscal.ListPeriods(r.Context(), dto.List[dto.FiscalPeriodFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromFiscalPeriod), err
	})
}

// GET /api/v1/companies/{companyID}/fiscal-periods/{fiscalPeriodID}
func (h *Handler) GetFiscalPeriod(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.FiscalPeriodID](r, "fiscalPeriodID")
		if err != nil {
			return nil, err
		}
		p, err := h.uc.Fiscal.GetPeriod(r.Context(), dto.ByID[vo.FiscalPeriodID]{Actor: a, ID: id})
		return one(responses.FromFiscalPeriod(p), err)
	})
}

// PATCH /api/v1/companies/{companyID}/fiscal-periods/{fiscalPeriodID} {"status": "closed" | "open"}
func (h *Handler) UpdateFiscalPeriod(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.FiscalPeriodID](r, "fiscalPeriodID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.UpdateFiscalPeriod](w, r)
		if err != nil {
			return nil, err
		}
		st, err := req.Parse()
		if err != nil {
			return nil, err
		}
		p, err := h.uc.Fiscal.ChangePeriodStatus(r.Context(), dto.ChangePeriodStatusInput{Actor: a, PeriodID: id, Status: st})
		return one(responses.FromFiscalPeriod(p), err)
	})
}
