package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/application/ports"
	"apexis/backend/internal/modules/finance/application/ports/repositories"
	"apexis/backend/internal/modules/finance/domain/entities"
	domain "apexis/backend/internal/modules/finance/domain/errors"
	"apexis/backend/internal/modules/finance/domain/services"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

func wrapf(err error, format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{err}, args...)...)
}

func partyFrom(c ports.Counterparty) entities.Party {
	id := c.ID
	return entities.Party{ID: &id, Code: c.Code, Name: c.Name, TaxID: c.TaxID, BranchNo: c.BranchNo, Address: c.Address, TaxType: c.TaxType}
}

func partyInput(p dto.PartyInput) entities.Party {
	return entities.Party{Name: p.Name, TaxID: p.TaxID, BranchNo: p.BranchNo, Address: p.Address, TaxType: p.TaxType}
}

func openInvoices(rows []ports.OpenInvoice) []dto.OpenInvoice {
	out := make([]dto.OpenInvoice, len(rows))
	for i, r := range rows {
		out[i] = dto.OpenInvoice{ID: uuid.UUID(r.ID).String(), DocumentNo: r.DocumentNo, PartnerRef: r.PartnerRef, Date: r.Date,
			DueDate: r.DueDate, Net: r.Net, Gross: r.Gross, Outstanding: r.Outstanding}
	}
	return out
}

// ---------- ใบรับเงิน ----------

type ReceiptUseCases struct {
	base
	receipts    repositories.ReceiptRepository
	receivables ports.Settlements
}

