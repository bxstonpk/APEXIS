package entities

import (
	"fmt"
	"time"

	domain "apexis/backend/internal/modules/inventory/domain/errors"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// MaxLines คือจำนวนบรรทัดสูงสุดต่อเอกสาร
const MaxLines = 500

// LineData คือบรรทัดของเอกสารสต็อก
//
//	receipt:    Quantity > 0, UnitCost >= 0 (ต้องมี)
//	issue:      Quantity > 0 (ต้นทุน FIFO คำนวณตอนผ่านเอกสาร)
//	transfer:   Quantity > 0 (ต้นทุนตามชั้น FIFO ที่ตัดออกจากคลังต้นทาง)
//	adjustment: Quantity != 0 — บวก = เพิ่ม (ต้องมี UnitCost), ลบ = ลด (FIFO)
type LineData struct {
	ItemID      vo.ItemID
	Quantity    vo.Quantity
	UnitCost    *vo.Money
	Description string
	Value       vo.Money // มูลค่าของบรรทัด — คำนวณตอนผ่านเอกสาร (ขาออก = ต้นทุน FIFO, ติดลบ)
}

// Inbound: บรรทัดนี้เพิ่มสต็อก (ไม่รวมฝั่งรับของ transfer ซึ่งคิดจากชั้นที่โอนออก)
func (l LineData) Inbound(t vo.DocumentType) bool {
	return t == vo.DocReceipt || (t == vo.DocAdjustment && l.Quantity.IsPositive())
}

type DocumentData struct {
	Type          vo.DocumentType
	Date          time.Time
	WarehouseID   vo.WarehouseID  // receipt: คลังรับ, อื่นๆ: คลังต้นทาง
	ToWarehouseID *vo.WarehouseID // เฉพาะ transfer: คลังปลายทาง
	Reference     string
	Description   string
	Lines         []LineData
}

type Posting struct {
	DocumentNo string // เช่น GR2026-000001 — เรียงไม่ขาดตอนแยกตามชนิดเอกสารและปีบัญชี
	PeriodID   vo.FiscalPeriodID
	By         vo.UserID
	At         time.Time
}

type Cancellation struct {
	By vo.UserID
	At time.Time
}

type DocumentState struct {
	ID           vo.DocumentID
	CompanyID    vo.CompanyID
	Data         DocumentData
	Status       vo.DocumentStatus
	Posting      *Posting
	Cancellation *Cancellation
	CreatedBy    vo.UserID
}

// Document คือเอกสารเคลื่อนไหวสต็อก
//
// กฎ:
//   - draft แก้ / ลบได้; posted แก้ / ลบไม่ได้ — ยกเลิก (cancelled) ได้ด้วยการกลับผลที่วันที่เดิมของเอกสาร
//   - จำนวนสินค้ามีทศนิยมไม่เกินที่บริษัทกำหนด, ต้นทุนต่อหน่วยไม่เกิน 6 ตำแหน่ง
//   - transfer ต้องมีคลังปลายทางที่ไม่ใช่คลังเดียวกัน
type Document struct{ s DocumentState }

func NewDocument(companyID vo.CompanyID, createdBy vo.UserID, d DocumentData, quantityDecimals int) (*Document, error) {
	n, err := normalizeDocument(d, quantityDecimals)
	if err != nil {
		return nil, err
	}
	return &Document{s: DocumentState{
		ID: vo.NewID[vo.DocumentID](), CompanyID: companyID, Data: n, Status: vo.DocDraft, CreatedBy: createdBy,
	}}, nil
}

func RehydrateDocument(s DocumentState) *Document { return &Document{s: s} }

func (d *Document) ID() vo.DocumentID       { return d.s.ID }
func (d *Document) CompanyID() vo.CompanyID { return d.s.CompanyID }
func (d *Document) Data() DocumentData      { return d.s.Data }
func (d *Document) Status() vo.DocumentStatus {
	return d.s.Status
}
func (d *Document) State() DocumentState { return d.s }

// ItemIDs / WarehouseIDs คืน id ที่อ้างถึง (ไม่ซ้ำ) — ใช้โหลดข้อมูลอ้างอิงใน query เดียว
func (d *Document) ItemIDs() []vo.ItemID {
	seen := map[vo.ItemID]bool{}
	var out []vo.ItemID
	for _, l := range d.s.Data.Lines {
		if !seen[l.ItemID] {
			seen[l.ItemID] = true
			out = append(out, l.ItemID)
		}
	}
	return out
}

func (d *Document) WarehouseIDs() []vo.WarehouseID {
	out := []vo.WarehouseID{d.s.Data.WarehouseID}
	if to := d.s.Data.ToWarehouseID; to != nil {
		out = append(out, *to)
	}
	return out
}

func (d *Document) Update(data DocumentData, quantityDecimals int) error {
	if d.s.Status != vo.DocDraft {
		return domain.ErrDocumentNotDraft
	}
	n, err := normalizeDocument(data, quantityDecimals)
	if err != nil {
		return err
	}
	d.s.Data = n
	return nil
}

func (d *Document) EnsureDraft() error {
	if d.s.Status != vo.DocDraft {
		return domain.ErrDocumentNotDraft
	}
	return nil
}

// Post: lineValues คือมูลค่าของแต่ละบรรทัดที่คำนวณจาก FIFO (เรียงตามบรรทัด)
func (d *Document) Post(p Posting, lineValues []vo.Money) error {
	if err := d.EnsureDraft(); err != nil {
		return err
	}
	if len(lineValues) != len(d.s.Data.Lines) {
		return fmt.Errorf("inventory: %d line values for %d lines", len(lineValues), len(d.s.Data.Lines))
	}
	for i := range d.s.Data.Lines {
		d.s.Data.Lines[i].Value = lineValues[i]
	}
	d.s.Status = vo.DocPosted
	d.s.Posting = &p
	return nil
}

func (d *Document) Cancel(c Cancellation) error {
	if d.s.Status != vo.DocPosted {
		return domain.ErrDocumentNotPosted
	}
	d.s.Status = vo.DocCancelled
	d.s.Cancellation = &c
	return nil
}

func normalizeDocument(d DocumentData, quantityDecimals int) (DocumentData, error) {
	var err error
	if !vo.IsDateOnly(d.Date) {
		return DocumentData{}, domain.ErrInvalidDate
	}
	if _, err := vo.ParseDocumentType(string(d.Type)); err != nil {
		return DocumentData{}, err
	}
	out := DocumentData{Type: d.Type, Date: d.Date, WarehouseID: d.WarehouseID}
	if d.Type == vo.DocTransfer {
		if d.ToWarehouseID == nil || *d.ToWarehouseID == d.WarehouseID {
			return DocumentData{}, domain.ErrSameWarehouse
		}
		to := *d.ToWarehouseID
		out.ToWarehouseID = &to
	}
	if out.Reference, err = vo.OptionalText(d.Reference, 100); err != nil {
		return DocumentData{}, err
	}
	if out.Description, err = vo.OptionalText(d.Description, 500); err != nil {
		return DocumentData{}, err
	}
	if len(d.Lines) == 0 || len(d.Lines) > MaxLines {
		return DocumentData{}, fmt.Errorf("%w: a document must have 1 to %d lines", domain.ErrInvalidLine, MaxLines)
	}
	out.Lines = make([]LineData, len(d.Lines))
	for i, l := range d.Lines {
		n, err := normalizeLine(d.Type, l, i+1, quantityDecimals)
		if err != nil {
			return DocumentData{}, err
		}
		out.Lines[i] = n
	}
	return out, nil
}

func normalizeLine(t vo.DocumentType, l LineData, n, quantityDecimals int) (LineData, error) {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: line %d "+format, append([]any{domain.ErrInvalidLine, n}, args...)...)
	}
	q := l.Quantity
	switch {
	case q.IsZero() || !numeric.InRange(q):
		return LineData{}, bad("quantity must be non-zero")
	case q.IsNegative() && t != vo.DocAdjustment:
		return LineData{}, bad("quantity must be positive (only adjustments may be negative)")
	case !numeric.FitsScale(q, quantityDecimals):
		return LineData{}, bad("quantity has more than %d decimal places", quantityDecimals)
	}
	out := LineData{ItemID: l.ItemID, Quantity: q}
	if out.Inbound(t) {
		if l.UnitCost == nil {
			return LineData{}, bad("unit_cost is required")
		}
		c := *l.UnitCost
		if c.IsNegative() || !numeric.InRange(c) || !numeric.FitsScale(c, numeric.MaxScale) {
			return LineData{}, bad("unit_cost must be zero or positive with at most %d decimal places", numeric.MaxScale)
		}
		out.UnitCost = &c
	} else if l.UnitCost != nil {
		return LineData{}, bad("unit_cost is only allowed on receipts and positive adjustments (outgoing cost is FIFO)")
	}
	var err error
	if out.Description, err = vo.OptionalText(l.Description, 500); err != nil {
		return LineData{}, bad("description is too long")
	}
	return out, nil
}
