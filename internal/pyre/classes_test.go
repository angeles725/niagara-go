package pyre

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// classFixture is testdata/classes.json, written by testdata/gen_classes.py from CPython.
type classFixture struct {
	Unicode  string   `json:"unicode"`
	Subjects []string `json:"subjects"`
	Classes  []struct {
		Pattern string `json:"pattern"`
		Flags   Flags  `json:"flags"`
		Count   int    `json:"count"`
		SHA256  string `json:"sha256"`
	} `json:"classes"`
	Anchors []struct {
		Pattern string    `json:"pattern"`
		Flags   Flags     `json:"flags"`
		Spans   [][][]int `json:"spans"`
	} `json:"anchors"`
	Callsites []struct {
		Pattern  string      `json:"pattern"`
		Subjects []string    `json:"subjects"`
		Matches  [][][][]int `json:"matches"`
	} `json:"callsites"`
}

func iterSpans(re *Regexp, s string) [][][]int {
	out := [][][]int{}
	for _, m := range re.FindIter([]rune(s)) {
		var groups [][]int
		for g := 0; g <= re.Groups(); g++ {
			if a, b := m.Span(g); a >= 0 {
				groups = append(groups, []int{a, b})
			} else {
				groups = append(groups, nil)
			}
		}
		out = append(out, groups)
	}
	return out
}

// TestClassesEveryCodePoint matches each one-character class against every code point
// (surrogates excluded) and compares the members with CPython's (count and SHA-256).
func TestClassesEveryCodePoint(t *testing.T) {
	var f classFixture
	loadJSON(t, "testdata/classes.json", &f)
	text := make([]rune, 0, 0x110000)
	for r := rune(0); r < 0x110000; r++ {
		if r < 0xD800 || r > 0xDFFF {
			text = append(text, r)
		}
	}
	for _, c := range f.Classes {
		t.Run(fmt.Sprintf("%s/flags=%d", c.Pattern, c.Flags), func(t *testing.T) {
			t.Parallel()
			checkClass(t, f.Unicode, text, c.Pattern, c.Flags, c.Count, c.SHA256)
		})
	}
}

// checkClass matches one one-character class against every code point of text and
// compares the members with CPython's count and SHA-256.
func checkClass(t *testing.T, unicode string, text []rune, pattern string, flags Flags, count int, digest string) {
	// one match per run of members keeps the scan fast; each class matches one character
	head, class := "", pattern
	if strings.HasPrefix(class, "(?a)") {
		head, class = "(?a)", class[4:]
	}
	re := MustCompile(head+"(?:"+class+")+", flags)
	var members []string
	for _, m := range re.FindIter(text) {
		for i := m.Start(); i < m.End(); i++ {
			members = append(members, strconv.Itoa(int(text[i])))
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(members, ",")))
	if len(members) != count || hex.EncodeToString(sum[:]) != digest {
		t.Errorf("%q (flags %d): %d code points, want %d (Python, Unicode %s)", pattern, flags, len(members), count, unicode)
	}
}

// TestAnchors pins $ (end or before a final newline; before every newline with
// MULTILINE), \Z and ^ against CPython's finditer spans.
func TestAnchors(t *testing.T) {
	var f classFixture
	loadJSON(t, "testdata/classes.json", &f)
	for _, a := range f.Anchors {
		re := MustCompile(a.Pattern, a.Flags)
		for i, s := range f.Subjects {
			var got [][]int
			for _, m := range re.FindIter([]rune(s)) {
				got = append(got, []int{m.Start(), m.End()})
			}
			if len(got) == 0 && len(a.Spans[i]) == 0 {
				continue
			}
			if !reflect.DeepEqual(got, a.Spans[i]) {
				t.Errorf("%q (flags %d).finditer(%q) = %v, want %v", a.Pattern, a.Flags, s, got, a.Spans[i])
			}
		}
	}
}

// TestCallsites runs Integrante3's patterns (secaudit, greenmaxmatrix, greenmaxtablero) on
// sample lines against CPython: every match and group span.
func TestCallsites(t *testing.T) {
	var f classFixture
	loadJSON(t, "testdata/classes.json", &f)
	for _, c := range f.Callsites {
		re, err := Compile(c.Pattern, 0)
		if err != nil {
			t.Errorf("%q: %v", c.Pattern, err)
			continue
		}
		for i, s := range c.Subjects {
			if got := iterSpans(re, s); !reflect.DeepEqual(got, c.Matches[i]) {
				t.Errorf("%.40q.finditer(%q) = %v, want %v", c.Pattern, s, got, c.Matches[i])
			}
		}
	}
}
