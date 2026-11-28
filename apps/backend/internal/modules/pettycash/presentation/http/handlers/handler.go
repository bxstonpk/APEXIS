// Package handlers คือ HTTP handler ของ pettycash — แปลง request -> use case input -> response
// ไม่มี business logic, ไม่เรียก repository และไม่เปิด transaction
package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/application/usecases"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/modules/pettycash/presentation/http/requests"
	"apexis/backend/internal/modules/pettycash/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/integration"
)

type CurrentUser func(ctx context.Context) (uuid.UUID, bool)

type Handler struct {
	uc          *usecases.UseCases
	currentUser CurrentUser
}

func NewHandler(uc *usecases.UseCases, currentUser CurrentUser) *Handler {
	return &Handler{uc: uc, currentUser: currentUser}
}

// ชื่อ book ของรายงานเงินสดย่อย (GET /reports?book=...) — ขึ้นต้นด้วย petty-cash- เพราะชื่อ book ใช้ร่วมทั้งระบบ
const (
	BookVoucherRegister = "petty-cash-voucher-register"
	BookFundBalance     = "petty-cash-fund-balance"
	BookInputVat        = "petty-cash-input-vat"
)

var errUnauthenticated = errors.New("unauthenticated")

const (
	badRequest = http.StatusBadRequest
	notFound   = http.StatusNotFound
	conflict   = http.StatusConflict
	rejected   = http.StatusUnprocessableEntity
)

var errorMapper = httpx.NewErrorMapper([]httpx.ErrorRule{
	{Err: requests.ErrValidation, Status: badRequest},
	{Err: domain.ErrInvalidID, Status: badRequest},
	{Err: domain.ErrInvalidCode, Status: badRequest},
	{Err: domain.ErrInvalidName, Status: badRequest},
	{Err: domain.ErrInvalidText, Status: badRequest},
	{Err: domain.ErrInvalidStatus, Status: badRequest},
	{Err: domain.ErrInvalidKind, Status: badRequest},
	{Err: domain.ErrInvalidDate, Status: badRequest},
	{Err: domain.ErrInvalidAmount, Status: badRequest},
	{Err: domain.ErrInvalidLine, Status: badRequest},
	{Err: domain.ErrInvalidVatRate, Status: badRequest},
	{Err: errUnauthenticated, Status: http.StatusUnauthorized},
	{Err: domain.ErrForbidden, Status: http.StatusForbidden},
	{Err: domain.ErrFundNotFound, Status: notFound},
	{Err: domain.ErrVoucherNotFound, Status: notFound},
	{Err: domain.ErrReplenishmentNotFound, Status: notFound},
	{Err: domain.ErrCodeExists, Status: conflict},
	{Err: domain.ErrDocumentNoUsed, Status: conflict},
	{Err: domain.ErrFundInUse, Status: conflict},
	{Err: domain.ErrCompanyInactive, Status: rejected},
	{Err: domain.ErrFundInactive, Status: rejected},
	{Err: domain.ErrCustodianNotMember, Status: rejected},
	{Err: domain.ErrTaxInvoiceRequired, Status: rejected},
	{Err: domain.ErrVoucherNotDraft, Status: rejected},
	{Err: domain.ErrVoucherNotApproved, Status: rejected},
	{Err: domain.ErrVoucherInRequest, Status: rejected},
	{Err: domain.ErrVoucherOtherFund, Status: rejected},
	{Err: domain.ErrVoucherReplenished, Status: rejected},
	{Err: domain.ErrAlreadyCancelled, Status: rejected},
	{Err: domain.ErrInsufficientCash, Status: rejected},
	{Err: domain.ErrFundAlreadyFunded, Status: rejected},
	{Err: domain.ErrFundNotFunded, Status: rejected},
	{Err: domain.ErrRequestNotRequested, Status: rejected},
	{Err: domain.ErrRequestPaid, Status: rejected},
	{Err: domain.ErrOpenRequestExists, Status: rejected},
	{Err: domain.ErrNoFiscalPeriod, Status: rejected},
	{Err: domain.ErrSettingsMissing, Status: rejected},
	{Err: domain.ErrAccountInvalid, Status: rejected},
	{Err: domain.ErrDateBeforeVouchers, Status: rejected},
	{Err: domain.ErrPaymentBeforeRequest, Status: rejected},
	{Err: integration.ErrRejected, Status: rejected},
}, requests.ErrValidation, domain.ErrInvalidAmount, domain.ErrInvalidLine, domain.ErrInvalidVatRate, domain.ErrVoucherNotFound,
	domain.ErrVoucherNotApproved, domain.ErrVoucherInRequest, domain.ErrVoucherOtherFund, domain.ErrInsufficientCash,
	domain.ErrSettingsMissing, domain.ErrAccountInvalid, integration.ErrRejected)

// serve: อ่าน actor (user + {companyID}) -> fn -> เขียน response
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
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, status, body)
}

func one[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return httpx.Data[T]{Data: v}, nil
}

func pathID[T ~[16]byte](r *http.Request, name string) (T, error) {
	return vo.ParseID[T](r.PathValue(name))
}

