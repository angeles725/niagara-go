package pyre

import (
	"reflect"
	"strings"
	"testing"
)

type matchFixture struct {
	Subjects []string `json:"subjects"`
	Cases    []struct {
		Pattern string     `json:"pattern"`
		Flags   Flags      `json:"flags"`
		Search  [][][]int  `json:"search"`
		FindAll [][]string `json:"findall"`
	} `json:"cases"`
}

// TestMatches compares the search spans and findall of every valid pattern with what
// CPython's re produced (testdata/gen.py).
func TestMatches(t *testing.T) {
	var f matchFixture
	loadJSON(t, "testdata/matches.json", &f)
	for _, c := range f.Cases {
		re, err := Compile(c.Pattern, c.Flags)
		if err != nil {
			t.Errorf("Compile(%q, %d): %v", c.Pattern, c.Flags, err)
			continue
		}
		for i, s := range f.Subjects {
			var got [][]int
			if m := re.Search(s); m != nil {
				for g := 0; g <= re.Groups(); g++ {
					if a, b := m.Span(g); a >= 0 {
						got = append(got, []int{a, b})
					} else {
						got = append(got, nil)
					}
				}
			}
			if !reflect.DeepEqual(got, c.Search[i]) {
				t.Errorf("%q (flags %d).search(%q) = %v, want %v", c.Pattern, c.Flags, s, got, c.Search[i])
			}
			if c.FindAll != nil {
				want, all := c.FindAll[i], re.FindAll(s)
				if len(want) != 0 || len(all) != 0 {
					if !reflect.DeepEqual(all, want) {
						t.Errorf("%q.findall(%q) = %q, want %q", c.Pattern, s, all, want)
					}
				}
			}
		}
	}
}

func TestNamedGroupsAndRunes(t *testing.T) {
	re := MustCompile(`(?P<stem>.+)-block(?P<n>\d+)\.md$`, 0)
	m := re.Search("niagara-reflow-block2.md")
	if m == nil {
		t.Fatal("no match")
	}
	if s, _ := m.Named("stem"); s != "niagara-reflow" {
		t.Errorf("stem = %q", s)
	}
	if n, _ := m.Named("n"); n != "2" {
		t.Errorf("n = %q", n)
	}
	text := []rune("añoB5 x")
	if m := MustCompile(`B(\d)`, 0).SearchRunes(text, 0); m == nil || m.Start() != 3 {
		t.Errorf("rune positions: %v", m)
	}
	if _, err := Compile("abc\n(", 0); err == nil || !strings.Contains(err.Error(), "line 2, column 1") {
		t.Errorf("multi-line error = %v", err)
	}
}
