package handlers

import (
	"apexis/backend/internal/platform/dashperiod"
	"net/http"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/usecases"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/modules/purchasing/presentation/http/requests"
	"apexis/backend/internal/modules/purchasing/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// ---------- ค่าตั้ง ----------

// Dashboard: GET /dashboards/purchasing?start_date=&end_date= (บังคับทั้งคู่)
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		p, err := dashperiod.Parse(r.URL.Query())
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Reports.Dashboard(r.Context(), dto.List[dashperiod.Period]{Actor: a, Filter: p})
		return one(responses.FromDashboard(d), err)
	})
}

// GetSettings: GET /purchasing-settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		s, err := h.uc.Settings.Get(r.Context(), a)
		return one(responses.FromSettings(s), err)
	})
}

// UpdateSettings: PUT /purchasing-settings
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

// ---------- ผู้ขาย ----------

func (h *Handler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseSupplierFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Suppliers.List(r.Context(), dto.List[dto.SupplierFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromSupplier), err
	})
}

func (h *Handler) GetSupplier(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.SupplierID](r, "supplierID")
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Suppliers.Get(r.Context(), dto.ByID[vo.SupplierID]{Actor: a, ID: id})
		return one(responses.FromSupplier(s), err)
	})
}

func (h *Handler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Supplier](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Suppliers.Create(r.Context(), dto.Create[entities.SupplierData]{Actor: a, Data: data})
		return one(responses.FromSupplier(s), err)
	})
}

func (h *Handler) UpdateSupplier(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.SupplierID](r, "supplierID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Supplier](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		s, err := h.uc.Suppliers.Update(r.Context(), dto.Update[vo.SupplierID, entities.SupplierData]{Actor: a, ID: id, Data: data})
		return one(responses.FromSupplier(s), err)
	})
}

func (h *Handler) DeleteSupplier(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.SupplierID](r, "supplierID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Suppliers.Delete(r.Context(), dto.ByID[vo.SupplierID]{Actor: a, ID: id})
	})
}

// ---------- ใบสั่งซื้อ ----------

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), true)
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Orders.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromOrderSummary), err
	})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Get(r.Context(), dto.ByID[vo.OrderID]{Actor: a, ID: id})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Order](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Create(r.Context(), dto.Create[entities.OrderData]{Actor: a, Data: data})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Order](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Update(r.Context(), dto.Update[vo.OrderID, entities.OrderData]{Actor: a, ID: id, Data: data})
		return one(responses.FromOrder(o), err)
	})
}

// UpdateOrderStatus: PATCH /purchase-orders/{orderID} — {"status":"approved"|"closed"|"cancelled"}
func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Status](w, r)
		if err != nil {
			return nil, err
		}
		st, err := vo.ParseOrderStatus(req.Status)
		if err != nil || st == vo.OrderDraft {
			return nil, requests.Invalid("status must be one of approved, closed, cancelled")
		}
		o, err := h.uc.Orders.SetStatus(r.Context(), usecases.SetOrderStatus{ByID: dto.ByID[vo.OrderID]{Actor: a, ID: id}, Status: st})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Orders.Delete(r.Context(), dto.ByID[vo.OrderID]{Actor: a, ID: id})
	})
}

// ---------- ใบรับสินค้า ----------

func (h *Handler) ListReceipts(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), false)
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Receipts.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromReceiptSummary), err
	})
}

func (h *Handler) GetReceipt(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ReceiptID](r, "receiptID")
		if err != nil {
			return nil, err
		}
		rc, err := h.uc.Receipts.Get(r.Context(), dto.ByID[vo.ReceiptID]{Actor: a, ID: id})
		return one(responses.FromReceipt(rc), err)
	})
}

// CreateReceipt: POST /goods-receipts — รับสินค้า + รับเข้าคลัง + ลงบัญชีทันที
func (h *Handler) CreateReceipt(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Receipt](w, r)
		if err != nil {
			return nil, err
		}
		in, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		rc, err := h.uc.Receipts.Create(r.Context(), dto.Create[dto.ReceiptInput]{Actor: a, Data: in})
		return one(responses.FromReceipt(rc), err)
	})
}

// CancelReceipt: PATCH /goods-receipts/{receiptID} — {"status":"cancelled"}
func (h *Handler) CancelReceipt(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ReceiptID](r, "receiptID")
		if err != nil {
			return nil, err
		}
		if err := cancelled(w, r); err != nil {
			return nil, err
		}
		rc, err := h.uc.Receipts.Cancel(r.Context(), dto.ByID[vo.ReceiptID]{Actor: a, ID: id})
		return one(responses.FromReceipt(rc), err)
	})
}

// ---------- ใบแจ้งหนี้ ----------

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), false)
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Invoices.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromInvoiceSummary), err
	})
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.InvoiceID](r, "invoiceID")
		if err != nil {
			return nil, err
		}
		inv, err := h.uc.Invoices.Get(r.Context(), dto.ByID[vo.InvoiceID]{Actor: a, ID: id})
		return one(responses.FromInvoice(inv), err)
	})
}

// CreateInvoice: POST /supplier-invoices — ตั้งเจ้าหนี้ + ลงบัญชีทันที
func (h *Handler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Invoice](w, r)
		if err != nil {
			return nil, err
		}
		in, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		inv, err := h.uc.Invoices.Create(r.Context(), dto.Create[dto.InvoiceInput]{Actor: a, Data: in})
		return one(responses.FromInvoice(inv), err)
	})
}

// CancelInvoice: PATCH /supplier-invoices/{invoiceID} — {"status":"cancelled"}
func (h *Handler) CancelInvoice(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.InvoiceID](r, "invoiceID")
		if err != nil {
			return nil, err
		}
		if err := cancelled(w, r); err != nil {
			return nil, err
		}
		inv, err := h.uc.Invoices.Cancel(r.Context(), dto.ByID[vo.InvoiceID]{Actor: a, ID: id})
		return one(responses.FromInvoice(inv), err)
	})
}

// ---------- รายงาน (GET /reports?book=) ----------

const (
	BookOutstandingOrders = "outstanding-purchase-orders"
	BookAPAging           = "ap-aging"
)

func (h *Handler) OutstandingOrders(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseOutstanding(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.Outstanding(r.Context(), dto.List[dto.OutstandingFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromOutstanding(BookOutstandingOrders, httpx.ToList(res, responses.FromOutstandingRow)), nil
	})
}

func (h *Handler) APAging(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseAging(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.Aging(r.Context(), dto.List[dto.AgingFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromAging(BookAPAging, res), nil
	})
}
