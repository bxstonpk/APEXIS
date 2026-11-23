// Package services คือ adapter ไปยัง identity / organization / ledger / sales / purchasing
// เรียกผ่าน public API (uuid / primitive / platform/integration) เท่านั้น
package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/ports"
	domain "apexis/backend/internal/modules/finance/domain/errors"
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
	OpenReceivables(ctx context.Context, companyID, customerID uuid.UUID) ([]integration.OpenInvoice, error)
}

// PayablesAccess คือ public API ของ purchasing ที่ finance ใช้ (*purchasing.Module)
type PayablesAccess interface {
	SettlePayables(ctx context.Context, companyID, supplierID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error)
	UnsettlePayables(ctx context.Context, companyID, documentID uuid.UUID) error
	OpenPayables(ctx context.Context, companyID, supplierID uuid.UUID) ([]integration.OpenInvoice, error)
}

type settleFunc func(ctx context.Context, companyID, partyID uuid.UUID, doc integration.SettlementDoc, lines []integration.Settlement) (integration.SettlementResult, error)

// Settlements แปลง public API ของ sales / purchasing เป็น ports.Settlements
type Settlements struct {
	settle   settleFunc
	unsettle func(ctx context.Context, companyID, documentID uuid.UUID) error
	open     func(ctx context.Context, companyID, partyID uuid.UUID) ([]integration.OpenInvoice, error)
}

var _ ports.Settlements = (*Settlements)(nil)

func NewReceivables(s ReceivablesAccess) *Settlements {
	return &Settlements{settle: s.SettleReceivables, unsettle: s.UnsettleReceivables, open: s.OpenReceivables}
}

func NewPayables(p PayablesAccess) *Settlements {
	return &Settlements{settle: p.SettlePayables, unsettle: p.UnsettlePayables, open: p.OpenPayables}
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
		out.Invoices[i] = ports.SettledInvoice{ID: vo.InvoiceID(x.InvoiceID), DocumentNo: x.DocumentNo, PartnerRef: x.PartnerRef, Net: x.Net, Gross: x.Gross}
	}
	return out, nil
}

func (s *Settlements) Unsettle(ctx context.Context, companyID vo.CompanyID, documentID uuid.UUID) error {
	return s.unsettle(ctx, uuid.UUID(companyID), documentID)
}

func (s *Settlements) Open(ctx context.Context, companyID vo.CompanyID, partyID uuid.UUID) ([]ports.OpenInvoice, error) {
	rows, err := s.open(ctx, uuid.UUID(companyID), partyID)
	out := make([]ports.OpenInvoice, len(rows))
	for i, r := range rows {
		out[i] = ports.OpenInvoice{ID: vo.InvoiceID(r.ID), DocumentNo: r.DocumentNo, PartnerRef: r.PartnerRef, Date: r.Date, DueDate: r.DueDate,
			Net: r.Net, Gross: r.Gross, Outstanding: r.Outstanding}
	}
	return out, err
}
