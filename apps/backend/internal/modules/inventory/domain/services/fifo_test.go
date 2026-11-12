package services

import (
	"errors"
	"testing"
	"time"

	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

var (
	company = vo.NewID[vo.CompanyID]()
	user    = vo.NewID[vo.UserID]()
	item    = vo.NewID[vo.ItemID]()
	whA     = vo.NewID[vo.WarehouseID]()
	whB     = vo.NewID[vo.WarehouseID]()
)

func d(s string) vo.Quantity { return vo.MustDecimal(s) }
func day(n int) time.Time    { return time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC) }

func doc(t *testing.T, typ vo.DocumentType, date time.Time, lines ...entities.LineData) *entities.Document {
	t.Helper()
	data := entities.DocumentData{Type: typ, Date: date, WarehouseID: whA, Lines: lines}
	if typ == vo.DocTransfer {
		to := whB
		data.ToWarehouseID = &to
	}
	doc, err := entities.NewDocument(company, user, data, 2)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func in(qty, cost string) entities.LineData {
	c := d(cost)
	return entities.LineData{ItemID: item, Quantity: d(qty), UnitCost: &c}
}

func out(qty string) entities.LineData { return entities.LineData{ItemID: item, Quantity: d(qty)} }

func layer(date time.Time, qty, value string) *Layer {
	return &Layer{
		ID: vo.NewID[vo.LayerID](), Key: StockKey{item, whA}, Date: date,
		Quantity: d(qty), Value: d(value), RemainingQuantity: d(qty), RemainingValue: d(value),
	}
}

func eq(t *testing.T, what string, got vo.Quantity, want string) {
	t.Helper()
	if !got.Equal(d(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

func TestReceiptValueRoundsToCompanyDecimals(t *testing.T) {
	p, err := PlanPosting(doc(t, vo.DocReceipt, day(1), in("3", "10.3333")), nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "value", p.LineValues[0], "31") // 30.9999 -> 31.00
	if len(p.NewLayers) != 1 || len(p.Movements) != 1 {
		t.Fatalf("plan = %+v", p)
	}
}

func TestIssueConsumesOldestLayersFirst(t *testing.T) {
	old := layer(day(1), "10", "100") // 10 @ 10
	mid := layer(day(2), "10", "120") // 10 @ 12
	p, err := PlanPosting(doc(t, vo.DocIssue, day(5), out("15")), []*Layer{mid, old}, 2)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "issue value", p.LineValues[0], "160") // 10*10 + 5*12
	eq(t, "old remaining", old.RemainingQuantity, "0")
	eq(t, "mid remaining", mid.RemainingQuantity, "5")
	eq(t, "mid remaining value", mid.RemainingValue, "60")
	m := p.Movements[0]
	eq(t, "movement qty", m.Quantity, "-15")
	eq(t, "movement value", m.Value, "-160")
	if len(m.Allocations) != 2 || len(p.UpdatedLayers) != 2 {
		t.Fatalf("allocations = %+v updated = %d", m.Allocations, len(p.UpdatedLayers))
	}
}

func TestPartialConsumptionLeavesNoRoundingDust(t *testing.T) {
	l := layer(day(1), "3", "10") // 3.3333.. ต่อหน่วย
	for i := 0; i < 3; i++ {
		if _, err := PlanPosting(doc(t, vo.DocIssue, day(2), out("1")), []*Layer{l}, 2); err != nil {
			t.Fatal(err)
		}
	}
	eq(t, "remaining qty", l.RemainingQuantity, "0")
	eq(t, "remaining value", l.RemainingValue, "0") // 3.33 + 3.34 + 3.33 = 10 พอดี
}

func TestNoNegativeStockAndNoFutureLayers(t *testing.T) {
	future := layer(day(10), "100", "1000")
	_, err := PlanPosting(doc(t, vo.DocIssue, day(5), out("1")), []*Layer{future}, 2)
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("issue before receipt date: %v", err)
	}
	_, err = PlanPosting(doc(t, vo.DocIssue, day(20), out("101")), []*Layer{future}, 2)
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("issue more than on hand: %v", err)
	}
	eq(t, "untouched after failure", future.RemainingQuantity, "100")
}

func TestTransferCarriesFifoCost(t *testing.T) {
	a := layer(day(1), "5", "50")
	b := layer(day(2), "5", "60")
	p, err := PlanPosting(doc(t, vo.DocTransfer, day(3), out("7")), []*Layer{a, b}, 2)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "transfer value", p.LineValues[0], "74") // 50 + 2*12
	if len(p.NewLayers) != 2 || p.NewLayers[0].Key.WarehouseID != whB {
		t.Fatalf("destination layers = %+v", p.NewLayers)
	}
	eq(t, "dest layer 2 value", p.NewLayers[1].Value, "24")
	if len(p.Movements) != 2 {
		t.Fatalf("movements = %d", len(p.Movements))
	}
	eq(t, "dest movement", p.Movements[1].Value, "74")
}

func TestAdjustmentInThenOutInSameDocument(t *testing.T) {
	p, err := PlanPosting(doc(t, vo.DocAdjustment, day(1), in("10", "2"), out("-4")), nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "out value", p.LineValues[1], "8")
	eq(t, "layer remaining", p.NewLayers[0].RemainingQuantity, "6")
}

func TestCancelRestoresLayersAndRejectsUsedReceipt(t *testing.T) {
	l := layer(day(1), "10", "100")
	issue := doc(t, vo.DocIssue, day(2), out("4"))
	p, err := PlanPosting(issue, []*Layer{l}, 2)
	if err != nil {
		t.Fatal(err)
	}
	c, err := PlanCancel(CancelInput{Movements: p.Movements, Layers: map[vo.LayerID]*Layer{l.ID: l}})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "restored qty", l.RemainingQuantity, "10")
	eq(t, "restored value", l.RemainingValue, "100")
	eq(t, "reversing movement", c.Movements[0].Quantity, "4")

	// ใบรับที่ถูกเบิกไปแล้ว ยกเลิกไม่ได้
	receipt := doc(t, vo.DocReceipt, day(1), in("10", "10"))
	rp, _ := PlanPosting(receipt, nil, 2)
	created := rp.NewLayers[0]
	if _, err := PlanPosting(doc(t, vo.DocIssue, day(2), out("1")), []*Layer{created}, 2); err != nil {
		t.Fatal(err)
	}
	_, err = PlanCancel(CancelInput{Movements: rp.Movements, CreatedLayerIDs: []vo.LayerID{created.ID}, Layers: map[vo.LayerID]*Layer{created.ID: created}})
	if !errors.Is(err, domain.ErrStockAlreadyUsed) {
		t.Fatalf("cancel used receipt: %v", err)
	}
}
