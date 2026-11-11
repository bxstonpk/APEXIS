package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/modules/ledger/presentation/http/requests"
	"apexis/backend/internal/modules/ledger/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GET /api/v1/companies/{companyID}/journal-entries?q=&status=&journal_id=&account_id=&start_date=&end_date=&sort=&page=&page_size=
func (h *Handler) ListEntries(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseEntryFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Entries.List(r.Context(), dto.List[dto.EntryFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromEntrySummary), err
	})
}

// GET /api/v1/companies/{companyID}/journal-entries/{entryID} — พร้อมบรรทัด
func (h *Handler) GetEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.EntryID](r, "entryID")
		if err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Get(r.Context(), dto.ByID[vo.EntryID]{Actor: a, ID: id})
		return one(responses.FromEntry(e), err)
	})
}

// POST /api/v1/companies/{companyID}/journal-entries — status "posted" = สร้างแล้วผ่านรายการทันที
func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Entry](w, r)
		if err != nil {
			return nil, err
		}
		post, err := req.PostOnCreate()
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Create(r.Context(), dto.CreateEntry[entities.EntryData]{Actor: a, Data: data, Post: post})
		return one(responses.FromEntry(e), err)
	})
}

// PUT /api/v1/companies/{companyID}/journal-entries/{entryID} — แทนที่ draft ทั้งก้อน
func (h *Handler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.EntryID](r, "entryID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Entry](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Update(r.Context(), dto.Update[vo.EntryID, entities.EntryData]{Actor: a, ID: id, Data: data})
		return one(responses.FromEntry(e), err)
	})
}

// PATCH /api/v1/companies/{companyID}/journal-entries/{entryID} {"status": "posted"} — ผ่านรายการ
func (h *Handler) UpdateEntryStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.EntryID](r, "entryID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.UpdateEntryStatus](w, r)
		if err != nil {
			return nil, err
		}
		if err := req.Validate(); err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Post(r.Context(), dto.ByID[vo.EntryID]{Actor: a, ID: id})
		return one(responses.FromEntry(e), err)
	})
}

// DELETE /api/v1/companies/{companyID}/journal-entries/{entryID} — เฉพาะ draft
func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.EntryID](r, "entryID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Entries.Delete(r.Context(), dto.ByID[vo.EntryID]{Actor: a, ID: id})
	})
}

// POST /api/v1/companies/{companyID}/journal-entries/{entryID}/reversals {"date": "2026-12-31"}
// สร้างรายการกลับ (ผ่านรายการทันที) — คืนรายการใหม่ (201)
func (h *Handler) ReverseEntry(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.EntryID](r, "entryID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Reversal](w, r)
		if err != nil {
			return nil, err
		}
		date, desc, err := req.Parse()
		if err != nil {
			return nil, err
		}
		e, err := h.uc.Entries.Reverse(r.Context(), dto.ReverseEntry{Actor: a, ID: id, Date: date, Description: desc})
		return one(responses.FromEntry(e), err)
	})
}
