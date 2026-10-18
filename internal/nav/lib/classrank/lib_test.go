package classrank

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// goldenLines is the stdout of a module-nav golden case, one line per entry.
func goldenLines(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

// TestOpenRankFormatRowMatchesGolden calls the library without any runtime and
// checks its rows against the `rank` golden (nav-rank: -n 3 of 11).
func TestOpenRankFormatRowMatchesGolden(t *testing.T) {
	src := SourceOf(filepath.Join("..", "..", "testdata", "fixture", "nav"))
	src.Cache = ""
	r, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	rows := r.Rank("como funciona el ack de alarmas")
	if len(rows) != 11 {
		t.Fatalf("Rank: %d rows, golden 11", len(rows))
	}
	if got := strings.Join(r.QueryTerms("como funciona el ack de alarmas"), ", "); got != "ack, alarm" {
		t.Errorf("QueryTerms = %q, golden \"ack, alarm\"", got)
	}
	golden := goldenLines(t, "nav-rank")
	for k := 0; k < 3; k++ {
		if got, want := FormatRow(k+1, rows[k]), golden[4+k]; got != want {
			t.Errorf("FormatRow(%d) = %q, golden %q", k+1, got, want)
		}
	}
}

// TestOpenErrors pins the message of each unusable input.
func TestOpenErrors(t *testing.T) {
	cases := []struct {
		name  string
		files map[string][]byte
		want  string
	}{
		{"no class index", nil, "class index not found: "},
		{"bad json", map[string][]byte{ClassIndexFile: []byte("{")}, "class index is unreadable: "},
		{"no classes table", map[string][]byte{ClassIndexFile: []byte(`{"x": 1}`)},
			"class index has no 'classes' table: "},
		{"bad xref table", map[string][]byte{ClassIndexFile: []byte(`{"classes": {}}`),
			XrefIndexFile: []byte(`{"class_imported_by": []}`)}, "xref index has no 'class_imported_by' table: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(SourceOf(newBase(t, tc.files)))
			if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("Open error = %v, want prefix %q", err, tc.want)
			}
		})
	}
}

// TestOpenWritesTheCacheItReadsBack: a first Open builds and writes the
// cache, which then holds a ranker that ranks the same.
func TestOpenWritesTheCacheItReadsBack(t *testing.T) {
	old := cacheMinBytes
	cacheMinBytes = 0
	t.Cleanup(func() { cacheMinBytes = old })
	src := SourceOf(newBase(t, map[string][]byte{ClassIndexFile: skipsIndex(t)}))
	built, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	hit := readCache(src.Cache, keyOf(src))
	if hit == nil {
		t.Fatal("no cache readable under the current key after Open")
	}
	want := built.Rank("alarm service")
	if len(want) == 0 {
		t.Fatal("the fixture ranks nothing for 'alarm service'")
	}
	if got := hit.Rank("alarm service"); !reflect.DeepEqual(got, want) {
		t.Errorf("cached Rank = %v, built %v", got, want)
	}
	if again, err := Open(src); err != nil || !reflect.DeepEqual(again.Rank("alarm service"), want) {
		t.Errorf("Open on a cache hit = %v, %v", again, err)
	}
}
