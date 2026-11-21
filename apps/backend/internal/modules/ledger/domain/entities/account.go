// Package entities คือ entity ของ ledger
//
// รูปแบบร่วม: แต่ละ entity รับข้อมูลดิบผ่าน *Data struct แล้ว validate / normalize เก็บไว้
// Data() คืนข้อมูลที่ผ่านการตรวจแล้ว (ใช้ใน mapper) — Rehydrate* ใช้โดย repository เท่านั้น (ไม่ validate ซ้ำ)
package entities

import (
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

type AccountData struct {
	Code     string
	Name     string // ชื่อภาษาไทย
	NameEN   string
	Type     vo.AccountType
	ParentID *vo.AccountID
	Postable bool // true = บัญชีย่อยที่ลงรายการได้, false = บัญชีคุม (header) ใช้รวมยอด
	Status   vo.Status
	// Description คำอธิบายการใช้บัญชี (ไม่บังคับ)
	Description string
}

// AccountUsage คือข้อเท็จจริงจาก repository ที่ต้องใช้ตัดสินการแก้ / ลบบัญชี
type AccountUsage struct {
	HasChildren bool // มีบัญชีย่อย (ที่ยังไม่ถูกลบ)
	HasLines    bool // ถูกใช้ในบรรทัดรายการบัญชีแล้ว (ทุกสถานะ)
}

// Account คือบัญชีในผังบัญชี
//
// กฎ:
//   - parent ต้องเป็นบัญชีคุม (postable = false) หมวดเดียวกัน และห้ามวนลูป
//   - บัญชีที่มีบัญชีย่อยต้องเป็นบัญชีคุม และเปลี่ยนหมวดไม่ได้
//   - บัญชีที่ถูกใช้ในรายการแล้วเปลี่ยนหมวด / เปลี่ยนเป็นบัญชีคุมไม่ได้ และลบไม่ได้
//   - ลงรายการได้เฉพาะบัญชีที่ postable และ active
type Account struct {
	id        vo.AccountID
	companyID vo.CompanyID
	data      AccountData
}

// NewAccount: parent = บัญชี parent ที่โหลดมาแล้ว (nil ถ้าไม่มี)
func NewAccount(companyID vo.CompanyID, d AccountData, parent *Account) (*Account, error) {
	n, err := normalizeAccount(d)
	if err != nil {
		return nil, err
	}
	if err := checkParent(n, parent); err != nil {
		return nil, err
	}
	return &Account{id: vo.NewID[vo.AccountID](), companyID: companyID, data: n}, nil
}

func RehydrateAccount(id vo.AccountID, companyID vo.CompanyID, d AccountData) *Account {
	return &Account{id: id, companyID: companyID, data: d}
}

func (a *Account) ID() vo.AccountID        { return a.id }
func (a *Account) CompanyID() vo.CompanyID { return a.companyID }
func (a *Account) Data() AccountData       { return a.data }

// CanPost: ลงรายการในบัญชีนี้ได้หรือไม่
func (a *Account) CanPost() bool { return a.data.Postable && a.data.Status == vo.StatusActive }

// Update แทนที่ข้อมูลทั้งก้อน — parent ตรวจลูปแยกใน use case (ใช้ recursive query)
func (a *Account) Update(d AccountData, parent *Account, usage AccountUsage) error {
	n, err := normalizeAccount(d)
	if err != nil {
		return err
	}
	if err := checkParent(n, parent); err != nil {
		return err
	}
	typeChanged := n.Type != a.data.Type
	if usage.HasChildren && (typeChanged || n.Postable) {
		return domain.ErrAccountHasChildren
	}
	if usage.HasLines && (typeChanged || !n.Postable) {
		return domain.ErrInUse
	}
	a.data = n
	return nil
}

func (a *Account) EnsureDeletable(usage AccountUsage) error {
	if usage.HasChildren || usage.HasLines {
		return domain.ErrInUse
	}
	return nil
}

func checkParent(d AccountData, parent *Account) error {
	if d.ParentID == nil {
		return nil
	}
	if parent == nil || parent.id != *d.ParentID {
		return domain.ErrAccountNotFound
	}
	if parent.data.Postable {
		return domain.ErrParentNotHeader
	}
	if parent.data.Type != d.Type {
		return domain.ErrParentTypeMismatch
	}
	return nil
}

func normalizeAccount(d AccountData) (AccountData, error) {
	var err error
	out := AccountData{ParentID: d.ParentID, Postable: d.Postable}
	if out.Description, err = vo.OptionalText(d.Description, 500); err != nil {
		return AccountData{}, err
	}
	if out.Code, err = vo.NewAccountCode(d.Code); err != nil {
		return AccountData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return AccountData{}, err
	}
	if out.NameEN, err = vo.OptionalText(d.NameEN, 200); err != nil {
		return AccountData{}, err
	}
	if out.Type, err = vo.ParseAccountType(string(d.Type)); err != nil {
		return AccountData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return AccountData{}, err
	}
	return out, nil
}
