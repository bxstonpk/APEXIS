package responses

import (
	"time"

	"apexis/backend/internal/modules/finance/application/dto"
)

type TaxFiling struct {
	ID           string     `json:"id"`
	Form         string     `json:"form"`
	Month        string     `json:"month"`
	FiledDate    string     `json:"filed_date"`
	Amount       string     `json:"amount"`
	Surcharge    string     `json:"surcharge"`
	Penalty      string     `json:"penalty"`
	CarryForward string     `json:"carry_forward"`
	Total        string     `json:"total"`
	Reference    string     `json:"reference"`
	Note         string     `json:"note"`
	PaymentID    *string    `json:"payment_id"`
	Status       string     `json:"status"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	CancelledAt  *time.Time `json:"cancelled_at"`
}

func FromTaxFiling(f dto.TaxFiling) TaxFiling {
	return TaxFiling{ID: f.ID, Form: string(f.Form), Month: f.Month.String(), FiledDate: date(f.FiledDate), Amount: str(f.Amount),
		Surcharge: str(f.Surcharge), Penalty: str(f.Penalty), CarryForward: str(f.CarryForward), Total: str(f.Total), Reference: f.Reference,
		Note: f.Note, PaymentID: f.PaymentID, Status: string(f.Status), CreatedBy: f.CreatedBy, CreatedAt: f.CreatedAt, CancelledAt: f.CancelledAt}
}

func optFiling(f *dto.TaxFiling) *TaxFiling {
	if f == nil {
		return nil
	}
	x := FromTaxFiling(*f)
	return &x
}

func FromTaxFilings(fs []dto.TaxFiling) []TaxFiling {
	out := make([]TaxFiling, len(fs))
	for i, f := range fs {
		out[i] = FromTaxFiling(f)
	}
	return out
}

// ---------- ภ.ง.ด.3 / 53 ----------

type WhtFormLine struct {
	Seq           int    `json:"seq"`
	PaymentID     string `json:"payment_id"`
	DocumentNo    string `json:"document_no"`
	CertificateNo string `json:"certificate_no"`
	Date          string `json:"date"`
	TaxID         string `json:"tax_id"`
	BranchNo      string `json:"branch_no"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	IncomeType    string `json:"income_type"`
	WhtTypeName   string `json:"wht_type_name"`
	Rate          string `json:"rate"`
	Base          string `json:"base"`
	Wht           string `json:"wht"`
	Condition     int    `json:"condition"` // 1 = หัก ณ ที่จ่าย
}

type WhtFormSummary struct {
	Payees int    `json:"payees"`
	Lines  int    `json:"lines"`
	Base   string `json:"base"`
	Wht    string `json:"wht"`
}

type WhtForm struct {
	Form    string         `json:"form"`
	Month   string         `json:"month"`
	Summary WhtFormSummary `json:"summary"`
	Lines   []WhtFormLine  `json:"lines"`
	Filing  *TaxFiling     `json:"filing"`
}

func FromWhtForm(f dto.WhtForm) WhtForm {
	out := WhtForm{Form: string(f.Form), Month: f.Month.String(), Lines: make([]WhtFormLine, len(f.Lines)), Filing: optFiling(f.Filing),
		Summary: WhtFormSummary{Payees: f.Summary.Payees, Lines: f.Summary.Lines, Base: str(f.Summary.Base), Wht: str(f.Summary.Wht)}}
	for i, l := range f.Lines {
		out.Lines[i] = WhtFormLine{Seq: l.Seq, PaymentID: l.PaymentID, DocumentNo: l.DocumentNo, CertificateNo: l.CertificateNo, Date: date(l.Date),
			TaxID: l.TaxID, BranchNo: l.BranchNo, Name: l.Name, Address: l.Address, IncomeType: l.IncomeType, WhtTypeName: l.WhtTypeName,
			Rate: str(l.Rate), Base: str(l.Base), Wht: str(l.Wht), Condition: 1}
	}
	return out
}

// ---------- ภ.พ.30 ----------

