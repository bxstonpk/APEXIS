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
	"time"

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
	RevenueAccountID   uuid.UUID // uuid.Nil = ไม่ได้ตั้งที่สินค้า / หมวด (ใช้ค่าเริ่มต้นของค่าตั้งขาย)
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

// IssueLine คือบรรทัดใบเบิก (ตัดสต็อก FIFO) ที่ module ขายส่งให้ inventory
type IssueLine struct {
	ItemID   uuid.UUID
	Quantity decimal.Decimal
}

// Receipt คือผลของการออกเอกสารสต็อก (ใบรับเข้า / ใบเบิก) — LineValues คือมูลค่าที่ inventory บันทึกจริง
// (ใบเบิก = ต้นทุน FIFO ของแต่ละบรรทัด)
type Receipt struct {
	DocumentID uuid.UUID
	DocumentNo string
	LineValues []decimal.Decimal
}

// ---------- เลขที่เอกสาร ----------

// DocumentType คือชนิดเอกสารที่ module ลงทะเบียนกับ organization เพื่อให้บริษัทตั้งรูปแบบเลขที่ได้
//
//	Key: "<module>.<ชนิด>" เช่น "purchasing.payment" — ไม่เปลี่ยนหลังใช้งานจริง
type DocumentType struct {
	Key           string
	Name          string
	DefaultPrefix string
}

// ---------- การตัดยอดใบแจ้งหนี้ (finance -> sales / purchasing) ----------
//
// finance เป็นเจ้าของเอกสารรับ / จ่ายเงิน ส่วนใบแจ้งหนี้และยอดค้างเป็นของ sales / purchasing:
// finance ขอตัดยอดผ่าน public API ของ module นั้นใน transaction เดียวกับใบรับ / จ่าย
// module ปลายทางเก็บประวัติการตัดยอด (วันที่ + เอกสารการเงิน) ไว้เอง — รายงานอายุหนี้จึงไม่ต้อง JOIN ข้าม module

// Settlement คือยอดที่ตัดจากใบแจ้งหนี้หนึ่งใบ (รวม VAT, ก่อนหักภาษี ณ ที่จ่าย)
type Settlement struct {
	InvoiceID uuid.UUID
	Amount    decimal.Decimal
}

// SettlementDoc คือเอกสารของ finance ที่ตัดยอด (ใช้ยกเลิกทั้งชุดด้วย DocumentID)
type SettlementDoc struct {
	DocumentID uuid.UUID
	DocumentNo string
	Date       time.Time
}

// Counterparty คือข้อมูลคู่ค้า ณ วันเอกสาร (finance เก็บติดเอกสารไว้ใช้ทำรายงานภาษี)
type Counterparty struct {
	ID       uuid.UUID
	Code     string
	Name     string
	TaxID    string
	BranchNo string
	Address  string
	TaxType  string // individual / juristic (ผู้ขาย — กำหนดแบบ ภ.ง.ด.3 / 53); ลูกค้า = ""
}

// SettledInvoice คือข้อมูลใบแจ้งหนี้ที่ตัดยอดแล้ว — Net / Gross ใช้คำนวณฐานภาษีหัก ณ ที่จ่ายตามสัดส่วน
type SettledInvoice struct {
	InvoiceID  uuid.UUID
	DocumentNo string
	PartnerRef string // เลขที่ใบแจ้งหนี้ของผู้ขาย (จัดซื้อ)
	Net        decimal.Decimal
	Gross      decimal.Decimal
}

// SettlementResult คือผลการตัดยอด: คู่ค้า + บัญชีคุม (ลูกหนี้ / เจ้าหนี้) ที่ต้องลงบัญชี + ใบแจ้งหนี้ตามลำดับที่ส่งมา
type SettlementResult struct {
	Party            Counterparty
	ControlAccountID uuid.UUID
	Invoices         []SettledInvoice
}

// OpenInvoice คือใบแจ้งหนี้ที่ยังค้างชำระ (ให้หน้ารับ / จ่ายเงินเลือก)
type OpenInvoice struct {
	ID          uuid.UUID
	DocumentNo  string
	PartnerRef  string
	Date        time.Time
	DueDate     time.Time
	Net         decimal.Decimal // ยอดก่อน VAT (ใช้คำนวณฐานภาษีหัก ณ ที่จ่าย)
	Gross       decimal.Decimal
	Outstanding decimal.Decimal
}
