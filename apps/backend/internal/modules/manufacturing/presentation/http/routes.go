package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/manufacturing/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// RegisterRoutes ผูก route ของ manufacturing (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ + ประเภทธุรกิจตรวจภายใน use case; การค้นหาใช้ URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	handle("GET "+base+"/manufacturing-settings", h.GetSettings)
	handle("PUT "+base+"/manufacturing-settings", h.UpdateSettings)

	handle("GET "+base+"/boms", h.ListBoms)
	handle("POST "+base+"/boms", h.CreateBom)
	handle("GET "+base+"/boms/{bomID}", h.GetBom)
	handle("PUT "+base+"/boms/{bomID}", h.UpdateBom)
	handle("DELETE "+base+"/boms/{bomID}", h.DeleteBom)

	// ใบสั่งผลิต: draft -> release -> (issue / return)* -> complete; draft / released / completed -> cancel
	handle("GET "+base+"/production-orders", h.ListOrders)
	handle("POST "+base+"/production-orders", h.CreateOrder)
	handle("GET "+base+"/production-orders/{orderID}", h.GetOrder)
	handle("PUT "+base+"/production-orders/{orderID}", h.UpdateOrder)
	handle("POST "+base+"/production-orders/{orderID}/release", h.ReleaseOrder)
	handle("POST "+base+"/production-orders/{orderID}/issue", h.IssueMaterials)
	handle("POST "+base+"/production-orders/{orderID}/return", h.ReturnMaterials)
	handle("POST "+base+"/production-orders/{orderID}/complete", h.CompleteOrder)
	handle("POST "+base+"/production-orders/{orderID}/cancel", h.CancelOrder)
}

// RegisterReports ลงทะเบียนรายงานของ manufacturing กับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *handlers.Handler) {
	reg.Register(handlers.BookProductionCost, nethttp.HandlerFunc(h.ProductionCost))
	reg.Register(handlers.BookMaterialUsage, nethttp.HandlerFunc(h.MaterialUsage))
	reg.Register(handlers.BookWip, nethttp.HandlerFunc(h.Wip))
}
