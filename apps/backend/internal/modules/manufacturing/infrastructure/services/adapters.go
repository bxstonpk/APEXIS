// Package services คือ adapter ไปยัง module อื่น (identity, organization, ledger, inventory)
// เรียกผ่าน public API (uuid / primitive / platform/integration) เท่านั้น
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/manufacturing/application/ports"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

func toUUIDs[T ~[16]byte](ids []T) []uuid.UUID {
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuid.UUID(id)
	}
	return out
}

// ---------- identity ----------

type IdentityAccess interface {
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

var _ ports.Authorizer = (*IdentityAuthorizer)(nil)

type IdentityAuthorizer struct{ identity IdentityAccess }

func NewIdentityAuthorizer(identity IdentityAccess) *IdentityAuthorizer {
	return &IdentityAuthorizer{identity: identity}
}

func (a *IdentityAuthorizer) Authorize(ctx context.Context, userID vo.UserID, companyID vo.CompanyID, perm vo.Permission) error {
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
	CompanyBusinessType(ctx context.Context, companyID uuid.UUID) (integration.BusinessType, error)
	FiscalPeriodAt(ctx context.Context, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
	RegisterDocumentTypes(types ...integration.DocumentType)
	DocumentFormat(ctx context.Context, companyID uuid.UUID, docType string) (numbering.Format, error)
}

var _ ports.Organization = (*Organization)(nil)

type Organization struct{ org OrganizationAccess }

func NewOrganization(org OrganizationAccess) *Organization { return &Organization{org: org} }

func (o *Organization) CompanySettings(ctx context.Context, companyID vo.CompanyID) (ports.CompanySettings, error) {
	active, amount, quantity, err := o.org.CompanySettings(ctx, uuid.UUID(companyID))
	return ports.CompanySettings{Active: active, AmountDecimals: amount, QuantityDecimals: quantity}, err
}

func (o *Organization) BusinessType(ctx context.Context, companyID vo.CompanyID) (integration.BusinessType, error) {
	return o.org.CompanyBusinessType(ctx, uuid.UUID(companyID))
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
	AccountRefs(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Ref, error)
	JournalRefs(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Ref, error)
}

var _ ports.Ledger = (*Ledger)(nil)

type Ledger struct{ ledger LedgerAccess }

func NewLedger(l LedgerAccess) *Ledger { return &Ledger{ledger: l} }

func (l *Ledger) InvalidAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error) {
	bad, err := l.ledger.InvalidPostingAccounts(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make([]vo.AccountID, len(bad))
	for i, id := range bad {
		out[i] = vo.AccountID(id)
	}
	return out, err
}

func (l *Ledger) JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error) {
	return l.ledger.JournalUsable(ctx, uuid.UUID(companyID), uuid.UUID(id))
}

func (l *Ledger) AccountsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error) {
	found, err := l.ledger.AccountIDsByCode(ctx, uuid.UUID(companyID), codes)
	out := make(map[string]vo.AccountID, len(found))
	for code, id := range found {
		out[code] = vo.AccountID(id)
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

func (l *Ledger) AccountRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]ports.Ref, error) {
	if len(ids) == 0 {
		return map[vo.AccountID]ports.Ref{}, nil
	}
	found, err := l.ledger.AccountRefs(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make(map[vo.AccountID]ports.Ref, len(found))
	for id, r := range found {
		out[vo.AccountID(id)] = ports.Ref{Code: r.Code, Name: r.Name}
	}
	return out, err
}

func (l *Ledger) JournalRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.JournalID) (map[vo.JournalID]ports.Ref, error) {
	if len(ids) == 0 {
		return map[vo.JournalID]ports.Ref{}, nil
	}
	found, err := l.ledger.JournalRefs(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make(map[vo.JournalID]ports.Ref, len(found))
	for id, r := range found {
		out[vo.JournalID(id)] = ports.Ref{Code: r.Code, Name: r.Name}
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

// ---------- inventory ----------

type InventoryAccess interface {
	PurchaseItems(ctx context.Context, companyID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]integration.Item, error)
	PurchaseContext(ctx context.Context, companyID uuid.UUID, date time.Time, itemIDs []uuid.UUID) (integration.PurchaseContext, error)
	Warehouses(ctx context.Context, companyID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]integration.Warehouse, error)
	PostReceipt(ctx context.Context, companyID, userID, warehouseID uuid.UUID, date time.Time, reference, description string, lines []integration.ReceiptLine) (integration.Receipt, error)
	PostIssue(ctx context.Context, companyID, userID, warehouseID uuid.UUID, date time.Time, reference, description string, lines []integration.IssueLine) (integration.Receipt, error)
	CancelDocument(ctx context.Context, companyID, userID, documentID uuid.UUID) error
}

var _ ports.Inventory = (*Inventory)(nil)

type Inventory struct{ inv InventoryAccess }

func NewInventory(inv InventoryAccess) *Inventory { return &Inventory{inv: inv} }

func items(in map[uuid.UUID]integration.Item) map[vo.ItemID]ports.Item {
	out := make(map[vo.ItemID]ports.Item, len(in))
	for id, i := range in {
		var acc *vo.AccountID
		if i.InventoryAccountID != uuid.Nil {
			a := vo.AccountID(i.InventoryAccountID)
			acc = &a
		}
		out[vo.ItemID(id)] = ports.Item{ID: vo.ItemID(id), Code: i.Code, Name: i.Name, Uom: i.Uom, Stock: i.Stock, Active: i.Active,
			InventoryAccountID: acc}
	}
	return out
}

func (a *Inventory) Items(ctx context.Context, companyID vo.CompanyID, ids []vo.ItemID) (map[vo.ItemID]ports.Item, error) {
	found, err := a.inv.PurchaseItems(ctx, uuid.UUID(companyID), toUUIDs(ids))
	return items(found), err
}

func (a *Inventory) Context(ctx context.Context, companyID vo.CompanyID, date time.Time, ids []vo.ItemID) (vo.AccountingSystem, map[vo.ItemID]ports.Item, error) {
	pc, err := a.inv.PurchaseContext(ctx, uuid.UUID(companyID), date, toUUIDs(ids))
	if err != nil {
		return "", nil, err
	}
	return vo.AccountingSystem(pc.System), items(pc.Items), nil
}

func (a *Inventory) Warehouses(ctx context.Context, companyID vo.CompanyID, ids []vo.WarehouseID) (map[vo.WarehouseID]ports.Warehouse, error) {
	found, err := a.inv.Warehouses(ctx, uuid.UUID(companyID), toUUIDs(ids))
	out := make(map[vo.WarehouseID]ports.Warehouse, len(found))
	for id, w := range found {
		out[vo.WarehouseID(id)] = ports.Warehouse{ID: vo.WarehouseID(id), Code: w.Code, Name: w.Name, Active: w.Active}
	}
	return out, err
}

func doc(r integration.Receipt) ports.StockDocument {
	return ports.StockDocument{DocumentID: vo.StockDocumentID(r.DocumentID), DocumentNo: r.DocumentNo, LineValues: r.LineValues}
}

func (a *Inventory) PostReceipt(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []ports.StockLine) (ports.StockDocument, error) {
	in := make([]integration.ReceiptLine, len(lines))
	for i, l := range lines {
		in[i] = integration.ReceiptLine{ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity, UnitCost: l.UnitCost}
	}
	r, err := a.inv.PostReceipt(ctx, uuid.UUID(companyID), uuid.UUID(userID), uuid.UUID(warehouseID), date, reference, description, in)
	if err != nil {
		return ports.StockDocument{}, err
	}
	return doc(r), nil
}

func (a *Inventory) PostIssue(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, warehouseID vo.WarehouseID, date time.Time, reference, description string, lines []ports.StockLine) (ports.StockDocument, error) {
	in := make([]integration.IssueLine, len(lines))
	for i, l := range lines {
		in[i] = integration.IssueLine{ItemID: uuid.UUID(l.ItemID), Quantity: l.Quantity}
	}
	r, err := a.inv.PostIssue(ctx, uuid.UUID(companyID), uuid.UUID(userID), uuid.UUID(warehouseID), date, reference, description, in)
	if err != nil {
		return ports.StockDocument{}, err
	}
	return doc(r), nil
}

func (a *Inventory) CancelDocument(ctx context.Context, companyID vo.CompanyID, userID vo.UserID, id vo.StockDocumentID) error {
	return a.inv.CancelDocument(ctx, uuid.UUID(companyID), uuid.UUID(userID), uuid.UUID(id))
}
