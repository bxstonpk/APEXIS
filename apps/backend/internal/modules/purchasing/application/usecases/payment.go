package usecases

import (
	"context"
	"fmt"
	"slices"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	"apexis/backend/internal/modules/purchasing/domain/entities"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	"apexis/backend/internal/modules/purchasing/domain/services"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ---------- ประเภทภาษีหัก ณ ที่จ่าย (สิทธิ์ของค่าตั้งจัดซื้อ) ----------

type WhtTypeUseCases struct {
	base
	repo repositories.WhtTypeRepository
}

func (uc *WhtTypeUseCases) List(ctx context.Context, in dto.List[dto.WhtTypeFilter]) (listing.Result[dto.WhtType], error) {
	return read(ctx, uc.base, in.Actor, vo.PermSettingsRead, func() (listing.Result[dto.WhtType], error) {
		return uc.repo.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *WhtTypeUseCases) Get(ctx context.Context, in dto.ByID[vo.WhtTypeID]) (dto.WhtType, error) {
	return read(ctx, uc.base, in.Actor, vo.PermSettingsRead, func() (dto.WhtType, error) {
		w, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return dto.WhtType{}, err
		}
		return whtDTO(w), nil
	})
}

func (uc *WhtTypeUseCases) Create(ctx context.Context, in dto.Create[entities.WhtTypeData]) (dto.WhtType, error) {
	w, err := entities.NewWhtType(in.CompanyID, in.Data)
	if err != nil {
		return dto.WhtType{}, err
	}
	if err := uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error { return uc.repo.Create(ctx, w) }); err != nil {
		return dto.WhtType{}, err
	}
	return whtDTO(w), nil
}

// Update: อัตราใหม่มีผลกับใบจ่ายหลังจากนี้ (ใบจ่ายเดิมเก็บอัตรา ณ วันจ่ายไว้แล้ว)
func (uc *WhtTypeUseCases) Update(ctx context.Context, in dto.Update[vo.WhtTypeID, entities.WhtTypeData]) (dto.WhtType, error) {
	var out dto.WhtType
	err := uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		w, err := uc.repo.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := w.Update(in.Data); err != nil {
			return err
		}
		out = whtDTO(w)
		return uc.repo.Update(ctx, w)
	})
	return out, err
}

// Delete: soft delete — ใบจ่ายเดิมยังอ้างถึงได้
func (uc *WhtTypeUseCases) Delete(ctx context.Context, in dto.ByID[vo.WhtTypeID]) error {
	return uc.write(ctx, in.Actor, vo.PermSettingsManage, func(ctx context.Context) error {
		return uc.repo.Delete(ctx, in.CompanyID, in.ID)
	})
}

func whtDTO(w *entities.WhtType) dto.WhtType {
	d := w.Data()
	return dto.WhtType{ID: w.ID().String(), Code: d.Code, Name: d.Name, Rate: d.Rate, Status: string(d.Status)}
}

// ---------- ใบจ่ายชำระ ----------

type PaymentUseCases struct {
	base
	payments  repositories.PaymentRepository
	invoices  repositories.InvoiceRepository
	suppliers repositories.SupplierRepository
	whtTypes  repositories.WhtTypeRepository
	cashbank  ports.CashBank
}

func (uc *PaymentUseCases) Get(ctx context.Context, in dto.ByID[vo.PaymentID]) (dto.Payment, error) {
	return read(ctx, uc.base, in.Actor, vo.PermPaymentRead, func() (dto.Payment, error) { return uc.payments.Get(ctx, in.CompanyID, in.ID) })
}

