package listing

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func q(raw string) url.Values {
	v, err := url.ParseQuery(raw)
	if err != nil {
		panic(err)
	}
	return v
}

func TestParsePage(t *testing.T) {
	p, err := ParsePage(q(""))
	if err != nil || p != (Page{Number: 1, Size: DefaultPageSize}) {
		t.Fatalf("default = %+v, %v", p, err)
	}
	p, err = ParsePage(q("page=3&page_size=50"))
	if err != nil || p.Offset() != 100 || p.Limit() != 50 {
		t.Fatalf("page 3 = %+v, %v", p, err)
	}
	for _, bad := range []string{"page=0", "page=-1", "page=x", "page_size=0", "page_size=101"} {
		if _, err := ParsePage(q(bad)); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: got %v, want ErrInvalidQuery", bad, err)
		}
	}
}

func TestParseSort(t *testing.T) {
	allowed := []string{"name", "joined_at"}
	def := []Sort{{Key: "name"}}

	got, err := ParseSort(q(""), allowed, def)
	if err != nil || !slices.Equal(got, def) {
		t.Fatalf("default = %v, %v", got, err)
	}
	got, err = ParseSort(q("sort=-joined_at,name"), allowed, def)
	if err != nil || !slices.Equal(got, []Sort{{Key: "joined_at", Desc: true}, {Key: "name"}}) {
		t.Fatalf("multi = %v, %v", got, err)
	}
	// คีย์ที่ไม่อยู่ใน whitelist ต้องถูกปฏิเสธ (กัน SQL injection ผ่าน ORDER BY)
	for _, bad := range []string{"sort=password_hash", "sort=name%3BDROP+TABLE+x", "sort=name,name", "sort=-"} {
		if _, err := ParseSort(q(bad), allowed, def); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: got %v, want ErrInvalidQuery", bad, err)
		}
	}
}

func TestParseSearch(t *testing.T) {
	s, err := ParseSearch(q("q=+somchai+"))
	if err != nil || s != "somchai" {
		t.Fatalf("got %q, %v", s, err)
	}
	if _, err := ParseSearch(url.Values{"q": {strings.Repeat("ก", MaxSearchLength+1)}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("too long: got %v", err)
	}
}

func TestParseDateRange(t *testing.T) {
	from, to, err := ParseDateRange(q("start_date=2026-12-19&end_date=2026-12-31"), "start_date", "end_date")
	if err != nil || from.Format(time.DateOnly) != "2026-12-19" || to.Day() != 31 || from.Location() != time.UTC {
		t.Fatalf("got %v %v %v", from, to, err)
	}
	if from, to, err := ParseDateRange(q(""), "start_date", "end_date"); err != nil || from != nil || to != nil {
		t.Fatalf("empty = %v %v %v", from, to, err)
	}
	// DDMMYYYY, วันที่ไม่มีจริง และช่วงกลับด้าน ต้องถูกปฏิเสธ
	for _, bad := range []string{"start_date=19122026", "start_date=2026-02-30", "start_date=2026-12-31&end_date=2026-12-01"} {
		if _, _, err := ParseDateRange(q(bad), "start_date", "end_date"); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: got %v, want ErrInvalidQuery", bad, err)
		}
	}
}

func TestParseListAndOptional(t *testing.T) {
	upper := func(s string) (string, error) {
		if s == "" || s == "bad" {
			return "", errors.New("bad")
		}
		return strings.ToUpper(s), nil
	}
	got, err := ParseList(q("status=active,suspended"), "status", upper)
	if err != nil || !slices.Equal(got, []string{"ACTIVE", "SUSPENDED"}) {
		t.Fatalf("list = %v, %v", got, err)
	}
	if got, err := ParseList(q(""), "status", upper); err != nil || got != nil {
		t.Fatalf("empty list = %v, %v", got, err)
	}
	if _, err := ParseList(q("status=active,bad"), "status", upper); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("bad list: got %v", err)
	}
	b, err := ParseOptional(q("is_system=true"), "is_system", ParseBool)
	if err != nil || b == nil || !*b {
		t.Fatalf("bool = %v, %v", b, err)
	}
	if b, err := ParseOptional(q(""), "is_system", ParseBool); err != nil || b != nil {
		t.Fatalf("missing bool = %v, %v", b, err)
	}
	if _, err := ParseOptional(q("is_system=yes"), "is_system", ParseBool); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("bad bool: got %v", err)
	}
}
