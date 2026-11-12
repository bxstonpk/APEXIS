package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/modules/inventory/presentation/http/requests"
	"apexis/backend/internal/modules/inventory/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GET /api/v1/companies/{companyID}/stock-documents?q=&type=&status=&warehouse_id=&item_id=&start_date=&end_date=&sort=&page=&page_size=
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Documents.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromDocumentSummary), err
	})
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentID](r, "documentID")
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Documents.Get(r.Context(), dto.ByID[vo.DocumentID]{Actor: a, ID: id})
		return one(responses.FromDocument(d), err)
	})
}

// POST /stock-documents — status "posted" = สร้างแล้วผ่านเอกสารทันที
func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Document](w, r)
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
		d, err := h.uc.Documents.Create(r.Context(), dto.CreateDocument[entities.DocumentData]{Actor: a, Data: data, Post: post})
		return one(responses.FromDocument(d), err)
	})
}

// PUT /stock-documents/{documentID} — แทนที่ draft ทั้งก้อน
func (h *Handler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentID](r, "documentID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Document](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Documents.Update(r.Context(), dto.Update[vo.DocumentID, entities.DocumentData]{Actor: a, ID: id, Data: data})
		return one(responses.FromDocument(d), err)
	})
}

// PATCH /stock-documents/{documentID} {"status": "posted" | "cancelled"}
func (h *Handler) UpdateDocumentStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentID](r, "documentID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.UpdateDocumentStatus](w, r)
		if err != nil {
			return nil, err
		}
		st, err := req.Parse()
		if err != nil {
			return nil, err
		}
		in := dto.ByID[vo.DocumentID]{Actor: a, ID: id}
		var d dto.Document
		if st == vo.DocPosted {
			d, err = h.uc.Documents.Post(r.Context(), in)
		} else {
			d, err = h.uc.Documents.Cancel(r.Context(), in)
		}
		return one(responses.FromDocument(d), err)
	})
}

// DELETE /stock-documents/{documentID} — เฉพาะ draft
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentID](r, "documentID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Documents.Delete(r.Context(), dto.ByID[vo.DocumentID]{Actor: a, ID: id})
	})
}

// ---------- รายงาน (ลงทะเบียนกับ GET /reports?book=) ----------

// StockOnHand: book=stock-on-hand
func (h *Handler) StockOnHand(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseStockOnHand(r.URL.Query())
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Reports.StockOnHand(r.Context(), dto.List[dto.StockOnHandFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromStockOnHand(BookStockOnHand, s), nil
	})
}

// StockCard: book=stock-card
func (h *Handler) StockCard(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseStockCard(r.URL.Query())
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Reports.StockCard(r.Context(), dto.List[dto.StockCardFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromStockCard(BookStockCard, c), nil
	})
}

// ชื่อ book ของรายงาน inventory
const (
	BookStockOnHand = "stock-on-hand"
	BookStockCard   = "stock-card"
)