func (uc *ReceiptUseCases) Get(ctx context.Context, in dto.ByDoc[vo.ReceiptID]) (dto.Receipt, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReceiptRead, func() (dto.Receipt, error) { return uc.receipts.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *ReceiptUseCases) List(ctx context.Context, a dto.Actor, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error) {
	return read(ctx, uc.base, a, vo.PermReceiptRead, func() (listing.Result[dto.DocumentSummary], error) { return uc.receipts.List(ctx, a.CompanyID, f) })
}

// OpenInvoices: ใบแจ้งหนี้ขายค้างรับของลูกค้า (เลือกในหน้ารับเงิน)
func (uc *ReceiptUseCases) OpenInvoices(ctx context.Context, a dto.Actor, customerID uuid.UUID) ([]dto.OpenInvoice, error) {
	return read(ctx, uc.base, a, vo.PermReceiptRead, func() ([]dto.OpenInvoice, error) {
		rows, err := uc.receivables.Open(ctx, a.CompanyID, customerID)
		return openInvoices(rows), err
	})
}

// Create รับเงินใน transaction เดียว:
// ค่าตั้ง -> ทะเบียนเงิน -> (บัญชีที่ระบุเอง) -> งวดเปิด + เลขที่ -> ตัดยอดใบแจ้งหนี้ขาย (customer) -> ตรวจกฎ -> ลงบัญชี -> บันทึก
func (uc *ReceiptUseCases) Create(ctx context.Context, a dto.Actor, in dto.ReceiptInput) (dto.Receipt, error) {
	var id vo.ReceiptID
	err := uc.writeWith(ctx, a, vo.PermReceiptPost, func(ctx context.Context, cs ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("receipt_journal_id"); err != nil {
			return err
		}
		if in.Kind == vo.ReceiptCustomer && in.CustomerID == nil {
			return wrapf(domain.ErrInvalidReceipt, "customer_id is required for a customer receipt")
		}
		methods, err := uc.methods(ctx, a.CompanyID, in.Methods)
		if err != nil {
			return err
		}
		var accountIDs []vo.GLAccountID
		lines := make([]entities.ReceiptLine, len(in.Lines))
		for i, l := range in.Lines {
			lines[i] = entities.ReceiptLine{InvoiceID: l.InvoiceID, AccountID: l.AccountID, Description: l.Description, Amount: l.Amount, Wht: l.Wht}
			if l.AccountID != nil {
				accountIDs = append(accountIDs, *l.AccountID)
			}
		}
		if err := uc.checkAccounts(ctx, a.CompanyID, accountIDs); err != nil {
			return err
		}
		posting, err := uc.posting(ctx, a, vo.DocReceipt, in.Date)
		if err != nil {
			return err
		}
		state := entities.ReceiptState{CompanyID: a.CompanyID, Kind: in.Kind, Party: partyInput(in.Payer), Date: in.Date,
			Description: in.Description, Lines: lines, Methods: methods, Posting: posting}
		// ตรวจกฎของเอกสารก่อนตัดยอดลูกหนี้ (ข้อความผิดพลาดชัดกว่า) — id ใหม่ใช้อ้างอิงการตัดยอด
		r, err := entities.NewReceipt(state, cs.AmountDecimals)
		if err != nil {
			return err
		}
		ar := vo.GLAccountID{}
		if in.Kind == vo.ReceiptCustomer {
			settle := make([]ports.Settlement, len(lines))
			for i, l := range lines {
				settle[i] = ports.Settlement{InvoiceID: *l.InvoiceID, Amount: l.Amount}
			}
			res, err := uc.receivables.Settle(ctx, a.CompanyID, *in.CustomerID,
				ports.SettlementDoc{ID: uuid.UUID(r.ID()), No: posting.DocumentNo, Date: in.Date}, settle)
			if err != nil {
				return err
			}
			for i := range state.Lines {
				state.Lines[i].InvoiceNo = res.Invoices[i].DocumentNo
			}
			state.Party, ar = partyFrom(res.Party), res.ControlAccountID
			r = entities.RehydrateReceipt(withReceiptParty(r.State(), state))
		}
		whtAccount := vo.GLAccountID{}
		if !r.State().Totals.Wht.IsZero() {
			if err := settings.Require("wht_received_account_id"); err != nil {
				return err
			}
			whtAccount = *settings.WhtReceivedAccountID
		}
		entryID, entryNo, err := uc.post(ctx, a, *settings.ReceiptJournalID, in.Date, receiptDescription(r.State()), posting.DocumentNo,
			ports.LedgerEntry{Lines: r.Journal(ar, whtAccount)})
		if err != nil {
			return err
		}
		if entryID != nil {
			r.SetEntry(*entryID, entryNo)
		}
		id = r.ID()
		return uc.receipts.Create(ctx, r)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.receipts.Get(ctx, a.CompanyID, id)
}

// withReceiptParty: คงค่าที่ entity คำนวณแล้ว (id, ยอดรวม, สถานะ) และเติมคู่ค้า / เลขที่ใบแจ้งหนี้จาก sales
func withReceiptParty(s, filled entities.ReceiptState) entities.ReceiptState {
	s.Party = filled.Party
	for i := range s.Lines {
		s.Lines[i].InvoiceNo = filled.Lines[i].InvoiceNo
	}
	return s
}

func receiptDescription(s entities.ReceiptState) string {
	if s.Kind == vo.ReceiptCustomer {
		return "Customer receipt " + s.Posting.DocumentNo + " / " + s.Party.Code
	}
	return "Receipt " + s.Posting.DocumentNo + " / " + s.Party.Name
}

// Cancel: กลับรายการบัญชี ณ วันที่เดิม (งวดต้องเปิด) และคืนยอดค้างให้ใบแจ้งหนี้ขาย
func (uc *ReceiptUseCases) Cancel(ctx context.Context, in dto.ByDoc[vo.ReceiptID]) (dto.Receipt, error) {
	err := uc.write(ctx, in.Actor, vo.PermReceiptPost, func(ctx context.Context) error {
		r, err := uc.receipts.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := r.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := r.State()
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "receipt"); err != nil {
			return err
		}
		if s.Kind == vo.ReceiptCustomer {
			if err := uc.receivables.Unsettle(ctx, in.CompanyID, uuid.UUID(s.ID)); err != nil {
				return err
			}
		}
		return uc.receipts.UpdateStatus(ctx, r)
	})
	if err != nil {
		return dto.Receipt{}, err
	}
	return uc.receipts.Get(ctx, in.CompanyID, in.ID)
}

// ---------- ใบสำคัญจ่าย ----------

type PaymentUseCases struct {
	base
	payments repositories.PaymentRepository
	payables ports.Settlements
	whtTypes repositories.WhtTypeRepository
}

func (uc *PaymentUseCases) Get(ctx context.Context, in dto.ByDoc[vo.PaymentID]) (dto.Payment, error) {
	return read(ctx, uc.base, in.Actor, vo.PermPaymentRead, func() (dto.Payment, error) { return uc.payments.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *PaymentUseCases) List(ctx context.Context, a dto.Actor, f dto.DocumentFilter) (listing.Result[dto.DocumentSummary], error) {
	return read(ctx, uc.base, a, vo.PermPaymentRead, func() (listing.Result[dto.DocumentSummary], error) { return uc.payments.List(ctx, a.CompanyID, f) })
}

// OpenInvoices: ใบแจ้งหนี้ผู้ขายค้างจ่าย (เลือกในหน้าจ่ายเงิน)
func (uc *PaymentUseCases) OpenInvoices(ctx context.Context, a dto.Actor, supplierID uuid.UUID) ([]dto.OpenInvoice, error) {
	return read(ctx, uc.base, a, vo.PermPaymentRead, func() ([]dto.OpenInvoice, error) {
		rows, err := uc.payables.Open(ctx, a.CompanyID, supplierID)
		return openInvoices(rows), err
	})
}

// Create จ่ายเงินใน transaction เดียว:
// ค่าตั้ง -> ทะเบียนเงิน -> ประเภทภาษี -> (บัญชีที่ระบุเอง) -> งวดเปิด + เลขที่ -> ตัดยอดใบแจ้งหนี้ซื้อ (supplier)
// -> ภาษีหัก ณ ที่จ่าย (อัตรา ณ วันจ่าย, ฐานตามสัดส่วนก่อน VAT) -> ตรวจกฎ -> ลงบัญชี -> บันทึก
func (uc *PaymentUseCases) Create(ctx context.Context, a dto.Actor, in dto.PaymentInput) (dto.Payment, error) {
	var id vo.PaymentID
	err := uc.writeWith(ctx, a, vo.PermPaymentPost, func(ctx context.Context, cs ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("payment_journal_id"); err != nil {
			return err
		}
		if in.Kind == vo.PaymentSupplier && in.SupplierID == nil {
			return wrapf(domain.ErrInvalidPayment, "supplier_id is required for a supplier payment")
		}
		methods, err := uc.methods(ctx, a.CompanyID, in.Methods)
		if err != nil {
			return err
		}
		var whtIDs []vo.WhtTypeID
		var accountIDs []vo.GLAccountID
		for _, l := range in.Lines {
			if l.WhtTypeID != nil && !slices.Contains(whtIDs, *l.WhtTypeID) {
				whtIDs = append(whtIDs, *l.WhtTypeID)
			}
			if l.AccountID != nil {
				accountIDs = append(accountIDs, *l.AccountID)
			}
		}
		rates, err := uc.rates(ctx, a.CompanyID, whtIDs)
		if err != nil {
			return err
		}
		if err := uc.checkAccounts(ctx, a.CompanyID, accountIDs); err != nil {
			return err
		}
		posting, err := uc.posting(ctx, a, vo.DocPayment, in.Date)
		if err != nil {
			return err
		}
		docID := vo.NewID[vo.PaymentID]()
		party := partyInput(in.Payee)
		ap := vo.GLAccountID{}
		var settled []ports.SettledInvoice
		if in.Kind == vo.PaymentSupplier {
			settle := make([]ports.Settlement, len(in.Lines))
			for i, l := range in.Lines {
				if l.InvoiceID == nil {
					return wrapf(domain.ErrInvalidPayment, "line %d: invoice_id is required for a supplier payment", i+1)
				}
				settle[i] = ports.Settlement{InvoiceID: *l.InvoiceID, Amount: l.Amount}
			}
			res, err := uc.payables.Settle(ctx, a.CompanyID, *in.SupplierID, ports.SettlementDoc{ID: uuid.UUID(docID), No: posting.DocumentNo, Date: in.Date}, settle)
			if err != nil {
				return err
			}
			party, ap, settled = partyFrom(res.Party), res.ControlAccountID, res.Invoices
		}
		lines := make([]entities.PaymentLine, len(in.Lines))
		for i, l := range in.Lines {
			line := entities.PaymentLine{InvoiceID: l.InvoiceID, AccountID: l.AccountID, Description: l.Description, Amount: l.Amount,
				WhtRate: decimal.Zero, WhtBase: decimal.Zero, Wht: decimal.Zero}
			if settled != nil {
				line.InvoiceNo, line.SupplierInvoiceNo = settled[i].DocumentNo, settled[i].PartnerRef
			}
			if l.WhtTypeID != nil {
				line.WhtTypeID, line.WhtRate = l.WhtTypeID, rates[*l.WhtTypeID]
				if settled != nil {
					line.WhtBase, line.Wht = services.Withholding(l.Amount, settled[i].Net, settled[i].Gross, line.WhtRate, cs.AmountDecimals)
				} else {
					line.WhtBase, line.Wht = services.WithholdingOnAmount(l.Amount, line.WhtRate, cs.AmountDecimals)
				}
			}
			lines[i] = line
		}
		p, err := entities.NewPayment(entities.PaymentState{CompanyID: a.CompanyID, Kind: in.Kind, Party: party, Date: in.Date,
			Description: in.Description, Lines: lines, Methods: methods, Posting: posting}, cs.AmountDecimals)
		if err != nil {
			return err
		}
		st := p.State()
		st.ID = docID // id เดียวกับที่ใช้ตัดยอดใบแจ้งหนี้ (ยกเลิกแล้วคืนยอดได้ถูกเอกสาร)
		p = entities.RehydratePayment(st)
		whtAccount := vo.GLAccountID{}
		if !st.Totals.Wht.IsZero() {
			acc, field := settings.WhtPayable(st.Party.TaxType)
			if acc == nil {
				return settings.Require(field)
			}
			whtAccount = *acc
		}
		entryID, entryNo, err := uc.post(ctx, a, *settings.PaymentJournalID, in.Date, paymentDescription(st), posting.DocumentNo,
			ports.LedgerEntry{Lines: p.Journal(ap, whtAccount)})
		if err != nil {
			return err
		}
		if entryID != nil {
			p.SetEntry(*entryID, entryNo)
		}
		id = p.ID()
		return uc.payments.Create(ctx, p)
	})
	if err != nil {
		return dto.Payment{}, err
	}
	return uc.payments.Get(ctx, a.CompanyID, id)
}

// rates: อัตราภาษี ณ วันจ่ายของประเภทที่ใช้ — ต้องมีอยู่และใช้งานอยู่ (query เดียว)
func (uc *PaymentUseCases) rates(ctx context.Context, companyID vo.CompanyID, ids []vo.WhtTypeID) (map[vo.WhtTypeID]vo.Money, error) {
	found, err := uc.whtTypes.FindByIDs(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[vo.WhtTypeID]vo.Money, len(found))
	for _, w := range found {
		if !w.IsActive() {
			return nil, wrapf(domain.ErrInactiveRef, "withholding tax type %s", w.Data().Code)
		}
		out[w.ID()] = w.Data().Rate
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			return nil, wrapf(domain.ErrWhtTypeNotFound, "%s", id)
		}
	}
	return out, nil
}

func paymentDescription(s entities.PaymentState) string {
	if s.Kind == vo.PaymentSupplier {
		return "Supplier payment " + s.Posting.DocumentNo + " / " + s.Party.Code
	}
	return "Payment " + s.Posting.DocumentNo + " / " + s.Party.Name
}

// Cancel: กลับรายการบัญชี ณ วันที่จ่ายเดิม (งวดต้องเปิด) และคืนยอดค้างให้ใบแจ้งหนี้ซื้อ
func (uc *PaymentUseCases) Cancel(ctx context.Context, in dto.ByDoc[vo.PaymentID]) (dto.Payment, error) {
	err := uc.write(ctx, in.Actor, vo.PermPaymentPost, func(ctx context.Context) error {
		p, err := uc.payments.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := p.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := p.State()
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "payment"); err != nil {
			return err
		}
		if s.Kind == vo.PaymentSupplier {
			if err := uc.payables.Unsettle(ctx, in.CompanyID, uuid.UUID(s.ID)); err != nil {
				return err
			}
		}
		return uc.payments.UpdateStatus(ctx, p)
	})
	if err != nil {
		return dto.Payment{}, err
	}
	return uc.payments.Get(ctx, in.CompanyID, in.ID)
}

// ---------- โอนเงินระหว่างบัญชี ----------

type TransferUseCases struct {
	base
	transfers repositories.TransferRepository
}

func (uc *TransferUseCases) Get(ctx context.Context, in dto.ByDoc[vo.TransferID]) (dto.Transfer, error) {
	return read(ctx, uc.base, in.Actor, vo.PermTransferRead, func() (dto.Transfer, error) { return uc.transfers.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *TransferUseCases) List(ctx context.Context, a dto.Actor, f dto.DocumentFilter) (listing.Result[dto.Transfer], error) {
	return read(ctx, uc.base, a, vo.PermTransferRead, func() (listing.Result[dto.Transfer], error) { return uc.transfers.List(ctx, a.CompanyID, f) })
}

// Create: ทะเบียนทั้งสองฝั่ง -> งวดเปิด + เลขที่ -> ลงบัญชี Dr ปลายทาง / Cr ต้นทาง -> บันทึก (transaction เดียว)
func (uc *TransferUseCases) Create(ctx context.Context, a dto.Actor, in dto.TransferInput) (dto.Transfer, error) {
	var id vo.TransferID
	err := uc.writeWith(ctx, a, vo.PermTransferPost, func(ctx context.Context, cs ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, a.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("transfer_journal_id"); err != nil {
			return err
		}
		ms, err := uc.methods(ctx, a.CompanyID, []dto.MethodInput{{CashAccountID: in.FromID, Amount: in.Amount}, {CashAccountID: in.ToID, Amount: in.Amount}})
		if err != nil {
			return err
		}
		posting, err := uc.posting(ctx, a, vo.DocTransfer, in.Date)
		if err != nil {
			return err
		}
		t, err := entities.NewTransfer(entities.TransferState{CompanyID: a.CompanyID, Date: in.Date, Description: in.Description,
			Reference: in.Reference, From: ms[0], To: ms[1], Posting: posting}, cs.AmountDecimals)
		if err != nil {
			return err
		}
		entryID, entryNo, err := uc.post(ctx, a, *settings.TransferJournalID, in.Date, "Cash transfer "+posting.DocumentNo, posting.DocumentNo,
			ports.LedgerEntry{Lines: t.Journal()})
		if err != nil {
			return err
		}
		if entryID != nil {
			t.SetEntry(*entryID, entryNo)
		}
		id = t.ID()
		return uc.transfers.Create(ctx, t)
	})
	if err != nil {
		return dto.Transfer{}, err
	}
	return uc.transfers.Get(ctx, a.CompanyID, id)
}

func (uc *TransferUseCases) Cancel(ctx context.Context, in dto.ByDoc[vo.TransferID]) (dto.Transfer, error) {
	err := uc.write(ctx, in.Actor, vo.PermTransferPost, func(ctx context.Context) error {
		t, err := uc.transfers.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := t.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := t.State()
		if err := uc.unpost(ctx, in.Actor, s.Posting, s.Date, "transfer"); err != nil {
			return err
		}
		return uc.transfers.UpdateStatus(ctx, t)
	})
	if err != nil {
		return dto.Transfer{}, err
	}
	return uc.transfers.Get(ctx, in.CompanyID, in.ID)
}
