package entities

import (
	"time"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
)

// ---------- ใบลดหนี้ผู้ขาย (ส่งคืนสินค้า / ลดราคาภายหลัง) ----------

// CreditNoteLine คือบรรทัดที่ลดหนี้ อ้างบรรทัดของใบแจ้งหนี้เดิม
//
//	ส่งคืนสินค้า stock + perpetual: Cr สินค้าคงเหลือ (InventoryAccountID) ด้วยต้นทุน FIFO ที่ตัดออก (Cost)
//	  ส่วนต่าง (ยอดก่อน VAT - ต้นทุน) = Variance ลงบัญชีผลต่างราคา (AccountID)
//	อื่นๆ (ลดราคา / ไม่นับสต็อก / periodic): Cr AccountID (ส่งคืนสินค้าและส่วนลดรับ หรือบัญชีซื้อของสินค้า) ด้วยยอดก่อน VAT
type CreditNoteLine struct {
	InvoiceLineNo      int
	ItemID             vo.ItemID
	Quantity           vo.Quantity // จำนวนที่ส่งคืน (ลดราคา = 0)
	UnitPrice          vo.Money    // ราคาต่อหน่วยที่ลด (ลดราคา = ยอดที่ลด)
	Amounts            services.Amounts
	AccountID          vo.AccountID
	Stock              bool          // ส่งคืนสินค้า stock (มีใบเบิกออกของ inventory)
	Cost               vo.Money      // ต้นทุน FIFO ที่ตัดออก (0 = ไม่มีสินค้าเคลื่อนไหว)
	InventoryAccountID *vo.AccountID // perpetual + stock เท่านั้น
	Variance           vo.Money      // perpetual + stock: ยอดก่อน VAT - ต้นทุน
}

type CreditNoteState struct {
	ID                   vo.CreditNoteID
	CompanyID            vo.CompanyID
	InvoiceID            vo.InvoiceID
	OrderID              vo.OrderID
	SupplierID           vo.SupplierID
	BillFrom             BillFrom  // ผู้ขายตามใบกำกับภาษีเดิม
	SupplierCreditNoteNo string    // เลขที่ใบลดหนี้ของผู้ขาย (ไม่ซ้ำต่อผู้ขาย)
	CreditNoteDate       time.Time // วันที่ในใบลดหนี้ของผู้ขาย
	Date                 time.Time // วันที่ลงบัญชี
	Kind                 vo.CreditNoteKind
	Reason               string // เหตุผลที่ลดหนี้ (ม.86/10 ต้องระบุ)
	Description          string
	Vat                  services.VatTerms // ตามใบแจ้งหนี้เดิม
	WarehouseID          *vo.WarehouseID
	System               vo.AccountingSystem
	StockDocumentID      *vo.StockDocumentID // ใบเบิกออกของ inventory (ส่งคืนสินค้า stock)
	StockDocumentNo      string
	Lines                []CreditNoteLine
	Totals               services.Amounts
	Status               vo.DocStatus
	Posting              Posting
	Cancellation         *Cancellation
}

// CreditNote คือใบลดหนี้ที่ได้รับจากผู้ขาย — สร้างแล้วลงบัญชีทันที (ลดเจ้าหนี้ของใบแจ้งหนี้เดิม), ยกเลิกด้วยการกลับรายการ
type CreditNote struct{ s CreditNoteState }

func NewCreditNote(s CreditNoteState) (*CreditNote, error) {
	var err error
	if !vo.IsDateOnly(s.Date) || !vo.IsDateOnly(s.CreditNoteDate) {
		return nil, domain.ErrInvalidDate
	}
	if s.SupplierCreditNoteNo, err = vo.NewName(s.SupplierCreditNoteNo, 50); err != nil {
		return nil, domain.ErrInvalidText
	}
	if s.Reason, err = vo.NewName(s.Reason, 500); err != nil {
		return nil, domain.ErrInvalidReason
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if len(s.Lines) == 0 {
		return nil, domain.ErrInvalidLine
	}
	s.Totals = services.ZeroAmounts()
	for _, l := range s.Lines {
		s.Totals = s.Totals.Add(l.Amounts)
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.CreditNoteID](), vo.DocPosted, nil
	return &CreditNote{s: s}, nil
}

func RehydrateCreditNote(s CreditNoteState) *CreditNote { return &CreditNote{s: s} }

func (c *CreditNote) ID() vo.CreditNoteID    { return c.s.ID }
func (c *CreditNote) State() CreditNoteState { return c.s }

// Journal: Dr เจ้าหนี้ (ยอดรวม VAT) — Cr ภาษีซื้อ + Cr สินค้าคงเหลือ / ผลต่างราคา / ส่งคืนสินค้า (ตามบรรทัด)
func (c *CreditNote) Journal(ap, inputVat vo.AccountID) []services.JournalLine {
	j := services.NewJournal()
	j.Debit(ap, c.s.Totals.Gross)
	j.Credit(inputVat, c.s.Totals.Vat)
	for _, l := range c.s.Lines {
		if l.InventoryAccountID != nil {
			j.Credit(*l.InventoryAccountID, l.Cost)
			j.Credit(l.AccountID, l.Variance) // ติดลบ = เดบิต
			continue
		}
		j.Credit(l.AccountID, l.Amounts.Net)
	}
	return j.Lines()
}

func (c *CreditNote) SetEntry(id vo.EntryID, no string) {
	c.s.Posting.EntryID, c.s.Posting.EntryNo = &id, no
}

func (c *CreditNote) SetStockDocument(id vo.StockDocumentID, no string) {
	c.s.StockDocumentID, c.s.StockDocumentNo = &id, no
}

func (c *CreditNote) Cancel(by vo.UserID, at time.Time) error {
	if c.s.Status != vo.DocPosted {
		return domain.ErrAlreadyCancelled
	}
	c.s.Status, c.s.Cancellation = vo.DocCancelled, &Cancellation{By: by, At: at}
	return nil
}
