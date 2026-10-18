package corpusnav

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
)

// testCorpus is the fixture corpus (niagara-research tools/tests/fixtures/golden-cli/corpus).
func testCorpus(t *testing.T) (*corpus, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	return newCorpus("testdata/corpus", "", &out, &errb), &out, &errb
}

func TestPyPath(t *testing.T) {
	for in, want := range map[string]string{"a//b/./c/": "a/b/c", "./a": "a", ".": ".", "": ".", "/x/../y": "/x/../y",
		"//srv/x": "/srv/x"} {
		if got := pyPath(in); got != want {
			t.Errorf("pyPath(%q) = %q, want %q", in, got, want)
		}
	}
	if !blocksearch.PathLess("docs/a/x.md", "docs/a-b.md") {
		t.Error("Path order compares components, not strings")
	}
}

// TestTargets pins corpus_targets("all") and load_catalog (values from the Python tool).
func TestTargets(t *testing.T) {
	c, _, _ := testCorpus(t)
	var got [][2]string
	for _, tg := range c.targets("all") {
		got = append(got, [2]string{tg.display, relTo(tg.path, c.root)})
	}
	want := [][2]string{{"B1-3", "niagara-mental-model.md"}, {"B4", "niagara-mental-model-bloque4.md"},
		{"B5", "niagara-mental-model-bloque5.md"}, {"B6", "niagara-mental-model-bloque6-security-incident.md"},
		{"BTI", "niagara-mental-model-bloque-test-infrastructure.md"},
		{"notes/bloque40-A-palettes-misc", "notes/bloque40-A-palettes-misc.md"},
		{"os413-B3", "examinacion-optimizer-4.13/os413-bloque3.md"}, {"R2", "niagara-reflow-block2.md"},
		{"docs/guide-alarms.md", "docs/guide-alarms.md"}, {"docs/sub/deep.md", "docs/sub/deep.md"},
		{"retros/2026-01-01-retro.md", "retros/2026-01-01-retro.md"}}
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
