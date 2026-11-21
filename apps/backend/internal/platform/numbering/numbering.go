// Package numbering คือรูปแบบเลขที่เอกสารที่บริษัทตั้งได้ (pure — ไม่มี I/O)
//
// รูปแบบ (template) ประกอบด้วยข้อความคงที่ + ตัวแปร:
//
//	{prefix}        prefix ที่ตั้งไว้ (เช่น PV)
//	{yyyy} / {yy}   ปี ค.ศ. ของวันที่เอกสาร
//	{bbbb} / {bb}   ปี พ.ศ. ของวันที่เอกสาร
//	{mm}            เดือนของวันที่เอกสาร
//	{fy}            ปีบัญชี
//	{seq:N}         เลขลำดับ N หลัก (1–10, ไม่ระบุ = 6) — ต้องมี 1 ครั้งพอดี
//
// การนับเลขใหม่ดูจากตัวแปรที่มี: มีเดือน = ทุกเดือน, มีปี = ทุกปี, ไม่มีทั้งสอง = ไม่นับใหม่
// (PeriodKey คือคีย์ของชุดลำดับ — module เก็บเลขล่าสุดต่อ ชนิดเอกสาร + คีย์)
package numbering

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaxLength คือความยาวสูงสุดของเลขที่เอกสาร (คอลัมน์ในฐานข้อมูล)
const MaxLength = 30

// DefaultTemplate ออกเลขแบบเดิมของระบบ เช่น PV2026-000001 (ปีบัญชี, นับใหม่ทุกปีบัญชี)
const DefaultTemplate = "{prefix}{fy}-{seq:6}"

var ErrInvalid = errors.New("invalid document number format")

var (
	tokenPattern   = regexp.MustCompile(`\{([a-z]+)(?::(\d+))?\}`)
	literalPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]*$`)
	prefixPattern  = regexp.MustCompile(`^[A-Z0-9._/-]{0,10}$`)
)

// Format คือรูปแบบเลขที่ของชนิดเอกสารหนึ่ง
type Format struct {
	Prefix   string
	Template string
}

// Context คือข้อมูลของเอกสารที่ใช้ออกเลข
type Context struct {
	Date       time.Time
	FiscalYear int
}

type part struct {
	literal string
	token   string
	width   int
}

func parse(template string) ([]part, error) {
	var parts []part
	seqs, last := 0, 0
	for _, m := range tokenPattern.FindAllStringSubmatchIndex(template, -1) {
		if lit := template[last:m[0]]; lit != "" {
			parts = append(parts, part{literal: lit})
		}
		name := template[m[2]:m[3]]
		p := part{token: name}
		switch name {
		case "prefix", "yyyy", "yy", "bbbb", "bb", "mm", "fy":
			if m[4] >= 0 {
				return nil, fmt.Errorf("%w: {%s} does not take a width", ErrInvalid, name)
			}
		case "seq":
			seqs++
			p.width = 6
			if m[4] >= 0 {
				w, _ := strconv.Atoi(template[m[4]:m[5]])
				if w < 1 || w > 10 {
					return nil, fmt.Errorf("%w: {seq:N} width must be 1 to 10", ErrInvalid)
				}
				p.width = w
			}
		default:
			return nil, fmt.Errorf("%w: unknown variable {%s}", ErrInvalid, name)
		}
		parts = append(parts, p)
		last = m[1]
	}
	if lit := template[last:]; lit != "" {
		parts = append(parts, part{literal: lit})
	}
	for _, p := range parts {
		if p.token == "" && !literalPattern.MatchString(p.literal) {
			return nil, fmt.Errorf("%w: text may only contain letters, digits and . _ / -", ErrInvalid)
		}
	}
	if seqs != 1 {
		return nil, fmt.Errorf("%w: template must contain {seq} exactly once", ErrInvalid)
	}
	return parts, nil
}

// Normalize ตรวจและจัดรูปแบบ (prefix เป็นตัวพิมพ์ใหญ่) — เลขที่ยาวสุดที่เป็นไปได้ต้องไม่เกิน MaxLength
func Normalize(f Format) (Format, error) {
	out := Format{Prefix: strings.ToUpper(strings.TrimSpace(f.Prefix)), Template: strings.TrimSpace(f.Template)}
	if !prefixPattern.MatchString(out.Prefix) {
		return Format{}, fmt.Errorf("%w: prefix must be up to 10 letters, digits or . _ / -", ErrInvalid)
	}
	parts, err := parse(out.Template)
	if err != nil {
		return Format{}, err
	}
	longest := render(parts, out.Prefix, Context{Date: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), FiscalYear: 2026}, -1)
	if len(longest) > MaxLength {
		return Format{}, fmt.Errorf("%w: document numbers would be up to %d characters (max %d)", ErrInvalid, len(longest), MaxLength)
	}
	return out, nil
}

// PeriodKey คือคีย์ของชุดลำดับ: F<ปีบัญชี> / Y<ปี> / M<เดือน> ต่อกันตามตัวแปรที่มี ("" = ไม่นับใหม่)
func (f Format) PeriodKey(c Context) string {
	parts, err := parse(f.Template)
	if err != nil {
		return ""
	}
	var fy, year, month bool
	for _, p := range parts {
		switch p.token {
		case "fy":
			fy = true
		case "yyyy", "yy", "bbbb", "bb":
			year = true
		case "mm":
			month = true
		}
	}
	var b strings.Builder
	if fy {
		fmt.Fprintf(&b, "F%d", c.FiscalYear)
	}
	if year || (month && !fy) {
		fmt.Fprintf(&b, "Y%d", c.Date.Year())
	}
	if month {
		fmt.Fprintf(&b, "M%02d", int(c.Date.Month()))
	}
	return b.String()
}

// Render สร้างเลขที่ — seq เกินจำนวนหลักที่ตั้งจะแสดงเต็ม (ไม่ตัด)
func (f Format) Render(c Context, seq int64) string {
	parts, err := parse(f.Template)
	if err != nil {
		return ""
	}
	return render(parts, f.Prefix, c, seq)
}

// render: seq < 0 = เลขลำดับสูงสุดของจำนวนหลัก (ใช้วัดความยาว)
func render(parts []part, prefix string, c Context, seq int64) string {
	var b strings.Builder
	for _, p := range parts {
		switch p.token {
		case "":
			b.WriteString(p.literal)
		case "prefix":
			b.WriteString(prefix)
		case "yyyy":
			fmt.Fprintf(&b, "%04d", c.Date.Year())
		case "yy":
			fmt.Fprintf(&b, "%02d", c.Date.Year()%100)
		case "bbbb":
			fmt.Fprintf(&b, "%04d", c.Date.Year()+543)
		case "bb":
			fmt.Fprintf(&b, "%02d", (c.Date.Year()+543)%100)
		case "mm":
			fmt.Fprintf(&b, "%02d", int(c.Date.Month()))
		case "fy":
			fmt.Fprintf(&b, "%d", c.FiscalYear)
		case "seq":
			if seq < 0 {
				b.WriteString(strings.Repeat("9", p.width))
			} else {
				fmt.Fprintf(&b, "%0*d", p.width, seq)
			}
		}
	}
	return b.String()
}
