package usecases

import (
	"context"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// Templates: แบบรายการปรับปรุงพร้อมบัญชีตั้งต้นจากผังมาตรฐาน — 1 query (หาบัญชีตามรหัส)
func (uc *EntryUseCases) Templates(ctx context.Context, a dto.Actor) ([]dto.AdjustmentTemplate, error) {
	return read(ctx, uc.base, a, vo.PermEntryRead, func() ([]dto.AdjustmentTemplate, error) {
		var codes []string
		for _, t := range entities.TemplateAccounts {
			codes = append(codes, t.BalanceCode, t.ResultCode)
		}
		ids, err := uc.accounts.PostableIDsByCode(ctx, a.CompanyID, codes)
		if err != nil {
			return nil, err
		}
		var list []vo.AccountID
		for _, id := range ids {
			list = append(list, id)
		}
		refs, err := uc.accounts.RefsByIDs(ctx, a.CompanyID, list)
		if err != nil {
			return nil, err
		}
		ref := func(code string) *dto.Ref {
			id, ok := ids[code]
			if !ok {
				return nil
			}
			r := refs[id]
			return &dto.Ref{ID: id.String(), Code: r.Code, Name: r.Name}
		}
		out := make([]dto.AdjustmentTemplate, 0, len(entities.AdjustmentTemplates))
		for _, t := range entities.AdjustmentTemplates {
			row := dto.AdjustmentTemplate{Template: string(t), AutoReverse: t != entities.TemplateOther}
			if acc, ok := entities.TemplateAccounts[t]; ok {
				row.BalanceDebit, row.BalanceAccount, row.ResultAccount = acc.BalanceDebit, ref(acc.BalanceCode), ref(acc.ResultCode)
			}
			out = append(out, row)
		}
		return out, nil
	})
}
