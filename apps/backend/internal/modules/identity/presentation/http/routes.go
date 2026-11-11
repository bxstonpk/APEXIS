package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/identity/presentation/http/handlers"
)

// RegisterRoutes ผูก route ของ identity เข้ากับ mux (ใช้ pattern ของ Go 1.22+)
//
// RESTful: path ระบุ resource (คำนามพหูพจน์), method ระบุการกระทำ, ไม่มีคำกริยาใน path
// การค้นหา / กรอง / เรียง / แบ่งหน้า ส่งผ่าน URL query ของ GET เท่านั้น
//
// route ใต้ /companies/{companyID}/... ตรวจสิทธิ์ภายใน use case อยู่แล้ว
// (ฝั่งเขียนตรวจใน transaction เดียวกับการแก้ข้อมูล) จึงใช้แค่ Authenticate
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, mw *handlers.Middleware) {
	auth := func(fn nethttp.HandlerFunc) nethttp.Handler { return mw.Authenticate(fn) }

	// public
	mux.HandleFunc("POST /api/v1/users", h.Register)    // สมัครสมาชิก
	mux.HandleFunc("POST /api/v1/auth/tokens", h.Login) // ล็อกอิน = สร้าง access token

	// ตัวเอง
	mux.Handle("GET /api/v1/me", auth(h.Me))
	mux.Handle("PUT /api/v1/me/password", auth(h.ChangePassword))

	// สิทธิ์ของตัวเองในบริษัท — ตัวบริษัท (สร้าง / ดู / แก้, บริษัทของฉัน) อยู่ที่ organization module
	mux.Handle("GET /api/v1/companies/{companyID}/access", auth(h.MyAccess))

	// role
	mux.Handle("GET /api/v1/companies/{companyID}/roles", auth(h.ListRoles)) // ?q=&is_system=&sort=&page=&page_size=
	mux.Handle("POST /api/v1/companies/{companyID}/roles", auth(h.CreateRole))
	mux.Handle("PUT /api/v1/companies/{companyID}/roles/{roleID}", auth(h.UpdateRole))
	mux.Handle("DELETE /api/v1/companies/{companyID}/roles/{roleID}", auth(h.DeleteRole))

	// พนักงาน (member)
	mux.Handle("GET /api/v1/companies/{companyID}/members", auth(h.ListMembers)) // ?q=&status=&role_id=&sort=&page=&page_size=
	mux.Handle("POST /api/v1/companies/{companyID}/members", auth(h.AddMember))
	mux.Handle("PATCH /api/v1/companies/{companyID}/members/{userID}", auth(h.UpdateMember)) // {"status": ...}
	mux.Handle("PUT /api/v1/companies/{companyID}/members/{userID}/roles", auth(h.AssignMemberRoles))
}
