package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/purchasing/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// RegisterRoutes ผูก route ของ purchasing (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case; การค้นหาใช้ URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	handle("GET "+base+"/purchasing-settings", h.GetSettings)
	handle("PUT "+base+"/purchasing-settings", h.UpdateSettings)

	handle("GET "+base+"/suppliers", h.ListSuppliers)
	handle("POST "+base+"/suppliers", h.CreateSupplier)
	handle("GET "+base+"/suppliers/{supplierID}", h.GetSupplier)
	handle("PUT "+base+"/suppliers/{supplierID}", h.UpdateSupplier)
	handle("DELETE "+base+"/suppliers/{supplierID}", h.DeleteSupplier)

	// ใบสั่งซื้อ: draft -> (PATCH status=approved) -> approved -> closed / cancelled
	handle("GET "+base+"/purchase-orders", h.ListOrders)
	handle("POST "+base+"/purchase-orders", h.CreateOrder)
	handle("GET "+base+"/purchase-orders/{orderID}", h.GetOrder)
	handle("PUT "+base+"/purchase-orders/{orderID}", h.UpdateOrder)
	handle("PATCH "+base+"/purchase-orders/{orderID}", h.UpdateOrderStatus)
	handle("DELETE "+base+"/purchase-orders/{orderID}", h.DeleteOrder)

	// ใบรับสินค้า / ใบแจ้งหนี้: สร้าง = ลงบัญชีทันที (posted) -> (PATCH status=cancelled) -> cancelled
	handle("GET "+base+"/goods-receipts", h.ListReceipts)
	handle("POST "+base+"/goods-receipts", h.CreateReceipt)
	handle("GET "+base+"/goods-receipts/{receiptID}", h.GetReceipt)
	handle("PATCH "+base+"/goods-receipts/{receiptID}", h.CancelReceipt)

	handle("GET "+base+"/supplier-invoices", h.ListInvoices)
	handle("POST "+base+"/supplier-invoices", h.CreateInvoice)
	handle("GET "+base+"/supplier-invoices/{invoiceID}", h.GetInvoice)
	handle("PATCH "+base+"/supplier-invoices/{invoiceID}", h.CancelInvoice)

	// ใบจ่ายชำระ / ประเภทภาษีหัก ณ ที่จ่าย / ภ.ง.ด.3, 53 อยู่ที่ finance — ตัดยอดใบแจ้งหนี้ผ่าน Module.SettlePayables
}

// RegisterReports ลงทะเบียนรายงานของ purchasing กับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *handlers.Handler) {
	reg.Register(handlers.BookOutstandingOrders, nethttp.HandlerFunc(h.OutstandingOrders))
	reg.Register(handlers.BookAPAging, nethttp.HandlerFunc(h.APAging))
}
