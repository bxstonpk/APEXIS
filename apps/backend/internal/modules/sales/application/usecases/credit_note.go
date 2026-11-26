package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/sales/application/dto"
	"apexis/backend/internal/modules/sales/application/ports"
	"apexis/backend/internal/modules/sales/application/ports/repositories"
	"apexis/backend/internal/modules/sales/domain/entities"
	domain "apexis/backend/internal/modules/sales/domain/errors"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// CreditNoteUseCases: ใบลดหนี้ขาย (รับคืนสินค้า / ลดราคา) — สิทธิ์เดียวกับใบแจ้งหนี้ (sales.invoice:read / post)
//
// ใบลดหนี้ตัดยอดลูกหนี้ของใบแจ้งหนี้เดิมแบบเดียวกับใบเสร็จ (received_amount + ประวัติการตัดยอด)
// ยอดค้าง / ใบค้างรับของ finance / อายุลูกหนี้ / วงเงินเครดิต จึงหักใบลดหนี้อัตโนมัติ และยกเลิกใบแจ้งหนี้ไม่ได้จนกว่าจะยกเลิกใบลดหนี้
type CreditNoteUseCases struct {
	base
	notes       repositories.CreditNoteRepository
	invoices    repositories.InvoiceRepository
	deliveries  repositories.DeliveryRepository
	settlements repositories.SettlementRepository
}

func (uc *CreditNoteUseCases) Get(ctx context.Context, in dto.ByID[vo.CreditNoteID]) (dto.CreditNote, error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (dto.CreditNote, error) { return uc.get(ctx, in.CompanyID, in.ID) })
}

func (uc *CreditNoteUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.CreditNoteID) (dto.CreditNote, error) {
	c, err := uc.notes.Get(ctx, companyID, id)
	if err != nil {
		return c, err
	}
	refs := make([]*dto.Ref, len(c.Lines))
	for i := range c.Lines {
		refs[i] = &c.Lines[i].Item
	}
	return c, uc.itemRefs(ctx, companyID, refs)
}

