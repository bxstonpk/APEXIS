package organization_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

// แม่แบบเอกสารพิมพ์ + รูป: CRUD, ค่าเริ่มต้น 1 ตัวต่อชนิดเอกสาร, ตรวจเนื้อหาที่ไม่อนุญาต, รูป (ชนิด / ขนาด / header), สิทธิ์
func TestDocumentTemplates(t *testing.T) {
	a := setup(t)
	owner := a.user("owner@example.com")
	staff := a.user("staff@example.com")
	outsider := a.user("outsider@example.com")
	comp := a.expect(a.do("POST", "/api/v1/companies", owner, map[string]any{"code": "acme", "name": "ACME", "business_type": "trading"}), 201, "company").id()
	base := "/api/v1/companies/" + comp
	role := a.expect(a.do("POST", base+"/roles", owner, map[string]any{"code": "viewer", "name": "Viewer", "permissions": []string{"organization.branch:read"}}), 201, "role").id()
	a.expect(a.do("POST", base+"/members", owner, map[string]any{"email": "staff@example.com", "role_ids": []string{role}}), 201, "add staff")

	html := `<div class="sheet"><h1 style="color:#1f3a5f">{{title}}</h1>{{#each lines}}<p>{{description}}</p>{{/each}}<img src="{{asset "logo"}}"></div>`
	body := map[string]any{"doc_type": "sales.invoice", "name": "ใบกำกับแบบบริษัท", "html": html, "paper": "a4_portrait"}

	// ---------- สิทธิ์: เขียนต้อง manage, อ่าน = สมาชิกทุกคน, คนนอก = 404 ----------
	a.expect(a.do("POST", base+"/document-templates", staff, body), 403, "staff create")
	a.expect(a.do("GET", base+"/document-templates", outsider, nil), 404, "outsider list")

	t1 := a.expect(a.do("POST", base+"/document-templates", owner, body), 201, "create t1")
	if d := t1.data(); d["html"] != html || d["is_default"] != false || d["status"] != "active" || d["paper"] != "a4_portrait" {
		t.Fatalf("t1 = %v", d)
	}
	a.expect(a.do("GET", base+"/document-templates/default?doc_type=sales.invoice", staff, nil), 404, "no default yet")

	body2 := map[string]any{"doc_type": "sales.invoice", "name": "แบบที่สอง", "html": "<p>{{document_no}}</p>", "is_default": true}
	t2 := a.expect(a.do("POST", base+"/document-templates", owner, body2), 201, "create t2 default")
	if d := a.expect(a.do("GET", base+"/document-templates/default?doc_type=sales.invoice", staff, nil), 200, "default=t2").data(); d["id"] != t2.id() || d["html"] != "<p>{{document_no}}</p>" {
		t.Fatalf("default = %v", d)
	}
	// ตั้ง t1 เป็นค่าเริ่มต้น → t2 ถูกล้าง (ค่าเริ่มต้นได้ตัวเดียว)
	a.expect(a.do("PUT", base+"/document-templates/"+t1.id()+"/default", owner, nil), 200, "t1 default")
	list := a.expect(a.do("GET", base+"/document-templates?doc_type=sales.invoice", staff, nil), 200, "list")
	defaults := 0
	for _, row := range list.list() {
		if row["is_default"] == true {
			defaults++
			if row["id"] != t1.id() {
				t.Fatalf("default should be t1: %v", row)
			}
		}
		if _, ok := row["html"]; ok {
			t.Fatalf("list must not include html: %v", row)
		}
	}
	if defaults != 1 || list.total() != 2 {
		t.Fatalf("defaults=%d total=%d", defaults, list.total())
	}
	if n := a.expect(a.do("GET", base+"/document-templates?q=สอง", owner, nil), 200, "search").total(); n != 1 {
		t.Fatalf("search total = %d", n)
	}

	// แก้: ชนิดเอกสารเปลี่ยนไม่ได้ (ไม่สนค่าที่ส่ง), ค่าเริ่มต้นคงเดิม; ปิดใช้งาน = เลิกเป็นค่าเริ่มต้น
	upd := map[string]any{"doc_type": "sales.quotation", "name": "ชื่อใหม่", "html": html, "paper": "a5_portrait"}
	if d := a.expect(a.do("PUT", base+"/document-templates/"+t1.id(), owner, upd), 200, "update").data(); d["doc_type"] != "sales.invoice" || d["is_default"] != true || d["paper"] != "a5_portrait" {
		t.Fatalf("updated = %v", d)
	}
	upd["status"] = "inactive"
	if d := a.expect(a.do("PUT", base+"/document-templates/"+t1.id(), owner, upd), 200, "deactivate").data(); d["is_default"] != false {
		t.Fatalf("inactive must not stay default: %v", d)
	}
	a.expect(a.do("PUT", base+"/document-templates/"+t1.id()+"/default", owner, nil), 422, "inactive default")
	a.expect(a.do("GET", base+"/document-templates/default?doc_type=sales.invoice", owner, nil), 404, "no default after deactivate")

	// ---------- ตรวจเนื้อหา ----------
	for name, bad := range map[string]string{
		"script":     `<p>x</p><script>alert(1)</script>`,
		"handler":    `<img src="data:image/png;base64,AA" onerror="alert(1)">`,
		"js url":     `<a href="javascript:alert(1)">x</a>`,
		"iframe":     `<iframe src="/x"></iframe>`,
		"external":   `<img src="https://evil.example/x.png">`,
		"proto-rel":  `<img src="//evil.example/x.png">`,
		"css url":    `<div style="background:url(https://evil.example/x.png)"></div>`,
		"css import": `<style>@import "x.css";</style>`,
	} {
		r := a.expect(a.do("POST", base+"/document-templates", owner, map[string]any{"doc_type": "sales.invoice", "name": "bad", "html": bad}), 400, "reject "+name)
		if e := r.body["error"].(map[string]any); e["code"] != "template_contains_disallowed_content" {
			t.Fatalf("%s: error = %v", name, e)
		}
	}
	a.expect(a.do("POST", base+"/document-templates", owner, map[string]any{"doc_type": "nope", "name": "x", "html": "<p></p>"}), 400, "bad doc type")
	a.expect(a.do("POST", base+"/document-templates", owner, map[string]any{"doc_type": "sales.invoice", "name": "x", "html": "<p></p>", "paper": "letter"}), 400, "bad paper")
	a.expect(a.do("POST", base+"/document-templates", owner, map[string]any{"doc_type": "sales.invoice", "name": "x", "html": strings.Repeat("a", 210*1024)}), 400, "too large")

	// ---------- รูป ----------
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	a.expect(a.do("POST", base+"/document-assets", staff, map[string]any{"name": "logo", "data": base64.StdEncoding.EncodeToString(png)}), 403, "staff upload")
	asset := a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "logo", "data": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}), 201, "upload png")
	if d := asset.data(); d["content_type"] != "image/png" || d["size"] != float64(len(png)) {
		t.Fatalf("asset = %v", d)
	}
	a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "logo", "data": base64.StdEncoding.EncodeToString(png)}), 409, "dup name")
	a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "Bad Name", "data": base64.StdEncoding.EncodeToString(png)}), 400, "bad name")
	a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "gif", "data": base64.StdEncoding.EncodeToString([]byte("GIF89a....."))}), 400, "gif rejected")
	a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "evil", "data": base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))}), 400, "svg script")
	svg := a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "stamp", "data": base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><circle r="4"/></svg>`))}), 201, "svg ok")
	if svg.data()["content_type"] != "image/svg+xml" {
		t.Fatalf("svg = %v", svg.data())
	}
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 501*1024)...)
	a.expect(a.do("POST", base+"/document-assets", owner, map[string]any{"name": "big", "data": base64.StdEncoding.EncodeToString(big)}), 400, "asset too large")
	if n := len(a.expect(a.do("GET", base+"/document-assets", staff, nil), 200, "assets").list()); n != 2 {
		t.Fatalf("assets = %d", n)
	}

	// ตัวไฟล์: ชนิดจริง + CSP + nosniff, สมาชิกเห็นได้, คนนอกไม่ได้
	get := func(token string) *http.Response {
		req, _ := http.NewRequest("GET", a.srv.URL+base+"/document-assets/"+svg.id()+"/content", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := a.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := get(staff)
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/svg+xml" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "default-src 'none'") ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(string(b), "<svg") {
		t.Fatalf("content: %d %v %q", res.StatusCode, res.Header, b)
	}
	res = get(outsider)
	_ = res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("outsider content = %d", res.StatusCode)
	}

	a.expect(a.do("DELETE", base+"/document-assets/"+svg.id(), owner, nil), 204, "delete asset")
	a.expect(a.do("DELETE", base+"/document-templates/"+t2.id(), staff, nil), 403, "staff delete")
	a.expect(a.do("DELETE", base+"/document-templates/"+t2.id(), owner, nil), 204, "delete t2")
	a.expect(a.do("GET", base+"/document-templates/"+t2.id(), owner, nil), 404, "deleted")
}
