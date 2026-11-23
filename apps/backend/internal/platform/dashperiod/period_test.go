package dashperiod

import (
	"errors"
	"net/url"
	"reflect"
	"testing"

	"apexis/backend/internal/platform/listing"
)

func TestParseAndRanges(t *testing.T) {
	p, err := Parse(url.Values{"start_date": {"2026-03-01"}, "end_date": {"2026-03-31"}})
	if err != nil {
		t.Fatal(err)
	}
	prev := p.Previous()
	if p.Days() != 31 || prev.Start.Format("2006-01-02") != "2026-01-29" || prev.End.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("previous = %v..%v", prev.Start, prev.End)
	}
	m := p.Months()
	if len(m) != 12 || m[0] != "2025-04" || m[11] != "2026-03" {
		t.Fatalf("months = %v", m)
	}
	if p.TrendEnd().Format("2006-01-02") != "2026-03-31" || p.Earliest().Format("2006-01-02") != "2025-04-01" {
		t.Fatalf("trend = %v / %v", p.TrendEnd(), p.Earliest())
	}
	if !reflect.DeepEqual(Period{Start: p.Start, End: p.End}, p) {
		t.Fatal("copy")
	}
}

func TestParseErrors(t *testing.T) {
	for _, v := range []url.Values{
		{"end_date": {"2026-03-31"}},
		{"start_date": {"2026-03-01"}, "end_date": {"31/03/2026"}},
		{"start_date": {"2026-04-01"}, "end_date": {"2026-03-31"}},
	} {
		if _, err := Parse(v); !errors.Is(err, listing.ErrInvalidQuery) {
			t.Fatalf("%v: %v", v, err)
		}
	}
}