// ---------- ค่าตั้ง ----------

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
		out, err := h.uc.Settings.Update(r.Context(), dto.SettingsInput{Actor: a, Settings: s})
		return one(responses.FromSettings(out), err)
	})
}

// ---------- กองทุน ----------

func (h *Handler) ListFunds(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseFundFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Funds.List(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return httpx.ToList(res, responses.FromFund), nil
	})
}

func (h *Handler) GetFund(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.FundID](r, "fundID")
		if err != nil {
			return nil, err
		}
		f, err := h.uc.Funds.Get(r.Context(), a, id)
		return one(responses.FromFund(f), err)
	})
}

func (h *Handler) CreateFund(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.Fund
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		in, err := req.ToInput(a, true)
		if err != nil {
			return nil, err
		}
		f, err := h.uc.Funds.Create(r.Context(), in)
		return one(responses.FromFund(f), err)
	})
}

func (h *Handler) UpdateFund(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.FundID](r, "fundID")
		if err != nil {
			return nil, err
		}
		var req requests.Fund
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		in, err := req.ToInput(a, false)
		if err != nil {
			return nil, err
		}
		in.ID = id
		f, err := h.uc.Funds.Update(r.Context(), in)
		return one(responses.FromFund(f), err)
	})
}

// ---------- ใบสำคัญจ่าย ----------

func (h *Handler) ListVouchers(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseVoucherFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Vouchers.List(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return httpx.ToList(res, responses.FromVoucherRow), nil
	})
}

func (h *Handler) GetVoucher(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.VoucherID](r, "voucherID")
		if err != nil {
			return nil, err
		}
		v, err := h.uc.Vouchers.Get(r.Context(), a, id)
		return one(responses.FromVoucher(v), err)
	})
}

func (h *Handler) CreateVoucher(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.Voucher
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData()
		if err != nil {
			return nil, err
		}
		v, err := h.uc.Vouchers.Create(r.Context(), dto.VoucherInput{Actor: a, Data: d})
		return one(responses.FromVoucher(v), err)
	})
}

func (h *Handler) UpdateVoucher(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.VoucherID](r, "voucherID")
		if err != nil {
			return nil, err
		}
		var req requests.Voucher
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData()
		if err != nil {
			return nil, err
		}
		v, err := h.uc.Vouchers.Update(r.Context(), dto.VoucherInput{Actor: a, ID: id, Data: d})
		return one(responses.FromVoucher(v), err)
	})
}

func (h *Handler) voucherAction(fn func(ctx context.Context, in dto.ByVoucher) (dto.Voucher, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			id, err := pathID[vo.VoucherID](r, "voucherID")
			if err != nil {
				return nil, err
			}
			v, err := fn(r.Context(), dto.ByVoucher{Actor: a, ID: id})
			return one(responses.FromVoucher(v), err)
		})
	}
}

func (h *Handler) ApproveVoucher(w http.ResponseWriter, r *http.Request) {
	h.voucherAction(h.uc.Vouchers.Approve)(w, r)
}

func (h *Handler) CancelVoucher(w http.ResponseWriter, r *http.Request) {
	h.voucherAction(h.uc.Vouchers.Cancel)(w, r)
}

// ---------- คำขอ ----------

func (h *Handler) ListReplenishments(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseReplenishmentFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Replenishments.List(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return httpx.ToList(res, responses.FromReplenishmentRow), nil
	})
}

func (h *Handler) GetReplenishment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ReplenishmentID](r, "requestID")
		if err != nil {
			return nil, err
		}
		x, err := h.uc.Replenishments.Get(r.Context(), a, id)
		return one(responses.FromReplenishment(x), err)
	})
}

func (h *Handler) CreateReplenishment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.Replenishment
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		in, err := req.ToInput(a)
		if err != nil {
			return nil, err
		}
		x, err := h.uc.Replenishments.Create(r.Context(), in)
		return one(responses.FromReplenishment(x), err)
	})
}

func (h *Handler) CancelReplenishment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ReplenishmentID](r, "requestID")
		if err != nil {
			return nil, err
		}
		x, err := h.uc.Replenishments.Cancel(r.Context(), dto.ByReplenishment{Actor: a, ID: id})
		return one(responses.FromReplenishment(x), err)
	})
}

// ---------- รายงาน ----------

func (h *Handler) VoucherRegister(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseRegisterFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.Register(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return responses.FromRegister(BookVoucherRegister, res), nil
	})
}

func (h *Handler) FundBalance(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		now := time.Now().UTC()
		asOf, err := requests.ParseAsOf(r.URL.Query(), time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))
		if err != nil {
			return nil, err
		}
		rows, err := h.uc.Reports.FundBalance(r.Context(), a, asOf)
		if err != nil {
			return nil, err
		}
		return responses.FromFundBalance(BookFundBalance, asOf, rows), nil
	})
}

func (h *Handler) InputVat(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseInputVatFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.InputVat(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return responses.FromInputVat(BookInputVat, res), nil
	})
}
