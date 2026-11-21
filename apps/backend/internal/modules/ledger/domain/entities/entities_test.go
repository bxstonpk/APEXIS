package entities

import (
	"errors"
	"testing"
	"time"

	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

var (
	company = vo.NewID[vo.CompanyID]()
	user    = vo.NewID[vo.UserID]()
	journal = vo.NewID[vo.JournalID]()
	cash    = vo.NewID[vo.AccountID]()
	sales   = vo.NewID[vo.AccountID]()
	day     = time.Date(2026, 12, 19, 0, 0, 0, 0, time.UTC)
)

func header(t *testing.T, code string, typ vo.AccountType) *Account {
	t.Helper()
	a, err := NewAccount(company, AccountData{Code: code, Name: code, Type: typ, Status: vo.StatusActive}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAccountParentRules(t *testing.T) {
	assets := header(t, "1000", vo.AccountAsset)
	id := assets.ID()
	child := AccountData{Code: "1101", Name: "เงินสด", Type: vo.AccountAsset, ParentID: &id, Postable: true, Status: vo.StatusActive}
	c, err := NewAccount(company, child, assets)
	if err != nil || !c.CanPost() {
		t.Fatalf("child: %v", err)
	}

	wrongType := child
	wrongType.Type = vo.AccountRevenue
	if _, err := NewAccount(company, wrongType, assets); !errors.Is(err, domain.ErrParentTypeMismatch) {
		t.Fatalf("type mismatch: %v", err)
	}
	cid := c.ID()
	grandchild := AccountData{Code: "1101-01", Name: "x", Type: vo.AccountAsset, ParentID: &cid, Postable: true, Status: vo.StatusActive}
	if _, err := NewAccount(company, grandchild, c); !errors.Is(err, domain.ErrParentNotHeader) {
		t.Fatalf("postable parent: %v", err)
	}
}

func TestAccountUsageRules(t *testing.T) {
	a := header(t, "1000", vo.AccountAsset)
	d := a.Data()

	toPostable := d
	toPostable.Postable = true
	if err := a.Update(toPostable, nil, AccountUsage{HasChildren: true}); !errors.Is(err, domain.ErrAccountHasChildren) {
		t.Fatalf("header with children -> postable: %v", err)
	}
	if err := a.Update(toPostable, nil, AccountUsage{}); err != nil {
		t.Fatalf("header without children -> postable: %v", err)
	}
	toExpense := a.Data()
	toExpense.Type = vo.AccountExpense
	if err := a.Update(toExpense, nil, AccountUsage{HasLines: true}); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("change type of used account: %v", err)
	}
	if err := a.EnsureDeletable(AccountUsage{HasLines: true}); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete used account: %v", err)
	}
}

func line(acc vo.AccountID, debit, credit string) LineData {
	return LineData{AccountID: acc, Debit: vo.MustMoney(debit), Credit: vo.MustMoney(credit)}
}

func TestEntryLineValidation(t *testing.T) {
	cases := map[string][]LineData{
		"no lines":       nil,
		"both sides":     {line(cash, "100", "100")},
		"zero line":      {line(cash, "0", "0")},
		"negative debit": {line(cash, "-100", "0")},
	}
	for name, lines := range cases {
		if _, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: lines}, 2); !errors.Is(err, domain.ErrInvalidLine) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day.Add(time.Hour), Lines: []LineData{line(cash, "1", "0")}}, 2); !errors.Is(err, domain.ErrInvalidDate) {
		t.Errorf("non date-only: %v", err)
	}
}

func TestEntryPostAndReverse(t *testing.T) {
	// draft ไม่สมดุลได้ แต่ผ่านรายการไม่ได้
	e, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: []LineData{line(cash, "10000", "0"), line(sales, "0", "9000")}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Post(Posting{EntryNo: "JV2026-000001"}); !errors.Is(err, domain.ErrUnbalanced) {
		t.Fatalf("post unbalanced: %v", err)
	}
	if err := e.Update(EntryData{JournalID: journal, Date: day, Lines: []LineData{line(cash, "10000", "0"), line(sales, "0", "10000")}}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Reversal(day, "", user); !errors.Is(err, domain.ErrEntryNotPosted) {
		t.Fatalf("reverse draft: %v", err)
	}
	if err := e.Post(Posting{EntryNo: "JV2026-000001", By: user, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := e.Update(e.Data(), 2); !errors.Is(err, domain.ErrEntryNotDraft) {
		t.Fatalf("update posted: %v", err)
	}
	if err := e.EnsureDeletable(); !errors.Is(err, domain.ErrEntryNotDraft) {
		t.Fatalf("delete posted: %v", err)
	}

	r, err := e.Reversal(day, "", user)
	if err != nil {
		t.Fatal(err)
	}
	rl := r.Data().Lines
	if !rl[0].Credit.Equal(vo.MustMoney("10000")) || !rl[1].Debit.Equal(vo.MustMoney("10000")) || r.Data().Reference != "JV2026-000001" || *r.State().ReversalOf != e.ID() {
		t.Fatalf("reversal = %+v", r.State())
	}
	if err := e.MarkReversed(r.ID()); err != nil || e.Status() != vo.EntryReversed {
		t.Fatalf("mark reversed: %v", err)
	}
	if _, err := e.Reversal(day, "", user); !errors.Is(err, domain.ErrEntryNotPosted) {
		t.Fatalf("reverse twice: %v", err)
	}
}

func TestEntryAmountDecimalsFromCompany(t *testing.T) {
	lines := []LineData{line(cash, "10.125", "0"), line(sales, "0", "10.125")}
	if _, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: lines}, 2); !errors.Is(err, domain.ErrInvalidLine) {
		t.Fatalf("3 decimals with company setting 2: %v", err)
	}
	if _, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: lines}, 3); err != nil {
		t.Fatalf("3 decimals with company setting 3: %v", err)
	}
	whole := []LineData{line(cash, "10.00", "0"), line(sales, "0", "10")}
	if _, err := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: whole}, 0); err != nil {
		t.Fatalf("10.00 with company setting 0: %v", err)
	}
}

func TestEntryNeedsTwoLines(t *testing.T) {
	e, _ := NewEntry(company, user, EntryData{JournalID: journal, Date: day, Lines: []LineData{line(cash, "100", "0")}}, 2)
	if err := e.EnsurePostable(); !errors.Is(err, domain.ErrEntryIncomplete) {
		t.Fatalf("single line: %v", err)
	}
}
