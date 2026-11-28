package http

import (
	nethttp "net/http"

	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/modules/purchasing/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// RegisterRoutes ผูก route ของ purchasing (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case; การค้นหาใช้ URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	handle("GET "+base+"/dashboards/purchasing", h.Dashboard)
	handle("GET "+base+"/purchasing-settings", h.GetSettings)
	handle("PUT "+base+"/purchasing-settings", h.UpdateSettings)

	handle("GET "+base+"/suppliers", h.ListSuppliers)
	handle("POST "+base+"/suppliers", h.CreateSupplier)
	handle("GET "+base+"/suppliers/{supplierID}", h.GetSupplier)
	handle("PUT "+base+"/suppliers/{supplierID}", h.UpdateSupplier)
	handle("DELETE "+base+"/suppliers/{supplierID}", h.DeleteSupplier)

	// รายการรวม "เอกสารซื้อ": ใบสั่งซื้อ / ใบรับสินค้า / ซื้อสินค้า / ใบสั่งจ้าง / ตรวจรับงาน / ซื้อบริการ (เฉพาะชนิดที่มีสิทธิ์อ่าน)
	handle("GET "+base+"/purchase-documents", h.ListPurchaseDocuments)

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

	// สายจ้างบริการ: ใบสั่งจ้าง -> ใบตรวจรับงาน (ไม่ลงบัญชี) -> ซื้อบริการ (ภาษีซื้อยังไม่ถึงกำหนด) -> บันทึกใบกำกับภาษีที่ได้รับ
	// handler เดียวกับสายสินค้า (ส่งสายผ่าน context) — เอกสารต่างสาย = 404
	svc := func(fn nethttp.HandlerFunc) nethttp.HandlerFunc { return handlers.WithKind(vo.KindService, fn) }
	handle("GET "+base+"/work-orders", svc(h.ListOrders))
	handle("POST "+base+"/work-orders", svc(h.CreateOrder))
	handle("GET "+base+"/work-orders/{orderID}", svc(h.GetOrder))
	handle("PUT "+base+"/work-orders/{orderID}", svc(h.UpdateOrder))
	handle("PATCH "+base+"/work-orders/{orderID}", svc(h.UpdateOrderStatus))
	handle("DELETE "+base+"/work-orders/{orderID}", svc(h.DeleteOrder))

	handle("GET "+base+"/service-acceptances", svc(h.ListReceipts))
	handle("POST "+base+"/service-acceptances", svc(h.CreateReceipt))
	handle("GET "+base+"/service-acceptances/{receiptID}", svc(h.GetReceipt))
	handle("PATCH "+base+"/service-acceptances/{receiptID}", svc(h.CancelReceipt))

	handle("GET "+base+"/supplier-service-invoices", svc(h.ListInvoices))
	handle("POST "+base+"/supplier-service-invoices", svc(h.CreateInvoice))
	handle("GET "+base+"/supplier-service-invoices/{invoiceID}", svc(h.GetInvoice))
	handle("PATCH "+base+"/supplier-service-invoices/{invoiceID}", svc(h.CancelInvoice))
	handle("POST "+base+"/supplier-service-invoices/{invoiceID}/tax-invoices", h.RecordTaxInvoice)
	handle("PATCH "+base+"/supplier-service-invoices/{invoiceID}/tax-invoices/{taxInvoiceID}", h.CancelTaxInvoice)

	// ใบลดหนี้ผู้ขาย (ส่งคืนสินค้า / ลดราคา) อ้างใบแจ้งหนี้เดิม: สร้าง = ลงบัญชี + ตัดยอดเจ้าหนี้ทันที -> PATCH status=cancelled
	handle("GET "+base+"/supplier-credit-notes", h.ListCreditNotes)
	handle("POST "+base+"/supplier-credit-notes", h.CreateCreditNote)
	handle("GET "+base+"/supplier-credit-notes/{creditNoteID}", h.GetCreditNote)
	handle("PATCH "+base+"/supplier-credit-notes/{creditNoteID}", h.CancelCreditNote)

	// ใบจ่ายชำระ / ประเภทภาษีหัก ณ ที่จ่าย / ภ.ง.ด.3, 53 อยู่ที่ finance — ตัดยอดใบแจ้งหนี้ผ่าน Module.SettlePayables
}

// RegisterReports ลงทะเบียนรายงานของ purchasing กับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *handlers.Handler) {
	reg.Register(handlers.BookOutstandingOrders, nethttp.HandlerFunc(h.OutstandingOrders))
	reg.Register(handlers.BookAPAging, nethttp.HandlerFunc(h.APAging))
	reg.Register(handlers.BookInputVat, nethttp.HandlerFunc(h.InputVat))
	reg.Register(handlers.BookOutstandingWorkOrders, nethttp.HandlerFunc(h.OutstandingWorkOrders))
	reg.Register(handlers.BookUndueInputVat, nethttp.HandlerFunc(h.UndueInputVat))
}
