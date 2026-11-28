// Package entities คือ entity ของ organization
//
// รูปแบบร่วม: แต่ละ entity รับข้อมูลดิบผ่าน *Data struct แล้ว validate / normalize เก็บไว้
// Data() คืนข้อมูลที่ผ่านการตรวจแล้ว (ใช้ใน mapper) — Rehydrate* ใช้โดย repository เท่านั้น (ไม่ validate ซ้ำ)
package entities

import (
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// CompanyData คือข้อมูลบริษัท (นิติบุคคล) ซึ่งเป็น tenant ของระบบ
type CompanyData struct {
	Code                 string // แก้ไม่ได้หลังสร้าง
	Name                 string // ชื่อที่ใช้แสดง
	TaxID                string // เลขประจำตัวผู้เสียภาษี (ไม่บังคับ)
	LegalNameTH          string // ชื่อตามหนังสือรับรอง — ไม่ระบุ = ใช้ Name
	LegalNameEN          string
	Address              vo.Address // ที่อยู่จดทะเบียน
	Phone                string
	Email                string
	Website              string
	BaseCurrency         string
	FiscalYearStartMonth int             // 1 = มกราคม ... 10 = ตุลาคม (ปีงบประมาณราชการ)
	AmountDecimals       int             // ทศนิยมของจำนวนเงิน (0-6) — ทุก module ใช้ค่านี้ ไม่ fix ไว้ในโค้ด
	QuantityDecimals     int             // ทศนิยมของจำนวนสินค้า (0-6)
	BusinessType         vo.BusinessType // ประเภทธุรกิจ — แก้ไม่ได้หลังสร้าง (ผังบัญชีมาตรฐานสร้างตามประเภทนี้)
	Status               vo.Status
}

// Company คือบริษัท — ข้อมูลธุรกิจทุกตัวของ ERP ผูกกับ CompanyID
//
// กฎ:
//   - รหัสบริษัทไม่ซ้ำทั้งระบบ (unique constraint) และแก้ไม่ได้หลังสร้าง
//   - ประเภทธุรกิจ (บริการ / ซื้อขาย / ผลิต / ก่อสร้าง) ต้องระบุตอนสร้างและแก้ไม่ได้ — ผังบัญชีและค่าตั้งเริ่มต้นสร้างตามประเภทนี้
//   - บริษัทที่ inactive เป็น "อ่านอย่างเดียว": อ่านข้อมูลได้ แต่สร้าง / แก้ / ลบข้อมูลของบริษัทไม่ได้
//     ยกเว้นแก้ข้อมูลบริษัทเอง (เพื่อเปิดใช้งานกลับ)
//   - จำนวนทศนิยม (เงิน / จำนวนสินค้า) เพิ่มได้แต่ลดไม่ได้ — ข้อมูลเดิมที่มีทศนิยมมากกว่าจะผิดรูป
type Company struct {
	id   vo.CompanyID
	data CompanyData
}

func NewCompany(d CompanyData) (*Company, error) {
	n, err := normalizeCompany(d)
	if err != nil {
		return nil, err
	}
	return &Company{id: vo.NewID[vo.CompanyID](), data: n}, nil
}

func RehydrateCompany(id vo.CompanyID, d CompanyData) *Company {
	return &Company{id: id, data: d}
}

func (c *Company) ID() vo.CompanyID  { return c.id }
func (c *Company) Data() CompanyData { return c.data }
func (c *Company) IsActive() bool    { return c.data.Status == vo.StatusActive }

// Update แทนที่ข้อมูลทั้งก้อน ยกเว้นรหัสบริษัท (คงค่าเดิมเสมอ)
func (c *Company) Update(d CompanyData) error {
	d.Code = c.data.Code
	d.BusinessType = c.data.BusinessType
	n, err := normalizeCompany(d)
	if err != nil {
		return err
	}
	if n.AmountDecimals < c.data.AmountDecimals || n.QuantityDecimals < c.data.QuantityDecimals {
		return domain.ErrDecimalsDecrease
	}
	c.data = n
	return nil
}

func normalizeCompany(d CompanyData) (CompanyData, error) {
	code, err := vo.NewCode(d.Code)
	if err != nil {
		return CompanyData{}, err
	}
	out := CompanyData{Code: code.String(), FiscalYearStartMonth: d.FiscalYearStartMonth}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return CompanyData{}, err
	}
	if out.TaxID, err = vo.NewTaxID(d.TaxID); err != nil {
		return CompanyData{}, err
	}
	legalTH := d.LegalNameTH
	if legalTH == "" {
		legalTH = out.Name
	}
	if out.LegalNameTH, err = vo.NewName(legalTH, 300); err != nil {
		return CompanyData{}, err
	}
	if out.LegalNameEN, err = vo.OptionalText(d.LegalNameEN, 300, domain.ErrInvalidName); err != nil {
		return CompanyData{}, err
	}
	if out.Address, err = vo.NewAddress(d.Address); err != nil {
		return CompanyData{}, err
	}
	if out.Phone, err = vo.NewPhone(d.Phone); err != nil {
		return CompanyData{}, err
	}
	if out.Email, err = vo.NewEmail(d.Email); err != nil {
		return CompanyData{}, err
	}
	if out.Website, err = vo.NewWebsite(d.Website); err != nil {
		return CompanyData{}, err
	}
	currency, err := vo.NewCurrency(d.BaseCurrency)
	if err != nil {
		return CompanyData{}, err
	}
	out.BaseCurrency = string(currency)
	if out.FiscalYearStartMonth < 1 || out.FiscalYearStartMonth > 12 {
		return CompanyData{}, domain.ErrInvalidFiscalMonth
	}
	if out.AmountDecimals, err = vo.NewDecimals(d.AmountDecimals); err != nil {
		return CompanyData{}, err
	}
	if out.QuantityDecimals, err = vo.NewDecimals(d.QuantityDecimals); err != nil {
		return CompanyData{}, err
	}
	if out.BusinessType, err = vo.ParseBusinessType(string(d.BusinessType)); err != nil {
		return CompanyData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return CompanyData{}, err
	}
	return out, nil
}
