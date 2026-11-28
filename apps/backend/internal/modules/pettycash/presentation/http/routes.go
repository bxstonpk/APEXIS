// Package http ผูก route ของ pettycash
package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/pettycash/presentation/http/handlers"
	"apexis/backend/internal/platform/reports"
)

// RegisterRoutes ผูก route ของเงินสดย่อย (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case; การค้นหาใช้ URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	handle("GET "+base+"/petty-cash-settings", h.GetSettings)
	handle("PUT "+base+"/petty-cash-settings", h.UpdateSettings)

	handle("GET "+base+"/petty-cash-funds", h.ListFunds)
	handle("POST "+base+"/petty-cash-funds", h.CreateFund)
	handle("GET "+base+"/petty-cash-funds/{fundID}", h.GetFund)
	handle("PUT "+base+"/petty-cash-funds/{fundID}", h.UpdateFund)

	// ใบสำคัญจ่าย: draft -> approve -> (อยู่ในคำขอเบิกชดเชย -> การเงินจ่าย = replenished); draft / approved -> cancel
	handle("GET "+base+"/petty-cash-vouchers", h.ListVouchers)
	handle("POST "+base+"/petty-cash-vouchers", h.CreateVoucher)
	handle("GET "+base+"/petty-cash-vouchers/{voucherID}", h.GetVoucher)
	handle("PUT "+base+"/petty-cash-vouchers/{voucherID}", h.UpdateVoucher)
	handle("POST "+base+"/petty-cash-vouchers/{voucherID}/approve", h.ApproveVoucher)
	handle("POST "+base+"/petty-cash-vouchers/{voucherID}/cancel", h.CancelVoucher)

	// คำขอตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย — การเงินจ่าย (POST /payments kind=petty_cash)
	handle("GET "+base+"/petty-cash-replenishments", h.ListReplenishments)
	handle("POST "+base+"/petty-cash-replenishments", h.CreateReplenishment)
	handle("GET "+base+"/petty-cash-replenishments/{requestID}", h.GetReplenishment)
	handle("POST "+base+"/petty-cash-replenishments/{requestID}/cancel", h.CancelReplenishment)
}

// RegisterReports ลงทะเบียนรายงานของเงินสดย่อยกับ GET /reports?book=
func RegisterReports(reg *reports.Registry, h *handlers.Handler) {
	reg.Register(handlers.BookVoucherRegister, nethttp.HandlerFunc(h.VoucherRegister))
	reg.Register(handlers.BookFundBalance, nethttp.HandlerFunc(h.FundBalance))
	reg.Register(handlers.BookInputVat, nethttp.HandlerFunc(h.InputVat))
}
