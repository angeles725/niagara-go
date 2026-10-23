package blocksearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// classFormCorpus is a root whose blocks name Niagara classes in their B-prefixed form
// (B391 writes `BBacnetIpServerPort`), plus one block naming the bare form.
func classFormCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"niagara-mental-model-bloque1.md": "# Block 1 — port classes\n\nThe `BBacnetIpServerPort` slot sheet.\n",
		"niagara-mental-model-bloque2.md": "# Block 2 — prose\n\nThe BacnetIpServerPort is named bare here, twice: BacnetIpServerPort.\n",
		"niagara-mental-model-bloque3.md": "# Block 3 — unrelated\n\nA box of tools.\n",
		"niagara-mental-model-bloque4.md": "# Block 4 — both\n\nBacnetIpServerPort is BBacnetIpServerPort: BBacnetIpServerPort, BBacnetIpServerPort.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestFindMatchesNiagaraClassForm: a CamelCase query word T also finds the Niagara
// class form B+T, with or without the find cache and in AND mode, while an exact hit
// keeps ranking first; a plain lower-case word never grows a B ("ox" is not "box").
func TestFindMatchesNiagaraClassForm(t *testing.T) {
	for _, cache := range []string{"off", ""} {
		t.Run("index="+cache, func(t *testing.T) {
			t.Setenv("CORPUS_NAV_FIND_INDEX", cache)
			c := New(classFormCorpus(t), t.TempDir())
			for _, q := range []Query{
				{Text: "BacnetIpServerPort", Scope: "blocks", Limit: 50},
				{Text: "BacnetIpServerPort slot", Scope: "blocks", Limit: 50, RequireAll: true},
			} {
				r := c.Find(q)
				if !r.Found || !strings.Contains(r.Stdout(), "`BBacnetIpServerPort` slot sheet") {
					t.Errorf("%+v: found %v, stdout %q; want the B-prefixed class line", q, r.Found, r.Stdout())
				}
			}
			out := c.Find(Query{Text: "BacnetIpServerPort", Scope: "blocks", Limit: 50}).Stdout()
			if b2, b1 := strings.Index(out, "B2 "), strings.Index(out, "B1 "); b2 < 0 || b1 < 0 || b2 > b1 {
				t.Errorf("the exact (bare) hit B2 should rank before the class-form hit B1:\n%s", out)
			}
			// B2 names the word twice, B4 once (and its class form three times): the
			// class form adds nothing to an exact hit, so B2 keeps ranking first.
			if b2, b4 := strings.Index(out, "B2 "), strings.Index(out, "B4 "); b2 < 0 || b4 < 0 || b2 > b4 {
				t.Errorf("exact hits must keep their order (B2 before B4):\n%s", out)
			}
			for _, word := range []string{"ox", "Ox"} {
				if r := c.Find(Query{Text: word, Scope: "blocks", Limit: 50}); r.Found {
					t.Errorf("%s matched box: %q", word, r.Stdout())
				}
			}
		})
	}
}
