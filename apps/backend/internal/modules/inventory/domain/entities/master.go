// Package entities คือ entity ของ inventory
//
// รูปแบบร่วม: แต่ละ entity รับข้อมูลดิบผ่าน *Data struct แล้ว validate / normalize เก็บไว้
// Data() คืนข้อมูลที่ผ่านการตรวจแล้ว (ใช้ใน mapper) — Rehydrate* ใช้โดย repository เท่านั้น (ไม่ validate ซ้ำ)
package entities

import (
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
)

// MasterData คือข้อมูลของ master แบบง่าย (หน่วยนับ / หมวดสินค้า): รหัส + ชื่อ + สถานะ
type MasterData struct {
	Code     string
	Name     string
	Status   vo.Status
	Accounts vo.GLAccounts // เฉพาะหมวดสินค้า: บัญชีของสินค้าในหมวด (ใช้เมื่อสินค้าไม่ได้ตั้งเอง)
}

func normalizeMaster(d MasterData) (MasterData, error) {
	var err error
	out := MasterData{Accounts: d.Accounts}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return MasterData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return MasterData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return MasterData{}, err
	}
	return out, nil
}

// Uom คือหน่วยนับ (ชิ้น, กล่อง, กก.) — จำนวนทศนิยมของจำนวนสินค้ากำหนดที่บริษัท ไม่ใช่ที่หน่วยนับ
type Uom struct {
	id        vo.UomID
	companyID vo.CompanyID
	data      MasterData
}

func NewUom(companyID vo.CompanyID, d MasterData) (*Uom, error) {
	n, err := normalizeMaster(d)
	if err != nil {
		return nil, err
	}
	return &Uom{id: vo.NewID[vo.UomID](), companyID: companyID, data: n}, nil
}

func RehydrateUom(id vo.UomID, companyID vo.CompanyID, d MasterData) *Uom {
	return &Uom{id: id, companyID: companyID, data: d}
}

func (u *Uom) ID() vo.UomID            { return u.id }
func (u *Uom) CompanyID() vo.CompanyID { return u.companyID }
func (u *Uom) Data() MasterData        { return u.data }

func (u *Uom) Update(d MasterData) error {
	n, err := normalizeMaster(d)
	if err != nil {
		return err
	}
	u.data = n
	return nil
}

// Category คือหมวดสินค้า (ใช้จัดกลุ่ม / กรองรายงาน)
type Category struct {
	id        vo.CategoryID
	companyID vo.CompanyID
	data      MasterData
}

func NewCategory(companyID vo.CompanyID, d MasterData) (*Category, error) {
	n, err := normalizeMaster(d)
	if err != nil {
		return nil, err
	}
	return &Category{id: vo.NewID[vo.CategoryID](), companyID: companyID, data: n}, nil
}

func RehydrateCategory(id vo.CategoryID, companyID vo.CompanyID, d MasterData) *Category {
	return &Category{id: id, companyID: companyID, data: d}
}

func (c *Category) ID() vo.CategoryID       { return c.id }
func (c *Category) CompanyID() vo.CompanyID { return c.companyID }
func (c *Category) Data() MasterData        { return c.data }

func (c *Category) Update(d MasterData) error {
	n, err := normalizeMaster(d)
	if err != nil {
		return err
	}
	c.data = n
	return nil
}

// ---------- Warehouse ----------

type WarehouseData struct {
	Code     string
	Name     string
	BranchID *vo.BranchID // สาขาของ organization (ไม่บังคับ)
	Status   vo.Status
}

// Warehouse คือคลังสินค้า — ยอดคงเหลือและต้นทุน FIFO แยกตามคลัง
type Warehouse struct {
	id        vo.WarehouseID
	companyID vo.CompanyID
	data      WarehouseData
}

func NewWarehouse(companyID vo.CompanyID, d WarehouseData) (*Warehouse, error) {
	n, err := normalizeWarehouse(d)
	if err != nil {
		return nil, err
	}
	return &Warehouse{id: vo.NewID[vo.WarehouseID](), companyID: companyID, data: n}, nil
}

func RehydrateWarehouse(id vo.WarehouseID, companyID vo.CompanyID, d WarehouseData) *Warehouse {
	return &Warehouse{id: id, companyID: companyID, data: d}
}

func (w *Warehouse) ID() vo.WarehouseID      { return w.id }
func (w *Warehouse) CompanyID() vo.CompanyID { return w.companyID }
func (w *Warehouse) Data() WarehouseData     { return w.data }
func (w *Warehouse) IsActive() bool          { return w.data.Status == vo.StatusActive }

func (w *Warehouse) Update(d WarehouseData) error {
	n, err := normalizeWarehouse(d)
	if err != nil {
		return err
	}
	w.data = n
	return nil
}

func normalizeWarehouse(d WarehouseData) (WarehouseData, error) {
	m, err := normalizeMaster(MasterData{Code: d.Code, Name: d.Name, Status: d.Status})
	if err != nil {
		return WarehouseData{}, err
	}
	return WarehouseData{Code: m.Code, Name: m.Name, BranchID: d.BranchID, Status: m.Status}, nil
}

// EnsureUnused: ลบหน่วยนับ / หมวด / คลัง ไม่ได้ถ้ายังถูกใช้อยู่
func EnsureUnused(used bool) error {
	if used {
		return domain.ErrInUse
	}
	return nil
}
