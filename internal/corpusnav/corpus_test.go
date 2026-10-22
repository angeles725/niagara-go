package corpusnav

import (
	"bytes"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pypath"
)

// testCorpus is the fixture corpus (niagara-research tools/tests/fixtures/golden-cli/corpus).
func testCorpus(t *testing.T) (*corpus, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	return newCorpus("testdata/corpus", "", &out, &errb), &out, &errb
}

func TestPyPath(t *testing.T) {
	for in, want := range map[string]string{"a//b/./c/": "a/b/c", "./a": "a", ".": ".", "": ".", "/x/../y": "/x/../y",
		"//srv/x": "//srv/x", "///srv/x": "/srv/x"} {
		if runtime.GOOS == "windows" && strings.HasPrefix(in, "//") {
			continue // a UNC share on Windows (pypath TestWindowsPathlib)
		}
		if got := pyPath(in); got != filepath.FromSlash(want) {
			t.Errorf("pyPath(%q) = %q, want %q", in, got, filepath.FromSlash(want))
		}
	}
	if !pypath.Less(filepath.FromSlash("docs/a/x.md"), filepath.FromSlash("docs/a-b.md")) {
		t.Error("Path order compares components, not strings")
	}
}

// TestCorpusPathsFollowTheHost: corpus paths are pathlib's, so on Windows a docs id is
// "docs\\guide.md" as the Python tool prints it.
func TestCorpusPathsFollowTheHost(t *testing.T) {
	c, _, _ := testCorpus(t)
	if want := filepath.FromSlash("testdata/corpus/docs"); c.docs != want {
		t.Errorf("docs = %q, want %q", c.docs, want)
	}
	if got := relTo(pyJoin(c.docs, "a.md"), c.root); got != filepath.FromSlash("docs/a.md") {
		t.Errorf("relTo = %q", got)
	}
}

// TestTargets pins corpus_targets("all") and load_catalog (values from the Python tool).
func TestTargets(t *testing.T) {
	c, _, _ := testCorpus(t)
	targets, err := c.targets("all")
	if err != nil {
		t.Fatal(err)
	}
	var got [][2]string
	for _, tg := range targets {
		got = append(got, [2]string{tg.display, filepath.ToSlash(relTo(tg.path, c.root))})
	}
	want := [][2]string{{"B1-3", "niagara-mental-model.md"}, {"B4", "niagara-mental-model-bloque4.md"},
		{"B5", "niagara-mental-model-bloque5.md"}, {"B6", "niagara-mental-model-bloque6-security-incident.md"},
		{"BTI", "niagara-mental-model-bloque-test-infrastructure.md"},
		{"notes/bloque40-A-palettes-misc", "notes/bloque40-A-palettes-misc.md"},
		{"os413-B3", "examinacion-optimizer-4.13/os413-bloque3.md"}, {"R2", "niagara-reflow-block2.md"},
		{"docs/guide-alarms.md", "docs/guide-alarms.md"}, {"docs/sub/deep.md", "docs/sub/deep.md"},
		{"retros/2026-01-01-retro.md", "retros/2026-01-01-retro.md"}}
	for i, w := range want {
		if strings.HasPrefix(w[0], "docs/") || strings.HasPrefix(w[0], "retros/") {
			want[i][0] = filepath.FromSlash(w[0]) // a docs/retros id is str(path.relative_to(root))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("targets =\n%v\nwant\n%v", got, want)
	}
	cat := c.loadCatalog()
	wantCat := map[string]string{"1-3": "Bloques 1-3 consolidados", "4": "Enrutamiento de alarmas",
		"5": "Histories and trend logs", "6": "Security incident review", "TI": "test infrastructure"}
	if !reflect.DeepEqual(cat, wantCat) {
		t.Errorf("catalog = %v", cat)
	}
}
