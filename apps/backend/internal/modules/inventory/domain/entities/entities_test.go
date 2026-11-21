package entities

import (
	"errors"
	"testing"
	"time"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

var (
	company = vo.NewID[vo.CompanyID]()
	user    = vo.NewID[vo.UserID]()
	item    = vo.NewID[vo.ItemID]()
	uom     = vo.NewID[vo.UomID]()
	whA     = vo.NewID[vo.WarehouseID]()
	day     = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
)

func cost(s string) *vo.Money { c := vo.MustDecimal(s); return &c }

func TestDocumentLineRules(t *testing.T) {
	q := vo.MustDecimal
	cases := map[string]struct {
		typ  vo.DocumentType
		line LineData
	}{
		"zero qty":                   {vo.DocIssue, LineData{ItemID: item, Quantity: q("0")}},
		"negative issue":             {vo.DocIssue, LineData{ItemID: item, Quantity: q("-1")}},
		"receipt without cost":       {vo.DocReceipt, LineData{ItemID: item, Quantity: q("1")}},
		"issue with cost":            {vo.DocIssue, LineData{ItemID: item, Quantity: q("1"), UnitCost: cost("5")}},
		"qty decimals > company (2)": {vo.DocReceipt, LineData{ItemID: item, Quantity: q("1.125"), UnitCost: cost("1")}},
		"negative cost":              {vo.DocReceipt, LineData{ItemID: item, Quantity: q("1"), UnitCost: cost("-1")}},
	}
	for name, c := range cases {
		_, err := NewDocument(company, user, DocumentData{Type: c.typ, Date: day, WarehouseID: whA, Lines: []LineData{c.line}}, 2)
		if !errors.Is(err, domain.ErrInvalidLine) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	// ทศนิยมตามที่บริษัทกำหนด
	if _, err := NewDocument(company, user, DocumentData{Type: vo.DocReceipt, Date: day, WarehouseID: whA,
		Lines: []LineData{{ItemID: item, Quantity: q("1.125"), UnitCost: cost("1.123456")}}}, 3); err != nil {
		t.Fatalf("3 qty decimals with company setting 3: %v", err)
	}
	// transfer ต้องมีคลังปลายทางที่ต่างกัน
	same := whA
	if _, err := NewDocument(company, user, DocumentData{Type: vo.DocTransfer, Date: day, WarehouseID: whA, ToWarehouseID: &same,
		Lines: []LineData{{ItemID: item, Quantity: q("1")}}}, 2); !errors.Is(err, domain.ErrSameWarehouse) {
		t.Fatalf("transfer to same warehouse: %v", err)
	}
}

func TestDocumentLifecycle(t *testing.T) {
	doc, err := NewDocument(company, user, DocumentData{Type: vo.DocIssue, Date: day, WarehouseID: whA,
		Lines: []LineData{{ItemID: item, Quantity: vo.MustDecimal("2")}}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Cancel(Cancellation{By: user}); !errors.Is(err, domain.ErrDocumentNotPosted) {
		t.Fatalf("cancel draft: %v", err)
	}
	if err := doc.Post(Posting{DocumentNo: "GI2026-000001", By: user}, []vo.Money{vo.MustDecimal("20")}); err != nil {
		t.Fatal(err)
	}
	if err := doc.Update(doc.Data(), 2); !errors.Is(err, domain.ErrDocumentNotDraft) {
		t.Fatalf("update posted: %v", err)
	}
	if err := doc.Cancel(Cancellation{By: user}); err != nil || doc.Status() != vo.DocCancelled {
		t.Fatalf("cancel posted: %v", err)
	}
}

func TestItemRules(t *testing.T) {
	data := ItemData{Code: "sku-1", Name: "สินค้า", Type: vo.ItemStock, UomID: uom, Status: vo.StatusActive}
	i, err := NewItem(company, data, 2)
	if err != nil || i.Data().Code != "SKU-1" || !i.IsStockable() {
		t.Fatalf("item: %+v %v", i, err)
	}
	svc := data
	svc.Type = vo.ItemService
	if err := i.Update(svc, 2, true); !errors.Is(err, domain.ErrItemHasStock) {
		t.Fatalf("change type with movements: %v", err)
	}
	if err := i.Update(svc, 2, false); err != nil || i.IsStockable() {
		t.Fatalf("change type without movements: %v", err)
	}
	rp := vo.MustDecimal("1.234")
	data.ReorderPoint = &rp
	if _, err := NewItem(company, data, 2); !errors.Is(err, domain.ErrInvalidQuantity) {
		t.Fatalf("reorder point decimals: %v", err)
	}
}

func TestAccountingSystemByFiscalYear(t *testing.T) {
	changes := []SystemChange{{FromYear: 2026, System: vo.Periodic}, {FromYear: 2028, System: vo.Perpetual}}
	for year, want := range map[int]vo.AccountingSystem{2025: vo.Perpetual, 2026: vo.Periodic, 2027: vo.Periodic, 2028: vo.Perpetual} {
		if got := SystemFor(changes, year); got != want {
			t.Errorf("SystemFor(%d) = %s, want %s", year, got, want)
		}
	}
	if _, err := NewSystemChange(2026, vo.Periodic, true); !errors.Is(err, domain.ErrSystemLocked) {
		t.Fatalf("change year with postings: %v", err)
	}
	if _, err := NewSystemChange(2026, "fifo", false); !errors.Is(err, domain.ErrInvalidSystem) {
		t.Fatalf("invalid system: %v", err)
	}
}
