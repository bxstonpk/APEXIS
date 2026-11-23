// Package handlers คือ HTTP handler ของ sales — แปลง request -> use case -> response; ไม่มี business logic
package handlers

import (
	"apexis/backend/internal/platform/dashperiod"
	"apexis/backend/internal/platform/thaiaddress"
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/usecases"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/modules/sales/presentation/http/requests"
	"apexis/backend/internal/modules/sales/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/reports"
)

type CurrentUser func(ctx context.Context) (uuid.UUID, bool)

type Handler struct {
	uc          *usecases.UseCases
	currentUser CurrentUser
}

func NewHandler(uc *usecases.UseCases, currentUser CurrentUser) *Handler {
	return &Handler{uc: uc, currentUser: currentUser}
}

var errUnauthenticated = errors.New("unauthenticated")

const (
	bad      = http.StatusBadRequest
	notFound = http.StatusNotFound
	conflict = http.StatusConflict
	rejected = http.StatusUnprocessableEntity
)

var errorMapper = func() *httpx.ErrorMapper {
	rules := []httpx.ErrorRule{{Err: requests.ErrValidation, Status: bad}, {Err: errUnauthenticated, Status: http.StatusUnauthorized},
		{Err: domain.ErrForbidden, Status: http.StatusForbidden}, {Err: integration.ErrRejected, Status: rejected}}
	for status, errs := range map[int][]error{
		bad: {domain.ErrInvalidID, domain.ErrInvalidCode, domain.ErrInvalidName, domain.ErrInvalidText, domain.ErrInvalidStatus,
			domain.ErrInvalidDate, domain.ErrInvalidTaxID, domain.ErrInvalidBranchNo, domain.ErrDuplicateBranch, domain.ErrInvalidTerm, domain.ErrInvalidTaxType,
			domain.ErrInvalidVatRate, domain.ErrInvalidAmount, domain.ErrInvalidLine, thaiaddress.ErrInvalidAddress},
		notFound: {domain.ErrCustomerNotFound, domain.ErrQuotationNotFound, domain.ErrOrderNotFound, domain.ErrDeliveryNotFound,
			domain.ErrInvoiceNotFound, domain.ErrItemNotFound},
		conflict: {domain.ErrCodeExists, domain.ErrTaxIDExists, domain.ErrInUse, domain.ErrDocumentNoUsed, domain.ErrQuotationUsed},
		rejected: {domain.ErrCompanyInactive, domain.ErrCustomerInactive, domain.ErrItemInactive, domain.ErrVatNotApplicable,
			domain.ErrNotDraft, domain.ErrQuotationState, domain.ErrQuotationExpired, domain.ErrOrderNotApproved, domain.ErrOrderNotOpen,
			domain.ErrOrderHasActivity, domain.ErrOverDelivery, domain.ErrOverInvoice, domain.ErrDeliveryInvoiced, domain.ErrAlreadyCancelled,
			domain.ErrInvoicePaid, domain.ErrOverReceipt, domain.ErrInvoiceNotPayable, domain.ErrNoFiscalPeriod,
			domain.ErrPeriodClosed, domain.ErrSettingsMissing, domain.ErrAccountInvalid, domain.ErrJournalInvalid, domain.ErrWarehouseRequired, domain.ErrBranchNotFound},
	} {
		for _, e := range errs {
			rules = append(rules, httpx.ErrorRule{Err: e, Status: status})
		}
	}
	return httpx.NewErrorMapper(rules, requests.ErrValidation, integration.ErrRejected, thaiaddress.ErrInvalidAddress,
		domain.ErrDuplicateBranch, domain.ErrTaxIDExists, domain.ErrBranchNotFound, domain.ErrInvalidAmount, domain.ErrInvalidLine,
		domain.ErrInvoiceNotFound, domain.ErrItemNotFound, domain.ErrItemInactive,
		domain.ErrQuotationState, domain.ErrOverDelivery, domain.ErrOverInvoice, domain.ErrDeliveryInvoiced, domain.ErrOverReceipt,
		domain.ErrInvoiceNotPayable, domain.ErrSettingsMissing, domain.ErrAccountInvalid)
}()

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, status int, fn func(a dto.Actor) (any, error)) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := vo.ParseID[vo.CompanyID](r.PathValue("companyID"))
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	body, err := fn(dto.Actor{UserID: vo.UserID(userID), CompanyID: companyID})
	switch {
	case err != nil:
		errorMapper.Write(w, r, err)
	case status == http.StatusNoContent:
		w.WriteHeader(status)
	default:
		httpx.WriteJSON(w, status, body)
	}
}

