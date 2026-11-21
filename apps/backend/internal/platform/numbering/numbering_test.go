package numbering

import (
	"errors"
	"testing"
	"time"
)

func TestRenderAndPeriodKey(t *testing.T) {
	c := Context{Date: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC), FiscalYear: 2025}
	cases := []struct {
		template, want, key string
	}{
		{DefaultTemplate, "PV2025-000001", "F2025"},
		{"{prefix}{yy}{mm}-{seq:5}", "PV2602-00001", "Y2026M02"},
		{"{prefix}{bb}{mm}-{seq:4}", "PV6902-0001", "Y2026M02"},
		{"{prefix}{bbbb}/{seq}", "PV2569/000001", "Y2026"},
		{"{prefix}-{seq:3}", "PV-001", ""},
		{"{prefix}{fy}{mm}{seq:3}", "PV202502001", "F2025M02"},
	}
	for _, tc := range cases {
		f, err := Normalize(Format{Prefix: "pv", Template: tc.template})
		if err != nil {
			t.Fatalf("%s: %v", tc.template, err)
		}
		if got := f.Render(c, 1); got != tc.want {
			t.Errorf("%s: render = %s, want %s", tc.template, got, tc.want)
		}
		if got := f.PeriodKey(c); got != tc.key {
			t.Errorf("%s: key = %q, want %q", tc.template, got, tc.key)
		}
	}
	if got := (Format{Prefix: "A", Template: "{prefix}{seq:2}"}).Render(c, 123); got != "A123" {
		t.Errorf("overflowing seq = %s", got)
	}
}

func TestNormalizeRejects(t *testing.T) {
	for name, f := range map[string]Format{
		"no seq":        {Prefix: "PV", Template: "{prefix}{yy}"},
		"two seq":       {Prefix: "PV", Template: "{seq}{seq}"},
		"unknown token": {Prefix: "PV", Template: "{prefix}{dd}{seq}"},
		"bad literal":   {Prefix: "PV", Template: "{prefix} {seq}"},
		"bad width":     {Prefix: "PV", Template: "{seq:11}"},
		"width on year": {Prefix: "PV", Template: "{yy:2}{seq}"},
		"bad prefix":    {Prefix: "P V", Template: DefaultTemplate},
		"too long":      {Prefix: "ABCDEFGHIJ", Template: "{prefix}-{bbbb}{mm}-{fy}-{seq:10}"},
	} {
		if _, err := Normalize(f); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
