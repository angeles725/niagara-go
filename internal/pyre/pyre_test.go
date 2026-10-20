package pyre

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type matchFixture struct {
	Subjects []string `json:"subjects"`
	Cases    []struct {
		Pattern string     `json:"pattern"`
		Flags   Flags      `json:"flags"`
		Error   string     `json:"error"`
		Search  [][][]int  `json:"search"`
		FindAll [][]string `json:"findall"`
	} `json:"cases"`
}

// TestMatches compares the search spans and findall of every valid pattern with what
// CPython's re produced (the retired testdata/gen.py).
func TestMatches(t *testing.T) { checkMatches(t, "testdata/matches.json", nil) }

// TestFlagMatches pins IGNORECASE, re.ASCII, ASCII|IGNORECASE and the \b / \B
// boundaries on the characters whose folding or word class differs between Unicode and
// ASCII matching (testdata/flags.json). knownFolding lists the documented IGNORECASE
// differences (package doc); a listed case that starts to agree fails too.
func TestFlagMatches(t *testing.T) { checkMatches(t, "testdata/flags.json", knownFolding) }

// knownFolding is "pattern|flags|subject" for the Unicode IGNORECASE folds Python makes
// and .NET's simple lower-casing does not (package doc, Known differences).
var knownFolding = map[string]bool{
	"i|2|\u0131": true, "I|2|\u0131": true, "[a-z]+|2|\u0131": true, "[A-Z]+|2|\u0131": true, "[^a-z]|2|\u0131": true,
	"s|2|\u017f": true, "S|2|\u017f": true, "[a-z]+|2|\u017f": true, "[A-Z]+|2|\u017f": true, "[^a-z]|2|\u017f": true,
	"[s-t]|2|\u017f":                true,
	"\u00b5|2|\u00b5 \u03bc \u039c": true, "\u03bc|2|\u00b5 \u03bc \u039c": true,
	"\u03c3|2|\u03a3\u0391\u03a3 \u03c3\u03b1\u03c2": true,
}

func checkMatches(t *testing.T, path string, known map[string]bool) {
	t.Helper()
	var f matchFixture
	loadJSON(t, path, &f)
	if len(f.Cases) == 0 {
		t.Fatalf("%s: no cases", path)
	}
	seen := map[string]bool{}
	for _, c := range f.Cases {
		re, err := Compile(c.Pattern, c.Flags)
		if c.Error != "" || err != nil {
			if err == nil || err.Error() != c.Error {
				t.Errorf("Compile(%q, %d) error = %v, want %q", c.Pattern, c.Flags, err, c.Error)
			}
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
			key := fmt.Sprintf("%s|%d|%s", c.Pattern, c.Flags, s)
			var diffs []string
			if !reflect.DeepEqual(got, c.Search[i]) {
				diffs = append(diffs, fmt.Sprintf("%q (flags %d).search(%q) = %v, want %v", c.Pattern, c.Flags, s, got, c.Search[i]))
			}
			if c.FindAll != nil {
				want, all := c.FindAll[i], re.FindAll(s)
				if (len(want) != 0 || len(all) != 0) && !reflect.DeepEqual(all, want) {
					diffs = append(diffs, fmt.Sprintf("%q (flags %d).findall(%q) = %q, want %q", c.Pattern, c.Flags, s, all, want))
				}
			}
			if known[key] {
				seen[key] = true
				if len(diffs) == 0 {
					t.Errorf("known difference %q now matches Python: drop it from the list", key)
				}
				continue
			}
			for _, d := range diffs {
				t.Error(d)
			}
		}
	}
	for key := range known {
		if !seen[key] {
			t.Errorf("known difference %q is not a case of %s", key, path)
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
