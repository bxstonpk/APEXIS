// Package entities คือ entity ของ sales
//
// รูปแบบร่วม: รับข้อมูลดิบผ่าน *Data struct แล้ว validate / normalize; Rehydrate* ใช้โดย repository เท่านั้น
package entities

import (
	"apexis/backend/internal/platform/thaiaddress"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	"apexis/backend/internal/modules/sales/domain/services"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// MaxLines คือจำนวนบรรทัดสูงสุดต่อเอกสาร
const MaxLines = 500

// Decimals คือจำนวนทศนิยมที่บริษัทกำหนด
type Decimals struct{ Quantity, Amount int }

// ---------- ค่าตั้งขาย ----------

// Settings: บริษัทจด VAT หรือไม่ + อัตรา และบัญชีที่ใช้ลงรายการ (nil = ไม่ได้ตั้ง)
type Settings struct {
	VatRegistered      bool
	VatRate            decimal.Decimal
	JournalID          *vo.JournalID
	ARAccountID        *vo.AccountID // ลูกหนี้การค้า
	OutputVatAccountID *vo.AccountID // ภาษีขาย
	RevenueAccountID   *vo.AccountID // รายได้ค่าเริ่มต้น (เมื่อสินค้า / หมวดไม่ได้ตั้ง)
	CogsAccountID      *vo.AccountID // ต้นทุนขาย (perpetual)
	// SalesReturnsAccountID: รับคืนสินค้าและส่วนลดจ่าย (ใบลดหนี้) — ไม่ตั้ง = ลดบัญชีรายได้ของบรรทัดในใบแจ้งหนี้
	SalesReturnsAccountID *vo.AccountID
}

func DefaultSettings() Settings { return Settings{VatRegistered: true, VatRate: decimal.NewFromInt(7)} }

func NewSettings(s Settings) (Settings, error) {
	if s.VatRate.IsNegative() || s.VatRate.GreaterThan(decimal.NewFromInt(100)) || !numeric.FitsScale(s.VatRate, 2) {
		return Settings{}, domain.ErrInvalidVatRate
	}
	return s, nil
}

func (s Settings) AccountIDs() []vo.AccountID {
	var out []vo.AccountID
	for _, p := range []*vo.AccountID{s.ARAccountID, s.OutputVatAccountID, s.RevenueAccountID, s.CogsAccountID, s.SalesReturnsAccountID} {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// Require: ค่าตั้งที่ต้องมีก่อนลงบัญชี — คืน ErrSettingsMissing พร้อมชื่อที่ขาด
func (s Settings) Require(fields ...string) error {
	set := map[string]bool{
		"journal_id": s.JournalID != nil, "ar_account_id": s.ARAccountID != nil, "output_vat_account_id": s.OutputVatAccountID != nil,
		"revenue_account_id": s.RevenueAccountID != nil, "cogs_account_id": s.CogsAccountID != nil,
	}
	var missing []string
	for _, f := range fields {
		if !set[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s not set", domain.ErrSettingsMissing, strings.Join(missing, ", "))
	}
	return nil
}

// Terms: เงื่อนไข VAT ของเอกสารใหม่ — บริษัทไม่จด VAT = 0% (เลือกราคารวม VAT ไม่ได้)
func (s Settings) Terms(inclusive bool) (services.VatTerms, error) {
	if !s.VatRegistered {
		if inclusive {
			return services.VatTerms{}, domain.ErrVatNotApplicable
		}
		return services.VatTerms{Rate: decimal.Zero}, nil
	}
	return services.VatTerms{Registered: true, Rate: s.VatRate, Inclusive: inclusive}, nil
}

// ---------- ลูกค้า ----------

type CustomerData struct {
	Code            string
	Name            string
	TaxID           string
	BranchNo        string
	Address         string
	AddressDetail   thaiaddress.Address // ที่อยู่ตามแบบสรรพากร — Address = ข้อความที่ประกอบจากช่องเหล่านี้
	PaymentTermDays int
	TaxType         vo.TaxType
	CreditLimit     *vo.Money // nil = ไม่จำกัด
	Status          vo.Status
	Branches        []CustomerBranch // สาขาเพิ่มเติม (ไม่รวมสาขาหลัก)
}

type Customer struct {
	id        vo.CustomerID
	companyID vo.CompanyID
	data      CustomerData
}

func NewCustomer(companyID vo.CompanyID, d CustomerData, amountDecimals int) (*Customer, error) {
	n, err := normalizeCustomer(d, amountDecimals)
	if err != nil {
		return nil, err
	}
	return &Customer{id: vo.NewID[vo.CustomerID](), companyID: companyID, data: n}, nil
}

func RehydrateCustomer(id vo.CustomerID, companyID vo.CompanyID, d CustomerData) *Customer {
	return &Customer{id: id, companyID: companyID, data: d}
}

func (c *Customer) ID() vo.CustomerID       { return c.id }
func (c *Customer) CompanyID() vo.CompanyID { return c.companyID }
func (c *Customer) Data() CustomerData      { return c.data }

func (c *Customer) Update(d CustomerData, amountDecimals int) error {
	n, err := normalizeCustomer(d, amountDecimals)
	if err != nil {
		return err
	}
	c.data = n
	return nil
}

func (c *Customer) EnsureActive() error {
	if c.data.Status != vo.StatusActive {
		return domain.ErrCustomerInactive
	}
	return nil
}

// CreditWarning: ยอดค้าง (รวมเอกสารใหม่) เกินวงเงิน = ข้อความเตือน (ไม่ห้าม) — ไม่ตั้งวงเงิน = ""
func (c *Customer) CreditWarning(exposure vo.Money) string {
	l := c.data.CreditLimit
	if l == nil || !exposure.GreaterThan(*l) {
		return ""
	}
	return fmt.Sprintf("credit limit exceeded: customer %s exposure %s, limit %s (over by %s)",
		c.data.Code, exposure, *l, exposure.Sub(*l))
}

func normalizeCustomer(d CustomerData, amountDecimals int) (CustomerData, error) {
	var err error
	out := CustomerData{}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return CustomerData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return CustomerData{}, err
	}
	if out.TaxID, err = vo.TaxID(d.TaxID); err != nil {
		return CustomerData{}, err
	}
	if out.BranchNo, err = vo.BranchNo(d.BranchNo); err != nil {
		return CustomerData{}, err
	}
	if out.AddressDetail, err = thaiaddress.New(d.AddressDetail); err != nil {
		return CustomerData{}, err
	}
	out.Address = out.AddressDetail.Format()
	if d.PaymentTermDays < 0 || d.PaymentTermDays > 365 {
		return CustomerData{}, domain.ErrInvalidTerm
	}
	out.PaymentTermDays = d.PaymentTermDays
	if out.TaxType, err = vo.ParseTaxType(string(d.TaxType)); err != nil {
		return CustomerData{}, err
	}
	if l := d.CreditLimit; l != nil {
		if l.IsNegative() || !numeric.InRange(*l) || !numeric.FitsScale(*l, amountDecimals) {
			return CustomerData{}, fmt.Errorf("%w: credit_limit must be zero or positive with at most %d decimal places", domain.ErrInvalidAmount, amountDecimals)
		}
		v := *l
		out.CreditLimit = &v
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return CustomerData{}, err
	}
	if out.Branches, err = normalizeBranches(out.BranchNo, d.Branches); err != nil {
		return CustomerData{}, err
	}
	return out, nil
}

func EnsureUnused(used bool) error {
	if used {
		return domain.ErrInUse
	}
	return nil
}

// ---------- บรรทัดราคา (ใช้ร่วมกันระหว่างใบเสนอราคา / ใบสั่งขาย) ----------

type LineData struct {
	ItemID      vo.ItemID
	Description string
	Quantity    vo.Quantity
	UnitPrice   vo.Money // รวมหรือไม่รวม VAT ตาม Terms.Vat.Inclusive
}

// Terms คือเงื่อนไขที่เอกสารเก็บ ณ ตอนสร้าง: VAT จากค่าตั้งขาย, เครดิตเทอมจากลูกค้า
type Terms struct {
	Vat             services.VatTerms
	PaymentTermDays int
}

// Priced คือบรรทัดที่ผ่านการตรวจ + ยอดต่อบรรทัด / ยอดรวม
type Priced struct {
	Lines   []LineData
	Amounts []services.Amounts
	Totals  services.Amounts
}

func price(lines []LineData, terms Terms, dec Decimals) (Priced, error) {
	if len(lines) == 0 || len(lines) > MaxLines {
		return Priced{}, fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	out := Priced{Lines: make([]LineData, len(lines)), Amounts: make([]services.Amounts, len(lines)), Totals: services.ZeroAmounts()}
	for i, l := range lines {
		bad := func(msg string) error { return fmt.Errorf("%w: line %d %s", domain.ErrInvalidLine, i+1, msg) }
		switch {
		case !l.Quantity.IsPositive() || !numeric.InRange(l.Quantity):
			return Priced{}, bad("quantity must be positive")
		case !numeric.FitsScale(l.Quantity, dec.Quantity):
			return Priced{}, bad(fmt.Sprintf("quantity has more than %d decimal places", dec.Quantity))
		case l.UnitPrice.IsNegative() || !numeric.InRange(l.UnitPrice) || !numeric.FitsScale(l.UnitPrice, numeric.MaxScale):
			return Priced{}, bad(fmt.Sprintf("unit_price must be zero or positive with at most %d decimal places", numeric.MaxScale))
		}
		desc, err := vo.OptionalText(l.Description, 500)
		if err != nil {
			return Priced{}, bad("description is too long")
		}
		out.Lines[i] = LineData{ItemID: l.ItemID, Description: desc, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
		out.Amounts[i] = services.LineAmounts(l.Quantity, l.UnitPrice, terms.Vat, dec.Amount)
		out.Totals = out.Totals.Add(out.Amounts[i])
	}
	return out, nil
}

func itemIDs(lines []LineData) []vo.ItemID {
	seen := map[vo.ItemID]bool{}
	var out []vo.ItemID
	for _, l := range lines {
		if !seen[l.ItemID] {
			seen[l.ItemID] = true
			out = append(out, l.ItemID)
		}
	}
	return out
}

func headerText(reference, description string) (string, string, error) {
	r, err := vo.OptionalText(reference, 100)
	if err != nil {
		return "", "", err
	}
	d, err := vo.OptionalText(description, 500)
	return r, d, err
}

type Cancellation struct {
	By vo.UserID
	At time.Time
}
