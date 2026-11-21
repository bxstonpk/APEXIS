package entities

import (
	"errors"
	"testing"
	"time"

	domain "apexis/backend/internal/modules/organization/domain/errors"
	"apexis/backend/internal/modules/organization/domain/services"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation(time.DateOnly, s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

var company = vo.NewID[vo.CompanyID]()

func TestHeadOfficeRules(t *testing.T) {
	hq, err := NewBranch(company, BranchData{Code: "hq", Name: "สำนักงานใหญ่", TaxBranchNo: "00000", Status: vo.StatusActive})
	if err != nil || !hq.IsHeadOffice() || hq.Data().Code != "HQ" || hq.Data().Address.Country != "TH" {
		t.Fatalf("hq = %+v %v", hq, err)
	}
	if err := hq.EnsureDeletable(); !errors.Is(err, domain.ErrHeadOfficeImmutable) {
		t.Fatalf("delete hq: %v", err)
	}
	d := hq.Data()
	d.Status = vo.StatusInactive
	if err := hq.Update(d); !errors.Is(err, domain.ErrHeadOfficeImmutable) {
		t.Fatalf("deactivate hq: %v", err)
	}
	d = hq.Data()
	d.TaxBranchNo = "00001"
	if err := hq.Update(d); !errors.Is(err, domain.ErrHeadOfficeImmutable) {
		t.Fatalf("change hq branch no: %v", err)
	}
	d = hq.Data()
	d.Name = "Head Office"
	if err := hq.Update(d); err != nil {
		t.Fatalf("rename hq: %v", err)
	}

	b, _ := NewBranch(company, BranchData{Code: "B1", Name: "สาขา 1", TaxBranchNo: "00001", Status: vo.StatusActive})
	if err := b.EnsureDeletable(); err != nil {
		t.Fatalf("delete branch: %v", err)
	}
	if _, err := NewBranch(company, BranchData{Code: "B2", Name: "x", TaxBranchNo: "00002", Status: "closed"}); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("status: %v", err)
	}
}

func TestHierarchyCycle(t *testing.T) {
	a, _ := NewDepartment(company, DepartmentData{Code: "A", Name: "A", Status: vo.StatusActive})
	self := a.ID()
	d := a.Data()
	d.ParentID = &self
	if err := a.Update(d); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatalf("self parent: %v", err)
	}
	child := vo.NewID[vo.DepartmentID]()
	// ancestry ของ parent ใหม่ (child) คือ [child, a] -> a เป็นบรรพบุรุษของ child = วนเป็นวง
	if err := services.EnsureNoCycle(a.ID(), []vo.DepartmentID{child, a.ID()}); !errors.Is(err, domain.ErrHierarchyCycle) {
		t.Fatalf("descendant parent: %v", err)
	}
	if err := services.EnsureNoCycle(a.ID(), []vo.DepartmentID{child}); err != nil {
		t.Fatalf("unrelated parent: %v", err)
	}
}

