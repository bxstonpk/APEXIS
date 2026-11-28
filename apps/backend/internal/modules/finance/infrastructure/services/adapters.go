// Package services คือ adapter ไปยัง identity / organization / ledger / sales / purchasing
// เรียกผ่าน public API (uuid / primitive / platform/integration) เท่านั้น
package services

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/ports"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	journal "apexis/backend/internal/modules/finance/domain/services"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

// ---------- identity ----------

type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

var _ ports.Authorizer = (*Authorizer)(nil)

type Authorizer struct{ identity IdentityAccess }

func NewAuthorizer(identity IdentityAccess) *Authorizer { return &Authorizer{identity: identity} }

func (a *Authorizer) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error {
	ok, err := a.identity.Authorize(ctx, uuid.UUID(userID), uuid.UUID(companyID), perm.String())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// ---------- organization ----------

type OrganizationAccess interface {
	CompanySettings(ctx context.Context, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	RegisterDocumentTypes(types ...integration.DocumentType)
	DocumentFormat(ctx context.Context, companyID uuid.UUID, docType string) (numbering.Format, error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanySettings(ctx context.Context, companyID vo.CompanyID) (ports.CompanySettings, error) {
	active, amount, _, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return ports.CompanySettings{Active: active, AmountDecimals: amount}, err
}

func (o *Organization) PeriodAt(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	id, year, _, open, err := o.org.FiscalPeriodAt(ctx, uuid.UUID(companyID), date)
	if err != nil {
		return ports.Period{}, err
	}
	if id == uuid.Nil {
		return ports.Period{}, domain.ErrNoFiscalPeriod
	}
	return ports.Period{ID: vo.FiscalPeriodID(id), FiscalYear: year, Open: open}, nil
}

func (o *Organization) DocumentFormat(ctx context.Context, companyID vo.CompanyID, docType vo.DocType) (numbering.Format, error) {
	return o.org.DocumentFormat(ctx, uuid.UUID(companyID), string(docType))
}

// ---------- ledger ----------

type LedgerAccess interface {
	PostEntry(ctx context.Context, companyID, userID, journalID uuid.UUID, date time.Time, description, reference string, lines []integration.JournalLine) (integration.PostedEntry, error)
	ReverseEntry(ctx context.Context, companyID, userID, entryID uuid.UUID, date time.Time, description string) (integration.PostedEntry, error)
	InvalidPostingAccounts(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID) ([]uuid.UUID, error)
	JournalUsable(ctx context.Context, companyID, journalID uuid.UUID) (bool, error)
	AccountIDsByCode(ctx context.Context, companyID uuid.UUID, codes []string) (map[string]uuid.UUID, error)
	JournalIDsByCode(ctx context.Context, companyID uuid.UUID, codes []string) (map[string]uuid.UUID, error)
	AccountBalances(ctx context.Context, companyID uuid.UUID, accountIDs []uuid.UUID, asOf time.Time) (map[uuid.UUID]decimal.Decimal, error)
}

var _ ports.Ledger = (*Ledger)(nil)

type Ledger struct{ ledger LedgerAccess }

func NewLedger(l LedgerAccess) *Ledger { return &Ledger{ledger: l} }

func (l *Ledger) InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID) ([]vo.GLAccountID, error) {
	in := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		in[i] = uuid.UUID(id)
	}
	bad, err := l.ledger.InvalidPostingAccounts(ctx, uuid.UUID(companyID), in)
	out := make([]vo.GLAccountID, len(bad))
	for i, id := range bad {
		out[i] = vo.GLAccountID(id)
	}
	return out, err
}

func (l *Ledger) Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.GLAccountID, asOf time.Time) (map[vo.GLAccountID]vo.Money, error) {
	raw := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		raw[i] = uuid.UUID(id)
	}
	b, err := l.ledger.AccountBalances(ctx, uuid.UUID(companyID), raw, asOf)
	out := make(map[vo.GLAccountID]vo.Money, len(b))
	for id, v := range b {
		out[vo.GLAccountID(id)] = v
	}
	return out, err
}

func (l *Ledger) JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error) {
	return l.ledger.JournalUsable(ctx, uuid.UUID(companyID), uuid.UUID(id))
}

