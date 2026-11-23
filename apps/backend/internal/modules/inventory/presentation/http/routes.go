package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/inventory/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// RegisterRoutes ผูก route ของ inventory (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case; การค้นหาใช้ URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }
	crud := func(collection, param string, c handlers.CRUD) {
		handle("GET "+base+collection, c.List)
		handle("POST "+base+collection, c.Create)
		handle("GET "+base+collection+"/{"+param+"}", c.Get)
		handle("PUT "+base+collection+"/{"+param+"}", c.Update)
		handle("DELETE "+base+collection+"/{"+param+"}", c.Delete)
	}

	crud("/uoms", "uomID", h.Uoms())
	crud("/item-categories", "categoryID", h.Categories())
	crud("/warehouses", "warehouseID", h.Warehouses())
	crud("/items", "itemID", h.Items())

	// เอกสารสต็อก: draft -> (PATCH status=posted) -> posted -> (PATCH status=cancelled) -> cancelled
	handle("GET "+base+"/stock-documents", h.ListDocuments)
	handle("POST "+base+"/stock-documents", h.CreateDocument)
	handle("GET "+base+"/stock-documents/{documentID}", h.GetDocument)
	handle("PUT "+base+"/stock-documents/{documentID}", h.UpdateDocument)
	handle("PATCH "+base+"/stock-documents/{documentID}", h.UpdateDocumentStatus)
	handle("DELETE "+base+"/stock-documents/{documentID}", h.DeleteDocument)

	// ค่าตั้งบัญชีสินค้า + ระบบบัญชี (perpetual / periodic) ตั้งแต่ปีบัญชี
	handle("GET "+base+"/dashboards/inventory", h.Dashboard)
	handle("GET "+base+"/inventory-settings", h.GetSettings)
	handle("PUT "+base+"/inventory-settings", h.UpdateSettings)
	handle("PUT "+base+"/inventory-settings/accounting-systems/{fiscalYear}", h.SetAccountingSystem)

	// ปรับปรุงสินค้าคงเหลือสิ้นงวด (periodic): posted -> (PATCH status=cancelled) -> cancelled
	handle("GET "+base+"/inventory-closings", h.ListClosings)
	handle("POST "+base+"/inventory-closings", h.CreateClosing)
	handle("GET "+base+"/inventory-closings/{closingID}", h.GetClosing)
	handle("PATCH "+base+"/inventory-closings/{closingID}", h.UpdateClosingStatus)
}

// RegisterReports ลงทะเบียนรายงานของ inventory กับ GET /reports?book= (registry ถูก mount พร้อม Authenticate แล้ว)
func RegisterReports(reg *reports.Registry, h *handlers.Handler) {
	reg.Register(handlers.BookStockOnHand, nethttp.HandlerFunc(h.StockOnHand))
	reg.Register(handlers.BookStockCard, nethttp.HandlerFunc(h.StockCard))
}
