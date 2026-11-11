package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/organization/presentation/http/handlers"
)

// RegisterRoutes ผูก route ของ organization (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case (ฝั่งเขียนตรวจใน transaction เดียวกับการแก้ข้อมูล)
// การค้นหา / กรอง / เรียง / แบ่งหน้า ส่งผ่าน URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	// บริษัท (tenant) — สร้างแล้วผู้สร้างเป็น owner, สิทธิ์ / สมาชิกจัดการที่ identity
	handle("POST /api/v1/companies", h.CreateCompany)
	handle("GET /api/v1/me/companies", h.ListMyCompanies)
	handle("GET "+base, h.GetCompany)
	handle("PUT "+base, h.UpdateCompany)

	// สาขา
	handle("GET "+base+"/branches", h.ListBranches)
	handle("POST "+base+"/branches", h.CreateBranch)
	handle("GET "+base+"/branches/{branchID}", h.GetBranch)
	handle("PUT "+base+"/branches/{branchID}", h.UpdateBranch)
	handle("DELETE "+base+"/branches/{branchID}", h.DeleteBranch)

	// หน่วยธุรกิจ
	handle("GET "+base+"/business-units", h.ListBusinessUnits)
	handle("POST "+base+"/business-units", h.CreateBusinessUnit)
	handle("GET "+base+"/business-units/{businessUnitID}", h.GetBusinessUnit)
	handle("PUT "+base+"/business-units/{businessUnitID}", h.UpdateBusinessUnit)
	handle("DELETE "+base+"/business-units/{businessUnitID}", h.DeleteBusinessUnit)

	// แผนก
	handle("GET "+base+"/departments", h.ListDepartments)
	handle("POST "+base+"/departments", h.CreateDepartment)
	handle("GET "+base+"/departments/{departmentID}", h.GetDepartment)
	handle("PUT "+base+"/departments/{departmentID}", h.UpdateDepartment)
	handle("DELETE "+base+"/departments/{departmentID}", h.DeleteDepartment)

	// ศูนย์ต้นทุน
	handle("GET "+base+"/cost-centers", h.ListCostCenters)
	handle("POST "+base+"/cost-centers", h.CreateCostCenter)
	handle("GET "+base+"/cost-centers/{costCenterID}", h.GetCostCenter)
	handle("PUT "+base+"/cost-centers/{costCenterID}", h.UpdateCostCenter)
	handle("DELETE "+base+"/cost-centers/{costCenterID}", h.DeleteCostCenter)

	// โครงการ / ไซต์งาน
	handle("GET "+base+"/project-sites", h.ListProjectSites)
	handle("POST "+base+"/project-sites", h.CreateProjectSite)
	handle("GET "+base+"/project-sites/{projectSiteID}", h.GetProjectSite)
	handle("PUT "+base+"/project-sites/{projectSiteID}", h.UpdateProjectSite)
	handle("DELETE "+base+"/project-sites/{projectSiteID}", h.DeleteProjectSite)

	// ปีบัญชี / งวดบัญชี
	handle("GET "+base+"/fiscal-years", h.ListFiscalYears)
	handle("POST "+base+"/fiscal-years", h.CreateFiscalYear)
	handle("GET "+base+"/fiscal-years/{fiscalYearID}", h.GetFiscalYear)
	handle("GET "+base+"/fiscal-periods", h.ListFiscalPeriods)
	handle("GET "+base+"/fiscal-periods/{fiscalPeriodID}", h.GetFiscalPeriod)
	handle("PATCH "+base+"/fiscal-periods/{fiscalPeriodID}", h.UpdateFiscalPeriod)
}
