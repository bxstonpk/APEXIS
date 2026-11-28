package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"apexis/backend/internal/modules/finance/application/dto"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/modules/finance/presentation/http/requests"
	"apexis/backend/internal/modules/finance/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// registerTaxRoutes: แบบยื่นภาษีรายเดือน + ทะเบียนการยื่น
func registerTaxRoutes(handle func(string, http.HandlerFunc), base string, h *Handler) {
	uc := h.uc.Tax
	handle("GET "+base+"/tax-forms/overview", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			year, err := requests.TaxYear(r.URL.Query())
			if err != nil {
				return nil, err
			}
			o, err := uc.Overview(r.Context(), a, year)
			return one(responses.FromTaxOverview(o), err)
		})
	})
	handle("GET "+base+"/tax-forms/withholding", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			form, m, err := requests.WhtForm(r.URL.Query())
			if err != nil {
				return nil, err
			}
			f, err := uc.WhtForm(r.Context(), a, form, m)
			return one(responses.FromWhtForm(f), err)
		})
	})
	handle("GET "+base+"/tax-forms/withholding/export", h.exportWhtForm)
	handle("GET "+base+"/tax-forms/pp30", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			m, err := requests.TaxMonth(r.URL.Query())
			if err != nil {
				return nil, err
			}
			p, err := uc.Pp30(r.Context(), a, m)
			return one(responses.FromPp30(p), err)
		})
	})
	handle("GET "+base+"/tax-forms/vat-report", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			m, err := requests.TaxMonth(r.URL.Query())
			if err != nil {
				return nil, err
			}
			rep, err := uc.VatReport(r.Context(), a, r.URL.Query().Get("kind"), m)
			return one(responses.FromVatReport(rep), err)
		})
	})

	handle("GET "+base+"/tax-filings", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			f, err := requests.ParseTaxFilingFilter(r.URL.Query())
			if err != nil {
				return nil, err
			}
			rows, err := uc.ListFilings(r.Context(), a, f)
			return one(responses.FromTaxFilings(rows), err)
		})
	})
	handle("POST "+base+"/tax-filings", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
			var req requests.TaxFiling
			if err := httpx.DecodeJSON(w, r, &req); err != nil {
				return nil, err
			}
			d, err := req.ToData()
			if err != nil {
				return nil, err
			}
			f, err := uc.CreateFiling(r.Context(), a, d)
			return one(responses.FromTaxFiling(f), err)
		})
	})
	handle("PATCH "+base+"/tax-filings/{taxFilingID}", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
			id, err := vo.ParseID[vo.TaxFilingID](r.PathValue("taxFilingID"))
			if err != nil {
				return nil, err
			}
			var req requests.CancelFiling
			if err := httpx.DecodeJSON(w, r, &req); err != nil {
				return nil, err
			}
			if err := req.Validate(); err != nil {
				return nil, err
			}
			f, err := uc.CancelFiling(r.Context(), dto.TaxFilingByID{Actor: a, ID: id})
			return one(responses.FromTaxFiling(f), err)
		})
	})
}

// exportWhtForm: GET /tax-forms/withholding/export?form=&month= — ไฟล์ข้อความสำหรับนำเข้าโปรแกรมยื่นแบบ (ใบแนบ ภ.ง.ด.)
// UTF-8 คั่นด้วย | บรรทัดละ 1 บรรทัดใบจ่าย (ไม่มีหัวคอลัมน์) — ลำดับคอลัมน์ดู docs/04-api.md
func (h *Handler) exportWhtForm(w http.ResponseWriter, r *http.Request) {
	var (
		f    dto.WhtForm
		form vo.TaxForm
		m    vo.TaxMonth
	)
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := vo.ParseID[vo.CompanyID](r.PathValue("companyID"))
	if err == nil {
		form, m, err = requests.WhtForm(r.URL.Query())
	}
	if err == nil {
		f, err = h.uc.Tax.WhtForm(r.Context(), dto.Actor{UserID: vo.UserID(userID), CompanyID: companyID}, form, m)
	}
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_%s.txt"`, form, m.String()))
	_, _ = w.Write([]byte(WhtExportText(f)))
}

// WhtExportText: 1 บรรทัด = ลำดับที่|เลขประจำตัวผู้เสียภาษี|สาขา|ชื่อ|ที่อยู่|วันที่จ่าย (วว/ดด/ปปปป พ.ศ.)|ประเภทเงินได้|อัตรา|
// จำนวนเงินที่จ่าย|ภาษีที่หัก|เงื่อนไข — จำนวนเงินทศนิยม 2 ตำแหน่ง, ขึ้นบรรทัดด้วย CRLF
func WhtExportText(f dto.WhtForm) string {
	var b strings.Builder
	clean := func(s string) string {
		return strings.NewReplacer("|", " ", "\r", " ", "\n", " ").Replace(strings.TrimSpace(s))
	}
	for _, l := range f.Lines {
		fmt.Fprintf(&b, "%d|%s|%s|%s|%s|%02d/%02d/%04d|%s|%s|%s|%s|1\r\n", l.Seq, clean(l.TaxID), clean(l.BranchNo), clean(l.Name), clean(l.Address),
			l.Date.Day(), int(l.Date.Month()), l.Date.Year()+543, l.IncomeType, l.Rate.StringFixed(2), l.Base.StringFixed(2), l.Wht.StringFixed(2))
	}
	return b.String()
}
