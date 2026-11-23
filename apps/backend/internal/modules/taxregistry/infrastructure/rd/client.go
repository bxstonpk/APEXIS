// Package rd คือ adapter ของระบบค้นหาผู้ประกอบการจดทะเบียนภาษีมูลค่าเพิ่ม กรมสรรพากร (vsinter.rd.go.th)
//
//	POST {base} {"nid": "<13 หลัก>", "brano": "<5 หลัก | ว่าง>", "searchType": "1", ...}
//	พบ:    {"status": "000", "data": [{nid, title, firnam, brano, addno, moono, ..., poscod, telno}]}
//	ไม่พบ: {"status": "001", "errdetail": "Not Found Data"}
package rd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"apexis/backend/internal/modules/taxregistry/application/ports"
	"apexis/backend/internal/modules/taxregistry/domain/entities"
	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
	"apexis/backend/internal/platform/thaiaddress"
)

// DefaultURL คือ endpoint ของกรมสรรพากร (เปลี่ยนได้ด้วย VAT_REGISTRY_URL)
const DefaultURL = "https://vsinter.rd.go.th/rd-commoninter-service/subother/vatsbtsearch/getVatInfo"

const (
	statusFound    = "000"
	statusNotFound = "001"
	maxBody        = 1 << 20
)

type Client struct {
	url  string
	http *http.Client
}

var _ ports.VatRegistry = (*Client)(nil)

// New: timeout 10 วินาที — ผู้ใช้รอที่หน้าฟอร์ม และกรอกเองได้ถ้าบริการช้า / ล่ม
func New(url string) *Client {
	if url == "" {
		url = DefaultURL
	}
	return &Client{url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

type request struct {
	NID        string `json:"nid"`
	BranchNo   string `json:"brano"`
	SearchType string `json:"searchType"`
	FirstName  string `json:"firNam"`
	MiddleName string `json:"midNam"`
	LastName   string `json:"lasNam"`
}

type record struct {
	NID         string `json:"nid"`
	Title       string `json:"title"`
	FirstName   string `json:"firnam"`
	MiddleName  string `json:"midnam"`
	LastName    string `json:"lasnam"`
	BranchNo    string `json:"brano"`
	BranchTitle string `json:"bratitle"`
	BranchName  string `json:"branam"`
	VatDate     string `json:"vataprdat"`
	Building    string `json:"bldgnam"`
	RoomNo      string `json:"roomno"`
	FloorNo     string `json:"floorno"`
	Village     string `json:"village"`
	HouseNo     string `json:"addno"`
	Moo         string `json:"moono"`
	Soi         string `json:"soinam"`
	Yaek        string `json:"yaek"`
	Street      string `json:"thnnam"`
	SubDistrict string `json:"tamnam"`
	District    string `json:"ampnam"`
	Province    string `json:"provnam"`
	PostalCode  string `json:"poscod"`
	Phone       string `json:"telno"`
}

type response struct {
	Status string   `json:"status"`
	Data   []record `json:"data"`
}

func (c *Client) Lookup(ctx context.Context, taxID, branchNo string) ([]entities.VatRegistrant, error) {
	body, _ := json.Marshal(request{NID: taxID, BranchNo: branchNo, SearchType: "1"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: http %d", domain.ErrUnavailable, res.StatusCode)
	}
	var out response
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", domain.ErrUnavailable, err)
	}
	switch out.Status {
	case statusNotFound:
		return nil, nil
	case statusFound:
	default:
		return nil, fmt.Errorf("%w: status %q", domain.ErrUnavailable, out.Status)
	}
	rows := make([]entities.VatRegistrant, 0, len(out.Data))
	for _, r := range out.Data {
		rows = append(rows, toEntity(r))
	}
	return rows, nil
}

// toEntity: แปลงข้อมูลสรรพากร ("-" = ว่าง); ที่อยู่ที่ยาว / ผิดรูปแบบยังคืนค่าตามจริง — ตรวจอีกครั้งตอนบันทึกลูกค้า / ผู้ขาย
func toEntity(r record) entities.VatRegistrant {
	c := thaiaddress.Clean
	return entities.VatRegistrant{
		TaxID: c(r.NID), BranchNo: c(r.BranchNo), Title: c(r.Title), FirstName: c(r.FirstName), MiddleName: c(r.MiddleName),
		LastName: c(r.LastName), BranchTitle: c(r.BranchTitle), BranchName: c(r.BranchName), Phone: c(r.Phone), RegisteredOn: c(r.VatDate),
		Address: thaiaddress.Address{
			Building: c(r.Building), RoomNo: c(r.RoomNo), FloorNo: c(r.FloorNo), Village: c(r.Village), HouseNo: c(r.HouseNo),
			Moo: c(r.Moo), Soi: c(r.Soi), Yaek: c(r.Yaek), Street: c(r.Street), SubDistrict: c(r.SubDistrict),
			District: c(r.District), Province: c(r.Province), PostalCode: c(r.PostalCode),
		},
	}
}
