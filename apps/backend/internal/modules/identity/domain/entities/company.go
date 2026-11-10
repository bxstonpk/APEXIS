package entities

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"account/backend/internal/modules/identity/domain"
	"account/backend/internal/modules/identity/domain/enums"
	vo "account/backend/internal/modules/identity/domain/valueobjects"
)

var (
	companyCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,19}$`)
	taxIDPattern       = regexp.MustCompile(`^[0-9A-Z-]{1,20}$`)
)

// Company คือ tenant — ข้อมูลธุรกิจทุกตัวในระบบ ERP ต้องผูกกับ CompanyID
type Company struct {
	id     vo.CompanyID
	code   string
	name   string
	taxID  string
	status enums.CompanyStatus
}

func NewCompany(code, name, taxID string) (*Company, error) {
	c, err := normalizeCompanyCode(code)
	if err != nil {
		return nil, err
	}
	n, err := normalizeCompanyName(name)
	if err != nil {
		return nil, err
	}
	t, err := normalizeTaxID(taxID)
	if err != nil {
		return nil, err
	}
	return &Company{id: vo.NewCompanyID(), code: c, name: n, taxID: t, status: enums.CompanyStatusActive}, nil
}

func RehydrateCompany(id vo.CompanyID, code, name, taxID string, status enums.CompanyStatus) *Company {
	return &Company{id: id, code: code, name: name, taxID: taxID, status: status}
}

func (c *Company) ID() vo.CompanyID            { return c.id }
func (c *Company) Code() string                { return c.code }
func (c *Company) Name() string                { return c.name }
func (c *Company) TaxID() string               { return c.taxID }
func (c *Company) Status() enums.CompanyStatus { return c.status }
func (c *Company) IsActive() bool              { return c.status == enums.CompanyStatusActive }

func (c *Company) UpdateProfile(name, taxID string) error {
	n, err := normalizeCompanyName(name)
	if err != nil {
		return err
	}
	t, err := normalizeTaxID(taxID)
	if err != nil {
		return err
	}
	c.name, c.taxID = n, t
	return nil
}

func (c *Company) Deactivate() { c.status = enums.CompanyStatusInactive }
func (c *Company) Activate()   { c.status = enums.CompanyStatusActive }

func normalizeCompanyCode(code string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(code))
	if !companyCodePattern.MatchString(c) {
		return "", domain.ErrInvalidCompanyCode
	}
	return c, nil
}

func normalizeCompanyName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || utf8.RuneCountInString(n) > 200 {
		return "", domain.ErrInvalidCompanyName
	}
	return n, nil
}

// taxID เป็น optional; ถ้าใส่ต้องเป็นตัวเลข/ตัวอักษร/ขีด ไม่เกิน 20 ตัว
func normalizeTaxID(taxID string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(taxID))
	if t != "" && !taxIDPattern.MatchString(t) {
		return "", domain.ErrInvalidTaxID
	}
	return t, nil
}
