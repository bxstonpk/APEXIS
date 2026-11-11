package http

import (
	nethttp "net/http"

	"apexis/backend/internal/modules/ledger/presentation/http/handlers"
)

// RegisterRoutes ผูก route ของ ledger (RESTful) — ทุก route อยู่ใต้บริษัท และต้อง Authenticate
// สิทธิ์ตรวจภายใน use case (ฝั่งเขียนตรวจใน transaction เดียวกับการแก้ข้อมูล)
// การค้นหา / รายงาน ส่งผ่าน URL query ของ GET เท่านั้น
func RegisterRoutes(mux *nethttp.ServeMux, h *handlers.Handler, authenticate func(nethttp.Handler) nethttp.Handler) {
	const base = "/api/v1/companies/{companyID}"
	handle := func(pattern string, fn nethttp.HandlerFunc) { mux.Handle(pattern, authenticate(fn)) }

	// ผังบัญชี
	handle("GET "+base+"/accounts", h.ListAccounts)
	handle("POST "+base+"/accounts", h.CreateAccount)
	handle("GET "+base+"/accounts/{accountID}", h.GetAccount)
	handle("PUT "+base+"/accounts/{accountID}", h.UpdateAccount)
	handle("DELETE "+base+"/accounts/{accountID}", h.DeleteAccount)

	// สมุดรายวัน
	handle("GET "+base+"/journals", h.ListJournals)
	handle("POST "+base+"/journals", h.CreateJournal)
	handle("GET "+base+"/journals/{journalID}", h.GetJournal)
	handle("PUT "+base+"/journals/{journalID}", h.UpdateJournal)
	handle("DELETE "+base+"/journals/{journalID}", h.DeleteJournal)

	// รายการบัญชี: draft -> (PATCH status=posted) -> posted -> (POST reversals) -> reversed
	handle("GET "+base+"/journal-entries", h.ListEntries)
	handle("POST "+base+"/journal-entries", h.CreateEntry)
	handle("GET "+base+"/journal-entries/{entryID}", h.GetEntry)
	handle("PUT "+base+"/journal-entries/{entryID}", h.UpdateEntry)
	handle("PATCH "+base+"/journal-entries/{entryID}", h.UpdateEntryStatus)
	handle("DELETE "+base+"/journal-entries/{entryID}", h.DeleteEntry)
	handle("POST "+base+"/journal-entries/{entryID}/reversals", h.ReverseEntry)

	// รายงาน — endpoint เดียว เลือกรายงานด้วย ?book= (trial-balance | balance-sheet | income-statement | general-ledger)
	handle("GET "+base+"/reports", h.Report)
}
