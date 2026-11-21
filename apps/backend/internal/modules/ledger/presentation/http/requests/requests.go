// Package requests คือรูปแบบ JSON ขาเข้า / URL query ของ ledger และการแปลงเป็น input ของ use case
//
// presentation validation: field ที่ต้องมี, รูปแบบ UUID / วันที่ (YYYY-MM-DD) / จำนวนเงิน / enum
// กฎธุรกิจ (สมดุล, ลำดับชั้นผังบัญชี, งวดบัญชี ฯลฯ) ตรวจใน domain / use case
package requests

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
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
	if raw == "" {
		return time.Time{}, invalid("%s is required", field)
	}
	t, err := listing.ParseDate(raw)
	if err != nil {
		return time.Time{}, invalid("%s must be a date in YYYY-MM-DD format", field)
	}
	return t, nil
}

// status: POST ไม่ส่ง = ค่าเริ่มต้น, PUT ต้องส่ง (แทนที่ทั้งก้อน)
func status(raw string, isCreate bool) (vo.Status, error) {
	if raw == "" {
		if isCreate {
			return vo.StatusActive, nil
		}
		return "", invalid("status is required")
	}
	s, err := vo.ParseStatus(raw)
	if err != nil {
		return "", invalid("status has an invalid value %q", raw)
	}
	return s, nil
}

// Amount รับจำนวนเงินเป็น string ("1234.50") หรือ number (1234.5) — เก็บข้อความดิบไว้แปลงแบบไม่ผ่าน float
type Amount string

func (a *Amount) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*a = ""
	case len(b) > 0 && b[0] == '"':
		*a = Amount(bytes.Trim(b, `"`))
	default:
		*a = Amount(b)
	}
	return nil
}

func (a Amount) money() (vo.Money, error) {
	if a == "" {
		return decimal.Zero, nil
	}
	return vo.ParseMoney(string(a))
}

// ---------- account ----------

// POST / PUT /accounts — postable: ค่าเริ่มต้น true ตอน POST, บังคับส่งตอน PUT
type Account struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	NameEN   string  `json:"name_en"`
	Type     string  `json:"type"`
	ParentID *string `json:"parent_id"`
	Postable *bool   `json:"postable"`
	Status   string  `json:"status"`
}

func (r Account) ToData(isCreate bool) (entities.AccountData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name, "type": r.Type}); err != nil {
		return entities.AccountData{}, err
	}
	t, err := vo.ParseAccountType(r.Type)
	if err != nil {
		return entities.AccountData{}, invalid("type must be one of asset, liability, equity, revenue, expense")
	}
	parent, err := optionalID[vo.AccountID]("parent_id", r.ParentID)
	if err != nil {
		return entities.AccountData{}, err
	}
	postable := true
	switch {
	case r.Postable != nil:
		postable = *r.Postable
	case !isCreate:
		return entities.AccountData{}, invalid("postable is required")
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.AccountData{}, err
	}
	return entities.AccountData{
		Code: r.Code, Name: r.Name, NameEN: r.NameEN, Type: t, ParentID: parent, Postable: postable, Status: st,
	}, nil
}

// ---------- journal ----------

// Journal: number_template ไม่ส่ง = "{prefix}{fy}-{seq:6}" (prefix = code) — ดูตัวแปรที่ platform/numbering
type Journal struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	NumberTemplate string `json:"number_template"`
	Status         string `json:"status"`
}

func (r Journal) ToData(isCreate bool) (entities.JournalData, error) {
	if err := required(map[string]string{"code": r.Code, "name": r.Name}); err != nil {
		return entities.JournalData{}, err
	}
	st, err := status(r.Status, isCreate)
	if err != nil {
		return entities.JournalData{}, err
	}
	return entities.JournalData{Code: r.Code, Name: r.Name, NumberTemplate: r.NumberTemplate, Status: st}, nil
}

// ---------- journal entry ----------

