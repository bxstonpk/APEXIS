package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/modules/purchasing/presentation/http/requests"
	"apexis/backend/internal/modules/purchasing/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// ---------- ประเภทภาษีหัก ณ ที่จ่าย ----------

func (h *Handler) ListWhtTypes(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseWhtTypeFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.WhtTypes.List(r.Context(), dto.List[dto.WhtTypeFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromWhtType), err
	})
}

func (h *Handler) GetWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.WhtTypeID](r, "whtTypeID")
		if err != nil {
			return nil, err
		}
		t, err := h.uc.WhtTypes.Get(r.Context(), dto.ByID[vo.WhtTypeID]{Actor: a, ID: id})
		return one(responses.FromWhtType(t), err)
	})
}

func (h *Handler) CreateWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.WhtType](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		t, err := h.uc.WhtTypes.Create(r.Context(), dto.Create[entities.WhtTypeData]{Actor: a, Data: data})
		return one(responses.FromWhtType(t), err)
	})
}

func (h *Handler) UpdateWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.WhtTypeID](r, "whtTypeID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.WhtType](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		t, err := h.uc.WhtTypes.Update(r.Context(), dto.Update[vo.WhtTypeID, entities.WhtTypeData]{Actor: a, ID: id, Data: data})
		return one(responses.FromWhtType(t), err)
	})
}

func (h *Handler) DeleteWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.WhtTypeID](r, "whtTypeID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.WhtTypes.Delete(r.Context(), dto.ByID[vo.WhtTypeID]{Actor: a, ID: id})
	})
}

// ---------- ใบจ่ายชำระ ----------

// ListPayments: GET /supplier-payments?q=&status=&supplier_id=&start_date=&end_date=&sort=date|document_no
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), false)
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Payments.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromPaymentSummary), err
	})
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.PaymentID](r, "paymentID")
		if err != nil {
			return nil, err
		}
		p, err := h.uc.Payments.Get(r.Context(), dto.ByID[vo.PaymentID]{Actor: a, ID: id})
		return one(responses.FromPayment(p), err)
	})
}

// CreatePayment: POST /supplier-payments — ตัดหนี้ + หักภาษี ณ ที่จ่าย + ลงบัญชีทันที
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Payment](w, r)
		if err != nil {
			return nil, err
		}
		in, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		p, err := h.uc.Payments.Create(r.Context(), dto.Create[dto.PaymentInput]{Actor: a, Data: in})
		return one(responses.FromPayment(p), err)
	})
}

// CancelPayment: PATCH /supplier-payments/{paymentID} — {"status":"cancelled"}
func (h *Handler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.PaymentID](r, "paymentID")
		if err != nil {
			return nil, err
		}
		if err := cancelled(w, r); err != nil {
			return nil, err
		}
		p, err := h.uc.Payments.Cancel(r.Context(), dto.ByID[vo.PaymentID]{Actor: a, ID: id})
		return one(responses.FromPayment(p), err)
	})
}

// BookWithholding: GET /reports?book=withholding-tax
const BookWithholding = "withholding-tax"

func (h *Handler) WithholdingTax(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseWithholding(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.Withholding(r.Context(), dto.List[dto.WithholdingFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromWithholding(BookWithholding, res), nil
	})
}