func one[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return httpx.Data[T]{Data: v}, nil
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	err := httpx.DecodeJSON(w, r, &v)
	return v, err
}

func byID[T ~[16]byte](r *http.Request, a dto.Actor, param string) (dto.ByID[T], error) {
	id, err := vo.ParseID[T](r.PathValue(param))
	return dto.ByID[T]{Actor: a, ID: id}, err
}

// getOne: GET /x/{id}
func getOne[ID ~[16]byte, D any, R any](h *Handler, param string, get func(context.Context, dto.ByID[ID]) (D, error), to func(D) R) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			in, err := byID[ID](r, a, param)
			if err != nil {
				return nil, err
			}
			d, err := get(r.Context(), in)
			return one(to(d), err)
		})
	}
}

// cancelOne: PATCH /x/{id} {"status":"cancelled"}
func cancelOne[ID ~[16]byte, D any, R any](h *Handler, param string, cancel func(context.Context, dto.ByID[ID]) (D, error), to func(D) R) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			in, err := byID[ID](r, a, param)
			if err != nil {
				return nil, err
			}
			req, err := decode[requests.Status](w, r)
			if err != nil {
				return nil, err
			}
			if req.Status != string(vo.DocCancelled) {
				return nil, requests.Invalid("status must be cancelled")
			}
			d, err := cancel(r.Context(), in)
			return one(to(d), err)
		})
	}
}

func list[F any, D any, R any](h *Handler, parse func(*http.Request) (F, error), run func(context.Context, dto.List[F]) (listing.Result[D], error), to func(D) R) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			f, err := parse(r)
			if err != nil {
				return nil, err
			}
			res, err := run(r.Context(), dto.List[F]{Actor: a, Filter: f})
			return httpx.ToList(res, to), err
		})
	}
}

func docFilter(kind requests.DocKind) func(*http.Request) (dto.DocumentFilter, error) {
	return func(r *http.Request) (dto.DocumentFilter, error) {
		return requests.ParseDocumentFilter(r.URL.Query(), kind)
	}
}

// ---------- ค่าตั้ง / ลูกค้า ----------

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		s, err := h.uc.Settings.Get(r.Context(), a)
		return one(responses.FromSettings(s), err)
	})
}

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

func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Customer](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Customers.Create(r.Context(), dto.Create[entities.CustomerData]{Actor: a, Data: data})
		return one(responses.FromCustomer(c), err)
	})
}

func (h *Handler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID[vo.CustomerID](r, a, "customerID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Customer](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.Customers.Update(r.Context(), dto.Update[vo.CustomerID, entities.CustomerData]{Actor: a, ID: in.ID, Data: data})
		return one(responses.FromCustomer(c), err)
	})
}

func (h *Handler) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		in, err := byID[vo.CustomerID](r, a, "customerID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Customers.Delete(r.Context(), in)
	})
}

// ---------- ใบเสนอราคา ----------

func (h *Handler) CreateQuotation(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Quotation](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		q, err := h.uc.Quotations.Create(r.Context(), dto.Create[entities.QuotationData]{Actor: a, Data: data})
		return one(responses.FromQuotation(q), err)
	})
}

func (h *Handler) UpdateQuotation(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID[vo.QuotationID](r, a, "quotationID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Quotation](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData()
		if err != nil {
			return nil, err
		}
		q, err := h.uc.Quotations.Update(r.Context(), dto.Update[vo.QuotationID, entities.QuotationData]{Actor: a, ID: in.ID, Data: data})
		return one(responses.FromQuotation(q), err)
	})
}

