// Package permissions คือรายการสิทธิ์ทั้งหมดของระบบ: GET /api/v1/permissions
//
// แต่ละ module ลงทะเบียนสิทธิ์ของตัวเองตอนเริ่มโปรแกรม (module.Permissions()) — หน้ากำหนดบทบาทของ frontend
// สร้างตารางสิทธิ์จากรายการนี้ (แถว = module.resource, คอลัมน์ = action) จึงไม่ต้องแก้ frontend เมื่อเพิ่มสิทธิ์
// ชื่อที่ผู้ใช้เห็นเป็นเรื่องของ frontend (config ข้อความ) — ที่นี่ส่งเฉพาะโครงสร้างของรหัสสิทธิ์
package permissions

import (
	"fmt"
	"net/http"
	"regexp"

	"apexis/backend/internal/platform/httpx"
)

// Entry คือสิทธิ์หนึ่งรายการ เช่น sales.order:approve → module "sales", resource "order", action "approve"
type Entry struct {
	Code     string `json:"code"`
	Module   string `json:"module"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

var codePattern = regexp.MustCompile(`^([a-z][a-z0-9_]*)\.([a-z][a-z0-9_]*):([a-z][a-z0-9_]*)$`)

// Catalog เก็บสิทธิ์ตามลำดับที่ลงทะเบียน (module ตามลำดับใน main.go, สิทธิ์ตามลำดับที่ module ประกาศ)
type Catalog struct {
	entries []Entry
	seen    map[string]bool
}

func NewCatalog() *Catalog { return &Catalog{seen: map[string]bool{}} }

// Register เพิ่มสิทธิ์ — รูปแบบผิด / ซ้ำ เป็น bug ของโปรแกรม จึง panic ตอนเริ่ม (ลงทะเบียนครั้งเดียวก่อนเปิดรับ request)
func (c *Catalog) Register(codes ...string) {
	for _, code := range codes {
		m := codePattern.FindStringSubmatch(code)
		if m == nil {
			panic(fmt.Sprintf("permissions: invalid code %q (want module.resource:action)", code))
		}
		if c.seen[code] {
			panic(fmt.Sprintf("permissions: %q registered twice", code))
		}
		c.seen[code] = true
		c.entries = append(c.entries, Entry{Code: code, Module: m[1], Resource: m[2], Action: m[3]})
	}
}

// All คืนสำเนาของรายการทั้งหมด
func (c *Catalog) All() []Entry { return append([]Entry(nil), c.entries...) }

// ServeHTTP: GET /api/v1/permissions — ผู้เรียกต้องผ่าน Authenticate (ผูกใน main.go)
// รายการเหมือนกันทุกบริษัท (ไม่มีข้อมูลของบริษัท) จึงไม่ต้องตรวจสิทธิ์ราย company
func (c *Catalog) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, httpx.Data[[]Entry]{Data: c.All()})
}