type VatAmounts struct {
	Base      string `json:"base"`
	Tax       string `json:"tax"`
	Documents int    `json:"documents"`
}

func vatAmounts(a dto.VatAmounts) VatAmounts {
	return VatAmounts{Base: str(a.Base), Tax: str(a.Tax), Documents: a.Documents}
}

type Pp30 struct {
	Month     string     `json:"month"`
	Sales     VatAmounts `json:"sales"`
	ZeroRated string     `json:"zero_rated"`
	Exempt    string     `json:"exempt"`
	Purchases VatAmounts `json:"purchases"`
	CarryIn   string     `json:"carry_in"`
	Net       string     `json:"net"`
	Payable   string     `json:"payable"`
	Overpaid  string     `json:"overpaid"`
	PrevFiled bool       `json:"prev_filed"`
	Filing    *TaxFiling `json:"filing"`
}

func FromPp30(p dto.Pp30) Pp30 {
	return Pp30{Month: p.Month.String(), Sales: vatAmounts(p.Sales), ZeroRated: str(p.ZeroRated), Exempt: str(p.Exempt),
		Purchases: vatAmounts(p.Purchases), CarryIn: str(p.CarryIn), Net: str(p.Net), Payable: str(p.Payable), Overpaid: str(p.Overpaid),
		PrevFiled: p.PrevFiled, Filing: optFiling(p.Filing)}
}

// ---------- รายงานภาษีขาย / ซื้อ ----------

type VatReportRow struct {
	Seq        int    `json:"seq"`
	Date       string `json:"date"`
	DocumentNo string `json:"document_no"`
	PartyName  string `json:"party_name"`
	TaxID      string `json:"tax_id"`
	BranchNo   string `json:"branch_no"`
	Base       string `json:"base"`
	Tax        string `json:"tax"`
}

type VatReport struct {
	Kind  string         `json:"kind"`
	Month string         `json:"month"`
	Rows  []VatReportRow `json:"rows"`
	Base  string         `json:"base"`
	Tax   string         `json:"tax"`
}

func FromVatReport(r dto.VatReport) VatReport {
	out := VatReport{Kind: r.Kind, Month: r.Month.String(), Rows: make([]VatReportRow, len(r.Rows)), Base: str(r.Base), Tax: str(r.Tax)}
	for i, x := range r.Rows {
		out.Rows[i] = VatReportRow{Seq: x.Seq, Date: date(x.Date), DocumentNo: x.DocumentNo, PartyName: x.PartyName, TaxID: x.TaxID,
			BranchNo: x.BranchNo, Base: str(x.Base), Tax: str(x.Tax)}
	}
	return out
}

// ---------- ภาพรวมรายเดือน ----------

type TaxOverviewCell struct {
	Amount   string     `json:"amount"`
	Payees   int        `json:"payees"`
	Computed bool       `json:"computed"`
	Filing   *TaxFiling `json:"filing"`
}

type TaxOverviewMonth struct {
	Month string          `json:"month"`
	Pnd3  TaxOverviewCell `json:"pnd3"`
	Pnd53 TaxOverviewCell `json:"pnd53"`
	Pp30  TaxOverviewCell `json:"pp30"`
}

type TaxOverview struct {
	Year        int                `json:"year"`
	CanSeeFiles bool               `json:"can_see_filings"`
	Months      []TaxOverviewMonth `json:"months"`
}

func FromTaxOverview(o dto.TaxOverview) TaxOverview {
	cell := func(c dto.TaxOverviewCell) TaxOverviewCell {
		return TaxOverviewCell{Amount: str(c.Amount), Payees: c.Payees, Computed: c.Computed, Filing: optFiling(c.Filing)}
	}
	out := TaxOverview{Year: o.Year, CanSeeFiles: o.CanSeeFiles, Months: make([]TaxOverviewMonth, len(o.Months))}
	for i, m := range o.Months {
		out.Months[i] = TaxOverviewMonth{Month: m.Month.String(), Pnd3: cell(m.Pnd3), Pnd53: cell(m.Pnd53), Pp30: cell(m.Pp30)}
	}
	return out
}
