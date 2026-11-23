// Package requests คือ JSON / URL query ขาเข้าของ finance
package requests

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"apexis/backend/internal/modules/finance/application/dto"
	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

var ErrValidation = errors.New("validation failed")

// Invalid: error ข้อมูลขาเข้าไม่ถูกต้อง (400) — ใช้จาก handler ได้
func Invalid(format string, args ...any) error { return invalid(format, args...) }

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

// CashAccount: POST / PUT /cash-accounts
type CashAccount struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	BankName    string `json:"bank_name"`
	BankBranch  string `json:"bank_branch"`
	AccountNo   string `json:"account_no"`
	GLAccountID string `json:"gl_account_id"`
	Status      string `json:"status"`
}

func (r CashAccount) ToData(isCreate bool) (entities.CashAccountData, error) {
	var missing []string
	for name, v := range map[string]string{"code": r.Code, "name": r.Name, "type": r.Type, "gl_account_id": r.GLAccountID} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return entities.CashAccountData{}, invalid("%s is required", strings.Join(missing, ", "))
	}
	gl, err := vo.ParseID[vo.GLAccountID](r.GLAccountID)
	if err != nil {
		return entities.CashAccountData{}, invalid("gl_account_id must be a UUID")
	}
	st := vo.Status(r.Status)
	if r.Status == "" {
		if !isCreate {
			return entities.CashAccountData{}, invalid("status is required")
		}
		st = vo.StatusActive
	}
	return entities.CashAccountData{
		Code: r.Code, Name: r.Name, Type: vo.AccountType(r.Type), BankName: r.BankName, BankBranch: r.BankBranch,
		AccountNo: r.AccountNo, GLAccountID: gl, Status: st,
	}, nil
}

// ParseFilter: GET /cash-accounts?q=&type=cash,bank&status=&sort=code|name&page=&page_size=
func ParseFilter(v url.Values) (dto.Filter, error) {
	var f dto.Filter
	var err error
	if f.Search, err = listing.ParseSearch(v); err != nil {
		return f, err
	}
	if f.Types, err = listing.ParseList(v, "type", vo.ParseAccountType); err != nil {
		return f, err
	}
	if f.Statuses, err = listing.ParseList(v, "status", vo.ParseStatus); err != nil {
		return f, err
	}
	if f.Sort, err = listing.ParseSort(v, dto.SortKeys, dto.DefaultSort); err != nil {
		return f, err
	}
	f.Page, err = listing.ParsePage(v)
	return f, err
}
