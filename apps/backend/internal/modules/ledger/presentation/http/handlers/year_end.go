package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/entities"
	"apexis/backend/internal/modules/ledger/presentation/http/requests"
	"apexis/backend/internal/modules/ledger/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// ---------- รายการปรับปรุงสิ้นงวด ----------

// GET /api/v1/companies/{companyID}/adjusting-entries?template=&status=&start_date=&end_date=&q=&sort=&page=&page_size=
func (h *Handler) ListAdjustingEntries(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseAdjustingFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Entries.List(r.Context(), dto.List[dto.EntryFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromEntrySummary), err
	})
}

// POST /api/v1/companies/{companyID}/adjusting-entries — สร้างและผ่านรายการปรับปรุงทันที (+ รายการกลับอัตโนมัติ)
func (h *Handler) CreateAdjustingEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Entry](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToAdjustingData()
		if err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Create(r.Context(), dto.CreateEntry[entities.EntryData]{Actor: a, Data: data, Post: true})
		return one(responses.FromEntry(e), err)
	})
}

// GET /api/v1/companies/{companyID}/adjusting-entries/templates — แบบพร้อมบัญชีตั้งต้น
func (h *Handler) AdjustingTemplates(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		ts, err := h.uc.Entries.Templates(r.Context(), a)
		return one(responses.FromTemplates(ts), err)
	})
}

// POST /api/v1/companies/{companyID}/adjusting-entries/pending-reversals — ผ่านรายการกลับอัตโนมัติที่รออยู่
func (h *Handler) PostPendingReversals(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		n, err := h.uc.Entries.PostPending(r.Context(), a)
		return one(map[string]int{"posted": n}, err)
	})
}

// ---------- ปิดบัญชีสิ้นปี ----------

// GET /api/v1/companies/{companyID}/year-end-closings — ทุกปีบัญชี (ล่าสุดก่อน)
func (h *Handler) ListYearEnds(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		ys, err := h.uc.YearEnd.List(r.Context(), a)
		return one(responses.FromYearEnds(ys), err)
	})
}

func (h *Handler) yearEnd(w http.ResponseWriter, r *http.Request, status int, fn func(in dto.YearEndInput) (dto.YearEnd, error)) {
	h.serve(w, r, status, func(a dto.Actor) (any, error) {
		year, err := requests.FiscalYearParam(r.PathValue("fiscalYear"))
		if err != nil {
			return nil, err
		}
		y, err := fn(dto.YearEndInput{Actor: a, FiscalYear: year})
		return one(responses.FromYearEnd(y), err)
	})
}

// GET /api/v1/companies/{companyID}/year-end-closings/{fiscalYear} — สถานะพร้อมเงื่อนไขก่อนปิด
func (h *Handler) GetYearEnd(w http.ResponseWriter, r *http.Request) {
	h.yearEnd(w, r, http.StatusOK, func(in dto.YearEndInput) (dto.YearEnd, error) { return h.uc.YearEnd.Get(r.Context(), in) })
}

// POST /api/v1/companies/{companyID}/year-end-closings/{fiscalYear}/close
func (h *Handler) CloseYear(w http.ResponseWriter, r *http.Request) {
	h.yearEnd(w, r, http.StatusOK, func(in dto.YearEndInput) (dto.YearEnd, error) { return h.uc.YearEnd.Close(r.Context(), in) })
}

// POST /api/v1/companies/{companyID}/year-end-closings/{fiscalYear}/reopen
func (h *Handler) ReopenYear(w http.ResponseWriter, r *http.Request) {
	h.yearEnd(w, r, http.StatusOK, func(in dto.YearEndInput) (dto.YearEnd, error) { return h.uc.YearEnd.Reopen(r.Context(), in) })
}

// ---------- ค่าตั้ง ----------

// GET /api/v1/companies/{companyID}/ledger-settings
func (h *Handler) GetLedgerSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		s, err := h.uc.YearEnd.Settings(r.Context(), a)
		return one(responses.FromLedgerSettings(s), err)
	})
}

// PUT /api/v1/companies/{companyID}/ledger-settings
func (h *Handler) UpdateLedgerSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		req, err := decode[requests.LedgerSettings](w, r)
		if err != nil {
			return nil, err
		}
		s, err := req.ToEntity()
		if err != nil {
			return nil, err
		}
		out, err := h.uc.YearEnd.UpdateSettings(r.Context(), a, s)
		return one(responses.FromLedgerSettings(out), err)
	})
}
