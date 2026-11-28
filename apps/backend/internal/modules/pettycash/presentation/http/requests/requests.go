// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ pettycash และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ / ตัวเลข / enum
// กฎธุรกิจ (VAT ต้องมีใบกำกับภาษี, เงินคงเหลือพอ, ทศนิยมตามบริษัท ฯลฯ) ตรวจใน domain / use case
package requests

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/listing"
	"apexis/backend/internal/platform/numeric"
)

// ErrValidation ถูก wrap พร้อมรายละเอียดที่ปลอดภัยต่อการแสดงให้ client (HTTP 400)
var ErrValidation = errors.New("validation failed")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

func required(fields map[string]string) error {
	var missing []string
	for name, val := range fields {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return invalid("%s is required", strings.Join(missing, ", "))
}

func id[T ~[16]byte](field, raw string) (T, error) {
	v, err := vo.ParseID[T](raw)
	if err != nil {
		return T{}, invalid("%s must be a UUID", field)
	}
	return v, nil
}

func optionalID[T ~[16]byte](field string, raw *string) (*T, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	v, err := id[T](field, *raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func date(field, raw string) (time.Time, error) {
	d, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be YYYY-MM-DD", field)
	}
	return d, nil
}

// Decimal รับตัวเลขเป็น string ("12.5") หรือ number (12.5) — เก็บข้อความดิบแล้วแปลงแบบไม่ผ่าน float
type Decimal string

func (d *Decimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		*d = Decimal(bytes.Trim(b, `"`))
	} else {
		*d = Decimal(b)
	}
	return nil
}

func (d *Decimal) parse(field string) (*decimal.Decimal, error) {
	if d == nil || *d == "" || *d == "null" {
		return nil, nil
	}
	v, ok := numeric.Parse(strings.TrimSpace(string(*d)))
	if !ok {
		return nil, invalid("%s must be a decimal number (up to 18 integer digits and 6 decimal places)", field)
	}
	return &v, nil
}

func (d *Decimal) required(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil {
		return decimal.Zero, err
	}
	if v == nil {
		return decimal.Zero, invalid("%s is required", field)
	}
	return *v, nil
}

func (d *Decimal) optional(field string) (decimal.Decimal, error) {
	v, err := d.parse(field)
	if err != nil || v == nil {
		return decimal.Zero, err
	}
	return *v, nil
}

// ---------- ค่าตั้ง ----------

// Settings: PUT /petty-cash-settings — แทนที่ทั้งก้อน (ไม่ส่ง = ไม่ตั้ง)
type Settings struct {
	InputVatAccountID *string `json:"input_vat_account_id"`
}

func (r Settings) ToSettings() (entities.Settings, error) {
	acc, err := optionalID[vo.AccountID]("input_vat_account_id", r.InputVatAccountID)
	return entities.Settings{InputVatAccountID: acc}, err
}

// ---------- กองทุน ----------

// Fund: POST / PUT /petty-cash-funds — gl_account_id ไม่ส่ง = บัญชีเงินสดย่อยของผังมาตรฐาน; status ไม่ส่ง = active
type Fund struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	CustodianID   string   `json:"custodian_user_id"`
	GLAccountID   *string  `json:"gl_account_id"`
	ImprestAmount *Decimal `json:"imprest_amount"`
	Status        string   `json:"status"`
}

func (r Fund) ToInput(a dto.Actor, isCreate bool) (dto.FundInput, error) {
	fields := map[string]string{"name": r.Name, "custodian_user_id": r.CustodianID}
	if isCreate {
		fields["code"] = r.Code
	}
	if err := required(fields); err != nil {
		return dto.FundInput{}, err
	}
	custodian, err := id[vo.UserID]("custodian_user_id", r.CustodianID)
	if err != nil {
		return dto.FundInput{}, err
	}
	gl, err := optionalID[vo.AccountID]("gl_account_id", r.GLAccountID)
	if err != nil {
		return dto.FundInput{}, err
	}
	imprest, err := r.ImprestAmount.required("imprest_amount")
	if err != nil {
		return dto.FundInput{}, err
	}
	status := vo.StatusActive
	if r.Status != "" {
		if status, err = vo.ParseStatus(r.Status); err != nil {
			return dto.FundInput{}, invalid("status must be active or inactive")
		}
	}
	return dto.FundInput{Actor: a, Code: r.Code, Name: r.Name, CustodianID: custodian, GLAccountID: gl, ImprestAmount: imprest, Status: status}, nil
}

// ---------- ใบสำคัญจ่าย ----------

type Payee struct {
	Name     string `json:"name"`
	TaxID    string `json:"tax_id"`
	BranchNo string `json:"branch_no"`
}

type TaxInvoice struct {
	No   string `json:"no"`
	Date string `json:"date"`
}

