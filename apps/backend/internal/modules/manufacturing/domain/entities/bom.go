// Package entities คือ entity ของ manufacturing: สูตรการผลิต (BOM), ใบสั่งผลิต และค่าตั้ง
package entities

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/manufacturing/domain/errors"
	vo "apexis/backend/internal/modules/manufacturing/domain/value_objects"
)

const (
	maxNameLength        = 200
	maxDescriptionLength = 500
	maxBomLines          = 200
)

var hundred = decimal.NewFromInt(100)

// ComponentData คือวัตถุดิบหนึ่งบรรทัดของสูตร: ปริมาณต่อชุด (OutputQuantity ของสูตร) + เปอร์เซ็นต์สูญเสีย
type ComponentData struct {
	ItemID       vo.ItemID
	Quantity     vo.Quantity
	ScrapPercent decimal.Decimal
}

// Component คือวัตถุดิบของสูตรที่ผ่านการตรวจแล้ว (LineNo เริ่มที่ 1)
type Component struct {
	LineNo int
	ComponentData
}

// ConversionData คือค่าแรง / ค่าใช้จ่ายการผลิตมาตรฐานต่อชุด
// AccountID = บัญชีที่เครดิตตอนปันเข้างาน (nil = ใช้บัญชีปันเข้างานในค่าตั้ง)
type ConversionData struct {
	Description string
	AccountID   *vo.AccountID
	Amount      vo.Money
}

type Conversion struct {
	LineNo int
	ConversionData
}

// BomData คือข้อมูลสูตรการผลิต: ผลิตสินค้า ItemID ได้ OutputQuantity ต่อชุด
type BomData struct {
	Code           string
	Name           string
	ItemID         vo.ItemID
	OutputQuantity vo.Quantity
	Status         vo.Status
	IsDefault      bool
	Components     []ComponentData
	Conversions    []ConversionData
}

// Bom คือสูตรการผลิต
//
// กฎ:
//   - รหัสไม่ซ้ำในบริษัท; สินค้าหนึ่งมีสูตรหลัก (is_default) ที่ใช้งานอยู่ได้ไม่เกิน 1 สูตร (unique index)
//   - มีวัตถุดิบอย่างน้อย 1 รายการ, ไม่ซ้ำกัน และไม่ใช่สินค้าที่ผลิตเอง
//   - ปริมาณ > 0 ตามทศนิยมจำนวนของบริษัท; %สูญเสีย 0 ถึงน้อยกว่า 100 (ทศนิยม 2 ตำแหน่ง)
//   - ค่าแรง / ค่าใช้จ่ายการผลิตต่อชุด >= 0 ตามทศนิยมเงินของบริษัท
//   - สินค้าทุกตัวต้องเป็นชนิด stock และใช้งานอยู่ (ตรวจกับ inventory ที่ use case)
//   - แก้สูตรไม่กระทบใบสั่งผลิตที่สร้างแล้ว (ใบสั่งผลิตเก็บสำเนาสูตร ณ ตอนสร้าง)
type Bom struct {
	id          vo.BomID
	companyID   vo.CompanyID
	data        BomData
	components  []Component
	conversions []Conversion
	createdAt   time.Time
	updatedAt   time.Time
}

// Decimals คือจำนวนทศนิยมของบริษัท (จำนวนสินค้า / เงิน)
type Decimals struct {
	Quantity int
	Amount   int
}

func NewBom(companyID vo.CompanyID, d BomData, dec Decimals) (*Bom, error) {
	b := &Bom{id: vo.NewID[vo.BomID](), companyID: companyID}
	if err := b.Update(d, dec); err != nil {
		return nil, err
	}
	return b, nil
}

func RehydrateBom(id vo.BomID, companyID vo.CompanyID, d BomData, components []Component, conversions []Conversion, createdAt, updatedAt time.Time) *Bom {
	return &Bom{id: id, companyID: companyID, data: d, components: components, conversions: conversions, createdAt: createdAt, updatedAt: updatedAt}
}

// Touch บันทึกเวลาที่สร้าง (ครั้งแรก) / แก้ไขล่าสุด
func (b *Bom) Touch(now time.Time) {
	if b.createdAt.IsZero() {
		b.createdAt = now
	}
	b.updatedAt = now
}

func (b *Bom) Times() (createdAt, updatedAt time.Time) { return b.createdAt, b.updatedAt }