func (l *Ledger) AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.GLAccountID, error) {
	found, err := l.ledger.AccountIDsByCode(ctx, uuid.UUID(companyID), codes)
	out := make(map[string]vo.GLAccountID, len(found))
	for code, id := range found {
		out[code] = vo.GLAccountID(id)
	}
	return out, err
}

func (l *Ledger) JournalsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error) {
	found, err := l.ledger.JournalIDsByCode(ctx, uuid.UUID(companyID), codes)
	out := make(map[string]vo.JournalID, len(found))
	for code, id := range found {
		out[code] = vo.JournalID(id)
	}
	return out, err
}

func (l *Ledger) Post(ctx context.Context, e ports.LedgerEntry) (vo.EntryID, string, error) {
	lines := make([]integration.JournalLine, len(e.Lines))
	for i, x := range e.Lines {
		lines[i] = integration.JournalLine{AccountID: uuid.UUID(x.AccountID), Debit: x.Debit, Credit: x.Credit}
	}
	p, err := l.ledger.PostEntry(ctx, uuid.UUID(e.CompanyID), uuid.UUID(e.UserID), uuid.UUID(e.JournalID), e.Date, e.Description, e.Reference, lines)
	return vo.EntryID(p.EntryID), p.EntryNo, err
}

func (l *Ledger) Reverse(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.EntryID, date time.Time, description string) error {
	_, err := l.ledger.ReverseEntry(ctx, uuid.UUID(companyID), uuid.UUID(userID), uuid.UUID(id), date, description)
	return err
}

// ---------- sales (ลูกหนี้) / purchasing (เจ้าหนี้) ----------

// ReceivablesAccess คือ public API ของ sales ที่ finance ใช้ (*sales.Module)
type ReceivablesAccess interface {
	SettleReceivables(ctx context.Context, companyID, customerID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error)
	UnsettleReceivables(ctx context.Context, companyID, documentID uuid.UUID) error
	OpenReceivables(ctx context.Context, companyID, customerID uuid.UUID, billingNoteID *uuid.UUID) ([]integration.OpenInvoice, error)
	OutputVat(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error)
	OutputVatRows(ctx context.Context, companyID uuid.UUID, start, end time.Time) ([]integration.VatRow, error)
}

// PayablesAccess คือ public API ของ purchasing ที่ finance ใช้ (*purchasing.Module)
type PayablesAccess interface {
	SettlePayables(ctx context.Context, companyID, supplierID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error)
	UnsettlePayables(ctx context.Context, companyID, documentID uuid.UUID) error
	OpenPayables(ctx context.Context, companyID, supplierID uuid.UUID) ([]integration.OpenInvoice, error)
	InputVat(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error)
	InputVatRows(ctx context.Context, companyID uuid.UUID, start, end time.Time) ([]integration.VatRow, error)
}

type settleFunc func(ctx context.Context, companyID, partyID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error)

// Settlements แปลง public API ของ sales / purchasing เป็น ports.Settlements
type Settlements struct {
	settle   settleFunc
	unsettle func(ctx context.Context, companyID, documentID uuid.UUID) error
	open     func(ctx context.Context, companyID, partyID uuid.UUID, billingNoteID *uuid.UUID) ([]integration.OpenInvoice, error)
}

var _ ports.Settlements = (*Settlements)(nil)

func NewReceivables(s ReceivablesAccess) *Settlements {
	return &Settlements{settle: s.SettleReceivables, unsettle: s.UnsettleReceivables, open: s.OpenReceivables}
}

func NewPayables(p PayablesAccess) *Settlements {
	open := func(ctx context.Context, companyID, partyID uuid.UUID, _ *uuid.UUID) ([]integration.OpenInvoice, error) {
		return p.OpenPayables(ctx, companyID, partyID)
	}
	return &Settlements{settle: p.SettlePayables, unsettle: p.UnsettlePayables, open: open}
}

