package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runLexicon runs `lexicon` on a root holding the lexicon-index.json fixture.
func runLexicon(t *testing.T, p LexiconParams) (string, int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "lexicon-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "indexes", "lexicon-index.json"), string(data))
	var out bytes.Buffer
	e := &Env{BaseDir: root, Out: &out}
	if err := e.Lexicon(p); err != nil {
		t.Fatal(err)
	}
	return out.String(), e.misses
}

const lexiconUnreadable = "\nUNREADABLE (1): broken-rt.jar: zip: not a valid zip file\n"

// TestLexiconKeyShowsEveryLanguage: a key is printed per module, the base
// value first and every translation of it after, each naming its file.
func TestLexiconKeyShowsEveryLanguage(t *testing.T) {
	got, misses := runLexicon(t, LexiconParams{Query: "BooleanLatch.displayName"})
	want := `LEXICON keys matching 'BooleanLatch.displayName' (2 keys, 3 matching entries):

kitControl:BooleanLatch.displayName
  base  Boolean Latch  (kitControl-rt.jar!kitControl-rt.lexicon)
  fr    Verrou booléen  (niagaraLexiconFr-rt.jar!kitControl.lexicon)

baja:BooleanLatch.displayName
  base  Latch (baja)  (baja.jar!baja.lexicon)
` + lexiconUnreadable
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestLexiconKeyFilters: --module keeps one module's lexicon (its
// translations included), --lang one language (a translation still shows
// the base value it overrides), --lang base the base files only; a key is
// matched exactly, then in any letter case, then as a glob.
func TestLexiconKeyFilters(t *testing.T) {
	for _, tt := range []struct {
		p          LexiconParams
		want, lack string
	}{
		{LexiconParams{Query: "BooleanLatch.displayName", Module: "BAJA"}, "(1 keys, 1 matching entries)", "kitControl:"},
		{LexiconParams{Query: "latch.description", Lang: "fr"}, "(1 keys, 1 matching entries):\n\nkitControl:latch.description\n" +
			"  base  Holds the input value when the clock goes true  (kitControl-rt.jar!kitControl-rt.lexicon)\n" +
			"  fr    Garde la valeur d'entrée  (niagaraLexiconFr-rt.jar!kitControl.lexicon)\n", ""},
		{LexiconParams{Query: "booleanlatch.DISPLAYNAME", Lang: "base"}, "(2 keys, 2 matching entries)", "  fr "},
		{LexiconParams{Query: "*Latch.*"}, "(3 keys, 4 matching entries)", ""},
		{LexiconParams{Query: "empty"}, "kitControl:empty\n  base    (kitControl-rt.jar!kitControl-rt.lexicon)\n", ""},
	} {
		got, misses := runLexicon(t, tt.p)
		if misses != 0 || !strings.Contains(got, tt.want) || (tt.lack != "" && strings.Contains(got, tt.lack)) {
			t.Errorf("%+v: misses %d, got:\n%s", tt.p, misses, got)
		}
	}
}

// TestLexiconKeysShownAreCapped: past lexiconKeysShown keys the text answer
// says how many more there are; --json lists them all.
func TestLexiconKeysShownAreCapped(t *testing.T) {
	defer func(n int) { lexiconKeysShown = n }(lexiconKeysShown)
	lexiconKeysShown = 1
	got, _ := runLexicon(t, LexiconParams{Query: "*"})
	if !strings.Contains(got, "(6 keys, 9 matching entries)") || !strings.Contains(got, "\n... and 5 more keys (--json lists them all)\n") {
		t.Errorf("got:\n%s", got)
	}
	got, _ = runLexicon(t, LexiconParams{Query: "*", JSON: true})
	if strings.Count(got, `"key": `) != 6 {
		t.Errorf("json does not list every key:\n%s", got)
	}
}

func TestLexiconJSON(t *testing.T) {
	got, _ := runLexicon(t, LexiconParams{Query: "clock", Lang: "fr"})
	if !strings.Contains(got, "  fr    Horloge  (niagaraLexiconFr-rt.jar!baja.lexicon)\n") {
		t.Errorf("got:\n%s", got)
	}
	got, _ = runLexicon(t, LexiconParams{Query: "clock", JSON: true})
	for _, want := range []string{"{\n  \"query\": \"clock\",\n  \"text\": \"\",\n  \"module\": \"\",\n  \"lang\": \"all\",\n  \"keys\": [\n    {\n      \"module\": \"baja\",\n      \"key\": \"clock\",",
		`"language": "fr",
          "overrides": "baja",
          "jar": "niagaraLexiconFr-rt.jar",
          "path": "baja.lexicon",
          "value": "Horloge"`, `"unreadable": [
    {
      "jar": "broken-rt.jar",`} {
		if !strings.Contains(got, want) {
			t.Errorf("json lacks %s:\n%s", want, got)
		}
	}
}

// TestLexiconMiss: an unknown key is a miss naming lexicon-index.json, its
// files and entries and its unreadable files; a call without a key is a
// usage error.
func TestLexiconMiss(t *testing.T) {
	got, misses := runLexicon(t, LexiconParams{Query: "nope"})
	if misses != 1 || !strings.Contains(got, "lexicon-index.json: 4 lexicon files (2 base, 2 translations), 9 entries (1 unreadable: broken-rt.jar)") {
		t.Errorf("misses %d, got:\n%s", misses, got)
	}
	e := &Env{BaseDir: t.TempDir(), Out: &bytes.Buffer{}}
	write(t, filepath.Join(e.BaseDir, "indexes", "lexicon-index.json"), `{"files": [], "errors": []}`)
	for _, p := range []LexiconParams{{}} {
		if err := e.Lexicon(p); err == nil || !strings.Contains(err.Error(), "lexicon needs a key or glob") {
			t.Errorf("%+v: err = %v", p, err)
		}
	}
}

// TestLexiconCLI: lexicon takes a key, --module, --lang and --json; a miss
// exits 1 and no key is a usage error (exit 2).
func TestLexiconCLI(t *testing.T) {
	dir := repo(t, `{"default": "1.0.0.1", "versions": {"1.0.0.1": {"install_dir": ".", "data_root": "."}}}`, ".")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "lexicon-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "indexes", "lexicon-index.json"), string(data))
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"lexicon", "clock"}, 0, "LEXICON keys matching 'clock' (1 keys, 2 matching entries):\n"},
		{[]string{"lexicon", "*Latch.*", "--module", "kit*", "--lang", "fr"}, 0, "LEXICON keys matching '*Latch.*' (1 keys, 1 matching entries):\n"},
		{[]string{"lexicon", "clock", "--json"}, 0, "{\n  \"query\": \"clock\","},
		{[]string{"lexicon", "nope"}, 1, ""},
		{[]string{"lexicon"}, 2, ""},
	} {
		out, errs, code := run(t, dir, tt.args...)
		if code != tt.code || !strings.HasPrefix(out, tt.out) {
			t.Errorf("%v: code %d stdout %q stderr %q", tt.args, code, out, errs)
		}
	}
}
