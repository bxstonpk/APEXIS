package handlers

import (
	"net/http"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/presentation/http/requests"
	"apexis/backend/internal/modules/ledger/presentation/http/responses"
)

// GET /api/v1/companies/{companyID}/reports?book=...&start_date=&end_date=&account_id=&page=&page_size=
//
// หน้ารายงานของ frontend เป็นหน้าเดียว — เลือกรายงานด้วย book และใช้ URL query เดียวกันเรียก API ตรงๆ
// response มี data.book บอกชนิดรายงาน; general-ledger มี meta สำหรับแบ่งหน้าบรรทัดรายการ
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		q, err := requests.ParseReport(r.URL.Query())
		if err != nil {
			return nil, err
		}
		book := string(q.Book)
		switch q.Book {
		case requests.BookTrialBalance:
			tb, err := h.uc.Reports.TrialBalance(r.Context(), dto.DateRange{Actor: a, Start: q.Start, End: q.End})
			out := responses.FromTrialBalance(tb)
			out.Book = book
			return one(out, err)
		case requests.BookBalanceSheet:
			bs, err := h.uc.Reports.BalanceSheet(r.Context(), dto.AsOf{Actor: a, Date: q.End})
			out := responses.FromBalanceSheet(bs)
			out.Book = book
			return one(out, err)
		case requests.BookIncomeStatement:
			is, err := h.uc.Reports.IncomeStatement(r.Context(), dto.DateRange{Actor: a, Start: q.Start, End: q.End})
			out := responses.FromIncomeStatement(is)
			out.Book = book
			return one(out, err)
		case requests.BookDBDBalanceSheet:
			s, err := h.uc.Reports.DBDBalanceSheet(r.Context(), dto.AsOf{Actor: a, Date: q.End})
			return one(responses.FromDBD(book, s), err)
		case requests.BookDBDIncomeStatement:
			s, err := h.uc.Reports.DBDIncomeStatement(r.Context(), dto.AsOf{Actor: a, Date: q.End})
			return one(responses.FromDBD(book, s), err)
		default: // requests.BookGeneralLedger (ParseReport ตรวจค่า book แล้ว)
			gl, err := h.uc.Reports.AccountLedger(r.Context(), dto.AccountLedgerInput{
				Actor: a, AccountID: q.AccountID, Start: q.Start, End: q.End, Page: q.Page,
			})
			if err != nil {
				return nil, err
			}
			out := responses.FromAccountLedger(gl)
			out.Data.Book = book
			return out, nil
		}
	})
}

// AccountingDashboard: GET /api/v1/companies/{companyID}/dashboards/accounting?start_date=&end_date=
func (h *Handler) AccountingDashboard(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		start, end, err := requests.ParseDateRange(r.URL.Query())
		if err != nil {
			return nil, err
		}
		d, err := h.uc.Reports.Dashboard(r.Context(), dto.DateRange{Actor: a, Start: start, End: end})
		return one(responses.FromAccountingDashboard(d), err)
	})
}