func (uc *CreditNoteUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.CreditNoteSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermInvoiceRead, func() (listing.Result[dto.CreditNoteSummary], error) {
		return uc.notes.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create ออกใบลดหนี้: lock ใบแจ้งหนี้ -> ตรวจยอดต่อบรรทัด (ไม่เกินที่เหลือ) -> งวดเปิด -> เลขที่
// -> รับคืนเข้าคลังที่ต้นทุนเดิม (return + สินค้า stock) -> ลงบัญชี -> ตัดยอดลูกหนี้ของใบแจ้งหนี้ (ไม่เกินยอดค้าง)
func (uc *CreditNoteUseCases) Create(ctx context.Context, in dto.Create[dto.CreditNoteInput]) (dto.CreditNote, error) {
	var id vo.CreditNoteID
	err := uc.writeWith(ctx, in.Actor, vo.PermInvoicePost, func(ctx context.Context, s ports.CompanySettings) error {
		d := in.Data
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ar_account_id"); err != nil {
			return err
		}
		inv, err := uc.invoices.FindForUpdate(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}
		ist := inv.State()
		if ist.Status != vo.DocPosted {
			return fmt.Errorf("%w: %s", domain.ErrInvoiceNotPayable, ist.Posting.DocumentNo)
		}
		if len(d.Lines) == 0 {
			return fmt.Errorf("%w: at least one line", domain.ErrInvalidLine)
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, d.Date)
		if err != nil {
			return err
		}
		credited, err := uc.notes.Credited(ctx, in.CompanyID, d.InvoiceID)
		if err != nil {
			return err
		}
		if credited == nil {
			credited = map[int]entities.Credited{}
		}
		// สินค้าของบรรทัด + ระบบบัญชีของปีที่ออกใบลดหนี้
		ids := make([]vo.ItemID, 0, len(d.Lines))
		for _, l := range d.Lines {
			il, err := inv.CreditLine(l.InvoiceLineNo)
			if err != nil {
				return err
			}
			if !slices.Contains(ids, il.ItemID) {
				ids = append(ids, il.ItemID)
			}
		}
		system, items, err := uc.inventory.Context(ctx, in.CompanyID, d.Date, ids)
		if err != nil {
			return err
		}
		if err := checkItems(items, ids, false); err != nil {
			return err
		}

		lines := make([]entities.CreditNoteLine, len(d.Lines))
		var stockIdx []int
		for i, l := range d.Lines {
			il, _ := inv.CreditLine(l.InvoiceLineNo)
			done := credited[l.InvoiceLineNo]
			price := il.UnitPrice
			if l.UnitPrice != nil {
				price = *l.UnitPrice
			}
			amounts, err := inv.CreditAmounts(d.Kind, l.InvoiceLineNo, l.Quantity, price, l.Amount, done, s.AmountDecimals)
			if err != nil {
				return err
			}
			// สะสมในใบเดียวกัน (บรรทัดใบแจ้งหนี้เดียวกันซ้ำได้)
			if d.Kind == vo.CreditReturn {
				done.Returned = done.Returned.Add(l.Quantity)
			}
			done.Net = done.Net.Add(amounts.Net)
			credited[l.InvoiceLineNo] = done

			account := il.RevenueAccountID
			if settings.SalesReturnsAccountID != nil {
				account = *settings.SalesReturnsAccountID
			}
			line := entities.CreditNoteLine{
				InvoiceLineNo: l.InvoiceLineNo, ItemID: il.ItemID, Quantity: decimal.Zero, UnitPrice: decimal.Zero, Amounts: amounts,
				AccountID: account, CostLine: entities.CostLine{Cost: decimal.Zero},
			}
			if d.Kind == vo.CreditReturn {
				line.Quantity, line.UnitPrice = l.Quantity, price
				if item := items[il.ItemID]; item.Stock {
					line.Stock = true
					if system == vo.Perpetual {
						if item.InventoryAccountID == nil {
							return fmt.Errorf("%w: inventory account of item %s", domain.ErrSettingsMissing, item.Code)
						}
						line.InventoryAccountID = item.InventoryAccountID
					}
					stockIdx = append(stockIdx, i)
				}
			} else {
				line.UnitPrice = l.Amount
			}
			lines[i] = line
		}

		no, err := uc.number(ctx, in.CompanyID, vo.DocCreditNote, d.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		stock := entities.Stock{System: system}
		if len(stockIdx) > 0 {
			if stock, err = uc.restock(ctx, in.Actor, settings, inv, lines, stockIdx, system, d, no); err != nil {
				return err
			}
		}

		cn, err := entities.NewCreditNote(entities.CreditNoteState{
			CompanyID: in.CompanyID, InvoiceID: inv.ID(), InvoiceNo: ist.Posting.DocumentNo, CustomerID: ist.CustomerID, BillTo: ist.BillTo,
			Date: d.Date, Kind: d.Kind, Reason: d.Reason, Description: d.Description, Vat: ist.Vat, Stock: stock, Lines: lines,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		}, ist.Date)
		if err != nil {
			return err
		}
		cs := cn.State()
		if !cs.Totals.Vat.IsZero() {
			if err := settings.Require("output_vat_account_id"); err != nil {
				return err
			}
		}
		// ตัดยอดลูกหนี้ของใบแจ้งหนี้: ไม่เกินยอดค้าง (ยอดรวม - รับชำระ - ลดหนี้ก่อนหน้า)
		if err := inv.Receive(ist.CustomerID, cs.Totals.Gross); err != nil {
			return fmt.Errorf("%w: %s (outstanding %s, credit %s)", domain.ErrOverCredit, ist.Posting.DocumentNo, inv.Outstanding(), cs.Totals.Gross)
		}
		journal := cn.Journal(*settings.ARAccountID, accountOr(settings.OutputVatAccountID), accountOr(settings.CogsAccountID))
		entryID, entryNo, err := uc.post(ctx, in.Actor, *settings.JournalID, d.Date, "ใบลดหนี้ "+no+" / "+ist.Posting.DocumentNo, no, journal)
		if err != nil {
			return err
		}
		if entryID != nil {
			cn.SetEntry(*entryID, entryNo)
		}
		id = cn.ID()
		if err := uc.notes.Create(ctx, cn); err != nil {
			return err
		}
		if err := uc.invoices.SaveReceived(ctx, []*entities.Invoice{inv}); err != nil {
			return err
		}
		return uc.settlements.Create(ctx, in.CompanyID, dto.SettlementDoc{ID: uuid.UUID(cn.ID()), No: no, Date: d.Date},
			[]dto.SettlementLine{{InvoiceID: inv.ID(), Amount: cs.Totals.Gross}})
	})
	if err != nil {
		return dto.CreditNote{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// restock รับสินค้า stock คืนเข้าคลังที่ต้นทุนเดิมต่อหน่วย:
// ใบส่งของ / ใบกำกับภาษี = ต้นทุนในบรรทัดใบแจ้งหนี้; ใบแจ้งหนี้แยก = ต้นทุนเฉลี่ยของใบส่งของตามบรรทัดใบสั่งขาย
// perpetual ต้องมีบัญชีต้นทุนขาย (Cr ต้นทุนขาย / Dr สินค้าคงเหลือ)
func (uc *CreditNoteUseCases) restock(ctx context.Context, a dto.Actor, settings entities.Settings, inv *entities.Invoice, lines []entities.CreditNoteLine,
	idx []int, system vo.AccountingSystem, d dto.CreditNoteInput, no string) (entities.Stock, error) {
	stock := entities.Stock{System: system}
	if d.WarehouseID == nil {
		return stock, domain.ErrWarehouseRequired
	}
	if system == vo.Perpetual {
		if err := settings.Require("cogs_account_id"); err != nil {
			return stock, err
		}
	}
	ist := inv.State()
	var averages map[int]vo.Money
	if !ist.WithDelivery {
		orderLines := make([]int, 0, len(idx))
		for _, i := range idx {
			il, _ := inv.CreditLine(lines[i].InvoiceLineNo)
			orderLines = append(orderLines, il.OrderLineNo)
		}
		var err error
		if averages, err = uc.deliveries.UnitCosts(ctx, a.CompanyID, ist.OrderID, orderLines); err != nil {
			return stock, err
		}
	}
	in := make([]ports.ReceiptLine, len(idx))
	for k, i := range idx {
		il, _ := inv.CreditLine(lines[i].InvoiceLineNo)
		var unit vo.Money
		var ok bool
		if ist.WithDelivery {
			ok = il.Stock && il.Quantity.IsPositive()
			if ok {
				unit = il.Cost.DivRound(il.Quantity, 6)
			}
		} else {
			unit, ok = averages[il.OrderLineNo]
		}
		if !ok {
			return stock, fmt.Errorf("%w: invoice line %d", domain.ErrCostUnknown, lines[i].InvoiceLineNo)
		}
		in[k] = ports.ReceiptLine{ItemID: lines[i].ItemID, Quantity: lines[i].Quantity, UnitCost: unit}
	}
	doc, err := uc.inventory.PostReceipt(ctx, a.CompanyID, a.UserID, *d.WarehouseID, d.Date, no, "Sales return "+no+" / "+ist.Posting.DocumentNo, in)
	if err != nil {
		return stock, err
	}
	for k, i := range idx {
		lines[i].Cost = doc.LineValues[k]
	}
	stock.WarehouseID, stock.StockDocumentID, stock.StockDocumentNo = d.WarehouseID, &doc.DocumentID, doc.DocumentNo
	return stock, nil
}

// Cancel: คืนยอดค้างให้ใบแจ้งหนี้ -> ยกเลิกใบรับคืนของ inventory (สินค้าที่รับคืนถูกใช้ไปแล้ว = ยกเลิกไม่ได้)
// -> กลับรายการบัญชี ณ วันที่ของใบลดหนี้
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
		if err := unsettle(ctx, uc.invoices, uc.settlements, in.CompanyID, uuid.UUID(cn.ID())); err != nil {
			return err
		}
		if s.Stock.StockDocumentID != nil {
			if err := uc.inventory.CancelDocument(ctx, in.CompanyID, in.UserID, *s.Stock.StockDocumentID); err != nil {
				return err
			}
		}
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "credit note"); err != nil {
			return err
		}
		return uc.notes.UpdateStatus(ctx, cn)
	})
	if err != nil {
		return dto.CreditNote{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}
