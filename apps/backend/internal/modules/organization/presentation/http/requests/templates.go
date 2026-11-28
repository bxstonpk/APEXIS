package requests

import (
	"encoding/base64"
	"net/url"
	"strings"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// GET /document-templates?doc_type=&q=&status=&page=&page_size=
func ParseDocumentTemplateFilter(v url.Values) (dto.DocumentTemplateFilter, error) {
	q := &query{v: v}
	f := dto.DocumentTemplateFilter{Search: q.search(), Page: q.page()}
	if q.err != nil {
		return f, q.err
	}
	if raw := strings.TrimSpace(v.Get("doc_type")); raw != "" {
		dt, err := vo.ParseTemplateDocType(raw)
		if err != nil {
			return f, err
		}
		f.DocType = dt
	}
	if raw := strings.TrimSpace(v.Get("status")); raw != "" {
		s, err := vo.ParseStatus(raw)
		if err != nil {
			return f, err
		}
		f.Status = string(s)
	}
	return f, nil
}

// DocumentTemplate: POST / PUT /document-templates — doc_type เปลี่ยนไม่ได้ (PUT ไม่สนค่าที่ส่ง)
// html = HTML + Handlebars ({{...}}) ไม่เกิน 200 KB; paper ไม่ส่ง = a4_portrait; is_default (POST) = ตั้งเป็นค่าเริ่มต้นทันที
type DocumentTemplate struct {
	DocType   string `json:"doc_type"`
	Name      string `json:"name"`
	HTML      string `json:"html"`
	Paper     string `json:"paper"`
	Status    string `json:"status"`
	IsDefault bool   `json:"is_default"`
}

func (r DocumentTemplate) ToInput(a dto.Actor) dto.DocumentTemplateInput {
	return dto.DocumentTemplateInput{Actor: a, DocType: r.DocType, Name: r.Name, HTML: r.HTML, Paper: r.Paper, Status: r.Status, Default: r.IsDefault}
}

// DocumentAsset: POST /document-assets {name, data} — data = เนื้อไฟล์แบบ base64 (ไม่เกิน 500 KB หลังถอดรหัส)
type DocumentAsset struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func (r DocumentAsset) Bytes() ([]byte, error) {
	raw := strings.TrimSpace(r.Data)
	if i := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && i > 0 {
		raw = raw[i+1:] // รับ data URL ได้ด้วย (ชนิดไฟล์ตรวจจากเนื้อไฟล์ ไม่ใช้ค่าที่ส่งมา)
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalid("data must be base64")
	}
	return b, nil
}
