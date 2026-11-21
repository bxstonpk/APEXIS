// Package responses คือ JSON ขาออกของ cashbank
package responses

import "apexis/backend/internal/modules/cashbank/application/dto"

type CashAccount struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	BankName    string `json:"bank_name"`
	BankBranch  string `json:"bank_branch"`
	AccountNo   string `json:"account_no"`
	GLAccountID string `json:"gl_account_id"`
	Status      string `json:"status"`
}

func From(a dto.CashAccount) CashAccount {
	return CashAccount{
		ID: a.ID, Code: a.Code, Name: a.Name, Type: a.Type, BankName: a.BankName, BankBranch: a.BankBranch,
		AccountNo: a.AccountNo, GLAccountID: a.GLAccountID, Status: a.Status,
	}
}