func (s *Settlements) Settle(ctx context.Context, companyID vo.CompanyID, partyID uuid.UUID, doc ports.SettlementDoc, lines []ports.Settlement) (ports.SettlementResult, error) {
	in := make([]integration.Settlement, len(lines))
	for i, l := range lines {
		in[i] = integration.Settlement{InvoiceID: uuid.UUID(l.InvoiceID), Amount: l.Amount}
	}
	res, err := s.settle(ctx, uuid.UUID(companyID), partyID, integration.SettlementDoc{DocumentID: doc.ID, DocumentNo: doc.No, Date: doc.Date}, in)
	if err != nil {
		return ports.SettlementResult{}, err
	}
	p := res.Party
	out := ports.SettlementResult{
		Party: ports.Counterparty{ID: p.ID, Code: p.Code, Name: p.Name, TaxID: p.TaxID, BranchNo: p.BranchNo, Address: p.Address,
			TaxType: vo.TaxType(p.TaxType)},
		ControlAccountID: vo.GLAccountID(res.ControlAccountID), Invoices: make([]ports.SettledInvoice, len(res.Invoices)),
	}
	for i, x := range res.Invoices {
		out.Invoices[i] = ports.SettledInvoice{ID: vo.InvoiceID(x.InvoiceID), DocumentNo: x.DocumentNo, PartnerRef: x.PartnerRef, Kind: x.Kind,
			Net: x.Net, Gross: x.Gross, VatBase: x.VatBase, Vat: x.Vat}
	}
	for _, l := range res.ExtraJournal {
		out.ExtraJournal = append(out.ExtraJournal, journal.JournalLine{AccountID: vo.GLAccountID(l.AccountID), Debit: l.Debit, Credit: l.Credit})
	}
	return out, nil
}

func (s *Settlements) Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return s.unsettle(ctx, uuid.UUID(companyID), documentID)
}

func (s *Settlements) Open(ctx context.Context, companyID vo.CompanyID, partyID uuid.UUID, billingNoteID *uuid.UUID) ([]ports.OpenInvoice, error) {
	rows, err := s.open(ctx, uuid.UUID(companyID), partyID, billingNoteID)
	out := make([]ports.OpenInvoice, len(rows))
	for i, r := range rows {
		out[i] = ports.OpenInvoice{ID: vo.InvoiceID(r.ID), DocumentNo: r.DocumentNo, PartnerRef: r.PartnerRef, Date: r.Date, DueDate: r.DueDate,
			Net: r.Net, Gross: r.Gross, Outstanding: r.Outstanding, Kind: r.Kind, VatUndue: r.VatUndue, TaxInvoice: r.TaxInvoice}
	}
	return out, err
}

// ---------- ภาษีมูลค่าเพิ่ม (ภ.พ.30) ----------

type (
	vatFunc     func(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error)
	vatRowsFunc func(ctx context.Context, companyID uuid.UUID, start, end time.Time) ([]integration.VatRow, error)
)

// VatSource คือแหล่งภาษีซื้อเพิ่มเติม (ยอดรวม + แถวรายงาน) เช่น ใบสำคัญจ่ายเงินสดย่อยที่มีใบกำกับภาษี
type VatSource struct {
	Totals vatFunc
	Rows   vatRowsFunc
}

// Vat แปลงยอดภาษีขายของ sales / ภาษีซื้อของ purchasing (+ เงินสดย่อย) เป็น ports.Vat
type Vat struct {
	output VatSource
	inputs []VatSource
}

var _ ports.Vat = (*Vat)(nil)

// NewVat: ภาษีซื้อ = จัดซื้อ + แหล่งอื่น (เช่น เงินสดย่อย)
func NewVat(r ReceivablesAccess, p PayablesAccess, more ...VatSource) *Vat {
	return &Vat{output: VatSource{Totals: r.OutputVat, Rows: r.OutputVatRows},
		inputs: append([]VatSource{{Totals: p.InputVat, Rows: p.InputVatRows}}, more...)}
}