type Line struct {
	AccountID     string  `json:"account_id"`
	Description   string  `json:"description"`
	Debit         Amount  `json:"debit"`
	Credit        Amount  `json:"credit"`
	BranchID      *string `json:"branch_id"`
	DepartmentID  *string `json:"department_id"`
	CostCenterID  *string `json:"cost_center_id"`
	ProjectSiteID *string `json:"project_site_id"`
}

// Entry: POST / PUT /journal-entries
// status (เฉพาะ POST): "draft" (ค่าเริ่มต้น) หรือ "posted" = สร้างแล้วผ่านรายการทันที
type Entry struct {
	JournalID   string `json:"journal_id"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Reference   string `json:"reference"`
	Status      string `json:"status"`
	Lines       []Line `json:"lines"`
}

// PostOnCreate: POST ที่ส่ง status = "posted"
func (r Entry) PostOnCreate() (bool, error) {
	switch r.Status {
	case "", string(vo.EntryDraft):
		return false, nil
	case string(vo.EntryPosted):
		return true, nil
	}
	return false, invalid(`status must be "draft" or "posted"`)
}

func (r Entry) ToData() (entities.EntryData, error) {
	if err := required(map[string]string{"journal_id": r.JournalID}); err != nil {
		return entities.EntryData{}, err
	}
	journal, err := id[vo.JournalID]("journal_id", r.JournalID)
	if err != nil {
		return entities.EntryData{}, err
	}
	d, err := date("date", r.Date)
	if err != nil {
		return entities.EntryData{}, err
	}
	if len(r.Lines) == 0 {
		return entities.EntryData{}, invalid("lines is required")
	}
	lines := make([]entities.LineData, len(r.Lines))
	for i, l := range r.Lines {
		n := i + 1
		if lines[i], err = l.toData(n); err != nil {
			return entities.EntryData{}, err
		}
	}
	return entities.EntryData{JournalID: journal, Date: d, Description: r.Description, Reference: r.Reference, Lines: lines}, nil
}

func (l Line) toData(n int) (entities.LineData, error) {
	field := func(name string) string { return fmt.Sprintf("lines[%d].%s", n, name) }
	var (
		out entities.LineData
		err error
	)
	if l.AccountID == "" {
		return out, invalid("%s is required", field("account_id"))
	}
	if out.AccountID, err = id[vo.AccountID](field("account_id"), l.AccountID); err != nil {
		return out, err
	}
	if out.Debit, err = l.Debit.money(); err != nil {
		return out, invalid("%s: %v", field("debit"), err)
	}
	if out.Credit, err = l.Credit.money(); err != nil {
		return out, invalid("%s: %v", field("credit"), err)
	}
	out.Description = l.Description
	if out.BranchID, err = optionalID[vo.BranchID](field("branch_id"), l.BranchID); err != nil {
		return out, err
	}
	if out.DepartmentID, err = optionalID[vo.DepartmentID](field("department_id"), l.DepartmentID); err != nil {
		return out, err
	}
	if out.CostCenterID, err = optionalID[vo.CostCenterID](field("cost_center_id"), l.CostCenterID); err != nil {
		return out, err
	}
	if out.ProjectSiteID, err = optionalID[vo.ProjectSiteID](field("project_site_id"), l.ProjectSiteID); err != nil {
		return out, err
	}
	return out, nil
}

// UpdateEntryStatus: PATCH /journal-entries/{entryID} {"status": "posted"}
// เปลี่ยนได้ทางเดียวคือ draft -> posted; การยกเลิกรายการที่ผ่านแล้วใช้ POST .../reversals
type UpdateEntryStatus struct {
	Status string `json:"status"`
}

func (r UpdateEntryStatus) Validate() error {
	if r.Status != string(vo.EntryPosted) {
		return invalid(`status can only be changed to "posted" (use POST /journal-entries/{id}/reversals to reverse)`)
	}
	return nil
}

// Reversal: POST /journal-entries/{entryID}/reversals
type Reversal struct {
	Date        string `json:"date"`
	Description string `json:"description"`
}

func (r Reversal) Parse() (time.Time, string, error) {
	d, err := date("date", r.Date)
	return d, r.Description, err
}
