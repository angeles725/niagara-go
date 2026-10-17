package searchdoor

import (
	"reflect"
	"testing"
)

func keysOf(items []Item) []string {
	var out []string
	for _, it := range items {
		if it.Keyed {
			out = append(out, it.Key)
		} else {
			out = append(out, "<none>")
		}
	}
	return out
}

func TestParsers(t *testing.T) {
	tests := []struct {
		name  string
		parse parser
		out   string
		scope []string
		keys  []string
	}{
		{"blocks", parseBlocks, "B7 · Block 7 — x\n    12: hit\nB9 · y", nil, []string{"B7", "B9"}},
		{"blocks fall back to generic rows", parseBlocks, "dump\n  row", nil, []string{"<none>", "<none>"}},
		{"docs: path, else title", parseDocs, "3 files\n  [g] A page\n    L1: hit\n    path: g/A.html\n  [g] B page\n  ... and 1 more",
			[]string{"3 files"}, []string{"g/A.html", "B page"}},
		{"rank keeps the backend order", parseRank, "Rank 'q': 2 matching classes\n1\t2.0\tBB\tm\tp\n2\t1.0\tBA\tm\tp",
			[]string{"Rank 'q': 2 matching classes"}, []string{"BB", "BA"}},
		{"license rows skip headers and rules", parseLicense, "Call sites: 2\nCLASS  MODULE\n-----\n  BX  m  c\nTry: x",
			[]string{"Call sites: 2"}, []string{"BX"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, items := tt.parse(tt.out)
			if !reflect.DeepEqual(scope, tt.scope) || !reflect.DeepEqual(keysOf(items), tt.keys) {
				t.Errorf("scope %q keys %q, want %q %q", scope, keysOf(items), tt.scope, tt.keys)
			}
		})
	}
}

func TestGrepRanksByTermsInTheClassName(t *testing.T) {
	out := "Grep /x/\n  [m]\n    a.BOther:1 alarm history\n    a.BOther:2 alarm\n    a.BAlarmX:3 only\n"
	_, items := parseGrep(out, []string{"alarm", "history"})
	if got := keysOf(items); !reflect.DeepEqual(got, []string{"a.BAlarmX", "a.BOther"}) {
		t.Errorf("keys = %q", got)
	}
	if items[1].Display != "a.BOther  [m]  2/2 terms, 2 lines" {
		t.Errorf("display = %q", items[1].Display)
	}
}

func TestTokensOrderExactClassesThenOccurrencesAndTheMirrorLast(t *testing.T) {
	out := "TOKEN: 'BFoo'\n  BUse  1  docSource-doc\n  BUse  2  a-rt\n  BUse  3  b-rt\n  BUse  4  c-rt\n  BFoo  5  foo-rt\n"
	scope, items := parseTokens([]string{out}, []string{"BFoo"})
	if !reflect.DeepEqual(scope, []string{"TOKEN: 'BFoo'"}) || !reflect.DeepEqual(keysOf(items), []string{"BFoo", "BUse"}) {
		t.Fatalf("scope %q keys %q", scope, keysOf(items))
	}
	if items[1].Display != "BUse  [a-rt, b-rt, c-rt, …]  4 occurrences" {
		t.Errorf("display = %q", items[1].Display)
	}
}

func TestMergeKeepsTheFirstRowPerKey(t *testing.T) {
	got := merge([]Item{keyed("A", "a1", "")}, []Item{keyed("B", "b", ""), keyed("A", "a2", ""), unkeyed("x"), unkeyed("x")})
	var shown []string
	for _, it := range got {
		shown = append(shown, it.Display)
	}
	if !reflect.DeepEqual(shown, []string{"a1", "b", "x", "x"}) {
		t.Errorf("merge = %q", shown)
	}
}

func TestClip(t *testing.T) {
	if got := clip("  a\t b  ", 10); got != "a b" {
		t.Errorf("clip = %q", got)
	}
	if got := clip("ábcdef", 4); got != "ábc…" {
		t.Errorf("clip = %q", got)
	}
}
