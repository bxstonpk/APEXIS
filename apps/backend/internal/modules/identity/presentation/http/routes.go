package http

import (
	nethttp "net/http"

	"account/backend/internal/modules/identity/presentation/http/handlers"
)

// RegisterRoutes ผูก route ของ identity เข้ากับ mux (ใช้ pattern ของ Go 1.22+)
//
// route ใต้ /companies/{companyID}/... ตรวจสิทธิ์ภายใน command/query อยู่แล้ว
// (ทำใน transaction เดียวกับการแก้ข้อมูล) จึงใช้แค่ Authenticate
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, mw *handlers.Middleware) {
	auth := func(fn nethttp.HandlerFunc) nethttp.Handler { return mw.Authenticate(fn) }

	// public
	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)

	// ตัวเอง
	mux.Handle("GET /api/v1/me", auth(h.Me))
	mux.Handle("PUT /api/v1/me/password", auth(h.ChangePassword))
	mux.Handle("GET /api/v1/me/companies", auth(h.MyCompanies))

	// บริษัท
	mux.Handle("POST /api/v1/companies", auth(h.CreateCompany))
	mux.Handle("GET /api/v1/companies/{companyID}", auth(h.GetCompany))
	mux.Handle("PUT /api/v1/companies/{companyID}", auth(h.UpdateCompany))
	mux.Handle("GET /api/v1/companies/{companyID}/access", auth(h.MyAccess))

	// role
	mux.Handle("GET /api/v1/companies/{companyID}/roles", auth(h.ListRoles))
	mux.Handle("POST /api/v1/companies/{companyID}/roles", auth(h.CreateRole))
	mux.Handle("PUT /api/v1/companies/{companyID}/roles/{roleID}", auth(h.UpdateRole))
	mux.Handle("DELETE /api/v1/companies/{companyID}/roles/{roleID}", auth(h.DeleteRole))

	// พนักงาน (member)
	mux.Handle("GET /api/v1/companies/{companyID}/members", auth(h.ListMembers))
	mux.Handle("POST /api/v1/companies/{companyID}/members", auth(h.AddMember))
	mux.Handle("PUT /api/v1/companies/{companyID}/members/{userID}/roles", auth(h.AssignMemberRoles))
	mux.Handle("POST /api/v1/companies/{companyID}/members/{userID}/suspend", auth(h.SuspendMember))
	mux.Handle("POST /api/v1/companies/{companyID}/members/{userID}/activate", auth(h.ActivateMember))
}
