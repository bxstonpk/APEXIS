package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/domain/entities"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
	"apexis/backend/internal/modules/organization/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// ---------- branches ----------

// GET /api/v1/companies/{companyID}/branches?q=&status=&sort=&page=&page_size=
func (h *Handler) ListBranches(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseBranchFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Branches.List(r.Context(), dto.List[dto.BranchFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromBranch), err
	})
}

// GET /api/v1/companies/{companyID}/branches/{branchID}
func (h *Handler) GetBranch(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BranchID](r, "branchID")
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Branches.Get(r.Context(), dto.ByID[vo.BranchID]{Actor: a, ID: id})
		return one(responses.FromBranch(b), err)
	})
}

// POST /api/v1/companies/{companyID}/branches
func (h *Handler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Branch](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Branches.Create(r.Context(), dto.Create[entities.BranchData]{Actor: a, Data: data})
		return one(responses.FromBranch(b), err)
	})
}

// PUT /api/v1/companies/{companyID}/branches/{branchID}
func (h *Handler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BranchID](r, "branchID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Branch](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		b, err := h.uc.Branches.Update(r.Context(), dto.Update[vo.BranchID, entities.BranchData]{Actor: a, ID: id, Data: data})
		return one(responses.FromBranch(b), err)
	})
}

// DELETE /api/v1/companies/{companyID}/branches/{branchID}
func (h *Handler) DeleteBranch(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BranchID](r, "branchID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Branches.Delete(r.Context(), dto.ByID[vo.BranchID]{Actor: a, ID: id})
	})
}

// ---------- business units ----------

// GET /api/v1/companies/{companyID}/business-units?q=&status=&parent_id=&sort=&page=&page_size=
func (h *Handler) ListBusinessUnits(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseBusinessUnitFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.BusinessUnits.List(r.Context(), dto.List[dto.BusinessUnitFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromBusinessUnit), err
	})
}

func (h *Handler) GetBusinessUnit(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BusinessUnitID](r, "businessUnitID")
		if err != nil {
			return nil, err
		}
		u, err := h.uc.BusinessUnits.Get(r.Context(), dto.ByID[vo.BusinessUnitID]{Actor: a, ID: id})
		return one(responses.FromBusinessUnit(u), err)
	})
}

func (h *Handler) CreateBusinessUnit(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.BusinessUnit](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		u, err := h.uc.BusinessUnits.Create(r.Context(), dto.Create[entities.BusinessUnitData]{Actor: a, Data: data})
		return one(responses.FromBusinessUnit(u), err)
	})
}

func (h *Handler) UpdateBusinessUnit(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BusinessUnitID](r, "businessUnitID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.BusinessUnit](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		u, err := h.uc.BusinessUnits.Update(r.Context(), dto.Update[vo.BusinessUnitID, entities.BusinessUnitData]{Actor: a, ID: id, Data: data})
		return one(responses.FromBusinessUnit(u), err)
	})
}

func (h *Handler) DeleteBusinessUnit(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.BusinessUnitID](r, "businessUnitID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.BusinessUnits.Delete(r.Context(), dto.ByID[vo.BusinessUnitID]{Actor: a, ID: id})
	})
}

// ---------- departments ----------

// GET /api/v1/companies/{companyID}/departments?q=&status=&parent_id=&business_unit_id=&branch_id=&sort=&page=&page_size=
func (h *Handler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDepartmentFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Departments.List(r.Context(), dto.List[dto.DepartmentFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromDepartment), err
	})
}

func (h *Handler) GetDepartment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DepartmentID](r, "departmentID")
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Departments.Get(r.Context(), dto.ByID[vo.DepartmentID]{Actor: a, ID: id})
		return one(responses.FromDepartment(d), err)
	})
}

func (h *Handler) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.Department](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Departments.Create(r.Context(), dto.Create[entities.DepartmentData]{Actor: a, Data: data})
		return one(responses.FromDepartment(d), err)
	})
}

func (h *Handler) UpdateDepartment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DepartmentID](r, "departmentID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.Department](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Departments.Update(r.Context(), dto.Update[vo.DepartmentID, entities.DepartmentData]{Actor: a, ID: id, Data: data})
		return one(responses.FromDepartment(d), err)
	})
}

func (h *Handler) DeleteDepartment(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DepartmentID](r, "departmentID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Departments.Delete(r.Context(), dto.ByID[vo.DepartmentID]{Actor: a, ID: id})
	})
}

// ---------- cost centers ----------

// GET /api/v1/companies/{companyID}/cost-centers?q=&status=&department_id=&active_on=&sort=&page=&page_size=
func (h *Handler) ListCostCenters(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseCostCenterFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.CostCenters.List(r.Context(), dto.List[dto.CostCenterFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromCostCenter), err
	})
}

func (h *Handler) GetCostCenter(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.CostCenterID](r, "costCenterID")
		if err != nil {
			return nil, err
		}
		c, err := h.uc.CostCenters.Get(r.Context(), dto.ByID[vo.CostCenterID]{Actor: a, ID: id})
		return one(responses.FromCostCenter(c), err)
	})
}

func (h *Handler) CreateCostCenter(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.CostCenter](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.CostCenters.Create(r.Context(), dto.Create[entities.CostCenterData]{Actor: a, Data: data})
		return one(responses.FromCostCenter(c), err)
	})
}

func (h *Handler) UpdateCostCenter(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.CostCenterID](r, "costCenterID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.CostCenter](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		c, err := h.uc.CostCenters.Update(r.Context(), dto.Update[vo.CostCenterID, entities.CostCenterData]{Actor: a, ID: id, Data: data})
		return one(responses.FromCostCenter(c), err)
	})
}

func (h *Handler) DeleteCostCenter(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.CostCenterID](r, "costCenterID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.CostCenters.Delete(r.Context(), dto.ByID[vo.CostCenterID]{Actor: a, ID: id})
	})
}

// ---------- project sites ----------

// GET /api/v1/companies/{companyID}/project-sites?q=&status=&branch_id=&cost_center_id=&start_date=&end_date=&sort=&page=&page_size=
func (h *Handler) ListProjectSites(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseProjectSiteFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.ProjectSites.List(r.Context(), dto.List[dto.ProjectSiteFilter]{Actor: a, Filter: f})
		return httpx.ToList(res, responses.FromProjectSite), err
	})
}

func (h *Handler) GetProjectSite(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ProjectSiteID](r, "projectSiteID")
		if err != nil {
			return nil, err
		}
		p, err := h.uc.ProjectSites.Get(r.Context(), dto.ByID[vo.ProjectSiteID]{Actor: a, ID: id})
		return one(responses.FromProjectSite(p), err)
	})
}

func (h *Handler) CreateProjectSite(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.ProjectSite](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(true)
		if err != nil {
			return nil, err
		}
		p, err := h.uc.ProjectSites.Create(r.Context(), dto.Create[entities.ProjectSiteData]{Actor: a, Data: data})
		return one(responses.FromProjectSite(p), err)
	})
}

func (h *Handler) UpdateProjectSite(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ProjectSiteID](r, "projectSiteID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.ProjectSite](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.ToData(false)
		if err != nil {
			return nil, err
		}
		p, err := h.uc.ProjectSites.Update(r.Context(), dto.Update[vo.ProjectSiteID, entities.ProjectSiteData]{Actor: a, ID: id, Data: data})
		return one(responses.FromProjectSite(p), err)
	})
}

func (h *Handler) DeleteProjectSite(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.ProjectSiteID](r, "projectSiteID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.ProjectSites.Delete(r.Context(), dto.ByID[vo.ProjectSiteID]{Actor: a, ID: id})
	})
}
