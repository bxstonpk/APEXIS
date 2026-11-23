// Package gormpersistence คือ adapter ระหว่าง domain กับ gorm ของ finance
package gormpersistence

import (
	"github.com/google/uuid"

	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/modules/finance/infrastructure/persistence/gorm/models"
)

func ToModel(a *entities.CashAccount) models.CashAccountModel {
	d := a.Data()
	return models.CashAccountModel{
		ID: uuid.UUID(a.ID()), CompanyID: uuid.UUID(a.CompanyID()), Code: d.Code, Name: d.Name, Type: string(d.Type),
		BankName: d.BankName, BankBranch: d.BankBranch, AccountNo: d.AccountNo, GLAccountID: uuid.UUID(d.GLAccountID),
		Status: string(d.Status),
	}
}

func ToEntity(m models.CashAccountModel) *entities.CashAccount {
	return entities.RehydrateCashAccount(vo.CashAccountID(m.ID), vo.CompanyID(m.CompanyID), entities.CashAccountData{
		Code: m.Code, Name: m.Name, Type: vo.AccountType(m.Type), BankName: m.BankName, BankBranch: m.BankBranch,
		AccountNo: m.AccountNo, GLAccountID: vo.GLAccountID(m.GLAccountID), Status: vo.Status(m.Status),
	})
}
