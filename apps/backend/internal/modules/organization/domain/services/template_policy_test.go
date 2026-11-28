package services

import (
	"errors"
	"strings"
	"testing"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

func TestCheckTemplateHTML(t *testing.T) {
	ok := []string{
		`<h1 style="color:red">{{title}}</h1>{{#each lines}}<td>{{money amount}}</td>{{/each}}`,
		`<img src="{{asset "logo"}}"><img src="data:image/png;base64,AAAA"><a href="#top">x</a>`,
		`<p>Button on = top</p><div style="background:url(data:image/png;base64,AA)"></div>`,
	}
	for _, h := range ok {
		if err := CheckTemplateHTML(h); err != nil {
			t.Fatalf("%q: %v", h, err)
		}
	}
	bad := map[string]string{
		`<SCRIPT>x</SCRIPT>`:                           "<script>",
		`<div onclick = "x">`:                          "on* event handler",
		`<a href="JavaScript:x">`:                      "javascript: URL",
		`<object data="x">`:                            "<iframe>",
		`<link rel="stylesheet" href="/x.css">`:        "<link>",
		`<img srcset="https://x/a.png 2x">`:            "external URL",
		`<div style="background: url('//x/a.png')">`:   "external URL in url()",
		`<style>@import url(data:text/css,x);</style>`: "@import",
	}
	for h, what := range bad {
		err := CheckTemplateHTML(h)
		if !errors.Is(err, domain.ErrTemplateDisallowed) || !strings.Contains(err.Error(), what) {
			t.Fatalf("%q: err = %v, want %s", h, err, what)
		}
	}
	if err := CheckTemplateHTML("   "); !errors.Is(err, domain.ErrInvalidTemplateHTML) {
		t.Fatalf("empty: %v", err)
	}
}

func TestDetectAssetType(t *testing.T) {
	cases := map[string]string{
		"\x89PNG\r\n\x1a\nxxxx": "image/png",
		"\xFF\xD8\xFF\xE0xxxx":  "image/jpeg",
		`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`: "image/svg+xml",
	}
	for data, want := range cases {
		if got, err := DetectAssetType([]byte(data)); err != nil || got != want {
			t.Fatalf("%q: %s %v", data[:4], got, err)
		}
	}
	if _, err := DetectAssetType([]byte(`<svg><a href="javascript:x"/></svg>`)); !errors.Is(err, domain.ErrTemplateDisallowed) {
		t.Fatalf("svg js: %v", err)
	}
	if _, err := DetectAssetType([]byte("GIF89a")); !errors.Is(err, domain.ErrInvalidAssetType) {
		t.Fatalf("gif: %v", err)
	}
	if err := CheckAssetName("logo_2"); err != nil {
		t.Fatal(err)
	}
	if err := CheckAssetName("-x"); !errors.Is(err, domain.ErrInvalidAssetName) {
		t.Fatalf("name: %v", err)
	}
}
