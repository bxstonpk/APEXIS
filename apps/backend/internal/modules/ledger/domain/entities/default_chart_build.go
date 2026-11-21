package entities

import (
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// NewDefaultChart สร้างผังบัญชีมาตรฐานทั้งชุดของบริษัท (เรียงบัญชีคุมก่อนบัญชีย่อย)
// ทุกบัญชีผ่านกฎเดียวกับการสร้างทีละบัญชี (รูปแบบรหัส, parent ต้องเป็นบัญชีคุมหมวดเดียวกัน)
func NewDefaultChart(companyID vo.CompanyID) ([]*Account, error) {
	byCode := make(map[string]*Account, len(DefaultChart))
	out := make([]*Account, 0, len(DefaultChart))
	for _, d := range DefaultChart {
		var parent *Account
		var parentID *vo.AccountID
		if d.ParentCode != "" {
			if parent = byCode[d.ParentCode]; parent == nil {
				return nil, domain.ErrAccountNotFound
			}
			id := parent.ID()
			parentID = &id
		}
		a, err := NewAccount(companyID, AccountData{
			Code: d.Code, Name: d.Name, NameEN: d.NameEN, Type: d.Type, ParentID: parentID,
			Postable: d.Postable, Status: vo.StatusActive, Description: d.Description,
		}, parent)
		if err != nil {
			return nil, err
		}
		byCode[d.Code] = a
		out = append(out, a)
	}
	return out, nil
}
