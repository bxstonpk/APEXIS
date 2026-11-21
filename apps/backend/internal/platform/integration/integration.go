// Package integration คือ "สัญญากลาง" ของการเรียกข้าม module (shared kernel)
//
// public API ของ module ใช้ uuid / primitive / type ในแพ็กเกจนี้เท่านั้น
// เพื่อไม่ให้ module หนึ่ง import type ภายในของอีก module
//
//   - ledger.Module: ลงบัญชีจาก module อื่น (JournalLine), กลับรายการ, ตรวจบัญชี, อ่านยอด
//   - inventory.Module: ข้อมูลสินค้าสำหรับจัดซื้อ (Item), ออกใบรับเข้า (ReceiptLine / Receipt), ยกเลิก
package integration

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ErrRejected: module ปลายทางปฏิเสธคำขอด้วยกฎธุรกิจ (เช่นงวดปิด, สต็อกไม่พอ)
// ถูก wrap ด้วยข้อความจาก module ปลายทาง — ผู้เรียกแปลงเป็น HTTP 422 พร้อมรายละเอียดได้
var ErrRejected = errors.New("rejected")

// Reject ห่อ error กฎธุรกิจของ module ปลายทางเป็น ErrRejected (คงข้อความเดิมไว้)
func Reject(module string, err error) error {
	return fmt.Errorf("%w by %s: %v", ErrRejected, module, err)
}

// ---------- ledger ----------

// JournalLine คือบรรทัดรายการบัญชีที่ module อื่นส่งให้ ledger ลงบัญชี (เดบิตหรือเครดิตด้านเดียว)
type JournalLine struct {
	AccountID   uuid.UUID
	Debit       decimal.Decimal
	Credit      decimal.Decimal
	Description string
}

// PostedEntry คือผลของการลงบัญชี
type PostedEntry struct {
	EntryID uuid.UUID
	EntryNo string
}

// ---------- inventory ----------

// AccountingSystem คือระบบบัญชีสินค้าของบริษัท (ตั้งต่อปีบัญชี)
type AccountingSystem string

const (
	Perpetual AccountingSystem = "perpetual" // บันทึกสินค้าคงเหลือ / ต้นทุนขายทันที
	Periodic  AccountingSystem = "periodic"  // ลงบัญชีซื้อระหว่างงวด ปรับปรุงสินค้าคงเหลือสิ้นงวด
)

// Item คือข้อมูลสินค้าที่จัดซื้อใช้ — บัญชีถูก resolve แล้ว (สินค้า -> หมวด -> ค่าเริ่มต้นของบริษัท; uuid.Nil = ไม่ได้ตั้ง)
type Item struct {
	ID                 uuid.UUID
	Code               string
	Name               string
	Stock              bool // ชนิด stock (นับสต็อก / มีใบรับเข้า)
	Active             bool
	PurchaseAccountID  uuid.UUID
	InventoryAccountID uuid.UUID
}

// PurchaseContext คือข้อมูลที่จัดซื้อต้องใช้ ณ วันที่เอกสาร
type PurchaseContext struct {
	System AccountingSystem
	Items  map[uuid.UUID]Item
}

// ReceiptLine คือบรรทัดใบรับเข้าที่จัดซื้อส่งให้ inventory
type ReceiptLine struct {
	ItemID   uuid.UUID
	Quantity decimal.Decimal
	UnitCost decimal.Decimal // ราคาก่อน VAT ต่อหน่วย
}

// Receipt คือผลของการออกใบรับเข้า — LineValues คือมูลค่าที่ inventory บันทึกจริง (เรียงตามบรรทัด)
type Receipt struct {
	DocumentID uuid.UUID
	DocumentNo string
	LineValues []decimal.Decimal
}
