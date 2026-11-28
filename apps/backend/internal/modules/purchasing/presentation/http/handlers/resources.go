package handlers

import (
	"apexis/backend/internal/platform/dashperiod"
	"context"
	"net/http"

	"github.com/google/uuid"

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

// ---------- สายเอกสาร (สินค้า / บริการ) ----------

type kindKey struct{}

// WithKind: route ของสายบริการ (ใบสั่งจ้าง / ใบตรวจรับงาน / ซื้อบริการ) ใช้ handler เดียวกับสายสินค้า โดยส่งสายผ่าน context
func WithKind(kind vo.PurchaseKind, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r.WithContext(context.WithValue(r.Context(), kindKey{}, kind)))
	}
}

// kindFrom: สายของ route ที่เรียก (ไม่ระบุ = สินค้า)
func kindFrom(r *http.Request) vo.PurchaseKind {
	if k, ok := r.Context().Value(kindKey{}).(vo.PurchaseKind); ok {
		return k
	}
	return vo.KindGoods
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
		f.Kind = kindFrom(r)
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
		o, err := h.uc.Orders.Get(r.Context(), dto.ByID[vo.OrderID]{Actor: a, ID: id, Kind: kindFrom(r)})
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
		data.Kind = kindFrom(r)
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
		data.Kind = kindFrom(r)
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
		o, err := h.uc.Orders.SetStatus(r.Context(), usecases.SetOrderStatus{ByID: dto.ByID[vo.OrderID]{Actor: a, ID: id, Kind: kindFrom(r)}, Status: st})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Orders.Delete(r.Context(), dto.ByID[vo.OrderID]{Actor: a, ID: id, Kind: kindFrom(r)})
	})
}

// ---------- ใบรับสินค้า ----------

func (h *Handler) ListReceipts(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), false)
		if err != nil {
			return nil, err
		}
		f.Kind = kindFrom(r)
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
		rc, err := h.uc.Receipts.Get(r.Context(), dto.ByID[vo.ReceiptID]{Actor: a, ID: id, Kind: kindFrom(r)})
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
		in.Kind = kindFrom(r)
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
		rc, err := h.uc.Receipts.Cancel(r.Context(), dto.ByID[vo.ReceiptID]{Actor: a, ID: id, Kind: kindFrom(r)})
		return one(responses.FromReceipt(rc), err)
	})
}

// ---------- ใบแจ้งหนี้ ----------

// GET /purchase-documents — รายการรวมเอกสารซื้อทุกชนิด (เฉพาะชนิดที่มีสิทธิ์อ่าน)
func (h *Handler) ListPurchaseDocuments(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParsePurchaseDocumentFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Documents.List(r.Context(), dto.List[dto.PurchaseDocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromPurchaseDocument), err
	})
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentFilter(r.URL.Query(), false)
		if err != nil {
			return nil, err
		}
		f.Kind = kindFrom(r)
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
		inv, err := h.uc.Invoices.Get(r.Context(), dto.ByID[vo.InvoiceID]{Actor: a, ID: id, Kind: kindFrom(r)})
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
		if req.Direct() {
			in, err := req.ToDirect()
			if err != nil {
				return nil, err
			}
			in.Kind = kindFrom(r)
			inv, err := h.uc.Invoices.CreateDirect(r.Context(), dto.Create[dto.DirectInvoiceInput]{Actor: a, Data: in})
			return one(responses.FromInvoice(inv), err)
		}
		in, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		in.Kind = kindFrom(r)
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
		inv, err := h.uc.Invoices.Cancel(r.Context(), dto.ByID[vo.InvoiceID]{Actor: a, ID: id, Kind: kindFrom(r)})
		return one(responses.FromInvoice(inv), err)
	})
}

// RecordTaxInvoice: POST /supplier-service-invoices/{invoiceID}/tax-invoices — บันทึกใบกำกับภาษีที่ได้รับจากผู้ขาย
func (h *Handler) RecordTaxInvoice(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.InvoiceID](r, "invoiceID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.TaxInvoice](w, r)
		if err != nil {
			return nil, err
		}
		in, err := req.ToInput(id)
		if err != nil {
			return nil, err
		}
		inv, err := h.uc.Invoices.RecordTaxInvoice(r.Context(), dto.Create[dto.TaxInvoiceInput]{Actor: a, Data: in})
		return one(responses.FromInvoice(inv), err)
	})
}

