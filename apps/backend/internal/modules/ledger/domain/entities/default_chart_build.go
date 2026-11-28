package entities

import (
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// NewDefaultChart สร้างผังบัญชีมาตรฐานของประเภทธุรกิจ bt ทั้งชุด (เรียงบัญชีคุมก่อนบัญชีย่อย)
// ทุกบัญชีผ่านกฎเดียวกับการสร้างทีละบัญชี (รูปแบบรหัส, parent ต้องเป็นบัญชีคุมหมวดเดียวกัน)
func NewDefaultChart(companyID vo.CompanyID, bt vo.BusinessType) ([]*Account, error) {
	if _, err := vo.ParseBusinessType(string(bt)); err != nil {
		return nil, err
	}
	chart := ChartFor(bt)
	byCode := make(map[string]*Account, len(chart))
	out := make([]*Account, 0, len(chart))
	for _, d := range chart {
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
