package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/usecases"
	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/modules/inventory/presentation/http/requests"
	"apexis/backend/internal/modules/inventory/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// CRUD คือ handler มาตรฐานของ resource หนึ่ง (ผูก route ใน routes.go)
type CRUD struct {
	List, Get, Create, Update, Delete http.HandlerFunc
}

// masterCRUD สร้าง handler ของหน่วยนับ / หมวดสินค้า (รูปแบบเดียวกัน) — param คือชื่อ path parameter ของ id
func masterCRUD[ID ~[16]byte, E any, Req interface {
	ToData(bool) (entities.MasterData, error)
}, Resp any](h *Handler, uc *usecases.MasterUseCases[ID, E], param string, toResp func(dto.Master) Resp) CRUD {
	return CRUD{
		List: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				f, err := requests.ParseMasterFilter(r.URL.Query(), false)
				if err != nil {
					return nil, err
				}
				res, err := uc.List(r.Context(), dto.List[dto.MasterFilter]{Actor: a, Filter: f})
				return httpx.ToList(res, toResp), err
			})
		},
		Get: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[ID](r, param)
				if err != nil {
					return nil, err
				}
				m, err := uc.Get(r.Context(), dto.ByID[ID]{Actor: a, ID: id})
				return one(toResp(m), err)
			})
		},
		Create: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
				data, err := decodeData[Req, entities.MasterData](w, r, true)
				if err != nil {
					return nil, err
				}
				m, err := uc.Create(r.Context(), dto.Create[entities.MasterData]{Actor: a, Data: data})
				return one(toResp(m), err)
			})
		},
		Update: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[ID](r, param)
				if err != nil {
					return nil, err
				}
				data, err := decodeData[Req, entities.MasterData](w, r, false)
				if err != nil {
					return nil, err
				}
				m, err := uc.Update(r.Context(), dto.Update[ID, entities.MasterData]{Actor: a, ID: id, Data: data})
				return one(toResp(m), err)
			})
		},
		Delete: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
				id, err := pathID[ID](r, param)
				if err != nil {
					return nil, err
				}
				return nil, uc.Delete(r.Context(), dto.ByID[ID]{Actor: a, ID: id})
			})
		},
	}
}

// Uoms: /uoms[/{uomID}]
func (h *Handler) Uoms() CRUD {
	return masterCRUD[vo.UomID, *entities.Uom, requests.Master](h, h.uc.Uoms, "uomID", responses.FromMaster)
}

// Categories: /item-categories[/{categoryID}]
func (h *Handler) Categories() CRUD {
	return masterCRUD[vo.CategoryID, *entities.Category, requests.Category](h, h.uc.Categories, "categoryID", responses.FromCategory)
}

// Warehouses: /warehouses[/{warehouseID}] — ?branch_id= กรองตามสาขา
func (h *Handler) Warehouses() CRUD {
	uc := h.uc.Warehouses
	return CRUD{
		List: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				f, err := requests.ParseMasterFilter(r.URL.Query(), true)
				if err != nil {
					return nil, err
				}
				res, err := uc.List(r.Context(), dto.List[dto.MasterFilter]{Actor: a, Filter: f})
				return httpx.ToList(res, responses.FromWarehouse), err
			})
		},
		Get: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.WarehouseID](r, "warehouseID")
				if err != nil {
					return nil, err
				}
				m, err := uc.Get(r.Context(), dto.ByID[vo.WarehouseID]{Actor: a, ID: id})
				return one(responses.FromWarehouse(m), err)
			})
		},
		Create: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
				data, err := decodeData[requests.Warehouse, entities.WarehouseData](w, r, true)
				if err != nil {
					return nil, err
				}
				m, err := uc.Create(r.Context(), dto.Create[entities.WarehouseData]{Actor: a, Data: data})
				return one(responses.FromWarehouse(m), err)
			})
		},
		Update: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.WarehouseID](r, "warehouseID")
				if err != nil {
					return nil, err
				}
				data, err := decodeData[requests.Warehouse, entities.WarehouseData](w, r, false)
				if err != nil {
					return nil, err
				}
				m, err := uc.Update(r.Context(), dto.Update[vo.WarehouseID, entities.WarehouseData]{Actor: a, ID: id, Data: data})
				return one(responses.FromWarehouse(m), err)
			})
		},
		Delete: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.WarehouseID](r, "warehouseID")
				if err != nil {
					return nil, err
				}
				return nil, uc.Delete(r.Context(), dto.ByID[vo.WarehouseID]{Actor: a, ID: id})
			})
		},
	}
}

// Items: /items[/{itemID}]
func (h *Handler) Items() CRUD {
	uc := h.uc.Items
	return CRUD{
		List: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				f, err := requests.ParseItemFilter(r.URL.Query())
				if err != nil {
					return nil, err
				}
				res, err := uc.List(r.Context(), dto.List[dto.ItemFilter]{Actor: a, Filter: f})
				return httpx.ToList(res, responses.FromItem), err
			})
		},
		Get: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.ItemID](r, "itemID")
				if err != nil {
					return nil, err
				}
				i, err := uc.Get(r.Context(), dto.ByID[vo.ItemID]{Actor: a, ID: id})
				return one(responses.FromItem(i), err)
			})
		},
		Create: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
				data, err := decodeData[requests.Item, entities.ItemData](w, r, true)
				if err != nil {
					return nil, err
				}
				i, err := uc.Create(r.Context(), dto.Create[entities.ItemData]{Actor: a, Data: data})
				return one(responses.FromItem(i), err)
			})
		},
		Update: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.ItemID](r, "itemID")
				if err != nil {
					return nil, err
				}
				data, err := decodeData[requests.Item, entities.ItemData](w, r, false)
				if err != nil {
					return nil, err
				}
				i, err := uc.Update(r.Context(), dto.Update[vo.ItemID, entities.ItemData]{Actor: a, ID: id, Data: data})
				return one(responses.FromItem(i), err)
			})
		},
		Delete: func(w http.ResponseWriter, r *http.Request) {
			h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
				id, err := pathID[vo.ItemID](r, "itemID")
				if err != nil {
					return nil, err
				}
				return nil, uc.Delete(r.Context(), dto.ByID[vo.ItemID]{Actor: a, ID: id})
			})
		},
	}
}
