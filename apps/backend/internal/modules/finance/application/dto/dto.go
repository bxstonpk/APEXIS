// Package dto คือ input / filter / read model ของ finance
package dto

import (
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
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

// ByDoc คือ input แบบ actor + id ของเอกสาร / ข้อมูลอื่นที่ไม่ใช่ทะเบียนเงิน
type ByDoc[ID any] struct {
	Actor
	ID ID
}
