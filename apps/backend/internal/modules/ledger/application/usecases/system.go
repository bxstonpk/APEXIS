package usecases

import (
	"context"
	"errors"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// ---------- การลงบัญชีจาก module อื่น (เรียกผ่าน ledger.Module เท่านั้น ไม่มี HTTP route) ----------
//
// ไม่ตรวจสิทธิ์ของ ledger — module ผู้เรียก (เช่น จัดซื้อ) ตรวจสิทธิ์ของเอกสารตัวเองแล้ว
// แต่ยังบังคับกฎบัญชีครบ: บริษัท active, ทศนิยมตามบริษัท, บัญชี / สมุด active, สมดุล, งวดต้องเปิด, เลขที่เรียง
// ทำงานใน transaction ของผู้เรียก (ctx) — ผู้เรียกล้มเหลว = รายการบัญชีถูก rollback ไปด้วย

// systemWrite: transaction (เข้าร่วมของผู้เรียก) -> บริษัทต้อง active -> fn
func (b base) systemWrite(ctx context.Context, companyID vo.CompanyID, fn func(ctx context.Context, s ports.CompanySettings) error) error {
	return b.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		s, err := b.org.CompanySettings(ctx, companyID)
		if err != nil {
			return err
		}
		if !s.Active {
			return domain.ErrCompanyInactive
		}
		return fn(ctx, s)
	})
}

// PostSystem สร้างรายการแล้วผ่านทันที — คืน id และเลขที่
func (uc *EntryUseCases) PostSystem(ctx context.Context, a dto.Actor, d entities.EntryData) (vo.EntryID, string, error) {
	var e *entities.Entry
	err := uc.systemWrite(ctx, a.CompanyID, func(ctx context.Context, s ports.CompanySettings) error {
		var err error
		if e, err = entities.NewEntry(a.CompanyID, a.UserID, d, s.AmountDecimals); err != nil {
			return err
		}
		journal, err := uc.checkRefs(ctx, a.CompanyID, e, true)
		if err != nil {
			return err
		}
		if err := uc.post(ctx, a, e, journal); err != nil {
			return err
		}
		return uc.entries.Create(ctx, e)
	})
	if err != nil {
		return vo.EntryID{}, "", err
	}
	return e.ID(), e.State().Posting.EntryNo, nil
}

// ReverseSystem กลับรายการที่ module อื่นเคยลงไว้ (เช่น ยกเลิกใบรับสินค้า)
func (uc *EntryUseCases) ReverseSystem(ctx context.Context, a dto.Actor, id vo.EntryID, date time.Time, description string) (vo.EntryID, string, error) {
	var r *entities.Entry
	err := uc.systemWrite(ctx, a.CompanyID, func(ctx context.Context, _ ports.CompanySettings) error {
		var err error
		r, err = uc.reverse(ctx, a, id, date, description)
		return err
	})
	if err != nil {
		return vo.EntryID{}, "", err
	}
	return r.ID(), r.State().Posting.EntryNo, nil
}

// InvalidPostingAccounts คืน id ของบัญชีที่ใช้ลงรายการไม่ได้ (ไม่พบ / บัญชีคุม / inactive) — query เดียว
func (uc *AccountUseCases) InvalidPostingAccounts(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) ([]vo.AccountID, error) {
	accounts, err := uc.accounts.FindByIDs(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	ok := map[vo.AccountID]bool{}
	for _, a := range accounts {
		ok[a.ID()] = a.CanPost()
	}
	var bad []vo.AccountID
	for _, id := range ids {
		if !ok[id] {
			bad = append(bad, id)
		}
	}
	return bad, nil
}

// Balances คือยอด (เดบิต - เครดิต) สะสมถึงวันที่ ของบัญชีที่ขอ — query เดียว
func (uc *ReportUseCases) Balances(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID, asOf time.Time) (map[vo.AccountID]vo.Money, error) {
	sums, err := uc.reports.SumByAccount(ctx, companyID, nil, []repositories.Bucket{{To: asOf}})
	if err != nil {
		return nil, err
	}
	out := make(map[vo.AccountID]vo.Money, len(ids))
	for _, id := range ids {
		if s, ok := sums[id]; ok {
			out[id] = s[0].Net()
		} else {
			out[id] = vo.MustMoney("0")
		}
	}
	return out, nil
}

// JournalUsable: สมุดรายวันมีอยู่และ active (ใช้ตรวจค่าตั้งของ module อื่น)
func (uc *JournalUseCases) JournalUsable(ctx context.Context, companyID vo.CompanyID, id vo.JournalID) (bool, error) {
	j, err := uc.journals.FindByID(ctx, companyID, id)
	if errors.Is(err, domain.ErrJournalNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return j.IsActive(), nil
}
