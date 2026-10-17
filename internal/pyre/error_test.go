package pyre

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type errorFixture struct {
	Cases []struct {
		Pattern string `json:"pattern"`
		Flags   Flags  `json:"flags"`
		Error   string `json:"error"`
	} `json:"cases"`
}

func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// escapeError tokenizes a one-escape pattern and returns the re.error it raises.
func escapeError(pattern string) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = r.(*Error).Error()
		}
	}()
	t := newTokenizer(pattern)
	st := &state{groupdict: map[string]int{}, groupwidths: [][2]int{{}}, closed: []bool{false},
		groupnames: []string{""}, lookbehindgroups: -1}
	t.escape(t.get(), st)
	return ""
}

// TestEscapeErrors checks the escape-level cases of testdata/errors.json (CPython's text).
func TestEscapeErrors(t *testing.T) {
	var f errorFixture
	loadJSON(t, "testdata/errors.json", &f)
	n := 0
	for _, c := range f.Cases {
		if !strings.HasPrefix(c.Pattern, `\`) || strings.ContainsAny(c.Pattern, "+*?{") || c.Flags != 0 {
			continue
		}
		n++
		if got := escapeError(c.Pattern); got != c.Error {
			t.Errorf("%q: %q, want %q", c.Pattern, got, c.Error)
		}
	}
	if n < 6 {
		t.Fatalf("only %d escape cases in the fixture", n)
	}
	if got := newError("x", "ab\ncd", 4).Error(); got != "x at position 4 (line 2, column 2)" {
		t.Errorf("multi-line position: %q", got)
	}
}
