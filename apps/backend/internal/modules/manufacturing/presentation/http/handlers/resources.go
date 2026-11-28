package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/manufacturing/application/dto"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/modules/manufacturing/presentation/http/requests"
	"apexis/backend/internal/modules/manufacturing/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// ชื่อ book ของรายงานการผลิต (GET /reports?book=...)
const (
	BookProductionCost = "production-cost"
	BookMaterialUsage  = "material-usage"
	BookWip            = "wip"
)

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

// ---------- สูตรการผลิต ----------

func (h *Handler) ListBoms(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseBomFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Boms.List(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return httpx.ToList(res, responses.FromBomRow), nil
	})
}

func (h *Handler) GetBom(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BomID](r, "bomID")
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Boms.Get(r.Context(), a, id)
		return one(responses.FromBom(b), err)
	})
}

func (h *Handler) CreateBom(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.Bom
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Boms.Create(r.Context(), dto.BomInput{Actor: a, Data: d})
		return one(responses.FromBom(b), err)
	})
}

func (h *Handler) UpdateBom(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BomID](r, "bomID")
		if err != nil {
			return nil, err
		}
		var req requests.Bom
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Boms.Update(r.Context(), dto.BomInput{Actor: a, ID: id, Data: d})
		return one(responses.FromBom(b), err)
	})
}

func (h *Handler) DeleteBom(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BomID](r, "bomID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Boms.Delete(r.Context(), a, id)
	})
}

// ---------- ใบสั่งผลิต ----------

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseOrderFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Orders.List(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return httpx.ToList(res, responses.FromOrderRow), nil
	})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Get(r.Context(), a, id)
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		var req requests.Order
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, _, err := req.ToData()
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Create(r.Context(), dto.CreateOrderInput{Actor: a, Data: d})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		var req requests.Order
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		d, materials, err := req.ToData()
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Update(r.Context(), dto.UpdateOrderInput{Actor: a, ID: id, Data: d, Materials: materials})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) ReleaseOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Release(r.Context(), dto.OrderAction{Actor: a, ID: id})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) movement(w http.ResponseWriter, r *http.Request, issue bool) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		var req requests.Movement
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		date, lines, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		in := dto.MovementInput{Actor: a, ID: id, Date: date, Lines: lines}
		if issue {
			o, err := h.uc.Orders.Issue(r.Context(), in)
			return one(responses.FromOrder(o), err)
		}
		o, err := h.uc.Orders.Return(r.Context(), in)
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) IssueMaterials(w http.ResponseWriter, r *http.Request)  { h.movement(w, r, true) }
func (h *Handler) ReturnMaterials(w http.ResponseWriter, r *http.Request) { h.movement(w, r, false) }

func (h *Handler) CompleteOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		var req requests.Complete
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		date, qty, err := req.ToInput()
		if err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Complete(r.Context(), dto.CompleteInput{Actor: a, ID: id, Date: date, Quantity: qty})
		return one(responses.FromOrder(o), err)
	})
}

func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.OrderID](r, "orderID")
		if err != nil {
			return nil, err
		}
		var req requests.Cancel
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return nil, err
		}
		o, err := h.uc.Orders.Cancel(r.Context(), dto.CancelInput{Actor: a, ID: id, Reason: req.Reason})
		return one(responses.FromOrder(o), err)
	})
}

// ---------- รายงาน ----------

func (h *Handler) ProductionCost(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseCostFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.ProductionCost(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return responses.FromCost(BookProductionCost, res), nil
	})
}

func (h *Handler) MaterialUsage(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseCostFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.MaterialUsage(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return responses.FromUsage(BookMaterialUsage, httpx.ToList(res, responses.FromUsageRow)), nil
	})
}

func (h *Handler) Wip(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseWipFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Reports.Wip(r.Context(), a, f)
		if err != nil {
			return nil, err
		}
		return responses.FromWip(BookWip, f.AsOf, res), nil
	})
}