func (uc *PaymentUseCases) List(ctx context.Context, in dto.List[dto.DocumentFilter]) (listing.Result[dto.PaymentSummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermPaymentRead, func() (listing.Result[dto.PaymentSummary], error) {
		return uc.payments.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create จ่ายชำระใน transaction เดียว:
// lock ใบแจ้งหนี้ (เรียง id) -> ตรวจยอดค้าง -> ภาษีหัก ณ ที่จ่ายตามประเภท (อัตรา ณ วันจ่าย)
// -> ทะเบียนเงินสด / ธนาคาร (cashbank) -> งวดต้องเปิด -> เลขที่ -> ลงบัญชี 1 รายการ -> บันทึกยอดจ่ายในใบแจ้งหนี้
func (uc *PaymentUseCases) Create(ctx context.Context, in dto.Create[dto.PaymentInput]) (dto.Payment, error) {
	var id vo.PaymentID
	err := uc.writeWith(ctx, in.Actor, vo.PermPaymentPost, func(ctx context.Context, s ports.CompanySettings) error {
		settings, err := uc.settings.Get(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if err := settings.Require("journal_id", "ap_account_id"); err != nil {
			return err
		}
		supplier, err := uc.suppliers.FindByID(ctx, in.CompanyID, in.Data.SupplierID)
		if err != nil {
			return err
		}
		if !vo.IsDateOnly(in.Data.Date) {
			return domain.ErrInvalidDate
		}

		invoiceIDs := make([]vo.InvoiceID, 0, len(in.Data.Lines))
		var whtIDs []vo.WhtTypeID
		for _, l := range in.Data.Lines {
			invoiceIDs = append(invoiceIDs, l.InvoiceID)
			if l.WhtTypeID != nil && !slices.Contains(whtIDs, *l.WhtTypeID) {
				whtIDs = append(whtIDs, *l.WhtTypeID)
			}
		}
		invoices, err := uc.invoices.LockForPayment(ctx, in.CompanyID, invoiceIDs)
		if err != nil {
			return err
		}
		byInvoice := make(map[vo.InvoiceID]*entities.Invoice, len(invoices))
		for _, inv := range invoices {
			byInvoice[inv.ID()] = inv
		}
		whtTypes, err := uc.whtTypes.FindByIDs(ctx, in.CompanyID, whtIDs)
		if err != nil {
			return err
		}
		byWht := make(map[vo.WhtTypeID]*entities.WhtType, len(whtTypes))
		for _, w := range whtTypes {
			byWht[w.ID()] = w
		}

		lines := make([]entities.PaymentLine, len(in.Data.Lines))
		for i, l := range in.Data.Lines {
			inv, ok := byInvoice[l.InvoiceID]
			if !ok {
				return fmt.Errorf("%w: line %d", domain.ErrInvoiceNotFound, i+1)
			}
			if !numericAmount(l.Amount, s.AmountDecimals) {
				return fmt.Errorf("%w: line %d amount must be positive with at most %d decimal places", domain.ErrInvalidPayment, i+1, s.AmountDecimals)
			}
			if err := inv.Pay(supplier.ID(), l.Amount); err != nil {
				return err
			}
			line := entities.PaymentLine{InvoiceID: l.InvoiceID, Amount: l.Amount, WhtRate: zero, WhtBase: zero, Wht: zero}
			if l.WhtTypeID != nil {
				w, ok := byWht[*l.WhtTypeID]
				switch {
				case !ok:
					return fmt.Errorf("%w: line %d", domain.ErrWhtTypeNotFound, i+1)
				case !w.IsActive():
					return fmt.Errorf("%w: withholding tax type %s", domain.ErrInactiveRef, w.Data().Code)
				}
				st := inv.State()
				line.WhtTypeID, line.WhtRate = l.WhtTypeID, w.Data().Rate
				line.WhtBase, line.Wht = services.Withholding(l.Amount, st.Totals.Net, st.Totals.Gross, line.WhtRate, s.AmountDecimals)
			}
			lines[i] = line
		}

		cashIDs := make([]vo.CashAccountID, len(in.Data.Methods))
		for i, m := range in.Data.Methods {
			cashIDs[i] = m.CashAccountID
		}
		cash, err := uc.cashbank.CashAccounts(ctx, in.CompanyID, cashIDs)
		if err != nil {
			return err
		}
		methods := make([]entities.PaymentMethod, len(in.Data.Methods))
		for i, m := range in.Data.Methods {
			c, ok := cash[m.CashAccountID]
			switch {
			case !ok:
				return fmt.Errorf("%w: method %d", domain.ErrCashAccountNotFound, i+1)
			case !c.Active:
				return fmt.Errorf("%w: cash / bank account %s", domain.ErrInactiveRef, c.Code)
			case !numericAmount(m.Amount, s.AmountDecimals):
				return fmt.Errorf("%w: method %d amount must be positive with at most %d decimal places", domain.ErrInvalidPayment, i+1, s.AmountDecimals)
			}
			methods[i] = entities.PaymentMethod{CashAccountID: m.CashAccountID, GLAccountID: c.GLAccountID, Amount: m.Amount, Reference: m.Reference}
		}

		period, err := uc.openPeriod(ctx, in.CompanyID, in.Data.Date)
		if err != nil {
			return err
		}
		no, err := uc.number(ctx, in.CompanyID, vo.DocPayment, in.Data.Date, period.FiscalYear)
		if err != nil {
			return err
		}
		p, err := entities.NewPayment(entities.PaymentState{
			CompanyID: in.CompanyID, SupplierID: supplier.ID(), TaxType: supplier.Data().TaxType, Date: in.Data.Date,
			Description: in.Data.Description, Lines: lines, Methods: methods,
			Posting: entities.Posting{DocumentNo: no, FiscalYear: period.FiscalYear, PeriodID: period.ID, By: in.UserID, At: uc.now()},
		})
		if err != nil {
			return err
		}
		whtAccount := vo.AccountID{}
		if !p.State().Totals.Wht.IsZero() {
			acc, field := settings.WhtAccount(supplier.Data().TaxType)
			if acc == nil {
				return settings.Require(field)
			}
			whtAccount = *acc
		}
		entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
			CompanyID: in.CompanyID, UserID: in.UserID, JournalID: *settings.JournalID, Date: in.Data.Date,
			Description: "Supplier payment " + no + " / " + supplier.Data().Code, Reference: no,
			Lines: p.Journal(*settings.APAccountID, whtAccount),
		})
		if err != nil {
			return err
		}
		p.SetEntry(entryID, entryNo)
		id = p.ID()
		if err := uc.payments.Create(ctx, p); err != nil {
			return err
		}
		return uc.invoices.SavePaid(ctx, invoices)
	})
	if err != nil {
		return dto.Payment{}, err
	}
	return uc.payments.Get(ctx, in.CompanyID, id)
}

// Cancel: กลับรายการบัญชี ณ วันที่จ่ายเดิม (งวดต้องเปิด) และคืนยอดค้างให้ใบแจ้งหนี้
func (uc *PaymentUseCases) Cancel(ctx context.Context, in dto.ByID[vo.PaymentID]) (dto.Payment, error) {
	err := uc.write(ctx, in.Actor, vo.PermPaymentPost, func(ctx context.Context) error {
		p, err := uc.payments.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := p.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		s := p.State()
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.Date); err != nil {
			return err
		}
		ids := make([]vo.InvoiceID, len(s.Lines))
		amounts := make(map[vo.InvoiceID]vo.Money, len(s.Lines))
		for i, l := range s.Lines {
			ids[i], amounts[l.InvoiceID] = l.InvoiceID, l.Amount
		}
		invoices, err := uc.invoices.LockForPayment(ctx, in.CompanyID, ids)
		if err != nil {
			return err
		}
		for _, inv := range invoices {
			inv.Unpay(amounts[inv.ID()])
		}
		if s.Posting.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *s.Posting.EntryID, s.Date, "Cancel supplier payment "+s.Posting.DocumentNo); err != nil {
				return err
			}
		}
		if err := uc.payments.UpdateStatus(ctx, p); err != nil {
			return err
		}
		return uc.invoices.SavePaid(ctx, invoices)
	})
	if err != nil {
		return dto.Payment{}, err
	}
	return uc.payments.Get(ctx, in.CompanyID, in.ID)
}
