// Package entities คือ entity ของ purchasing
//
// รูปแบบร่วม: รับข้อมูลดิบผ่าน *Data struct แล้ว validate / normalize เก็บไว้
// Data() / State() คืนข้อมูลที่ผ่านการตรวจแล้ว — Rehydrate* ใช้โดย repository เท่านั้น
package entities

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// ---------- ค่าตั้งจัดซื้อ ----------

// DefaultVatRate คืออัตรา VAT เริ่มต้น (%) เมื่อบริษัทยังไม่ได้ตั้ง
var DefaultVatRate = decimal.NewFromInt(7)

// Settings คือค่าตั้งจัดซื้อของบริษัท: อัตรา VAT และบัญชีที่ใช้ลงรายการ (nil = ไม่ได้ตั้ง)
type Settings struct {
	VatRate                decimal.Decimal
	JournalID              *vo.JournalID
	GRIRAccountID          *vo.AccountID // รับสินค้ายังไม่ได้รับใบแจ้งหนี้
	APAccountID            *vo.AccountID // เจ้าหนี้การค้า
	InputVatAccountID      *vo.AccountID // ภาษีซื้อ
	PriceVarianceAccountID *vo.AccountID // ผลต่างราคา (perpetual: ราคาใบแจ้งหนี้ต่างจากราคารับสินค้า)
}

func DefaultSettings() Settings { return Settings{VatRate: DefaultVatRate} }

func NewSettings(s Settings) (Settings, error) {
	if s.VatRate.IsNegative() || s.VatRate.GreaterThan(decimal.NewFromInt(100)) || !numeric.FitsScale(s.VatRate, 2) {
		return Settings{}, domain.ErrInvalidVatRate
	}
	return s, nil
}

// AccountIDs คือบัญชีทั้งหมดที่ตั้งไว้ (ใช้ตรวจกับ ledger)
func (s Settings) AccountIDs() []vo.AccountID {
	var out []vo.AccountID
	for _, p := range []*vo.AccountID{s.GRIRAccountID, s.APAccountID, s.InputVatAccountID, s.PriceVarianceAccountID} {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// Require: ค่าตั้งที่ต้องมีก่อนลงบัญชี — คืน ErrSettingsMissing พร้อมชื่อที่ขาด
func (s Settings) Require(fields ...string) error {
	set := map[string]bool{
		"journal_id": s.JournalID != nil, "grir_account_id": s.GRIRAccountID != nil, "ap_account_id": s.APAccountID != nil,
		"input_vat_account_id": s.InputVatAccountID != nil, "price_variance_account_id": s.PriceVarianceAccountID != nil,
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

// ---------- ผู้ขาย ----------

type SupplierData struct {
	Code            string
	Name            string
	TaxID           string
	BranchNo        string // เลขสาขาตามใบกำกับภาษี (00000 = สำนักงานใหญ่)
	Address         string
	PaymentTermDays int
	VatRegistered   bool // จด VAT: ใบสั่งซื้อที่สร้างหลังจากนี้คิด VAT ตามอัตราที่ตั้ง
	Status          vo.Status
}

// Supplier คือผู้ขาย — แก้สถานะจด VAT ได้ มีผลกับใบสั่งซื้อที่สร้างหลังจากแก้เท่านั้น
type Supplier struct {
	id        vo.SupplierID
	companyID vo.CompanyID
	data      SupplierData
}

func NewSupplier(companyID vo.CompanyID, d SupplierData) (*Supplier, error) {
	n, err := normalizeSupplier(d)
	if err != nil {
		return nil, err
	}
	return &Supplier{id: vo.NewID[vo.SupplierID](), companyID: companyID, data: n}, nil
}

func RehydrateSupplier(id vo.SupplierID, companyID vo.CompanyID, d SupplierData) *Supplier {
	return &Supplier{id: id, companyID: companyID, data: d}
}

func (s *Supplier) ID() vo.SupplierID       { return s.id }
func (s *Supplier) CompanyID() vo.CompanyID { return s.companyID }
func (s *Supplier) Data() SupplierData      { return s.data }
func (s *Supplier) IsActive() bool          { return s.data.Status == vo.StatusActive }

func (s *Supplier) Update(d SupplierData) error {
	n, err := normalizeSupplier(d)
	if err != nil {
		return err
	}
	s.data = n
	return nil
}

// EnsureActive: ใช้กับเอกสารใหม่ได้
func (s *Supplier) EnsureActive() error {
	if !s.IsActive() {
		return domain.ErrSupplierInactive
	}
	return nil
}

func normalizeSupplier(d SupplierData) (SupplierData, error) {
	var err error
	out := SupplierData{VatRegistered: d.VatRegistered}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return SupplierData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return SupplierData{}, err
	}
	if out.TaxID, err = vo.TaxID(d.TaxID); err != nil {
		return SupplierData{}, err
	}
	if out.BranchNo, err = vo.BranchNo(d.BranchNo); err != nil {
		return SupplierData{}, err
	}
	if out.Address, err = vo.OptionalText(d.Address, 500); err != nil {
		return SupplierData{}, err
	}
	if d.PaymentTermDays < 0 || d.PaymentTermDays > 365 {
		return SupplierData{}, domain.ErrInvalidTerm
	}
	out.PaymentTermDays = d.PaymentTermDays
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return SupplierData{}, err
	}
	return out, nil
}

// EnsureUnused: ลบไม่ได้ถ้ายังถูกใช้อยู่
func EnsureUnused(used bool) error {
	if used {
		return domain.ErrInUse
	}
	return nil
}