// CancelTaxInvoice: PATCH /supplier-service-invoices/{invoiceID}/tax-invoices/{taxInvoiceID} — {"status":"cancelled"}
func (h *Handler) CancelTaxInvoice(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.InvoiceID](r, "invoiceID")
		if err != nil {
			return nil, err
		}
		tid, err := uuid.Parse(r.PathValue("taxInvoiceID"))
		if err != nil {
			return nil, requests.Invalid("taxInvoiceID must be a UUID")
		}
		if err := cancelled(w, r); err != nil {
			return nil, err
		}
		inv, err := h.uc.Invoices.CancelTaxInvoice(r.Context(), dto.Create[dto.TaxInvoiceRef]{Actor: a, Data: dto.TaxInvoiceRef{InvoiceID: id, ID: tid}})
		return one(responses.FromInvoice(inv), err)
	})
}

// ---------- ใบลดหนี้ผู้ขาย ----------

func (h *Handler) ListCreditNotes(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseCreditNoteFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.CreditNotes.List(r.Context(), dto.List[dto.DocumentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromCreditNoteSummary), err
	})
}

func (h *Handler) GetCreditNote(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.CreditNoteID](r, "creditNoteID")
		if err != nil {
			return nil, err
		}
		c, err := h.uc.CreditNotes.Get(r.Context(), dto.ByID[vo.CreditNoteID]{Actor: a, ID: id})
		return one(responses.FromCreditNote(c), err)
	})
}

// CreateCreditNote: POST /supplier-credit-notes — ลดเจ้าหนี้ของใบแจ้งหนี้เดิม + ลงบัญชีทันที (ส่งคืนสินค้า stock: เบิกออกจากคลัง)
func (h *Handler) CreateCreditNote(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.CreditNote](w, r)
		if err != nil {
			return nil, err
		}
		in, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		c, err := h.uc.CreditNotes.Create(r.Context(), dto.Create[dto.CreditNoteInput]{Actor: a, Data: in})
		return one(responses.FromCreditNote(c), err)
	})
}

// CancelCreditNote: PATCH /supplier-credit-notes/{creditNoteID} — {"status":"cancelled"}
func (h *Handler) CancelCreditNote(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.CreditNoteID](r, "creditNoteID")
		if err != nil {
			return nil, err
		}
		if err := cancelled(w, r); err != nil {
			return nil, err
		}
		c, err := h.uc.CreditNotes.Cancel(r.Context(), dto.ByID[vo.CreditNoteID]{Actor: a, ID: id})
		return one(responses.FromCreditNote(c), err)
	})
}

// ---------- รายงาน (GET /reports?book=) ----------

const (
	BookOutstandingOrders     = "outstanding-purchase-orders"
	BookOutstandingWorkOrders = "outstanding-work-orders"
	BookAPAging               = "ap-aging"
	BookInputVat              = "input-vat"
	BookUndueInputVat         = "undue-input-vat"
)

// UndueInputVat: book=undue-input-vat — ซื้อบริการที่ภาษีซื้อยังรอใบกำกับ ณ วันที่
func (h *Handler) UndueInputVat(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseUndueInputVat(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.UndueInputVat(r.Context(), dto.List[dto.UndueInputVatFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromUndueInputVat(BookUndueInputVat, res), nil
	})
}

// OutstandingWorkOrders: book=outstanding-work-orders — บรรทัดใบสั่งจ้างที่ยังตรวจรับ / แจ้งหนี้ไม่ครบ
func (h *Handler) OutstandingWorkOrders(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseOutstanding(r.URL.Query())
		if err != nil {
			return nil, err
		}
		f.Kind = vo.KindService
		res, err := h.uc.Reports.Outstanding(r.Context(), dto.List[dto.OutstandingFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromOutstanding(BookOutstandingWorkOrders, httpx.ToList(res, responses.FromOutstandingRow)), nil
	})
}

func (h *Handler) InputVat(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseInputVat(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.InputVat(r.Context(), dto.List[dto.InputVatFilter]{Actor: a, Filter: f})
		if err != nil {
			return nil, err
		}
		return responses.FromInputVat(BookInputVat, res), nil
	})
}

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
