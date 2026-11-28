package responses

import (
	"time"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/platform/httpx"
	"apexis/backend/internal/platform/listing"
)

func str(v interface{ String() string }) string { return v.String() }

func date(t time.Time) string { return t.Format("2006-01-02") }

type Ref struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func ref(r dto.Ref) Ref { return Ref(r) }

func optRef(r *dto.Ref) *Ref {
	if r == nil {
		return nil
	}
	x := ref(*r)
	return &x
}

type Party struct {
	ID       *string `json:"id"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	TaxID    string  `json:"tax_id"`
	BranchNo string  `json:"branch_no"`
	Address  string  `json:"address"`
	TaxType  string  `json:"tax_type"`
}

func party(p dto.Party) Party { return Party(p) }

type Method struct {
	CashAccount Ref    `json:"cash_account"`
	GLAccountID string `json:"gl_account_id"`
	Amount      string `json:"amount"`
	Reference   string `json:"reference"`
}

func methods(ms []dto.Method) []Method {
	out := make([]Method, len(ms))
	for i, m := range ms {
		out[i] = Method{CashAccount: ref(m.CashAccount), GLAccountID: m.GLAccountID, Amount: str(m.Amount), Reference: m.Reference}
	}
	return out
}

// DocumentSummary: net = รับจริง (ใบรับ) / จ่ายจริง (ใบจ่าย)
type DocumentSummary struct {
	ID          string     `json:"id"`
	DocumentNo  string     `json:"document_no"`
	Date        string     `json:"date"`
	Kind        string     `json:"kind"`
	Party       Party      `json:"party"`
	Amount      string     `json:"amount"`
	Wht         string     `json:"wht"`
	Net         string     `json:"net"`
	Status      string     `json:"status"`
	EntryID     *string    `json:"entry_id"`
	EntryNo     string     `json:"entry_no"`
	CancelledAt *time.Time `json:"cancelled_at"`
	Description string     `json:"description"`
}

func FromSummary(d dto.DocumentSummary) DocumentSummary {
	return DocumentSummary{
		ID: d.ID, DocumentNo: d.DocumentNo, Date: date(d.Date), Kind: d.Kind, Party: party(d.Party),
		Amount: str(d.Amount), Wht: str(d.Wht), Net: str(d.Net), Status: d.Status, EntryID: d.EntryID, EntryNo: d.EntryNo,
		CancelledAt: d.CancelledAt, Description: d.Description,
	}
}

type ReceiptLine struct {
	Invoice     *Ref    `json:"invoice"`      // code = เลขที่ใบแจ้งหนี้
	InvoiceKind string  `json:"invoice_kind"` // invoice | service_invoice | deposit ("" = รับเงินอื่น)
	AccountID   *string `json:"account_id"`
	Description string  `json:"description"`
	Amount      string  `json:"amount"`
	Wht         string  `json:"wht"`
	VatBase     string  `json:"vat_base"` // ภาษีขายที่ถึงกำหนดด้วยใบรับเงินนี้ (ขายบริการ)
	Vat         string  `json:"vat"`
}

// VatLine: ภาษีขายของใบแจ้งหนี้บริการที่ถึงกำหนดด้วยใบรับเงินนี้ (ม.78/1)
type VatLine struct {
	Invoice InvoiceRef `json:"invoice"`
	Base    string     `json:"base"`
	Vat     string     `json:"vat"`
}

type InvoiceRef struct {
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
}

// Receipt: tax_invoice = มีภาษีขายถึงกำหนด (พิมพ์เป็น "ใบเสร็จรับเงิน / ใบกำกับภาษี")
type Receipt struct {
	DocumentSummary
	CreatedBy  string        `json:"created_by"`
	Lines      []ReceiptLine `json:"lines"`
	Methods    []Method      `json:"methods"`
	TaxInvoice bool          `json:"tax_invoice"`
	VatLines   []VatLine     `json:"vat_lines"`
}

func FromReceipt(r dto.Receipt) Receipt {
	out := Receipt{DocumentSummary: FromSummary(r.DocumentSummary), CreatedBy: r.CreatedBy, Lines: make([]ReceiptLine, len(r.Lines)),
		Methods: methods(r.Methods), VatLines: []VatLine{}}
	for i, l := range r.Lines {
		out.Lines[i] = ReceiptLine{Invoice: optRef(l.Invoice), InvoiceKind: l.InvoiceKind, AccountID: l.AccountID, Description: l.Description,
			Amount: str(l.Amount), Wht: str(l.Wht), VatBase: str(l.VatBase), Vat: str(l.Vat)}
		if l.Vat.IsPositive() && l.Invoice != nil {
			out.TaxInvoice = true
			out.VatLines = append(out.VatLines, VatLine{Invoice: InvoiceRef{ID: l.Invoice.ID, DocumentNo: l.Invoice.Code}, Base: str(l.VatBase), Vat: str(l.Vat)})
		}
	}
	return out
}

type PaymentLine struct {
	Invoice     *Ref        `json:"invoice"` // code = เลขที่ใบแจ้งหนี้, name = เลขที่ของผู้ขาย
	AccountID   *string     `json:"account_id"`
	Description string      `json:"description"`
	Amount      string      `json:"amount"`
	WhtType     *WhtTypeRef `json:"wht_type"`
	WhtRate     string      `json:"wht_rate"`
	WhtBase     string      `json:"wht_base"`
	Wht         string      `json:"wht"`
}

// WhtTypeRef: ประเภทภาษีหัก ณ ที่จ่ายในบรรทัดใบจ่าย / รายงาน — income_type ใช้เลือกช่องใน 50 ทวิ
type WhtTypeRef struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	IncomeType string `json:"income_type"`
}

func optWhtType(r *dto.WhtTypeRef) *WhtTypeRef {
	if r == nil {
		return nil
	}
	return &WhtTypeRef{ID: r.ID, Code: r.Code, Name: r.Name, IncomeType: r.IncomeType}
}

// PaymentSummary: รายการใบจ่าย + เลขที่หนังสือรับรอง 50 ทวิ (null = ไม่มีภาษีหัก)
type PaymentSummary struct {
	DocumentSummary
	WhtCertificateNo *string `json:"wht_certificate_no"`
}

func FromPaymentSummary(d dto.DocumentSummary) PaymentSummary {
	return PaymentSummary{DocumentSummary: FromSummary(d), WhtCertificateNo: d.WhtCertificateNo}
}

type Payment struct {
	PaymentSummary
	CreatedBy string        `json:"created_by"`
	Lines     []PaymentLine `json:"lines"`
	Methods   []Method      `json:"methods"`
}

func FromPayment(p dto.Payment) Payment {
	out := Payment{PaymentSummary: FromPaymentSummary(p.DocumentSummary), CreatedBy: p.CreatedBy, Lines: make([]PaymentLine, len(p.Lines)), Methods: methods(p.Methods)}
	for i, l := range p.Lines {
		out.Lines[i] = PaymentLine{Invoice: optRef(l.Invoice), AccountID: l.AccountID, Description: l.Description, Amount: str(l.Amount),
			WhtType: optWhtType(l.WhtType), WhtRate: str(l.WhtRate), WhtBase: str(l.WhtBase), Wht: str(l.Wht)}
	}
	return out
}

type Transfer struct {
	ID          string     `json:"id"`
	DocumentNo  string     `json:"document_no"`
	Date        string     `json:"date"`
	From        Ref        `json:"from"`
	To          Ref        `json:"to"`
	Amount      string     `json:"amount"`
	Reference   string     `json:"reference"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	EntryID     *string    `json:"entry_id"`
	EntryNo     string     `json:"entry_no"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

func FromTransfer(t dto.Transfer) Transfer {
	return Transfer{ID: t.ID, DocumentNo: t.DocumentNo, Date: date(t.Date), From: ref(t.From), To: ref(t.To), Amount: str(t.Amount),
		Reference: t.Reference, Description: t.Description, Status: t.Status, EntryID: t.EntryID, EntryNo: t.EntryNo, CancelledAt: t.CancelledAt}
}

type WhtType struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Rate       string `json:"rate"`
	IncomeType string `json:"income_type"`
	Status     string `json:"status"`
}

func FromWhtType(w dto.WhtType) WhtType {
	return WhtType{ID: w.ID, Code: w.Code, Name: w.Name, Rate: str(w.Rate), IncomeType: w.IncomeType, Status: w.Status}
}

type Settings struct {
	ReceiptJournalID     *string `json:"receipt_journal_id"`
	PaymentJournalID     *string `json:"payment_journal_id"`
	TransferJournalID    *string `json:"transfer_journal_id"`
	WhtReceivedAccountID *string `json:"wht_received_account_id"`
	WhtPnd3AccountID     *string `json:"wht_pnd3_account_id"`
	WhtPnd53AccountID    *string `json:"wht_pnd53_account_id"`
}

func FromSettings(s dto.Settings) Settings { return Settings(s) }

type OpenInvoice struct {
	ID          string `json:"id"`
	DocumentNo  string `json:"document_no"`
	PartnerRef  string `json:"partner_ref"` // เลขที่ใบแจ้งหนี้ของผู้ขาย
	Date        string `json:"date"`
	DueDate     string `json:"due_date"`
	Net         string `json:"net"` // ยอดก่อน VAT
	Gross       string `json:"gross"`
	Outstanding string `json:"outstanding"`
	Kind        string `json:"kind"`        // ขาย: invoice | service_invoice | deposit; จัดซื้อ: goods | service
	VatUndue    string `json:"vat_undue"`   // ขายบริการ: ภาษีขายยังไม่ถึงกำหนด
	TaxInvoice  bool   `json:"tax_invoice"` // ขายบริการที่ออกใบกำกับภาษีแล้ว
}

// PettyCashRequest คือคำขอเงินสดย่อยที่รอการเงินจ่าย (kind: establish | increase | replenish)
type PettyCashRequest struct {
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
	FundCode   string `json:"fund_code"`
	FundName   string `json:"fund_name"`
	Kind       string `json:"kind"`
	Date       string `json:"date"`
	Amount     string `json:"amount"`
}

func FromPettyCashRequests(rows []dto.PettyCashRequest) []PettyCashRequest {
	out := make([]PettyCashRequest, len(rows))
	for i, r := range rows {
		out[i] = PettyCashRequest{ID: r.ID, DocumentNo: r.DocumentNo, FundCode: r.FundCode, FundName: r.FundName, Kind: r.Kind,
			Date: date(r.Date), Amount: str(r.Amount)}
	}
	return out
}

func FromOpenInvoices(rows []dto.OpenInvoice) []OpenInvoice {
	out := make([]OpenInvoice, len(rows))
	for i, r := range rows {
		out[i] = OpenInvoice{ID: r.ID, DocumentNo: r.DocumentNo, PartnerRef: r.PartnerRef, Date: date(r.Date), DueDate: date(r.DueDate),
			Net: str(r.Net), Gross: str(r.Gross), Outstanding: str(r.Outstanding), Kind: r.Kind, VatUndue: str(r.VatUndue), TaxInvoice: r.TaxInvoice}
	}
	return out
}

// ---------- รายงาน ----------

type WithholdingRow struct {
	Date       string      `json:"date"`
	DocumentID string      `json:"document_id"`
	DocumentNo string      `json:"document_no"`
	Party      Party       `json:"party"`
	TaxForm    string      `json:"tax_form"` // pnd3 / pnd53 (ภาษีที่จ่าย)
	WhtType    *WhtTypeRef `json:"wht_type"`
	// CertificateNo: เลขที่หนังสือรับรอง 50 ทวิ (ภาษีถูกหัก = "")
	CertificateNo string `json:"certificate_no"`
	Rate          string `json:"rate"`
	Base          string `json:"base"`
	Wht           string `json:"wht"`
}

type Withholding struct {
	Book      string           `json:"book"`
	StartDate string           `json:"start_date"`
	EndDate   string           `json:"end_date"`
	Rows      []WithholdingRow `json:"rows"`
	TotalBase string           `json:"total_base"`
	TotalWht  string           `json:"total_wht"`
}

// WithholdingBody: {"data": {...}, "meta": {...}} — meta คือการแบ่งหน้าของ rows
type WithholdingBody struct {
	Data Withholding `json:"data"`
	Meta httpx.Meta  `json:"meta"`
}

func FromWithholding(book string, w dto.Withholding) WithholdingBody {
	rows := make([]WithholdingRow, len(w.Rows.Items))
	for i, r := range w.Rows.Items {
		rows[i] = WithholdingRow{Date: date(r.Date), DocumentID: r.DocumentID, DocumentNo: r.DocumentNo, Party: party(r.Party), TaxForm: r.TaxForm,
			WhtType: optWhtType(r.WhtType), CertificateNo: r.CertificateNo, Rate: str(r.Rate), Base: str(r.Base), Wht: str(r.Wht)}
	}
	return WithholdingBody{
		Data: Withholding{Book: book, StartDate: date(w.Start), EndDate: date(w.End), Rows: rows, TotalBase: str(w.Totals.Base), TotalWht: str(w.Totals.Wht)},
		Meta: meta(w.Rows),
	}
}

func meta[T any](r listing.Result[T]) httpx.Meta {
	return httpx.Meta{Page: r.Page.Number, PageSize: r.Page.Size, Total: r.Total}
}

// ---------- dashboard ----------

type DashboardCashAccount struct {
	Account Ref    `json:"account"`
	Type    string `json:"type"`
	Balance string `json:"balance"`
}

type MonthlyFlow struct {
	Month    string `json:"month"` // YYYY-MM
	Received string `json:"received"`
	Paid     string `json:"paid"`
}

type DashboardWht struct {
	Deducted string `json:"deducted"` // หักจากใบจ่าย (นำส่ง ภ.ง.ด.3 / 53)
	Suffered string `json:"suffered"` // ถูกหักจากใบรับ
}

type RecentDocument struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	DocumentNo string `json:"document_no"`
	Date       string `json:"date"`
	Party      string `json:"party"`
	Amount     string `json:"amount"`
}

type FinanceDashboard struct {
	StartDate     string                 `json:"start_date"`
	EndDate       string                 `json:"end_date"`
	CashAccounts  []DashboardCashAccount `json:"cash_accounts"`
	CashTotal     string                 `json:"cash_total"`
	Received      string                 `json:"received"`
	Paid          string                 `json:"paid"`
	NetFlow       string                 `json:"net_flow"`
	TransferCount int64                  `json:"transfer_count"`
	Monthly       []MonthlyFlow          `json:"monthly"`
	Wht           DashboardWht           `json:"wht"`
	Recent        []RecentDocument       `json:"recent"`
}

func FromFinanceDashboard(d dto.FinanceDashboard) FinanceDashboard {
	f := d.Flows
	out := FinanceDashboard{
		StartDate: date(d.Start), EndDate: date(d.End), CashAccounts: make([]DashboardCashAccount, len(d.CashAccounts)),
		CashTotal: str(d.CashTotal), Received: str(f.Received), Paid: str(f.Paid), NetFlow: str(f.Received.Sub(f.Paid)),
		TransferCount: f.TransferCount, Monthly: make([]MonthlyFlow, len(d.Monthly)),
		Wht:    DashboardWht{Deducted: str(f.WhtDeducted), Suffered: str(f.WhtSuffered)},
		Recent: make([]RecentDocument, len(d.Recent)),
	}
	for i, c := range d.CashAccounts {
		out.CashAccounts[i] = DashboardCashAccount{Account: ref(c.Account), Type: c.Type, Balance: str(c.Balance)}
	}
	for i, m := range d.Monthly {
		out.Monthly[i] = MonthlyFlow{Month: m.Month.Format("2006-01"), Received: str(m.Received), Paid: str(m.Paid)}
	}
	for i, x := range d.Recent {
		out.Recent[i] = RecentDocument{Kind: x.Kind, ID: x.ID, DocumentNo: x.DocumentNo, Date: date(x.Date), Party: x.Party, Amount: str(x.Amount)}
	}
	return out
}

// ---------- สรุป ภ.พ.30 ----------

type VatTotals struct {
	Base      string `json:"base"`
	Tax       string `json:"tax"`
	Documents int    `json:"documents"`
}

// VatSummary: book=vat-summary — net_tax = output.tax - input.tax, payable / refundable แยกตามเครื่องหมาย
type VatSummary struct {
	Book       string    `json:"book"`
	StartDate  string    `json:"start_date"`
	EndDate    string    `json:"end_date"`
	Output     VatTotals `json:"output"`
	Input      VatTotals `json:"input"`
	NetTax     string    `json:"net_tax"`
	Payable    string    `json:"payable"`
	Refundable string    `json:"refundable"`
}

func FromVatSummary(book string, s dto.VatSummary) VatSummary {
	t := func(x dto.VatTotals) VatTotals {
		return VatTotals{Base: str(x.Base), Tax: str(x.Tax), Documents: x.Documents}
	}
	return VatSummary{Book: book, StartDate: date(s.Start), EndDate: date(s.End), Output: t(s.Output), Input: t(s.Input),
		NetTax: str(s.NetTax), Payable: str(s.Payable), Refundable: str(s.Refundable)}
}
