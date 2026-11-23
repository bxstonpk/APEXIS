package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/finance/application/dto"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/modules/finance/presentation/http/requests"
	"apexis/backend/internal/modules/finance/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/reports"
)

// ---------- ตัวช่วยของ handler เอกสาร ----------

func docID[T ~[16]byte](r *http.Request, name string, a dto.Actor) (dto.ByDoc[T], error) {
	id, err := vo.ParseID[T](r.PathValue(name))
	return dto.ByDoc[T]{Actor: a, ID: id}, err
}

func listDocs[T any, B any](h *Handler, kinds []string, run func(context.Context, dto.Actor, dto.DocumentFilter) (listing.Result[T], error), to func(T) B) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			f, err := requests.ParseDocumentFilter(r.URL.Query(), kinds)
			if err != nil {
				return nil, err
			}
			res, err := run(r.Context(), a, f)
			return httpx.ToList(res, to), err
		})
	}
}

func getDoc[ID ~[16]byte, T any, B any](h *Handler, param string, run func(context.Context, dto.ByDoc[ID]) (T, error), to func(T) B) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			in, err := docID[ID](r, param, a)
			if err != nil {
				return nil, err
			}
			v, err := run(r.Context(), in)
			return one(to(v), err)
		})
	}
}

// cancelDoc: PATCH {"status":"cancelled"} — ยกเลิกด้วยการกลับรายการ
func cancelDoc[ID ~[16]byte, T any, B any](h *Handler, param string, run func(context.Context, dto.ByDoc[ID]) (T, error), to func(T) B) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			in, err := docID[ID](r, param, a)
			if err != nil {
				return nil, err
			}
			var req requests.Cancel
			if err := httpx.DecodeJSON(w, r, &req); err != nil {
				return nil, err
			}
			if err := req.Validate(); err != nil {
				return nil, err
			}
			v, err := run(r.Context(), in)
			return one(to(v), err)
		})
	}
}

func createDoc[R interface{ ToInput() (In, error) }, In any, T any, B any](h *Handler, run func(context.Context, dto.Actor, In) (T, error), to func(T) B) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
			var req R
			if err := httpx.DecodeJSON(w, r, &req); err != nil {
				return nil, err
			}
			in, err := req.ToInput()
			if err != nil {
				return nil, err
			}
			v, err := run(r.Context(), a, in)
			return one(to(v), err)
		})
	}
}

// openInvoices: GET .../open-invoices?{param}=<uuid>
func openInvoices(h *Handler, param string, run func(context.Context, dto.Actor, uuid.UUID) ([]dto.OpenInvoice, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			id, err := uuid.Parse(r.URL.Query().Get(param))
			if err != nil || id == uuid.Nil {
				return nil, requests.Invalid("%s is required (UUID)", param)
			}
			rows, err := run(r.Context(), a, id)
			return one(responses.FromOpenInvoices(rows), err)
		})
	}
}

// ---------- ค่าตั้ง / ประเภทภาษี ----------

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		s, err := h.uc.Settings.Get(r.Context(), a)
		return one(responses.FromSettings(s), err)
	})
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		var req requests.Settings
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		s, err := req.ToSettings()
		if err != nil {
			return nil, err
		}
		out, err := h.uc.Settings.Update(r.Context(), a, s)
		return one(responses.FromSettings(out), err)
	})
}

func (h *Handler) listWhtTypes(forPayment bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			f, err := requests.ParseWhtTypeFilter(r.URL.Query())
			if err != nil {
				return nil, err
			}
			run := h.uc.WhtTypes.List
			if forPayment {
				run = h.uc.WhtTypes.ListForPayment
			}
			res, err := run(r.Context(), a, f)
			return httpx.ToList(res, responses.FromWhtType), err
		})
	}
}

func (h *Handler) CreateWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.WhtType
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		out, err := h.uc.WhtTypes.Create(r.Context(), a, d)
		return one(responses.FromWhtType(out), err)
	})
}

func (h *Handler) UpdateWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := docID[vo.WhtTypeID](r, "whtTypeID", a)
		if err != nil {
			return nil, err
		}
		var req requests.WhtType
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		out, err := h.uc.WhtTypes.Update(r.Context(), in, d)
		return one(responses.FromWhtType(out), err)
	})
}

func (h *Handler) DeleteWhtType(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		in, err := docID[vo.WhtTypeID](r, "whtTypeID", a)
		if err != nil {
			return nil, err
		}
		return nil, h.uc.WhtTypes.Delete(r.Context(), in)
	})
}