// PATCH /quotations/{id} — {"status":"sent"|"accepted"|"rejected"}
func (h *Handler) UpdateQuotationStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID[vo.QuotationID](r, a, "quotationID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Status](w, r)
		if err != nil {
			return nil, err
		}
		st, err := vo.ParseQuotationStatus(req.Status)
		if err != nil || st == vo.QuotationDraft {
			return nil, requests.Invalid("status must be one of sent, accepted, rejected")
		}
		q, err := h.uc.Quotations.SetStatus(r.Context(), in, st)
		return one(responses.FromQuotation(q), err)
	})
}

func (h *Handler) DeleteQuotation(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		in, err := byID[vo.QuotationID](r, a, "quotationID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Quotations.Delete(r.Context(), in)
	})
}

// ---------- ใบสั่งขาย ----------

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Order](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Create(r.Context(), dto.Create[entities.OrderData]{Actor: a, Data: data})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID[vo.OrderID](r, a, "orderID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Order](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Update(r.Context(), dto.Update[vo.OrderID, entities.OrderData]{Actor: a, ID: in.ID, Data: data})
		return one(responses.FromOrder(o), err)
	})
}

// PATCH /sales-orders/{id} — {"status":"approved"|"closed"|"cancelled"} (approved ตอบ warnings เมื่อเกินวงเงิน)
func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		in, err := byID[vo.OrderID](r, a, "orderID")
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
		o, err := h.uc.Orders.SetStatus(r.Context(), in, st)
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		in, err := byID[vo.OrderID](r, a, "orderID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Orders.Delete(r.Context(), in)
	})
}

// ---------- ใบส่งของ / ใบแจ้งหนี้ / ใบเสร็จ ----------

func create[Req interface{ ToInput() (In, error) }, In any, D any, R any](h *Handler, run func(context.Context, dto.Create[In]) (D, error), to func(D) R) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
			req, err := decode[Req](w, r)
			if err != nil {
				return nil, err
			}
			in, err := req.ToInput()
			if err != nil {
				return nil, err
			}
			d, err := run(r.Context(), dto.Create[In]{Actor: a, Data: in})
			return one(to(d), err)
		})
	}
}

// ---------- routes / reports ----------

const (
	BookOutstanding = "outstanding-sales-orders"
	BookARAging     = "ar-aging"
	BookOutputVat   = "output-vat"
)

func report[F any, D any, B any](h *Handler, parse func(*http.Request) (F, error), run func(context.Context, dto.List[F]) (D, error), to func(D) B) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			f, err := parse(r)
			if err != nil {
				return nil, err
			}
			d, err := run(r.Context(), dto.List[F]{Actor: a, Filter: f})
			if err != nil {
				return nil, err
			}
			return to(d), nil
		})
	})
}

// Dashboard: GET /dashboards/sales?start_date=&end_date= (บังคับทั้งคู่)
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

