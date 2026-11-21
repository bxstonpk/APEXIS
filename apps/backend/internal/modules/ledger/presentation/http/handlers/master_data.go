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

// ---------- accounts (ผังบัญชี) ----------

// GET /api/v1/companies/{companyID}/accounts?q=&type=&status=&postable=&parent_id=&sort=&page=&page_size=
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseAccountFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Accounts.List(r.Context(), dto.List[dto.AccountFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromAccount), err
	})
}

// GET /api/v1/companies/{companyID}/accounts/tree — ทุกบัญชีเรียงตามรหัส (หน้าผังบัญชีแบบต้นไม้)
func (h *Handler) AccountTree(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		res, err := h.uc.Accounts.Tree(r.Context(), a)
		if err != nil {
			return nil, err
		}
		out := make([]responses.Account, len(res))
		for i, acc := range res {
			out[i] = responses.FromAccount(acc)
		}
		return one(out, nil)
	})
}

// POST /api/v1/companies/{companyID}/accounts/defaults — สร้างผังบัญชีมาตรฐาน (บริษัทที่ยังไม่มีบัญชี)
func (h *Handler) CreateDefaultAccounts(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		res, err := h.uc.Accounts.CreateDefaults(r.Context(), a)
		return one(responses.DefaultChart{Accounts: res.Accounts, Journals: res.Journals}, err)
	})
}

func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.AccountID](r, "accountID")
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Get(r.Context(), dto.ByID[vo.AccountID]{Actor: a, ID: id})
		return one(responses.FromAccount(acc), err)
	})
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		data, err := decodeData[requests.Account, entities.AccountData](w, r, true)
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Create(r.Context(), dto.Create[entities.AccountData]{Actor: a, Data: data})
		return one(responses.FromAccount(acc), err)
	})
}

func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.AccountID](r, "accountID")
		if err != nil {
			return nil, err
		}
		data, err := decodeData[requests.Account, entities.AccountData](w, r, false)
		if err != nil {
			return nil, err
		}
		acc, err := h.uc.Accounts.Update(r.Context(), dto.Update[vo.AccountID, entities.AccountData]{Actor: a, ID: id, Data: data})
		return one(responses.FromAccount(acc), err)
	})
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.AccountID](r, "accountID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Accounts.Delete(r.Context(), dto.ByID[vo.AccountID]{Actor: a, ID: id})
	})
}

// ---------- journals (สมุดรายวัน) ----------

// GET /api/v1/companies/{companyID}/journals?q=&status=&sort=&page=&page_size=
func (h *Handler) ListJournals(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseJournalFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Journals.List(r.Context(), dto.List[dto.JournalFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromJournal), err
	})
}

func (h *Handler) GetJournal(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.JournalID](r, "journalID")
		if err != nil {
			return nil, err
		}
		j, err := h.uc.Journals.Get(r.Context(), dto.ByID[vo.JournalID]{Actor: a, ID: id})
		return one(responses.FromJournal(j), err)
	})
}

func (h *Handler) CreateJournal(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		data, err := decodeData[requests.Journal, entities.JournalData](w, r, true)
		if err != nil {
			return nil, err
		}
		j, err := h.uc.Journals.Create(r.Context(), dto.Create[entities.JournalData]{Actor: a, Data: data})
		return one(responses.FromJournal(j), err)
	})
}

func (h *Handler) UpdateJournal(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.JournalID](r, "journalID")
		if err != nil {
			return nil, err
		}
		data, err := decodeData[requests.Journal, entities.JournalData](w, r, false)
		if err != nil {
			return nil, err
		}
		j, err := h.uc.Journals.Update(r.Context(), dto.Update[vo.JournalID, entities.JournalData]{Actor: a, ID: id, Data: data})
		return one(responses.FromJournal(j), err)
	})
}

func (h *Handler) DeleteJournal(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.JournalID](r, "journalID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Journals.Delete(r.Context(), dto.ByID[vo.JournalID]{Actor: a, ID: id})
	})
}

// decodeData อ่าน body แล้วแปลงเป็น entities.*Data
func decodeData[R interface{ ToData(bool) (D, error) }, D any](w http.ResponseWriter, r *http.Request, isCreate bool) (D, error) {
	req, err := decode[R](w, r)
	if err != nil {
		var zero D
		return zero, err
	}
	return req.ToData(isCreate)
}
