package usecases

import (
	"context"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// AccountUseCases จัดการผังบัญชี
type AccountUseCases struct {
	base
	accounts repositories.AccountRepository
	journals repositories.JournalRepository
}

// Tree ผังบัญชีทั้งชุด (ทุกสถานะ) เรียงตามรหัส — 1 query
func (uc *AccountUseCases) Tree(ctx context.Context, in dto.Actor) ([]dto.Account, error) {
	return read(ctx, uc.base, in, vo.PermAccountRead, func() ([]dto.Account, error) {
		return uc.accounts.Tree(ctx, in.CompanyID)
	})
}

// CreateDefaults สร้างผังบัญชีมาตรฐานตามประเภทธุรกิจของบริษัท + สมุดรายวันมาตรฐาน (GEN / CRJ / CDJ) ใน transaction เดียว
//
// ทำได้เฉพาะบริษัทที่ยังไม่มีบัญชี (กันผังซ้อนกับผังที่ตั้งเอง) — สมุดที่มีรหัสอยู่แล้วจะข้าม
// บริษัทใหม่ได้ผังนี้อัตโนมัติตอนสร้างบริษัท (Provision) — endpoint นี้ใช้กับบริษัทเดิมที่ยังไม่มีผังบัญชี
// query: ประเภทธุรกิจ 1 + ตรวจบัญชี 1 + INSERT บัญชีแบบ batch + ตรวจรหัสสมุด 1 + INSERT สมุด (ไม่เกิน 3)
func (uc *AccountUseCases) CreateDefaults(ctx context.Context, in dto.Actor) (dto.DefaultChartResult, error) {
	var out dto.DefaultChartResult
	err := uc.write(ctx, in, vo.PermAccountManage, func(ctx context.Context) error {
		bt, err := uc.org.BusinessType(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		out, err = uc.seed(ctx, in.CompanyID, bt)
		return err
	})
	return out, err
}

// Provision สร้างผังบัญชีมาตรฐานของประเภทธุรกิจ bt + สมุดรายวันมาตรฐานให้บริษัทที่เพิ่งสร้าง
// เรียกจาก hook ตอนสร้างบริษัท (organization) — ไม่ตรวจสิทธิ์ (ระบบทำให้) และเข้าร่วม transaction ของผู้เรียกใน ctx
func (uc *AccountUseCases) Provision(ctx context.Context, companyID vo.CompanyID, bt vo.BusinessType) (dto.DefaultChartResult, error) {
	var out dto.DefaultChartResult
	err := uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		out, err = uc.seed(ctx, companyID, bt)
		return err
	})
	return out, err
}

func (uc *AccountUseCases) seed(ctx context.Context, companyID vo.CompanyID, bt vo.BusinessType) (dto.DefaultChartResult, error) {
	var out dto.DefaultChartResult
	has, err := uc.accounts.HasAny(ctx, companyID)
	if err != nil {
		return out, err
	}
	if has {
		return out, domain.ErrChartExists
	}
	chart, err := entities.NewDefaultChart(companyID, bt)
	if err != nil {
		return out, err
	}
	if err := uc.accounts.CreateMany(ctx, chart); err != nil {
		return out, err
	}
	out.Accounts = len(chart)

	codes := make([]string, len(entities.DefaultJournals))
	for i, j := range entities.DefaultJournals {
		codes[i] = j.Code
	}
	existing, err := uc.journals.ExistingCodes(ctx, companyID, codes)
	if err != nil {
		return out, err
	}
	for _, d := range entities.DefaultJournals {
		if existing[d.Code] {
			continue
		}
		j, err := entities.NewJournal(companyID, entities.JournalData{Code: d.Code, Name: d.Name, Status: vo.StatusActive})
		if err != nil {
			return out, err
		}
		if err := uc.journals.Create(ctx, j); err != nil {
			return out, err
		}
		out.Journals++
	}
	return out, nil
}

// PostableIDsByCode / JournalIDsByCode: id ตามรหัส (module อื่นใช้ตั้งค่าเริ่มต้นจากผังมาตรฐาน) — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ใน ctx
func (uc *AccountUseCases) PostableIDsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.AccountID, error) {
	return uc.accounts.PostableIDsByCode(ctx, companyID, codes)
}