// RegisterRoutes ผูก route ของ sales — ทุก route ต้อง Authenticate, สิทธิ์ตรวจใน use case, ค้นหาด้วย URL query
func RegisterRoutes(mux *http.ServeMux, h *Handler, authenticate func(http.Handler) http.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }
	uc := h.uc

	handle("GET "+base+"/dashboards/sales", h.Dashboard)
	handle("GET "+base+"/sales-settings", h.GetSettings)
	handle("PUT "+base+"/sales-settings", h.UpdateSettings)

	handle("GET "+base+"/customers", list(h, func(r *http.Request) (dto.CustomerFilter, error) { return requests.ParseCustomerFilter(r.URL.Query()) },
		uc.Customers.List, responses.FromCustomer))
	handle("POST "+base+"/customers", h.CreateCustomer)
	handle("GET "+base+"/customers/{customerID}", getOne(h, "customerID", uc.Customers.Get, responses.FromCustomer))
	handle("PUT "+base+"/customers/{customerID}", h.UpdateCustomer)
	handle("DELETE "+base+"/customers/{customerID}", h.DeleteCustomer)

	// ใบเสนอราคา: draft -> sent (เลขที่) -> accepted / rejected; แปลงเป็นใบสั่งขายด้วย POST /sales-orders {"quotation_id"}
	handle("GET "+base+"/quotations", list(h, docFilter(requests.KindQuotation), uc.Quotations.List, responses.FromSalesDocSummary))
	handle("POST "+base+"/quotations", h.CreateQuotation)
	handle("GET "+base+"/quotations/{quotationID}", getOne(h, "quotationID", uc.Quotations.Get, responses.FromQuotation))
	handle("PUT "+base+"/quotations/{quotationID}", h.UpdateQuotation)
	handle("PATCH "+base+"/quotations/{quotationID}", h.UpdateQuotationStatus)
	handle("DELETE "+base+"/quotations/{quotationID}", h.DeleteQuotation)

	handle("GET "+base+"/sales-orders", list(h, docFilter(requests.KindOrder), uc.Orders.List, responses.FromSalesDocSummary))
	handle("POST "+base+"/sales-orders", h.CreateOrder)
	handle("GET "+base+"/sales-orders/{orderID}", getOne(h, "orderID", uc.Orders.Get, responses.FromOrder))
	handle("PUT "+base+"/sales-orders/{orderID}", h.UpdateOrder)
	handle("PATCH "+base+"/sales-orders/{orderID}", h.UpdateOrderStatus)
	handle("DELETE "+base+"/sales-orders/{orderID}", h.DeleteOrder)

	// เอกสารที่ลงบัญชีทันที: POST = posted -> PATCH {"status":"cancelled"}
	handle("GET "+base+"/deliveries", list(h, docFilter(requests.KindPosted), uc.Deliveries.List, responses.FromPostedSummary))
	handle("POST "+base+"/deliveries", create[requests.Delivery](h, uc.Deliveries.Create, responses.FromDelivery))
	handle("GET "+base+"/deliveries/{deliveryID}", getOne(h, "deliveryID", uc.Deliveries.Get, responses.FromDelivery))
	handle("PATCH "+base+"/deliveries/{deliveryID}", cancelOne(h, "deliveryID", uc.Deliveries.Cancel, responses.FromDelivery))

	handle("GET "+base+"/sales-invoices", list(h, docFilter(requests.KindPosted), uc.Invoices.List, responses.FromInvoiceSummary))
	handle("POST "+base+"/sales-invoices", create[requests.Invoice](h, uc.Invoices.Create, responses.FromInvoice))
	handle("GET "+base+"/sales-invoices/{invoiceID}", getOne(h, "invoiceID", uc.Invoices.Get, responses.FromInvoice))
	handle("PATCH "+base+"/sales-invoices/{invoiceID}", cancelOne(h, "invoiceID", uc.Invoices.Cancel, responses.FromInvoice))

	// ใบเสร็จรับเงินอยู่ที่ finance (POST /customer-receipts) — ตัดยอดใบแจ้งหนี้ผ่าน Module.SettleReceivables
}

// RegisterReports ลงทะเบียนรายงานของ sales กับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *Handler) {
	uc := h.uc
	reg.Register(BookOutstanding, report(h, func(r *http.Request) (dto.OutstandingFilter, error) { return requests.ParseOutstanding(r.URL.Query()) },
		uc.Reports.Outstanding, func(d listing.Result[dto.OutstandingRow]) any { return responses.FromOutstanding(BookOutstanding, d) }))
	reg.Register(BookARAging, report(h, func(r *http.Request) (dto.AgingFilter, error) { return requests.ParseAging(r.URL.Query()) },
		uc.Reports.Aging, func(d dto.Aging) any { return responses.FromAging(BookARAging, d) }))
	reg.Register(BookOutputVat, report(h, func(r *http.Request) (dto.TaxFilter, error) { return requests.ParseTax(r.URL.Query()) },
		uc.Reports.OutputVat, func(d dto.TaxReport) any { return responses.FromTax(BookOutputVat, d) }))
}
