// Package entities คือ entity ของ finance
package entities

import (
	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
)

type CashAccountData struct {
	Code        string
	Name        string
	Type        vo.AccountType
	BankName    string // เฉพาะ bank
	BankBranch  string
	AccountNo   string // เลขที่บัญชีธนาคาร
	GLAccountID vo.GLAccountID
	Status      vo.Status
}

// CashAccount คือทะเบียนเงินสด / บัญชีธนาคาร — ผูกกับผังบัญชีของ ledger 1 บัญชี
// เอกสารรับ / จ่ายเงินเลือกทะเบียนนี้ แล้วลงบัญชีที่ผูกไว้
type CashAccount struct {
	id        vo.CashAccountID
	companyID vo.CompanyID
	data      CashAccountData
}

func NewCashAccount(companyID vo.CompanyID, d CashAccountData) (*CashAccount, error) {
	n, err := normalize(d)
	if err != nil {
		return nil, err
	}
	return &CashAccount{id: vo.NewID[vo.CashAccountID](), companyID: companyID, data: n}, nil
}

func RehydrateCashAccount(id vo.CashAccountID, companyID vo.CompanyID, d CashAccountData) *CashAccount {
	return &CashAccount{id: id, companyID: companyID, data: d}
}

func (a *CashAccount) ID() vo.CashAccountID    { return a.id }
func (a *CashAccount) CompanyID() vo.CompanyID { return a.companyID }
func (a *CashAccount) Data() CashAccountData   { return a.data }

func (a *CashAccount) Update(d CashAccountData) error {
	n, err := normalize(d)
	if err != nil {
		return err
	}
	a.data = n
	return nil
}

func normalize(d CashAccountData) (CashAccountData, error) {
	var err error
	out := CashAccountData{GLAccountID: d.GLAccountID}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return CashAccountData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return CashAccountData{}, err
	}
	if out.Type, err = vo.ParseAccountType(string(d.Type)); err != nil {
		return CashAccountData{}, err
	}
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return CashAccountData{}, err
	}
	if out.BankName, err = vo.OptionalText(d.BankName, 100); err != nil {
		return CashAccountData{}, err
	}
	if out.BankBranch, err = vo.OptionalText(d.BankBranch, 100); err != nil {
		return CashAccountData{}, err
	}
	if out.AccountNo, err = vo.AccountNo(d.AccountNo); err != nil {
		return CashAccountData{}, err
	}
	if out.Type == vo.TypeBank && (out.BankName == "" || out.AccountNo == "") {
		return CashAccountData{}, domain.ErrBankDetails
	}
	return out, nil
}
