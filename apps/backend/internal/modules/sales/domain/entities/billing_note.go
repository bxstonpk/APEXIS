package entities

import (
	"fmt"
	"slices"
	"time"

	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
)

// ---------- ใบวางบิล ----------

// BillingNoteState: ใบวางบิลรวมใบแจ้งหนี้ (ขายสินค้า / ขายบริการ) ที่ค้างของลูกค้ารายเดียว เพื่อนัดรับชำระ
// ไม่มีผลทางบัญชี / ภาษี — "ชำระครบ" คำนวณจากยอดค้างของใบแจ้งหนี้ (ไม่เก็บสถานะ paid)
type BillingNoteState struct {
	ID           vo.BillingNoteID
	CompanyID    vo.CompanyID
	DocumentNo   string
	CustomerID   vo.CustomerID
	Date         time.Time
	DueDate      time.Time
	Description  string
	InvoiceIDs   []vo.InvoiceID
	Status       vo.BillingNoteStatus // open / cancelled
	By           vo.UserID
	At           time.Time
	Cancellation *Cancellation
}

type BillingNote struct{ s BillingNoteState }

// BillableInvoice คือข้อมูลใบแจ้งหนี้ที่ใช้ตรวจตอนวางบิล
type BillableInvoice struct {
	ID          vo.InvoiceID
	DocumentNo  string
	CustomerID  vo.CustomerID
	Kind        vo.InvoiceKind
	Status      vo.DocStatus
	Outstanding vo.Money
	OnOpenNote  bool // อยู่ในใบวางบิลอื่นที่ยังไม่ยกเลิก
}

// NewBillingNote: วันครบกำหนดไม่ก่อนวันที่, อย่างน้อย 1 ใบ, ไม่ซ้ำ; ใบแจ้งหนี้ต้องเป็นขายสินค้า / ขายบริการของลูกค้านี้
// ที่ยัง posted มียอดค้าง และไม่อยู่ในใบวางบิลอื่นที่ยังเปิด
func NewBillingNote(s BillingNoteState, invoices map[vo.InvoiceID]BillableInvoice) (*BillingNote, error) {
	var err error
	if !vo.IsDateOnly(s.Date) || !vo.IsDateOnly(s.DueDate) || s.DueDate.Before(s.Date) {
		return nil, domain.ErrInvalidDate
	}
	if s.Description, err = vo.OptionalText(s.Description, 500); err != nil {
		return nil, err
	}
	if len(s.InvoiceIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one invoice", domain.ErrInvalidLine)
	}
	seen := make([]vo.InvoiceID, 0, len(s.InvoiceIDs))
	for n, id := range s.InvoiceIDs {
		if slices.Contains(seen, id) {
			return nil, fmt.Errorf("%w: invoice %d is duplicated", domain.ErrInvalidLine, n+1)
		}
		seen = append(seen, id)
		inv, ok := invoices[id]
		if !ok {
			return nil, fmt.Errorf("%w: invoice %d", domain.ErrInvoiceNotFound, n+1)
		}
		switch {
		case inv.CustomerID != s.CustomerID:
			return nil, fmt.Errorf("%w: %s belongs to another customer", domain.ErrBillingInvoice, inv.DocumentNo)
		case inv.Kind != vo.KindInvoice && inv.Kind != vo.KindServiceInvoice:
			return nil, fmt.Errorf("%w: %s is not a goods or service invoice", domain.ErrBillingInvoice, inv.DocumentNo)
		case inv.Status != vo.DocPosted || !inv.Outstanding.IsPositive():
			return nil, fmt.Errorf("%w: %s has no outstanding balance", domain.ErrBillingInvoice, inv.DocumentNo)
		case inv.OnOpenNote:
			return nil, fmt.Errorf("%w: %s is on another open billing note", domain.ErrBillingInvoice, inv.DocumentNo)
		}
	}
	s.ID, s.Status, s.Cancellation = vo.NewID[vo.BillingNoteID](), vo.BillingOpen, nil
	return &BillingNote{s: s}, nil
}

func RehydrateBillingNote(s BillingNoteState) *BillingNote { return &BillingNote{s: s} }

func (b *BillingNote) ID() vo.BillingNoteID    { return b.s.ID }
func (b *BillingNote) State() BillingNoteState { return b.s }

func (b *BillingNote) Cancel(by vo.UserID, at time.Time) error {
	if b.s.Status == vo.BillingCancelled {
		return domain.ErrBillingNoteCancelled
	}
	b.s.Status, b.s.Cancellation = vo.BillingCancelled, &Cancellation{By: by, At: at}
	return nil
}
