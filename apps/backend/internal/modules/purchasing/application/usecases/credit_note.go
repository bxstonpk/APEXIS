package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

// CreditNoteUseCases: ใบลดหนี้ที่ได้รับจากผู้ขาย (ส่งคืนสินค้า / ลดราคาภายหลัง) อ้างใบแจ้งหนี้ผู้ขายเดิม
//
// ยอดลดหนี้ตัดยอดค้างของใบแจ้งหนี้เดิมด้วยกลไกเดียวกับใบจ่ายเงินของ finance (paid_amount + purchasing_invoice_settlements)
// อายุเจ้าหนี้ / ใบแจ้งหนี้ค้างจ่าย / ภาพรวมจึงหักใบลดหนี้ให้เอง — ลดหนี้ได้ไม่เกินยอดค้างของใบแจ้งหนี้
type CreditNoteUseCases struct {
	base
	notes       repositories.CreditNoteRepository
	invoices    repositories.InvoiceRepository
	settlements repositories.SettlementRepository
}

func (uc *CreditNoteUseCases) Get(ctx context.Context, in dto.ByID[vo.CreditNoteID]) (dto.CreditNote, error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (dto.CreditNote, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *CreditNoteUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.CreditNoteSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (listing.Result[dto.CreditNoteSummary], error) {
		return uc.notes.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create บันทึกใบลดหนี้ใน transaction เดียว:
// lock ใบแจ้งหนี้ -> จำนวน / ยอดไม่เกินที่เหลือของแต่ละบรรทัด -> VAT ตามใบแจ้งหนี้ -> ส่งคืนสินค้า stock: เบิกออก FIFO
// -> Dr เจ้าหนี้ / Cr ภาษีซื้อ + สินค้าคงเหลือ ± ผลต่างราคา (perpetual) หรือส่งคืนสินค้าและส่วนลดรับ / บัญชีซื้อ -> ตัดยอดใบแจ้งหนี้
func (uc *CreditNoteUseCases) Create(ctx context.Context, in dto.Create[dto.CreditNoteInput]) (dto.CreditNote, error) {
	var id vo.CreditNoteID
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, cs ports.CompanySettings) error {
		d := in.Data
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ap_account_id"); err != nil {
			return err
		}
		if !vo.IsDateOnly(d.Date) || !vo.IsDateOnly(d.CreditNoteDate) {
			return domain.ErrInvalidDate
		}
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}
		is := inv.State()
		if is.Status != vo.DocPosted {
			return domain.ErrInvoiceNotCreditable
		}
		if d.Date.Before(is.Date) {
			return domain.ErrCreditBeforeInvoice
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		credited, err := uc.notes.InvoiceCredits(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}

		// บรรทัด: จำนวน / ยอดตามประเภท, ไม่เกินที่เหลือของบรรทัดใบแจ้งหนี้ (รวมบรรทัดซ้ำในใบเดียวกัน)
		lines := make([]entities.CreditNoteLine, len(d.Lines))
		ids := make([]vo.ItemID, 0, len(d.Lines))
		for i, l := range d.Lines {
			n := i + 1
			if l.InvoiceLineNo < 1 || l.InvoiceLineNo > len(is.Lines) {
				return fmt.Errorf("%w: line %d invoice_line_no %d not found on invoice %s", domain.ErrInvalidLine, n, l.InvoiceLineNo, is.Posting.DocumentNo)
			}
			il := is.Lines[l.InvoiceLineNo-1]
			c := credited[l.InvoiceLineNo]
			line := entities.CreditNoteLine{InvoiceLineNo: l.InvoiceLineNo, ItemID: il.ItemID, Quantity: zero}
			switch d.Kind {
			case vo.CreditReturn:
				if l.Quantity == nil || !l.Quantity.IsPositive() || !numeric.FitsScale(*l.Quantity, cs.QuantityDecimals) {
					return fmt.Errorf("%w: line %d quantity must be positive with at most %d decimal places", domain.ErrInvalidLine, n, cs.QuantityDecimals)
				}
				if left := il.Quantity.Sub(c.Returned); l.Quantity.GreaterThan(left) {
					return fmt.Errorf("%w: line %d returns %s but only %s is left on invoice line %d", domain.ErrOverCredit, n, l.Quantity, left, l.InvoiceLineNo)
				}
				price := il.UnitPrice
				if l.UnitPrice != nil {
					if !numericAmount(*l.UnitPrice, numeric.MaxScale) || l.UnitPrice.GreaterThan(il.UnitPrice) {
						return fmt.Errorf("%w: line %d unit_price must be positive and not above the invoice price %s", domain.ErrInvalidLine, n, il.UnitPrice)
					}
					price = *l.UnitPrice
				}
				line.Quantity, line.UnitPrice = *l.Quantity, price
				line.Amounts = services.LineAmounts(*l.Quantity, price, is.Vat, cs.AmountDecimals)
				c.Returned = c.Returned.Add(*l.Quantity)
			case vo.CreditAllowance:
				if l.Amount == nil || !numericAmount(*l.Amount, cs.AmountDecimals) {
					return fmt.Errorf("%w: line %d amount must be positive with at most %d decimal places", domain.ErrInvalidLine, n, cs.AmountDecimals)
				}
				line.UnitPrice = *l.Amount
				line.Amounts = services.LineAmounts(decimalOne, *l.Amount, is.Vat, cs.AmountDecimals)
			default:
				return domain.ErrInvalidKind
			}
			if !line.Amounts.Net.IsPositive() {
				return fmt.Errorf("%w: line %d amount rounds to zero", domain.ErrInvalidLine, n)
			}
			if left := il.Amounts.Net.Sub(c.Net); line.Amounts.Net.GreaterThan(left) {
				return fmt.Errorf("%w: line %d credits %s (before VAT) but only %s is left on invoice line %d", domain.ErrOverCredit, n, line.Amounts.Net, left, l.InvoiceLineNo)
			}
			c.Net = c.Net.Add(line.Amounts.Net)
			credited[l.InvoiceLineNo] = c
			lines[i] = line
			ids = append(ids, il.ItemID)
		}
		system, found, err := uc.inventory.PurchaseContext(ctx, in.CompanyID, d.Date, ids)
		if err != nil {
			return err
		}
		if err := items(found, ids, false); err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocCreditNote, d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		// ตรวจหลังออกเลข: แถวลำดับถูก lock แล้ว (unique ที่ DB กันกรณีที่เหลือ)
		supNo := strings.TrimSpace(d.SupplierCreditNoteNo)
		if used, err := uc.notes.SupplierNoUsed(ctx, in.CompanyID, is.SupplierID, supNo); err != nil {
			return err
		} else if used {
			return domain.ErrCreditNoteNoExists
		}

		// ส่งคืนสินค้า stock: เบิกออกจากคลัง (FIFO) — ต้นทุนที่ตัดออกใช้ลงบัญชีสินค้าคงเหลือ (perpetual)
		st := entities.CreditNoteState{
			CompanyID: in.CompanyID, InvoiceID: inv.ID(), OrderID: is.OrderID, SupplierID: is.SupplierID, BillFrom: is.BillFrom,
			SupplierCreditNoteNo: supNo, CreditNoteDate: d.CreditNoteDate, Date: d.Date, Kind: d.Kind, Reason: d.Reason,
			Description: d.Description, Vat: is.Vat, System: system,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}
		var stock []ports.StockLine
		var stockIdx []int
		for i := range lines {
			if d.Kind == vo.CreditReturn && found[lines[i].ItemID].Stock {
				lines[i].Stock = true
				stock = append(stock, ports.StockLine{ItemID: lines[i].ItemID, Quantity: lines[i].Quantity})
				stockIdx = append(stockIdx, i)
			}
		}
		if len(stock) > 0 {
			if d.WarehouseID == nil {
				return domain.ErrWarehouseRequired
			}
			issued, err := uc.inventory.PostIssue(ctx, in.CompanyID, in.UserID, *d.WarehouseID, d.Date, no,
				"ใบลดหนี้ผู้ขาย "+no+" / "+is.SupplierInvoiceNo, stock)
			if err != nil {
				return err
			}
			for k, i := range stockIdx {
				lines[i].Cost = issued.LineValues[k]
			}
			st.WarehouseID, st.StockDocumentID, st.StockDocumentNo = d.WarehouseID, &issued.DocumentID, issued.DocumentNo
		}

		// บัญชีของแต่ละบรรทัด
		for i := range lines {
			l := &lines[i]
			item := found[l.ItemID]
			if l.Stock && system == vo.Perpetual {
				if item.InventoryAccountID == nil {
					return fmt.Errorf("%w: inventory account of item %s not set", domain.ErrSettingsMissing, item.Code)
				}
				l.InventoryAccountID, l.Variance = item.InventoryAccountID, l.Amounts.Net.Sub(l.Cost)
				l.AccountID = *item.InventoryAccountID
				if !l.Variance.IsZero() {
					if err := settings.Require("price_variance_account_id"); err != nil {
						return fmt.Errorf("%w (credit price differs from the stock cost of item %s)", err, item.Code)
					}
					l.AccountID = *settings.PriceVarianceAccountID
				}
				continue
			}
			acc := settings.PurchaseReturnsAccountID
			if acc == nil {
				acc = item.PurchaseAccountID
			}
			if acc == nil {
				return fmt.Errorf("%w: purchase_returns_account_id or purchase account of item %s not set", domain.ErrSettingsMissing, item.Code)
			}
			l.AccountID = *acc
		}
		st.Lines = lines
		cn, err := entities.NewCreditNote(st)
		if err != nil {
			return err
		}
		s := cn.State()

		inputVat := vo.AccountID{}
		if !s.Totals.Vat.IsZero() {
			if err := settings.Require("input_vat_account_id"); err != nil {
				return err
			}
			inputVat = *settings.InputVatAccountID
		}
		if journal := cn.Journal(*settings.APAccountID, inputVat); len(journal) > 0 {
			entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
				CompanyID: in.CompanyID, UserID: in.UserID, JournalID: *settings.JournalID, Date: s.Date,
				Description: "ใบลดหนี้ผู้ขาย " + s.SupplierCreditNoteNo + " / " + is.SupplierInvoiceNo,
				Reference:   no, Lines: journal,
			})
			if err != nil {
				return err
			}
			cn.SetEntry(entryID, entryNo)
		}

		// ตัดยอดค้างของใบแจ้งหนี้ (ลดหนี้ได้ไม่เกินยอดค้าง)
		if err := inv.Pay(is.SupplierID, s.Totals.Gross); err != nil {
			if errors.Is(err, domain.ErrOverPayment) {
				return fmt.Errorf("%w: credit %s exceeds the invoice outstanding balance %s (cancel payments first)",
					domain.ErrOverCredit, s.Totals.Gross, inv.Outstanding())
			}
			return err
		}
		if err := uc.invoices.SavePaid(ctx, []*entities.Invoice{inv}); err != nil {
			return err
		}
		id = cn.ID()
		if err := uc.notes.Create(ctx, cn); err != nil {
			return err
		}
		return uc.settlements.Create(ctx, in.CompanyID, dto.SettlementDoc{ID: uuid.UUID(id), No: no, Date: s.Date},
			[]dto.SettlementLine{{InvoiceID: inv.ID(), Amount: s.Totals.Gross}})
	})
	if err != nil {
		return dto.CreditNote{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Cancel ยกเลิกใบลดหนี้: คืนยอดค้างให้ใบแจ้งหนี้ -> คืนสต็อก (ยกเลิกใบเบิกออก) -> กลับรายการบัญชี ณ วันที่เดิม (งวดต้องยังเปิด)
func (uc *CreditNoteUseCases) Cancel(ctx context.Context, in dto.ByID[vo.CreditNoteID]) (dto.CreditNote, error) {
	err := uc.write(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context) error {
		cn, err := uc.notes.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := cn.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := cn.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		docID := uuid.UUID(s.ID)
		settled, err := uc.settlements.LockDocument(ctx, in.CompanyID, docID)
		if err != nil {
			return err
		}
		if len(settled) > 0 {
			ids := make([]vo.InvoiceID, len(settled))
			amounts := make(map[vo.InvoiceID]vo.Money, len(settled))
			for i, l := range settled {
				ids[i] = l.InvoiceID
				amounts[l.InvoiceID] = amounts[l.InvoiceID].Add(l.Amount)
			}
			invoices, err := uc.invoices.LockForSettlement(ctx, in.CompanyID, ids)
			if err != nil {
				return err
			}
			for _, inv := range invoices {
				inv.Unpay(amounts[inv.ID()])
			}
			if err := uc.invoices.SavePaid(ctx, invoices); err != nil {
				return err
			}
			if err := uc.settlements.Cancel(ctx, in.CompanyID, docID); err != nil {
				return err
			}
		}
		if s.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.StockDocumentID); err != nil {
				return err
			}
		}
		if s.Posting.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *s.Posting.EntryID, s.Date, "Cancel supplier credit note "+s.Posting.DocumentNo); err != nil {
				return err
			}
		}
		return uc.notes.UpdateStatus(ctx, cn)
	})
	if err != nil {
		return dto.CreditNote{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}

func (uc *CreditNoteUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (dto.CreditNote, error) {
	out, err := uc.notes.Get(ctx, companyID, id)
	if err != nil {
		return out, err
	}
	refs := make([]*dto.Ref, len(out.Lines))
	for i := range out.Lines {
		refs[i] = &out.Lines[i].Item
	}
	return out, uc.itemRefs(ctx, companyID, refs)
}
