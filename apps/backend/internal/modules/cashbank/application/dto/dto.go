// Package dto คือ input / filter / read model ของ cashbank
package dto

import (
	vo "apexis/backend/internal/modules/cashbank/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

type Actor struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
}

type ByID struct {
	Actor
	ID vo.CashAccountID
}

var (
	SortKeys    = []string{"code", "name"}
	DefaultSort = []listing.Sort{{Key: "code"}}
)

// Filter: GET /cash-accounts?q=&type=cash,bank&status=&sort=&page=&page_size=
type Filter struct {
	Search   string
	Types    []vo.AccountType
	Statuses []vo.Status
	Sort     []listing.Sort
	Page     listing.Page
}

type CashAccount struct {
	ID          string
	Code        string
	Name        string
	Type        string
	BankName    string
	BankBranch  string
	AccountNo   string
	GLAccountID string
	Status      string
}