func TestCostCenterValidity(t *testing.T) {
	to := day("2026-12-31")
	cc, err := NewCostCenter(company, CostCenterData{Code: "CC1", Name: "Marketing", ValidFrom: day("2026-01-01"), ValidTo: &to, Status: vo.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if !cc.IsValidOn(day("2026-06-30")) || cc.IsValidOn(day("2027-01-01")) || cc.IsValidOn(day("2025-12-31")) {
		t.Fatal("validity window is wrong")
	}
	bad := day("2025-01-01")
	if _, err := NewCostCenter(company, CostCenterData{Code: "CC2", Name: "x", ValidFrom: day("2026-01-01"), ValidTo: &bad, Status: vo.StatusActive}); !errors.Is(err, domain.ErrInvalidDateRange) {
		t.Fatalf("range: %v", err)
	}
}

func TestClosedProjectNeedsEndDate(t *testing.T) {
	d := ProjectSiteData{Code: "P1", Name: "Site", StartDate: day("2026-01-01"), Status: vo.ProjectClosed}
	if _, err := NewProjectSite(company, d); !errors.Is(err, domain.ErrProjectEndDateRequired) {
		t.Fatalf("got %v", err)
	}
	end := day("2026-06-30")
	d.EndDate = &end
	if _, err := NewProjectSite(company, d); err != nil {
		t.Fatal(err)
	}
}

func TestFiscalYearPeriods(t *testing.T) {
	// ปีงบประมาณแบบราชการ: 1 ต.ค. 2025 - 30 ก.ย. 2026 ข้ามปีอธิกสุรทินไม่ได้ แต่ต้องได้สิ้นเดือนถูกต้อง
	fy, err := NewFiscalYear(company, 2026, day("2025-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	ps := fy.Periods()
	if len(ps) != 12 || fy.EndDate() != day("2026-09-30") {
		t.Fatalf("periods=%d end=%v", len(ps), fy.EndDate())
	}
	if ps[0].EndDate() != day("2025-10-31") || ps[4].StartDate() != day("2026-02-01") || ps[4].EndDate() != day("2026-02-28") {
		t.Fatalf("period dates wrong: %v %v %v", ps[0].EndDate(), ps[4].StartDate(), ps[4].EndDate())
	}
	leap, _ := NewFiscalYear(company, 2028, day("2028-01-01"))
	if leap.Periods()[1].EndDate() != day("2028-02-29") {
		t.Fatalf("leap feb end = %v", leap.Periods()[1].EndDate())
	}
	if _, err := NewFiscalYear(company, 2026, day("2026-01-15")); !errors.Is(err, domain.ErrInvalidFiscalStart) {
		t.Fatalf("mid-month start: %v", err)
	}
}

func TestFiscalPeriodCloseOrder(t *testing.T) {
	fy, _ := NewFiscalYear(company, 2026, day("2026-01-01"))
	ps := fy.Periods()
	if _, err := fy.ChangePeriodStatus(ps[1].ID(), vo.PeriodClosed); !errors.Is(err, domain.ErrPeriodCloseOrder) {
		t.Fatalf("close feb before jan: %v", err)
	}
	for _, p := range ps[:2] {
		if _, err := fy.ChangePeriodStatus(p.ID(), vo.PeriodClosed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fy.ChangePeriodStatus(ps[0].ID(), vo.PeriodOpen); !errors.Is(err, domain.ErrPeriodReopenOrder) {
		t.Fatalf("reopen jan while feb closed: %v", err)
	}
	if p, err := fy.ChangePeriodStatus(ps[1].ID(), vo.PeriodOpen); err != nil || !p.IsOpen() {
		t.Fatalf("reopen feb: %v", err)
	}
	if _, err := fy.ChangePeriodStatus(vo.NewID[vo.FiscalPeriodID](), vo.PeriodClosed); !errors.Is(err, domain.ErrFiscalPeriodNotFound) {
		t.Fatalf("unknown period: %v", err)
	}
}

func TestCompanyRules(t *testing.T) {
	base := CompanyData{Code: "acme", Name: "ACME", BaseCurrency: "thb", FiscalYearStartMonth: 1, AmountDecimals: 2, QuantityDecimals: 2, Status: vo.StatusActive}
	c, err := NewCompany(base)
	if err != nil {
		t.Fatal(err)
	}
	if d := c.Data(); d.Code != "ACME" || d.BaseCurrency != "THB" || d.LegalNameTH != "ACME" || d.Address.Country != "TH" {
		t.Fatalf("normalized = %+v", d)
	}

	// รหัสบริษัทแก้ไม่ได้ — Update คงรหัสเดิม
	upd := base
	upd.Code, upd.Name, upd.Status = "OTHER", "ACME Thailand", vo.StatusInactive
	if err := c.Update(upd); err != nil || c.Data().Code != "ACME" || c.Data().Name != "ACME Thailand" || c.IsActive() {
		t.Fatalf("update: %+v %v", c.Data(), err)
	}

	// ทศนิยมเพิ่มได้ ลดไม่ได้
	more := c.Data()
	more.AmountDecimals, more.QuantityDecimals = 4, 3
	if err := c.Update(more); err != nil {
		t.Fatalf("increase decimals: %v", err)
	}
	less := c.Data()
	less.QuantityDecimals = 2
	if err := c.Update(less); !errors.Is(err, domain.ErrDecimalsDecrease) {
		t.Fatalf("decrease decimals: %v", err)
	}

	cases := map[string]struct {
		mutate func(*CompanyData)
		want   error
	}{
		"bad code":    {func(d *CompanyData) { d.Code = "a b" }, domain.ErrInvalidCode},
		"no name":     {func(d *CompanyData) { d.Name = " " }, domain.ErrInvalidName},
		"bad tax id":  {func(d *CompanyData) { d.TaxID = "01/05" }, domain.ErrInvalidTaxID},
		"month 13":    {func(d *CompanyData) { d.FiscalYearStartMonth = 13 }, domain.ErrInvalidFiscalMonth},
		"bad email":   {func(d *CompanyData) { d.Email = "nope" }, domain.ErrInvalidEmail},
		"bad website": {func(d *CompanyData) { d.Website = "javascript:alert(1)" }, domain.ErrInvalidWebsite},
		"currency":    {func(d *CompanyData) { d.BaseCurrency = "BAHT" }, domain.ErrInvalidCurrency},
		"status":      {func(d *CompanyData) { d.Status = "deleted" }, domain.ErrInvalidStatus},
		"decimals 7":  {func(d *CompanyData) { d.AmountDecimals = 7 }, domain.ErrInvalidDecimals},
		"decimals -1": {func(d *CompanyData) { d.QuantityDecimals = -1 }, domain.ErrInvalidDecimals},
	}
	for name, tc := range cases {
		d := base
		tc.mutate(&d)
		if _, err := NewCompany(d); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}