func (v *Vat) OutputVat(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (ports.VatTotals, error) {
	return vatTotals(v.output.Totals(ctx, uuid.UUID(companyID), start, end))
}

func (v *Vat) InputVat(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (ports.VatTotals, error) {
	var sum ports.VatTotals
	for _, in := range v.inputs {
		t, err := vatTotals(in.Totals(ctx, uuid.UUID(companyID), start, end))
		if err != nil {
			return ports.VatTotals{}, err
		}
		sum.Base, sum.Tax, sum.Documents = sum.Base.Add(t.Base), sum.Tax.Add(t.Tax), sum.Documents+t.Documents
	}
	return sum, nil
}

func (v *Vat) OutputRows(ctx context.Context, companyID vo.CompanyID, start, end time.Time) ([]ports.VatRow, error) {
	return vatRows(ctx, companyID, start, end, []VatSource{v.output})
}

// InputRows: แถวของทุกแหล่ง (จัดซื้อ + เงินสดย่อย) เรียงรวมตามวันที่ / เลขที่
func (v *Vat) InputRows(ctx context.Context, companyID vo.CompanyID, start, end time.Time) ([]ports.VatRow, error) {
	return vatRows(ctx, companyID, start, end, v.inputs)
}

func vatRows(ctx context.Context, companyID vo.CompanyID, start, end time.Time, sources []VatSource) ([]ports.VatRow, error) {
	var out []ports.VatRow
	for _, s := range sources {
		rows, err := s.Rows(ctx, uuid.UUID(companyID), start, end)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, ports.VatRow{Date: r.Date, DocumentNo: r.DocumentNo, PartyName: r.PartyName, TaxID: r.TaxID, BranchNo: r.BranchNo,
				Base: r.Base, Tax: r.Tax})
		}
	}
	if len(sources) > 1 {
		sort.SliceStable(out, func(i, j int) bool {
			if !out[i].Date.Equal(out[j].Date) {
				return out[i].Date.Before(out[j].Date)
			}
			return out[i].DocumentNo < out[j].DocumentNo
		})
	}
	return out, nil
}

// ---------- เงินสดย่อย ----------

// PettyCashAccess คือ public API ของ pettycash ที่ finance ใช้ (*pettycash.Module)
type PettyCashAccess interface {
	OpenRequests(ctx context.Context, companyID uuid.UUID) ([]integration.PettyCashRequest, error)
	PayRequest(ctx context.Context, companyID, requestID uuid.UUID, doc integration.SettlementDoc) (integration.PettyCashPayment, error)
	UnpayRequest(ctx context.Context, companyID, documentID uuid.UUID) error
	InputVat(ctx context.Context, companyID uuid.UUID, start, end time.Time) (integration.VatTotals, error)
	InputVatRows(ctx context.Context, companyID uuid.UUID, start, end time.Time) ([]integration.VatRow, error)
}

var _ ports.PettyCash = (*PettyCash)(nil)

type PettyCash struct{ pc PettyCashAccess }

func NewPettyCash(pc PettyCashAccess) *PettyCash { return &PettyCash{pc: pc} }

func (p *PettyCash) Open(ctx context.Context, companyID vo.CompanyID) ([]ports.PettyCashRequest, error) {
	rows, err := p.pc.OpenRequests(ctx, uuid.UUID(companyID))
	out := make([]ports.PettyCashRequest, len(rows))
	for i, r := range rows {
		out[i] = ports.PettyCashRequest{ID: r.ID, DocumentNo: r.DocumentNo, FundCode: r.FundCode, FundName: r.FundName, Kind: r.Kind,
			Date: r.Date, Amount: r.Amount}
	}
	return out, err
}

func (p *PettyCash) Pay(ctx context.Context, companyID vo.CompanyID, requestID uuid.UUID, doc ports.SettlementDoc) (ports.PettyCashPayment, error) {
	res, err := p.pc.PayRequest(ctx, uuid.UUID(companyID), requestID, integration.SettlementDoc{DocumentID: doc.ID, DocumentNo: doc.No, Date: doc.Date})
	if err != nil {
		return ports.PettyCashPayment{}, err
	}
	out := ports.PettyCashPayment{DocumentNo: res.DocumentNo, FundCode: res.FundCode, FundName: res.FundName, Amount: res.Amount,
		Debits: make([]ports.PettyCashDebit, len(res.Debits))}
	for i, d := range res.Debits {
		out.Debits[i] = ports.PettyCashDebit{AccountID: vo.GLAccountID(d.AccountID), Description: d.Description, Amount: d.Amount}
	}
	return out, nil
}

func (p *PettyCash) Unpay(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return p.pc.UnpayRequest(ctx, uuid.UUID(companyID), documentID)
}

func vatTotals(t integration.VatTotals, err error) (ports.VatTotals, error) {
	if err != nil {
		return ports.VatTotals{}, err
	}
	return ports.VatTotals{Base: t.Base, Tax: t.Tax, Documents: t.Documents}, nil
}