// AccountRefs / JournalRefs: รหัส / ชื่อตาม id ให้ module อื่นแสดงในเอกสาร — ไม่ตรวจสิทธิ์ (ผู้เรียกตรวจสิทธิ์เอกสารของตัวเองแล้ว)
func (uc *AccountUseCases) AccountRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.AccountID) (map[vo.AccountID]dto.CodeName, error) {
	return uc.accounts.RefsByIDs(ctx, companyID, ids)
}

func (uc *AccountUseCases) JournalRefs(ctx context.Context, companyID vo.CompanyID, ids []vo.JournalID) (map[vo.JournalID]dto.CodeName, error) {
	return uc.journals.RefsByIDs(ctx, companyID, ids)
}

func (uc *AccountUseCases) JournalIDsByCode(ctx context.Context, companyID vo.CompanyID, codes []string) (map[string]vo.JournalID, error) {
	return uc.journals.ActiveIDsByCode(ctx, companyID, codes)
}

func (uc *AccountUseCases) Get(ctx context.Context, in dto.ByID[vo.AccountID]) (dto.Account, error) {
	return read(ctx, uc.base, in.Actor, vo.PermAccountRead, func() (dto.Account, error) {
		return uc.accounts.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *AccountUseCases) List(ctx context.Context, in dto.List[dto.AccountFilter]) (listing.Result[dto.Account], error) {
	return read(ctx, uc.base, in.Actor, vo.PermAccountRead, func() (listing.Result[dto.Account], error) {
		return uc.accounts.List(ctx, in.CompanyID, in.Filter)
	})
}

func (uc *AccountUseCases) Create(ctx context.Context, in dto.Create[entities.AccountData]) (dto.Account, error) {
	var id vo.AccountID
	err := uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		parent, err := uc.parent(ctx, in.CompanyID, in.Data.ParentID)
		if err != nil {
			return err
		}
		a, err := entities.NewAccount(in.CompanyID, in.Data, parent)
		if err != nil {
			return err
		}
		id = a.ID()
		return uc.accounts.Create(ctx, a)
	})
	if err != nil {
		return dto.Account{}, err
	}
	return uc.accounts.Get(ctx, in.CompanyID, id)
}

// Update: โหลดบัญชี + parent + การใช้งาน -> ตรวจลูปลำดับชั้น -> กฎใน entity -> บันทึก (transaction เดียว)
func (uc *AccountUseCases) Update(ctx context.Context, in dto.Update[vo.AccountID, entities.AccountData]) (dto.Account, error) {
	err := uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		a, err := uc.accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		parent, err := uc.parent(ctx, in.CompanyID, in.Data.ParentID)
		if err != nil {
			return err
		}
		if parent != nil {
			chain, err := uc.accounts.Ancestry(ctx, in.CompanyID, parent.ID())
			if err != nil {
				return err
			}
			if err := services.EnsureNoCycle(a.ID(), chain); err != nil {
				return err
			}
		}
		usage, err := uc.accounts.Usage(ctx, in.CompanyID, a.ID())
		if err != nil {
			return err
		}
		if err := a.Update(in.Data, parent, usage); err != nil {
			return err
		}
		return uc.accounts.Update(ctx, a)
	})
	if err != nil {
		return dto.Account{}, err
	}
	return uc.accounts.Get(ctx, in.CompanyID, in.ID)
}

// Delete: soft delete ได้เมื่อไม่มีบัญชีย่อยและไม่เคยถูกใช้ในรายการ
func (uc *AccountUseCases) Delete(ctx context.Context, in dto.ByID[vo.AccountID]) error {
	return uc.write(ctx, in.Actor, vo.PermAccountManage, func(ctx context.Context) error {
		a, err := uc.accounts.FindByID(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		usage, err := uc.accounts.Usage(ctx, in.CompanyID, a.ID())
		if err != nil {
			return err
		}
		if err := a.EnsureDeletable(usage); err != nil {
			return err
		}
		return uc.accounts.Delete(ctx, in.CompanyID, in.ID)
	})
}

func (uc *AccountUseCases) parent(ctx context.Context, companyID vo.CompanyID, id *vo.AccountID) (*entities.Account, error) {
	if id == nil {
		return nil, nil
	}
	return uc.accounts.FindByID(ctx, companyID, *id) // ไม่พบ = domain.ErrAccountNotFound
}
