package valueobjects

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

var (
	codePattern        = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,19}$`)
	taxBranchNoPattern = regexp.MustCompile(`^[0-9]{5}$`)
	taxIDPattern       = regexp.MustCompile(`^[0-9A-Z-]{1,20}$`)
	thPostalPattern    = regexp.MustCompile(`^[0-9]{5}$`)
	countryPattern     = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
	phonePattern       = regexp.MustCompile(`^[0-9+()\- ]{3,30}$`)
)

// Code คือรหัสอ้างอิงของ master data (ไม่ซ้ำภายในบริษัท) เช่น HQ, SALES-01
// ตัวพิมพ์ใหญ่, ตัวเลข, - และ _ ยาว 1-20 ตัว
type Code string

func NewCode(raw string) (Code, error) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if !codePattern.MatchString(c) {
		return "", domain.ErrInvalidCode
	}
	return Code(c), nil
}

func (c Code) String() string { return string(c) }

// NewName ตรวจชื่อที่ต้องมี ยาวไม่เกิน max ตัวอักษร
func NewName(raw string, max int) (string, error) {
	n := strings.TrimSpace(raw)
	if n == "" || utf8.RuneCountInString(n) > max {
		return "", domain.ErrInvalidName
	}
	return n, nil
}

// OptionalText ตัดช่องว่างและตรวจความยาว (ว่างได้)
func OptionalText(raw string, max int, errInvalid error) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > max {
		return "", errInvalid
	}
	return s, nil
}

// TaxBranchNo คือเลขที่สาขาตามกรมสรรพากร 5 หลัก — "00000" = สำนักงานใหญ่
type TaxBranchNo string

const HeadOfficeTaxBranchNo TaxBranchNo = "00000"

func NewTaxBranchNo(raw string) (TaxBranchNo, error) {
	s := strings.TrimSpace(raw)
	if !taxBranchNoPattern.MatchString(s) {
		return "", domain.ErrInvalidTaxBranchNo
	}
	return TaxBranchNo(s), nil
}

func (t TaxBranchNo) IsHeadOffice() bool { return t == HeadOfficeTaxBranchNo }
func (t TaxBranchNo) String() string     { return string(t) }

// Address คือที่อยู่แบบไทย (ใช้กับประเทศอื่นได้ — รหัสไปรษณีย์ตรวจเข้มเฉพาะ TH)
type Address struct {
	Line        string // บ้านเลขที่ / อาคาร / ถนน
	SubDistrict string // ตำบล / แขวง
	District    string // อำเภอ / เขต
	Province    string
	PostalCode  string
	Country     string // ISO 3166-1 alpha-2, ค่าเริ่มต้น TH
}

// NewAddress ตรวจและ normalize ที่อยู่ — ที่อยู่ว่างทั้งหมดได้ (ยังไม่ระบุ)
func NewAddress(a Address) (Address, error) {
	var err error
	out := Address{}
	if out.Line, err = OptionalText(a.Line, 300, domain.ErrInvalidAddress); err != nil {
		return Address{}, err
	}
	if out.SubDistrict, err = OptionalText(a.SubDistrict, 100, domain.ErrInvalidAddress); err != nil {
		return Address{}, err
	}
	if out.District, err = OptionalText(a.District, 100, domain.ErrInvalidAddress); err != nil {
		return Address{}, err
	}
	if out.Province, err = OptionalText(a.Province, 100, domain.ErrInvalidAddress); err != nil {
		return Address{}, err
	}
	out.Country = strings.ToUpper(strings.TrimSpace(a.Country))
	if out.Country == "" {
		out.Country = "TH"
	}
	if !countryPattern.MatchString(out.Country) {
		return Address{}, domain.ErrInvalidCountry
	}
	out.PostalCode = strings.TrimSpace(a.PostalCode)
	switch {
	case out.PostalCode == "":
	case out.Country == "TH" && !thPostalPattern.MatchString(out.PostalCode):
		return Address{}, domain.ErrInvalidPostalCode
	case utf8.RuneCountInString(out.PostalCode) > 20:
		return Address{}, domain.ErrInvalidPostalCode
	}
	return out, nil
}

func NewPhone(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s != "" && !phonePattern.MatchString(s) {
		return "", domain.ErrInvalidPhone
	}
	return s, nil
}

func NewEmail(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > 320 {
		return "", domain.ErrInvalidEmail
	}
	return s, nil
}

func NewWebsite(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(s) > 300 {
		return "", domain.ErrInvalidWebsite
	}
	return s, nil
}

// NewTaxID คือเลขประจำตัวผู้เสียภาษี (ไม่บังคับ) — ตัวเลข / ตัวอักษร / ขีด ไม่เกิน 20 ตัว
// ไม่บังคับ 13 หลักเพื่อรองรับบริษัทต่างประเทศ
func NewTaxID(raw string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s != "" && !taxIDPattern.MatchString(s) {
		return "", domain.ErrInvalidTaxID
	}
	return s, nil
}

// Currency คือรหัสสกุลเงิน ISO 4217 เช่น THB
type Currency string

func NewCurrency(raw string) (Currency, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if !currencyPattern.MatchString(s) {
		return "", domain.ErrInvalidCurrency
	}
	return Currency(s), nil
}

// ---------- วันที่ ----------

// IsDateOnly: วันที่ใน domain ต้องเป็นเที่ยงคืน UTC (presentation แปลงจาก "YYYY-MM-DD" ให้แล้ว)
func IsDateOnly(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC && t.Equal(t.Truncate(24*time.Hour))
}

// EnsureDateRange ตรวจว่า end (ถ้ามี) ไม่ก่อน start
func EnsureDateRange(start time.Time, end *time.Time) error {
	if !IsDateOnly(start) || (end != nil && (!IsDateOnly(*end) || end.Before(start))) {
		return domain.ErrInvalidDateRange
	}
	return nil
}
