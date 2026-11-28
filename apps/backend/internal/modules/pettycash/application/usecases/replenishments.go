package usecases

import (
	"context"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type ReplenishmentUseCases struct{ base }

func (uc *ReplenishmentUseCases) List(ctx context.Context, a dto.Actor, f dto.ReplenishmentFilter) (listing.Result[dto.ReplenishmentSummary], error) {
	return read(ctx, uc.base, a, vo.PermReplenishmentRead, func() (listing.Result[dto.ReplenishmentSummary], error) {
		return uc.requests.List(ctx, a.CompanyID, f)
	})
}

func (uc *ReplenishmentUseCases) Get(ctx context.Context, a dto.Actor, id vo.ReplenishmentID) (dto.Replenishment, error) {
	return read(ctx, uc.base, a, vo.PermReplenishmentRead, func() (dto.Replenishment, error) { return uc.get(ctx, a.CompanyID, id) })
}

func (uc *ReplenishmentUseCases) get(ctx context.Context, companyID vo.CompanyID, id vo.ReplenishmentID) (dto.Replenishment, error) {
	r, err := uc.requests.Get(ctx, companyID, id)
	if err != nil {
		return r, err
	}
	return r, uc.userRefs(ctx, companyID, []*dto.Ref{&r.CreatedBy})
}

// Create ขอตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย (lock กองทุนเพื่อเรียงคิวคำขอของกองทุนเดียวกัน)
//   - establish: กองทุนยังไม่ได้ตั้งวงเงิน และไม่มีคำขอตั้ง / เพิ่มวงเงินค้าง — ยอด = วงเงินของกองทุน
//   - increase: ตั้งวงเงินแล้ว และไม่มีคำขอตั้ง / เพิ่มวงเงินค้าง
//   - replenish: ใบสำคัญจ่ายที่อนุมัติแล้วของกองทุนนี้ที่ยังไม่อยู่ในคำขออื่น (ยอด = ผลรวมยอดรวม)
func (uc *ReplenishmentUseCases) Create(ctx context.Context, in dto.ReplenishmentInput) (dto.Replenishment, error) {
	var id vo.ReplenishmentID
	err := uc.writeWith(ctx, in.Actor, vo.PermReplenishmentPost, func(ctx context.Context, dec int) error {
		f, err := uc.funds.FindForUpdate(ctx, in.CompanyID, in.FundID)
		if err != nil {
			return err
		}
		if !f.IsActive() {
			return domain.ErrFundInactive
		}
		bal, err := uc.funds.Balances(ctx, in.CompanyID, []vo.FundID{f.ID()}, nil)
		if err != nil {
			return err
		}
		funded := bal[f.ID()].Funded
		no, err := uc.number(ctx, in.CompanyID, vo.DocReplenishment, in.Date)
		if err != nil {
			return err
		}
		s := entities.ReplenishmentState{CompanyID: in.CompanyID, DocumentNo: no, FundID: in.FundID, Kind: in.Kind, Date: in.Date,
			Description: in.Description, Amount: in.Amount, CreatedBy: in.UserID, CreatedAt: uc.now()}
		var r *entities.Replenishment
		switch in.Kind {
		case vo.RequestEstablish, vo.RequestIncrease:
			open, err := uc.requests.HasOpenFunding(ctx, in.CompanyID, f.ID())
			if err != nil {
				return err
			}
			switch {
			case open:
				return domain.ErrOpenRequestExists
			case in.Kind == vo.RequestEstablish && funded:
				return domain.ErrFundAlreadyFunded
			case in.Kind == vo.RequestIncrease && !funded:
				return domain.ErrFundNotFunded
			}
			if in.Kind == vo.RequestEstablish {
				s.Amount = f.Imprest()
			}
			if r, err = entities.NewFundingRequest(s, dec); err != nil {
				return err
			}
		case vo.RequestReplenish:
			vouchers, err := uc.vouchers.FindManyForUpdate(ctx, in.CompanyID, in.VoucherIDs)
			if err != nil {
				return err
			}
			if r, err = entities.NewReplenishRequest(s, vouchers); err != nil {
				return err
			}
			if err := uc.requests.Create(ctx, r); err != nil {
				return err
			}
			id = r.ID()
			return uc.vouchers.UpdateMany(ctx, vouchers)
		default:
			return domain.ErrInvalidKind
		}
		id = r.ID()
		return uc.requests.Create(ctx, r)
	})
	if err != nil {
		return dto.Replenishment{}, err
	}
	return uc.get(ctx, in.CompanyID, id)
}

// Cancel: ยกเลิกคำขอที่การเงินยังไม่จ่าย — ใบสำคัญจ่ายในคำขอกลับไปรอขอเบิกใหม่
func (uc *ReplenishmentUseCases) Cancel(ctx context.Context, in dto.ByReplenishment) (dto.Replenishment, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermReplenishmentPost, func(ctx context.Context, _ int) error {
		r, err := uc.requests.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := r.Cancel(uc.now()); err != nil {
			return err
		}
		if r.State().Kind == vo.RequestReplenish {
			vouchers, err := uc.vouchers.ByReplenishmentForUpdate(ctx, in.CompanyID, r.ID())
			if err != nil {
				return err
			}
			for _, v := range vouchers {
				v.Detach()
			}
			if err := uc.vouchers.UpdateMany(ctx, vouchers); err != nil {
				return err
			}
		}
		return uc.requests.Update(ctx, r)
	})
	if err != nil {
		return dto.Replenishment{}, err
	}
	return uc.get(ctx, in.CompanyID, in.ID)
}
