package services

import (
	"bytes"
	"fmt"
	"regexp"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

// แม่แบบเอกสารเป็น HTML ที่ผู้ใช้เขียนเอง แล้วหน้าเว็บแสดงใน iframe แบบ sandbox (ไม่รันสคริปต์) + DOMPurify อีกชั้น
// ฝั่ง backend ปฏิเสธเนื้อหาที่รันโค้ดหรือโหลดทรัพยากรจากภายนอกตั้งแต่ตอนบันทึก (defense in depth)
//
// อนุญาต: data: URL, path ภายในระบบ (/api/v1/...), #anchor และ expression ของ Handlebars ({{...}})
// ไม่อนุญาต: <script>, on*= event handler, javascript: / vbscript: URL, <iframe> / <object> / <embed> / <frame> /
// <base> / <meta> / <link>, @import และ URL ภายนอก (http://, https://, //) ใน src / href / url()
type disallowRule struct {
	re   *regexp.Regexp
	what string
}

var templateRules = []disallowRule{
	{regexp.MustCompile(`(?i)<\s*script\b`), "<script>"},
	{regexp.MustCompile(`(?i)<[^>]*\s(on[a-z]+)\s*=`), "on* event handler"},
	{regexp.MustCompile(`(?i)(javascript|vbscript)\s*:`), "javascript: URL"},
	{regexp.MustCompile(`(?i)<\s*(iframe|object|embed|frame|frameset|base|meta|link|form|applet)\b`), "<iframe> / <object> / <embed> / <base> / <meta> / <link> / <form>"},
	{regexp.MustCompile(`(?i)@import\b`), "@import"},
	{regexp.MustCompile(`(?i)\b(src|href|srcset|action|poster|background|xlink:href)\s*=\s*["']?\s*(https?:)?//`), "external URL"},
	{regexp.MustCompile(`(?i)url\(\s*["']?\s*(https?:)?//`), "external URL in url()"},
	{regexp.MustCompile(`(?i)expression\s*\(`), "CSS expression()"},
}

// MaxTemplateBytes คือขนาดสูงสุดของ HTML ของแม่แบบ
const MaxTemplateBytes = 200 * 1024

// CheckTemplateHTML คืน ErrInvalidTemplateHTML (ว่าง / ใหญ่เกิน) หรือ ErrTemplateDisallowed พร้อมระบุสิ่งที่พบ
func CheckTemplateHTML(html string) error {
	if len(bytes.TrimSpace([]byte(html))) == 0 || len(html) > MaxTemplateBytes {
		return domain.ErrInvalidTemplateHTML
	}
	return checkDisallowed([]byte(html))
}

func checkDisallowed(b []byte) error {
	for _, r := range templateRules {
		if r.re.Match(b) {
			return fmt.Errorf("%w: %s", domain.ErrTemplateDisallowed, r.what)
		}
	}
	return nil
}

// MaxAssetBytes คือขนาดสูงสุดของรูป (โลโก้ / ลายเซ็น / ตราประทับ)
const MaxAssetBytes = 500 * 1024

var (
	assetName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	svgStart  = regexp.MustCompile(`(?is)^\s*(<\?xml[^>]*>\s*)?(<!--.*?-->\s*)*(<!DOCTYPE[^>]*>\s*)?<svg\b`)
)

// CheckAssetName: a-z, 0-9, - และ _ ยาว 1-40 ตัว (ใช้ใน {{asset "ชื่อ"}})
func CheckAssetName(name string) error {
	if !assetName.MatchString(name) {
		return domain.ErrInvalidAssetName
	}
	return nil
}

// DetectAssetType ตรวจเนื้อไฟล์จริง (ไม่เชื่อชนิดที่ client ส่งมา): png / jpeg / svg ที่ไม่มีสคริปต์
func DetectAssetType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", domain.ErrInvalidAssetType
	}
	if len(data) > MaxAssetBytes {
		return "", domain.ErrAssetTooLarge
	}
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case svgStart.Match(data):
		if err := checkDisallowed(data); err != nil {
			return "", err
		}
		return "image/svg+xml", nil
	}
	return "", domain.ErrInvalidAssetType
}