// ---------- routes / reports ----------

const (
	BookWithholding         = "withholding-tax"
	BookWithholdingReceived = "withholding-tax-received"
)

// RegisterRoutes ผูก route ของ finance — ทุก route ต้อง Authenticate, สิทธิ์ตรวจใน use case, ค้นหาด้วย URL query
// เอกสารลงบัญชีทันทีที่สร้าง: POST = posted -> PATCH {"status":"cancelled"} = กลับรายการ
func RegisterRoutes(mux *http.ServeMux, h *Handler, authenticate func(http.Handler) http.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }
	uc := h.uc

	handle("GET "+base+"/cash-accounts", h.List)
	handle("POST "+base+"/cash-accounts", h.Create)
	handle("GET "+base+"/cash-accounts/{cashAccountID}", h.Get)
	handle("PUT "+base+"/cash-accounts/{cashAccountID}", h.Update)
	handle("DELETE "+base+"/cash-accounts/{cashAccountID}", h.Delete)

	handle("GET "+base+"/finance-settings", h.GetSettings)
	handle("PUT "+base+"/finance-settings", h.UpdateSettings)

	handle("GET "+base+"/withholding-tax-types", h.listWhtTypes(false))
	handle("POST "+base+"/withholding-tax-types", h.CreateWhtType)
	handle("GET "+base+"/withholding-tax-types/{whtTypeID}", getDoc(h, "whtTypeID", uc.WhtTypes.Get, responses.FromWhtType))
	handle("PUT "+base+"/withholding-tax-types/{whtTypeID}", h.UpdateWhtType)
	handle("DELETE "+base+"/withholding-tax-types/{whtTypeID}", h.DeleteWhtType)

	kinds := func(ks ...string) []string { return ks }
	handle("GET "+base+"/receipts", listDocs(h, kinds("customer", "other"), uc.Receipts.List, responses.FromSummary))
	handle("POST "+base+"/receipts", createDoc[requests.Receipt](h, uc.Receipts.Create, responses.FromReceipt))
	handle("GET "+base+"/receipts/open-invoices", openInvoices(h, "customer_id", uc.Receipts.OpenInvoices))
	handle("GET "+base+"/receipts/{receiptID}", getDoc(h, "receiptID", uc.Receipts.Get, responses.FromReceipt))
	handle("PATCH "+base+"/receipts/{receiptID}", cancelDoc(h, "receiptID", uc.Receipts.Cancel, responses.FromReceipt))

	handle("GET "+base+"/payments", listDocs(h, kinds("supplier", "other"), uc.Payments.List, responses.FromSummary))
	handle("POST "+base+"/payments", createDoc[requests.Payment](h, uc.Payments.Create, responses.FromPayment))
	handle("GET "+base+"/payments/open-invoices", openInvoices(h, "supplier_id", uc.Payments.OpenInvoices))
	handle("GET "+base+"/payments/wht-types", h.listWhtTypes(true))
	handle("GET "+base+"/payments/{paymentID}", getDoc(h, "paymentID", uc.Payments.Get, responses.FromPayment))
	handle("PATCH "+base+"/payments/{paymentID}", cancelDoc(h, "paymentID", uc.Payments.Cancel, responses.FromPayment))

	handle("GET "+base+"/cash-transfers", listDocs(h, nil, uc.Transfers.List, responses.FromTransfer))
	handle("POST "+base+"/cash-transfers", createDoc[requests.Transfer](h, uc.Transfers.Create, responses.FromTransfer))
	handle("GET "+base+"/cash-transfers/{transferID}", getDoc(h, "transferID", uc.Transfers.Get, responses.FromTransfer))
	handle("PATCH "+base+"/cash-transfers/{transferID}", cancelDoc(h, "transferID", uc.Transfers.Cancel, responses.FromTransfer))
}

// RegisterReports ลงทะเบียนรายงานของ finance กับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *Handler) {
	for book, run := range map[string]func(context.Context, dto.Actor, dto.WithholdingFilter) (dto.Withholding, error){
		BookWithholding: h.uc.Reports.WithholdingPaid, BookWithholdingReceived: h.uc.Reports.WithholdingReceived,
	} {
		reg.Register(book, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				f, err := requests.ParseWithholding(r.URL.Query())
				if err != nil {
					return nil, err
				}
				d, err := run(r.Context(), a, f)
				if err != nil {
					return nil, err
				}
				return responses.FromWithholding(book, d), nil
			})
		}))
	}
}
