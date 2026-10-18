package codesearch

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyre"
)

// corpus writes an organized/ tree: module/sub/vineflower/<rel> -> text.
func corpus(t *testing.T, files map[string]string) string {
	t.Helper()
	org := t.TempDir()
	for rel, text := range files {
		p := filepath.Join(org, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return org
}

func testCorpus(t *testing.T) string {
	return corpus(t, map[string]string{
		"a/a-rt/vineflower/p/A.java":   "class A {\n  void ack() {}\n}\n",
		"a/a-rt/vineflower/p/B.java":   "class B {\r\n  ack();\r\n}",
		"a/a-wb/vineflower/C.java":     "ack\n",
		"b/b/vineflower/D.java":        "no match\n",
		"b/b-zz/vineflower/E.java":     "ack ack\n",
		"b/b-zz/vineflower/notes.txt":  "ack\n",
		"c/docs/readme/vineflower.txt": "ack\n", // not a submodule
	})
}

func hits(r GrepResult) []string {
	var out []string
	for _, h := range r.Hits {
		out = append(out, FormatGrepRow(h)+" @"+h.Sub)
	}
	return out
}

func TestGrepOrderCountsAndFilters(t *testing.T) {
	org := testCorpus(t)
	ack := pyre.MustCompile(`ack`, 0)
	tests := []struct {
		name                     string
		limit                    int
		f                        GrepFilter
		want                     []string
		searched, withHits, subs int
	}{
		{"whole corpus in sorted walk order", 30, GrepFilter{},
			[]string{"    A:2   void ack() {} @a-rt", "    B:2   ack(); @a-rt", "    C:1 ack @a-wb", "    E:1 ack ack @b-zz"},
			5, 4, 4},
		{"limit stops after the hit that reaches it", 2, GrepFilter{},
			[]string{"    A:2   void ack() {} @a-rt", "    B:2   ack(); @a-rt"}, 2, 2, 1},
		{"limit 0 stops after the first file, hit or not", 0, GrepFilter{},
			[]string{"    A:2   void ack() {} @a-rt"}, 1, 1, 1},
		{"submodule filter", 30, GrepFilter{Sub: "a-wb"}, []string{"    C:1 ack @a-wb"}, 1, 1, 1},
		{"type standalone", 30, GrepFilter{Type: "standalone"}, nil, 1, 0, 1},
		{"type other (unknown suffix)", 30, GrepFilter{Type: "other"}, []string{"    E:1 ack ack @b-zz"}, 1, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Grep(org, ack, tt.limit, tt.f)
			if got := hits(r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hits = %q, want %q", got, tt.want)
			}
			if r.FilesSearched != tt.searched || r.FilesWithMatches != tt.withHits || r.Submodules != tt.subs {
				t.Errorf("searched %d with hits %d submodules %d, want %d %d %d",
					r.FilesSearched, r.FilesWithMatches, r.Submodules, tt.searched, tt.withHits, tt.subs)
			}
		})
	}
}

func TestGrepMatchesTheLineWithItsNewline(t *testing.T) {
	org := corpus(t, map[string]string{"m/m-rt/vineflower/A.java": "x;\ny; \nz;"})
	// re.search sees "x;\n": `;\s` matches before the newline, `;$` matches
	// before a final newline and at the end of the last line.
	for pat, want := range map[string]int{`;\s`: 2, `;$`: 2, `;\Z`: 1, `\n`: 2} {
		if r := Grep(org, pyre.MustCompile(pat, 0), 30, GrepFilter{}); len(r.Hits) != want {
			t.Errorf("%s: %d hits, want %d", pat, len(r.Hits), want)
		}
	}
}

func TestGrepCountsAnUnreadableFile(t *testing.T) {
	org := corpus(t, map[string]string{"m/m-rt/vineflower/B.java": "ack\n"})
	if err := os.Symlink(filepath.Join(org, "gone"), filepath.Join(org, "m", "m-rt", "vineflower", "A.java")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("no symlink privilege: %v", err)
		}
		t.Fatal(err)
	}
	r := Grep(org, pyre.MustCompile("ack", 0), 30, GrepFilter{})
	if r.FilesSearched != 2 || len(r.Hits) != 1 || r.Hits[0].Class != "B" {
		t.Errorf("searched %d hits %+v", r.FilesSearched, r.Hits)
	}
}

func TestGrepFormatters(t *testing.T) {
	long := strings.Repeat("é", 130)
	if got := FormatGrepRow(GrepHit{Class: "A", Line: 7, Text: long}); got != "    A:7 "+strings.Repeat("é", 117)+"..." {
		t.Errorf("row not cut at 120 code points: %q", got)
	}
	r := GrepResult{Hits: make([]GrepHit, 3), FilesWithMatches: 2, FilesSearched: 12345, Submodules: 1234}
	if got := FormatGrepHeader("a|b", r, 5); got != "  grep /a|b/ — 3 matches in 2 files (12,345 files searched)" {
		t.Errorf("header %q", got)
	}
	if got := FormatGrepHeader("a|b", r, 3); got != "  grep /a|b/ — 3+ matches in 2 files (12,345 files searched)" {
		t.Errorf("full header %q", got)
	}
	if got := FormatGrepModule("alarm-rt"); got != "  [alarm-rt]" {
		t.Errorf("module %q", got)
	}
	for f, want := range map[GrepFilter]string{
		{}:                        "12,345 .java files in 1,234 submodules under /o",
		{Sub: "x-rt", Type: "rt"}: "12,345 .java files in 1,234 submodules under /o (--module x-rt)",
		{Type: "se"}:              "12,345 .java files in 1,234 submodules under /o (--type se)",
	} {
		if got := GrepScope("/o", r, f); got != want {
			t.Errorf("scope %+v = %q", f, got)
		}
	}
}
