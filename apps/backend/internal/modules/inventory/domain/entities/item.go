package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

type ItemData struct {
	Code         string
	Name         string
	NameEN       string
	Type         vo.ItemType
	UomID        vo.UomID
	CategoryID   *vo.CategoryID
	ReorderPoint *vo.Quantity  // จุดสั่งซื้อ (ไม่บังคับ)
	Accounts     vo.GLAccounts // บัญชีของสินค้า (ไม่ตั้ง = ใช้ของหมวด แล้วค่าเริ่มต้นของบริษัท)
	Status       vo.Status
}

// Item คือสินค้า / บริการ
//
// กฎ:
//   - เฉพาะสินค้าชนิด stock ที่ active ใช้ในเอกสารสต็อกได้
//   - มีการเคลื่อนไหวสต็อกแล้ว เปลี่ยนชนิดหรือหน่วยนับไม่ได้ (ยอดและต้นทุนเดิมจะผิดความหมาย)
//   - จุดสั่งซื้อต้องไม่ติดลบ และทศนิยมไม่เกินที่บริษัทกำหนด
type Item struct {
	id        vo.ItemID
	companyID vo.CompanyID
	data      ItemData
}

// NewItem: quantityDecimals คือทศนิยมของจำนวนสินค้าที่บริษัทกำหนด
func NewItem(companyID vo.CompanyID, d ItemData, quantityDecimals int) (*Item, error) {
	n, err := normalizeItem(d, quantityDecimals)
	if err != nil {
		return nil, err
	}
	return &Item{id: vo.NewID[vo.ItemID](), companyID: companyID, data: n}, nil
}

func RehydrateItem(id vo.ItemID, companyID vo.CompanyID, d ItemData) *Item {
	return &Item{id: id, companyID: companyID, data: d}
}

func (i *Item) ID() vo.ItemID           { return i.id }
func (i *Item) CompanyID() vo.CompanyID { return i.companyID }
func (i *Item) Data() ItemData          { return i.data }

// IsStockable: ใช้ในเอกสารสต็อกได้
func (i *Item) IsStockable() bool {
	return i.data.Type == vo.ItemStock && i.data.Status == vo.StatusActive
}

// Update: hasMovements = เคยมีการเคลื่อนไหวสต็อก
func (i *Item) Update(d ItemData, quantityDecimals int, hasMovements bool) error {
	n, err := normalizeItem(d, quantityDecimals)
	if err != nil {
		return err
	}
	if hasMovements && (n.Type != i.data.Type || n.UomID != i.data.UomID) {
		return domain.ErrItemHasStock
	}
	i.data = n
	return nil
}

func normalizeItem(d ItemData, quantityDecimals int) (ItemData, error) {
	var err error
	out := ItemData{UomID: d.UomID, CategoryID: d.CategoryID, Accounts: d.Accounts}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return ItemData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return ItemData{}, err
	}
	if out.NameEN, err = vo.OptionalText(d.NameEN, 200); err != nil {
		return ItemData{}, err
	}
	if out.Type, err = vo.ParseItemType(string(d.Type)); err != nil {
		return ItemData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return ItemData{}, err
	}
	if r := d.ReorderPoint; r != nil {
		if r.IsNegative() || !numeric.InRange(*r) || !numeric.FitsScale(*r, quantityDecimals) {
			return ItemData{}, fmt.Errorf("%w: reorder_point must be zero or positive with at most %d decimal places",
				domain.ErrInvalidQuantity, quantityDecimals)
		}
		v := *r
		out.ReorderPoint = &v
	}
	return out, nil
}
