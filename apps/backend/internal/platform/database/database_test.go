package database

import "testing"

func TestLikeContainsEscapesWildcards(t *testing.T) {
	cases := map[string]string{
		"Somchai": `%somchai%`,
		"50%":     `%50\%%`,
		"a_b":     `%a\_b%`,
		`c:\x`:    `%c:\\x%`,
	}
	for in, want := range cases {
		if got := LikeContains(in); got != want {
			t.Errorf("LikeContains(%q) = %q, want %q", in, got, want)
		}
	}
}