func (b *Bom) ID() vo.BomID              { return b.id }
func (b *Bom) CompanyID() vo.CompanyID   { return b.companyID }
func (b *Bom) Data() BomData             { return b.data }
func (b *Bom) Components() []Component   { return b.components }
func (b *Bom) Conversions() []Conversion { return b.conversions }
func (b *Bom) IsActive() bool            { return b.data.Status == vo.StatusActive }
func (b *Bom) ItemIDs() []vo.ItemID {
	return append([]vo.ItemID{b.data.ItemID}, b.ComponentItemIDs()...)
}
func (b *Bom) ComponentItemIDs() []vo.ItemID {
	out := make([]vo.ItemID, len(b.components))
	for i, c := range b.components {
		out[i] = c.ItemID
	}
	return out
}

// AccountIDs คือบัญชีที่ระบุในบรรทัดค่าแรง / ค่าใช้จ่ายการผลิต (ไม่ซ้ำ)
func (b *Bom) AccountIDs() []vo.AccountID {
	var out []vo.AccountID
	seen := map[vo.AccountID]bool{}
	for _, c := range b.conversions {
		if c.AccountID != nil && !seen[*c.AccountID] {
			seen[*c.AccountID] = true
			out = append(out, *c.AccountID)
		}
	}
	return out
}

// Update แทนที่ข้อมูลทั้งก้อน (ตรวจตามกฎทั้งหมด)
func (b *Bom) Update(d BomData, dec Decimals) error {
	var err error
	if d.Code, err = vo.NewCode(d.Code); err != nil {
		return err
	}
	if d.Name, err = vo.NewName(d.Name, maxNameLength); err != nil {
		return err
	}
	if d.Status == "" {
		d.Status = vo.StatusActive
	}
	if _, err = vo.ParseStatus(string(d.Status)); err != nil {
		return err
	}
	if !d.OutputQuantity.IsPositive() || !vo.FitsCompany(d.OutputQuantity, dec.Quantity) {
		return fmt.Errorf("%w: output_quantity must be greater than 0 with at most %d decimal places", domain.ErrInvalidQuantity, dec.Quantity)
	}
	if len(d.Components) == 0 || len(d.Components) > maxBomLines {
		return fmt.Errorf("%w: components must have 1 to %d lines", domain.ErrInvalidLine, maxBomLines)
	}
	if len(d.Conversions) > maxBomLines {
		return fmt.Errorf("%w: conversion_costs must have at most %d lines", domain.ErrInvalidLine, maxBomLines)
	}
	components := make([]Component, len(d.Components))
	seen := map[vo.ItemID]bool{}
	for i, c := range d.Components {
		n := i + 1
		switch {
		case c.ItemID == d.ItemID:
			return fmt.Errorf("%w: component %d is the produced item", domain.ErrInvalidLine, n)
		case seen[c.ItemID]:
			return fmt.Errorf("%w: component %d repeats an item", domain.ErrInvalidLine, n)
		case !c.Quantity.IsPositive() || !vo.FitsCompany(c.Quantity, dec.Quantity):
			return fmt.Errorf("%w: component %d quantity must be greater than 0 with at most %d decimal places", domain.ErrInvalidLine, n, dec.Quantity)
		case c.ScrapPercent.IsNegative() || !c.ScrapPercent.LessThan(hundred) || !vo.FitsCompany(c.ScrapPercent, 2):
			return fmt.Errorf("%w: component %d scrap_percent must be 0 to less than 100 with at most 2 decimal places", domain.ErrInvalidLine, n)
		}
		seen[c.ItemID] = true
		components[i] = Component{LineNo: n, ComponentData: c}
	}
	conversions := make([]Conversion, len(d.Conversions))
	for i, c := range d.Conversions {
		n := i + 1
		desc, err := vo.NewName(c.Description, maxNameLength)
		if err != nil {
			return fmt.Errorf("%w: conversion %d description is required (up to %d characters)", domain.ErrInvalidLine, n, maxNameLength)
		}
		if c.Amount.IsNegative() || !vo.FitsCompany(c.Amount, dec.Amount) {
			return fmt.Errorf("%w: conversion %d amount must be 0 or more with at most %d decimal places", domain.ErrInvalidLine, n, dec.Amount)
		}
		c.Description = desc
		conversions[i] = Conversion{LineNo: n, ConversionData: c}
	}
	d.Components, d.Conversions = nil, nil
	b.data, b.components, b.conversions = d, components, conversions
	return nil
}
