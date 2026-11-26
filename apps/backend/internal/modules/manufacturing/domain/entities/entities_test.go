package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/manufacturing/domain/entities"
	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	"apexis/backend/internal/modules/manufacturing/domain/services"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
)

var (
	d   = vo.MustDecimal
	dec = entities.Decimals{Quantity: 3, Amount: 2}
)

func ids() (vo.CompanyID, vo.ItemID, vo.ItemID, vo.ItemID) {
	return vo.NewID[vo.CompanyID](), vo.NewID[vo.ItemID](), vo.NewID[vo.ItemID](), vo.NewID[vo.ItemID]()
}

func bomData(product, a, b vo.ItemID) entities.BomData {
	return entities.BomData{
		Code: "bom-1", Name: " โต๊ะ ", ItemID: product, OutputQuantity: d("2"), IsDefault: true,
		Components: []entities.ComponentData{
			{ItemID: a, Quantity: d("4"), ScrapPercent: d("10")},
			{ItemID: b, Quantity: d("1.5"), ScrapPercent: decimal.Zero},
		},
		Conversions: []entities.ConversionData{{Description: "ค่าแรง", Amount: d("100")}, {Description: "โสหุ้ย", Amount: d("33.33")}},
	}
}

func TestBomValidation(t *testing.T) {
	company, product, a, b := ids()
	bom, err := entities.NewBom(company, bomData(product, a, b), dec)
	if err != nil {
		t.Fatal(err)
	}
	if bom.Data().Code != "BOM-1" || bom.Data().Name != "โต๊ะ" || bom.Data().Status != vo.StatusActive ||
		bom.Components()[1].LineNo != 2 || len(bom.ComponentItemIDs()) != 2 {
		t.Fatalf("bom = %+v %+v", bom.Data(), bom.Components())
	}
	cases := map[string]struct {
		mutate func(*entities.BomData)
		want   error
	}{
		"zero output":       {func(x *entities.BomData) { x.OutputQuantity = decimal.Zero }, domain.ErrInvalidQuantity},
		"output decimals":   {func(x *entities.BomData) { x.OutputQuantity = d("1.2345") }, domain.ErrInvalidQuantity},
		"no components":     {func(x *entities.BomData) { x.Components = nil }, domain.ErrInvalidLine},
		"self component":    {func(x *entities.BomData) { x.Components[0].ItemID = product }, domain.ErrInvalidLine},
		"duplicate":         {func(x *entities.BomData) { x.Components[1].ItemID = a }, domain.ErrInvalidLine},
		"zero quantity":     {func(x *entities.BomData) { x.Components[0].Quantity = decimal.Zero }, domain.ErrInvalidLine},
		"scrap 100":         {func(x *entities.BomData) { x.Components[0].ScrapPercent = d("100") }, domain.ErrInvalidLine},
		"scrap negative":    {func(x *entities.BomData) { x.Components[0].ScrapPercent = d("-1") }, domain.ErrInvalidLine},
		"negative amount":   {func(x *entities.BomData) { x.Conversions[0].Amount = d("-1") }, domain.ErrInvalidLine},
		"amount decimals":   {func(x *entities.BomData) { x.Conversions[0].Amount = d("1.005") }, domain.ErrInvalidLine},
		"blank description": {func(x *entities.BomData) { x.Conversions[0].Description = " " }, domain.ErrInvalidLine},
		"bad code":          {func(x *entities.BomData) { x.Code = "ก" }, domain.ErrInvalidCode},
		"bad status":        {func(x *entities.BomData) { x.Status = "x" }, domain.ErrInvalidStatus},
	}
	for name, c := range cases {
		x := bomData(product, a, b)
		c.mutate(&x)
		if _, err := entities.NewBom(company, x, dec); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

func TestRequirementAndScaling(t *testing.T) {
	// 4 ต่อ 2 ชุด, สั่ง 5, สูญเสีย 10% -> 4 × 5/2 × 1.1 = 11
	if got := services.Requirement(d("4"), d("10"), d("2"), d("5"), 3); !got.Equal(d("11")) {
		t.Fatalf("requirement = %s", got)
	}
	// 1 ต่อ 3 ชุด, สั่ง 1 -> 0.333 (ปัด 3 ตำแหน่ง)
	if got := services.Requirement(d("1"), decimal.Zero, d("3"), d("1"), 3); !got.Equal(d("0.333")) {
		t.Fatalf("requirement rounding = %s", got)
	}
	if got := services.Scaled(d("100"), d("3"), d("2"), 2); !got.Equal(d("66.67")) {
		t.Fatalf("scaled = %s", got)
	}
	if got := services.Proportional(d("100"), d("3"), d("3"), 2); !got.Equal(d("100")) {
		t.Fatalf("full return = %s", got)
	}
	if got := services.Proportional(d("100"), d("3"), d("1"), 2); !got.Equal(d("33.33")) {
		t.Fatalf("partial return = %s", got)
	}
}

func TestReceiptLinesKeepExactValue(t *testing.T) {
	for _, c := range []struct{ qty, value string }{{"3", "100"}, {"1", "0"}, {"12345.678", "99999.99"}, {"1000000", "1234567.89"}, {"0.5", "10.01"}} {
		lines := services.ReceiptLines(d(c.qty), d(c.value), 2)
		qty, value := decimal.Zero, decimal.Zero
		for _, l := range lines {
			qty = qty.Add(l.Quantity)
			value = value.Add(l.Quantity.Mul(l.UnitCost).Round(2))
			if l.UnitCost.IsNegative() || l.UnitCost.Exponent() < -6 {
				t.Fatalf("%v: unit cost %s", c, l.UnitCost)
			}
		}
		if !qty.Equal(d(c.qty)) || !value.Equal(d(c.value)) {
			t.Fatalf("%v: lines %v -> qty %s value %s", c, lines, qty, value)
		}
	}
}

func TestOrderLifecycle(t *testing.T) {
	company, product, a, b := ids()
	bom, _ := entities.NewBom(company, bomData(product, a, b), dec)
	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	data := entities.OrderData{Date: date, ItemID: product, BomID: bom.ID(), Quantity: d("5"), WarehouseID: vo.NewID[vo.WarehouseID]()}
	o, err := entities.NewOrder(company, "MO2026-000001", data, entities.SnapshotOf(bom), vo.NewID[vo.UserID](), date, dec)
	if err != nil {
		t.Fatal(err)
	}
	if m := o.Materials(); !m[0].Required.Equal(d("11")) || !m[1].Required.Equal(d("3.75")) {
		t.Fatalf("materials = %+v", m)
	}
	// (100 + 33.33) × 5/2 = 250 + 83.33 (ปัดต่อบรรทัด)
	if got := o.PlannedConversion(2); !got.Equal(d("333.33")) {
		t.Fatalf("planned conversion = %s", got)
	}
	wrong := entities.SnapshotOf(bom)
	wrong.ItemID = a
	if _, err := entities.NewOrder(company, "x", data, wrong, vo.NewID[vo.UserID](), date, dec); !errors.Is(err, domain.ErrBomItemMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}
	if err := o.UpdateDraft(data, entities.SnapshotOf(bom), []entities.MaterialInput{{ItemID: a, Required: d("12")}}, dec); err != nil {
		t.Fatal(err)
	}
	if len(o.Materials()) != 1 || !o.Materials()[0].Required.Equal(d("12")) {
		t.Fatalf("edited materials = %+v", o.Materials())
	}
	if err := o.UpdateDraft(data, entities.SnapshotOf(bom), nil, dec); err != nil || len(o.Materials()) != 2 {
		t.Fatalf("regenerate = %v %+v", err, o.Materials())
	}
	issue := []entities.MovementLine{{ItemID: a, Quantity: d("12")}}
	if err := o.CheckLines(date, issue, dec); !errors.Is(err, domain.ErrOrderNotReleased) {
		t.Fatalf("issue draft err = %v", err)
	}
	if err := o.Release(); err != nil {
		t.Fatal(err)
	}
	if err := o.UpdateDraft(data, entities.SnapshotOf(bom), nil, dec); !errors.Is(err, domain.ErrOrderNotDraft) {
		t.Fatalf("edit released err = %v", err)
	}
	if err := o.CheckLines(date.AddDate(0, 0, -1), issue, dec); !errors.Is(err, domain.ErrDateBeforeOrder) {
		t.Fatalf("early date err = %v", err)
	}
	if err := o.CheckLines(date, []entities.MovementLine{{ItemID: product, Quantity: d("1")}}, dec); !errors.Is(err, domain.ErrMaterialNotOnOrder) {
		t.Fatalf("foreign item err = %v", err)
	}
	o.ApplyIssue(issue, []vo.Money{d("120")}, entities.Movement{Kind: vo.MoveIssue, Value: d("120")})
	if _, err := o.ReturnValues([]entities.MovementLine{{ItemID: a, Quantity: d("13")}}, 2); !errors.Is(err, domain.ErrOverReturn) {
		t.Fatalf("over return err = %v", err)
	}
	ret := []entities.MovementLine{{ItemID: a, Quantity: d("1")}}
	values, err := o.ReturnValues(ret, 2)
	if err != nil || !values[0].Equal(d("10")) {
		t.Fatalf("return values = %v %v", values, err)
	}
	o.ApplyReturn(ret, values, entities.Movement{Kind: vo.MoveReturn, Value: values[0]})
	p, err := o.PlanCompletion(date, d("4"), dec)
	// วัตถุดิบ 110 + (100 + 33.33) × 4/2 = 200 + 66.66 -> 376.66
	if err != nil || !p.Materials.Equal(d("110")) || !p.Conversion.Equal(d("266.66")) || !p.Total.Equal(d("376.66")) || len(p.Conversions) != 2 {
		t.Fatalf("plan = %+v %v", p, err)
	}
	o.Complete(date, p, entities.Movement{Kind: vo.MoveReceipt, Value: p.Total})
	if u := o.UnitCost(); u == nil || !u.Equal(d("94.165")) {
		t.Fatalf("unit cost = %v", u)
	}
	if got := o.ActiveMovements(); len(got) != 3 || got[0].Kind != vo.MoveReceipt {
		t.Fatalf("active movements = %+v", got)
	}
	if _, err := o.CheckCancel(" "); !errors.Is(err, domain.ErrInvalidReason) {
		t.Fatalf("blank reason err = %v", err)
	}
	o.Cancel("ผลิตผิดสเปก", date)
	if _, err := o.CheckCancel("x"); !errors.Is(err, domain.ErrAlreadyCancelled) || len(o.ActiveMovements()) != 0 {
		t.Fatalf("cancel twice err = %v", err)
	}
}
