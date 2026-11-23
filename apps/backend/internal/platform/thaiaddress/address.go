// Package thaiaddress คือที่อยู่ตามแบบกรมสรรพากร (ระบบค้นหาผู้ประกอบการจดทะเบียนภาษีมูลค่าเพิ่ม)
//
// เก็บแยกช่อง: อาคาร / ห้อง / ชั้น / หมู่บ้าน / เลขที่ / หมู่ / ซอย / แยก / ถนน / ตำบล-แขวง / อำเภอ-เขต / จังหวัด / รหัสไปรษณีย์
// Format() ประกอบเป็นข้อความเดียวตามรูปแบบที่สรรพากรใช้ (กรุงเทพมหานครใช้ แขวง / เขต) — ใช้พิมพ์ใบกำกับภาษีและเก็บ snapshot
//
// เป็น value object ล้วน (ไม่มี I/O) จึงใช้ได้จาก domain ของทุก module
package thaiaddress

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrInvalidAddress คือที่อยู่ไม่ถูกต้อง (ยาวเกิน / รหัสไปรษณีย์ไม่ใช่ 5 หลัก)
var ErrInvalidAddress = errors.New("invalid address")

// MaxField คือความยาวสูงสุดต่อช่อง; MaxText คือความยาวสูงสุดของที่อยู่ที่ประกอบแล้ว
const (
	MaxField = 100
	MaxText  = 500
)

type Address struct {
	Building    string // อาคาร
	RoomNo      string // ห้องเลขที่
	FloorNo     string // ชั้นที่
	Village     string // หมู่บ้าน
	HouseNo     string // เลขที่
	Moo         string // หมู่ที่
	Soi         string // ซอย
	Yaek        string // แยก
	Street      string // ถนน
	SubDistrict string // ตำบล / แขวง
	District    string // อำเภอ / เขต
	Province    string // จังหวัด
	PostalCode  string // รหัสไปรษณีย์ 5 หลัก
}

// Clean: ตัดช่องว่าง และค่า "-" (สรรพากรใช้แทนค่าว่าง) เป็นค่าว่าง
func Clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "-" {
		return ""
	}
	return s
}

func (a Address) fields() []*string {
	return []*string{&a.Building, &a.RoomNo, &a.FloorNo, &a.Village, &a.HouseNo, &a.Moo, &a.Soi, &a.Yaek, &a.Street, &a.SubDistrict, &a.District, &a.Province, &a.PostalCode}
}

// New: ทำความสะอาดทุกช่องแล้วตรวจความยาว / รหัสไปรษณีย์ / ความยาวรวมหลังประกอบ
func New(a Address) (Address, error) {
	out := a
	for _, f := range []*string{&out.Building, &out.RoomNo, &out.FloorNo, &out.Village, &out.HouseNo, &out.Moo, &out.Soi, &out.Yaek, &out.Street, &out.SubDistrict, &out.District, &out.Province, &out.PostalCode} {
		*f = Clean(*f)
		if utf8.RuneCountInString(*f) > MaxField {
			return Address{}, fmt.Errorf("%w: each part must be at most %d characters", ErrInvalidAddress, MaxField)
		}
	}
	if p := out.PostalCode; p != "" && (len(p) != 5 || strings.Trim(p, "0123456789") != "") {
		return Address{}, fmt.Errorf("%w: postal_code must be 5 digits", ErrInvalidAddress)
	}
	if utf8.RuneCountInString(out.Format()) > MaxText {
		return Address{}, fmt.Errorf("%w: address must be at most %d characters", ErrInvalidAddress, MaxText)
	}
	return out, nil
}

func (a Address) IsEmpty() bool {
	for _, f := range a.fields() {
		if *f != "" {
			return false
		}
	}
	return true
}

// IsBangkok: กรุงเทพมหานครใช้ แขวง / เขต แทน ตำบล / อำเภอ
func (a Address) IsBangkok() bool {
	switch strings.TrimPrefix(a.Province, "จังหวัด") {
	case "กรุงเทพมหานคร", "กรุงเทพฯ", "กรุงเทพ", "กทม.", "กทม":
		return true
	}
	return false
}

// prefixed: เติมคำนำหน้าเมื่อค่ายังไม่ขึ้นต้นด้วยคำนั้น (เช่น "ถนน" + "สุขุมวิท", แต่ "ถนนสุขุมวิท" คงเดิม)
func prefixed(prefix, v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, prefix) {
		return v
	}
	return prefix + v
}

// Format: ที่อยู่บรรทัดเดียวตามแบบสรรพากร เช่น
// "เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000"
func (a Address) Format() string {
	sub, dist, prov := "ตำบล", "อำเภอ", "จังหวัด"
	province := prefixed(prov, a.Province)
	if a.IsBangkok() {
		sub, dist = "แขวง", "เขต"
		province = "กรุงเทพมหานคร"
	}
	parts := []string{
		prefixed("อาคาร", a.Building),
		prefixed("ห้องเลขที่ ", a.RoomNo),
		prefixed("ชั้นที่ ", a.FloorNo),
		prefixed("หมู่บ้าน", a.Village),
		prefixed("เลขที่ ", a.HouseNo),
		prefixed("หมู่ที่ ", a.Moo),
		prefixed("ซอย", a.Soi),
		prefixed("แยก", a.Yaek),
		prefixed("ถนน", a.Street),
		prefixed(sub, strings.TrimPrefix(strings.TrimPrefix(a.SubDistrict, "ตำบล"), "แขวง")),
		prefixed(dist, strings.TrimPrefix(strings.TrimPrefix(a.District, "อำเภอ"), "เขต")),
		province,
		a.PostalCode,
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