type VoucherLine struct {
	AccountID   string   `json:"account_id"`
	Description string   `json:"description"`
	Amount      *Decimal `json:"amount"`
}

// Voucher: POST / PUT /petty-cash-vouchers — vat_rate ไม่ส่ง = 0; มี VAT ต้องส่ง tax_invoice
type Voucher struct {
	FundID           string        `json:"fund_id"`
	Date             string        `json:"date"`
	Payee            Payee         `json:"payee"`
	Description      string        `json:"description"`
	Lines            []VoucherLine `json:"lines"`
	VatRate          *Decimal      `json:"vat_rate"`
	PriceIncludesVat bool          `json:"price_includes_vat"`
	TaxInvoice       *TaxInvoice   `json:"tax_invoice"`
}

func (r Voucher) ToData() (entities.VoucherData, error) {
	if err := required(map[string]string{"fund_id": r.FundID, "date": r.Date, "payee.name": r.Payee.Name}); err != nil {
		return entities.VoucherData{}, err
	}
	fund, err := id[vo.FundID]("fund_id", r.FundID)
	if err != nil {
		return entities.VoucherData{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return entities.VoucherData{}, err
	}
	rate, err := r.VatRate.optional("vat_rate")
	if err != nil {
		return entities.VoucherData{}, err
	}
	out := entities.VoucherData{FundID: fund, Date: d, Payee: entities.Payee{Name: r.Payee.Name, TaxID: r.Payee.TaxID, BranchNo: r.Payee.BranchNo},
		Description: r.Description, VatRate: rate, PriceIncludesVat: r.PriceIncludesVat, Lines: make([]entities.VoucherLine, len(r.Lines))}
	for i, l := range r.Lines {
		acc, err := id[vo.AccountID](fmt.Sprintf("lines[%d].account_id", i), l.AccountID)
		if err != nil {
			return out, err
		}
		amt, err := l.Amount.required(fmt.Sprintf("lines[%d].amount", i))
		if err != nil {
			return out, err
		}
		out.Lines[i] = entities.VoucherLine{AccountID: acc, Description: l.Description, Amount: amt}
	}
	if t := r.TaxInvoice; t != nil && (t.No != "" || t.Date != "") {
		td, err := date("tax_invoice.date", t.Date)
		if err != nil {
			return out, err
		}
		out.TaxInvoice = &entities.TaxInvoice{No: t.No, Date: td}
	}
	return out, nil
}

// ---------- คำขอ ----------

// Replenishment: POST /petty-cash-replenishments
//
//	establish: {fund_id, kind, date} (ยอด = วงเงินของกองทุน) — increase: + amount — replenish: + voucher_ids
type Replenishment struct {
	FundID      string   `json:"fund_id"`
	Kind        string   `json:"kind"`
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Amount      *Decimal `json:"amount"`
	VoucherIDs  []string `json:"voucher_ids"`
}

func (r Replenishment) ToInput(a dto.Actor) (dto.ReplenishmentInput, error) {
	if err := required(map[string]string{"fund_id": r.FundID, "kind": r.Kind, "date": r.Date}); err != nil {
		return dto.ReplenishmentInput{}, err
	}
	fund, err := id[vo.FundID]("fund_id", r.FundID)
	if err != nil {
		return dto.ReplenishmentInput{}, err
	}
	kind, err := vo.ParseRequestKind(r.Kind)
	if err != nil {
		return dto.ReplenishmentInput{}, invalid("kind must be establish, increase or replenish")
	}
	d, err := date("date", r.Date)
	if err != nil {
		return dto.ReplenishmentInput{}, err
	}
	out := dto.ReplenishmentInput{Actor: a, FundID: fund, Kind: kind, Date: d, Description: r.Description}
	switch kind {
	case vo.RequestIncrease:
		if out.Amount, err = r.Amount.required("amount"); err != nil {
			return out, err
		}
	case vo.RequestReplenish:
		if len(r.VoucherIDs) == 0 {
			return out, invalid("voucher_ids is required")
		}
		seen := map[vo.VoucherID]bool{}
		for i, raw := range r.VoucherIDs {
			v, err := id[vo.VoucherID](fmt.Sprintf("voucher_ids[%d]", i), raw)
			if err != nil {
				return out, err
			}
			if seen[v] {
				return out, invalid("voucher_ids[%d] appears more than once", i)
			}
			seen[v] = true
			out.VoucherIDs = append(out.VoucherIDs, v)
		}
	}
	return out, nil
}

// ---------- URL query ----------

type query struct {
	v   url.Values
	err error
}

func parse[T any](q *query, fn func() (T, error)) T {
	var zero T
	if q.err != nil {
		return zero
	}
	v, err := fn()
	if err != nil {
		q.err = err
		return zero
	}
	return v
}

func (q *query) search() string {
	return parse(q, func() (string, error) { return listing.ParseSearch(q.v) })
}

func (q *query) sort(keys []string, def []listing.Sort) []listing.Sort {
	return parse(q, func() ([]listing.Sort, error) { return listing.ParseSort(q.v, keys, def) })
}

func (q *query) page() listing.Page {
	return parse(q, func() (listing.Page, error) { return listing.ParsePage(q.v) })
}

func (q *query) fund() *vo.FundID {
	return parse(q, func() (*vo.FundID, error) { return listing.ParseOptional(q.v, "fund_id", vo.ParseID[vo.FundID]) })
}

func (q *query) dates(from, to string) (*time.Time, *time.Time) {
	if q.err != nil {
		return nil, nil
	}
	a, b, err := listing.ParseDateRange(q.v, from, to)
	q.err = err
	return a, b
}

// GET /petty-cash-funds?q=&status=&sort=code|name|updated_at&page=&page_size=
func ParseFundFilter(v url.Values) (dto.FundFilter, error) {
	q := &query{v: v}
	f := dto.FundFilter{Search: q.search(),
		Status: parse(q, func() ([]vo.Status, error) { return listing.ParseList(v, "status", vo.ParseStatus) }),
		Sort:   q.sort(dto.FundSortKeys, dto.DefaultFundSort), Page: q.page()}
	return f, q.err
}

// GET /petty-cash-vouchers?q=&fund_id=&status=&from=&to=&unrequested=true&sort=date|document_no|created_at&page=&page_size=
func ParseVoucherFilter(v url.Values) (dto.VoucherFilter, error) {
	q := &query{v: v}
	f := dto.VoucherFilter{Search: q.search(), FundID: q.fund(),
		Status: parse(q, func() ([]vo.VoucherStatus, error) { return listing.ParseList(v, "status", vo.ParseVoucherStatus) }),
		Sort:   q.sort(dto.VoucherSortKeys, dto.DefaultVoucherSort), Page: q.page()}
	f.From, f.To = q.dates("from", "to")
	if raw := v.Get("unrequested"); raw != "" && q.err == nil {
		f.Unrequested, q.err = listing.ParseBool(raw)
	}
	return f, q.err
}

// GET /petty-cash-replenishments?q=&fund_id=&kind=&status=&from=&to=&sort=&page=&page_size=
func ParseReplenishmentFilter(v url.Values) (dto.ReplenishmentFilter, error) {
	q := &query{v: v}
	f := dto.ReplenishmentFilter{Search: q.search(), FundID: q.fund(),
		Kind:   parse(q, func() ([]vo.RequestKind, error) { return listing.ParseList(v, "kind", vo.ParseRequestKind) }),
		Status: parse(q, func() ([]vo.RequestStatus, error) { return listing.ParseList(v, "status", vo.ParseRequestStatus) }),
		Sort:   q.sort(dto.ReplenishmentSortKeys, dto.DefaultReplenishmentSort), Page: q.page()}
	f.From, f.To = q.dates("from", "to")
	return f, q.err
}

func requiredRange(v url.Values) (dto.DateRange, error) {
	start, end, err := listing.ParseDateRange(v, "start_date", "end_date")
	if err != nil {
		return dto.DateRange{}, err
	}
	if start == nil || end == nil {
		return dto.DateRange{}, fmt.Errorf("%w: start_date and end_date are required (YYYY-MM-DD)", listing.ErrInvalidQuery)
	}
	return dto.DateRange{Start: *start, End: *end}, nil
}

// GET /reports?book=petty-cash-voucher-register&start_date=&end_date=&fund_id=&status=&page=&page_size=
func ParseRegisterFilter(v url.Values) (dto.RegisterFilter, error) {
	q := &query{v: v}
	f := dto.RegisterFilter{FundID: q.fund(),
		Status: parse(q, func() ([]vo.VoucherStatus, error) { return listing.ParseList(v, "status", vo.ParseVoucherStatus) }),
		Page:   q.page()}
	if q.err != nil {
		return f, q.err
	}
	r, err := requiredRange(v)
	f.DateRange = r
	return f, err
}

// GET /reports?book=petty-cash-fund-balance&end_date= (ไม่ส่ง = วันนี้)
func ParseAsOf(v url.Values, today time.Time) (time.Time, error) {
	if raw := v.Get("end_date"); raw != "" {
		d, err := listing.ParseDate(raw)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: end_date must be YYYY-MM-DD", listing.ErrInvalidQuery)
		}
		return d, nil
	}
	return today, nil
}

// GET /reports?book=petty-cash-input-vat&start_date=&end_date=&page=&page_size=
func ParseInputVatFilter(v url.Values) (dto.InputVatFilter, error) {
	q := &query{v: v}
	f := dto.InputVatFilter{Page: q.page()}
	if q.err != nil {
		return f, q.err
	}
	r, err := requiredRange(v)
	f.DateRange = r
	return f, err
}
