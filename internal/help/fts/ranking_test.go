package fts_test

// Search-quality checks carried over from niagara-help
// tests/test_search_quality.py (TestTokens, TestRanking,
// TestCodeDoesNotHideManuals) that the Python-parity fixtures
// (testdata/terms.json, ../testdata/fulltext-search.json) do not already pin:
// each builds a small synthetic corpus with the Go index builder and asserts
// the ranking rule by name, so a regression reads as the rule it breaks.
// This is an external test package because helpbuild/fulltext imports fts.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
)

// rankCorpus writes files (path relative to a fresh data root -> text),
// builds its indexes/fulltext.sqlite and returns the root.
func rankCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	addFiles(t, root, files)
	return root
}

// addFiles writes files under root and (re)builds its full-text index.
func addFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fulltext.Build(root, fulltext.Options{Workers: 1}); err != nil {
		t.Fatal(err)
	}
}

func search(t *testing.T, root, query string, srcs ...string) *fts.Result {
	t.Helper()
	r, err := fts.Search(root, "", query, srcs, "and")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func keys(r *fts.Result) []string {
	out := []string{}
	for _, h := range r.Hits {
		out = append(out, h.Key())
	}
	return out
}

func page(title, body string) string { return "TITLE: " + title + "\nSOURCE: x\n\n" + body + "\n" }

// TestRankingQueryTerms: the token rules of TestTokens that terms.json does
// not name (its other cases are pinned there against Python).
func TestRankingQueryTerms(t *testing.T) {
	// "alarma" was a substring of "alarmAck": a false hit before T8.
	if fts.TermSet("call alarmAck() now")["alarma"] {
		t.Error(`TermSet("call alarmAck() now") holds "alarma": whole words and camelCase parts only`)
	}
	for q, want := range map[string][][]string{
		"how do I configure BACnet BBMD": {{"configur*"}, {"bacnet"}, {"bbmd"}},
		"how do I":                       {{"how"}, {"do"}, {"i"}},
		"punto":                          {{"point"}},
		"horario":                        {{"schedul*"}},
		"alarmas":                        {{"alarm"}},
	} {
		got := [][]string{}
		for _, g := range fts.ParseQuery(q, false) {
			got = append(got, []string(g))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseQuery(%q) = %q, want %q", q, got, want)
		}
	}
	if a, b := fts.Stem("events"), fts.Stem("event"); a != b {
		t.Errorf("Stem(events) = %q, Stem(event) = %q: plural and singular must share a term", a, b)
	}
	if got := fts.Stem("properties"); got != "property" {
		t.Errorf("Stem(properties) = %q, want property", got)
	}
}

// TestRankingTitledShortPageBeatsLongPage: the title boost and BM25 length
// normalization put a short page titled with the words above a long page
// that mentions them once.
func TestRankingTitledShortPageBeatsLongPage(t *testing.T) {
	filler := strings.Repeat("unrelated filler words about other matters\n", 400)
	root := rankCorpus(t, map[string]string{
		"guides-clean/Alarms/escalation.txt": page("Alarm escalation", "Escalation raises an unacknowledged alarm."),
		"guides-clean/Misc/long.txt":         page("Everything", filler+"alarm escalation mentioned once\n"+filler),
	})
	got := keys(search(t, root, "alarm escalation", "guides-clean"))
	if len(got) == 0 || got[0] != "guides-clean/Alarms/escalation.txt" {
		t.Errorf("hits %q: the short titled page must rank first", got)
	}
}

// TestRankingChangeLogIsDemoted: a change log that repeats the words ranks
// below the API page that names them once.
func TestRankingChangeLogIsDemoted(t *testing.T) {
	root := rankCorpus(t, map[string]string{
		"devguide-clean/changes.txt": page("changes.txt", "alarm escalation alarm escalation fixed"),
		"devguide-clean/alarms.txt":  page("Alarm API", "Alarm escalation API."),
	})
	got := keys(search(t, root, "alarm escalation", "devguide-clean"))
	if want := []string{"devguide-clean/alarms.txt", "devguide-clean/changes.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hits %q, want %q", got, want)
	}
}

const codeQuery = "bacnet priority array export write"

// codeCorpus is TestCodeDoesNotHideManuals' corpus: five code files and one
// devguide example with every word, and a manual that matches most of them.
func codeCorpus(t *testing.T) string {
	files := map[string]string{
		"devguide-rc/x/Example.java": "// bacnet priority array export write example\n",
		"docs-text/docBacnet.txt": "TITLE: docBacnet\nSOURCE: docBacnet.pdf\n\n" +
			strings.Repeat("The BACnet priority array of an export point.\n", 20) +
			strings.Repeat("Commands to the priority array go to the export.\n", 20),
		"guides-clean/Misc/other.txt": page("Other", "Nothing about it."),
	}
	for i := range 5 {
		files[fmt.Sprintf("source/bacnet-rt/x/BDescriptor%d.java", i)] =
			fmt.Sprintf("class BDescriptor%d { // bacnet priority array export write\n}\n", i)
	}
	return rankCorpus(t, files)
}

func inTop5(ks []string, key string) bool {
	for i, k := range ks {
		if i < 5 && k == key {
			return true
		}
	}
	return false
}

// TestRankingCodeDoesNotHideManuals: code (and a few devguide pages) that
// match every word must not keep the search in AND mode when few
// documentation files do (T15b): the manual matching most words is the answer.
func TestRankingCodeDoesNotHideManuals(t *testing.T) {
	const manual = "docs-text/docBacnet.txt"
	t.Run("only code matches every word", func(t *testing.T) {
		r := search(t, codeCorpus(t), codeQuery)
		if !inTop5(keys(r), manual) || !r.Fallback || r.AndHits != 6 {
			t.Errorf("hits %q fallback %v and_hits %d: want the manual in the top 5, a fallback, 6 AND hits",
				keys(r), r.Fallback, r.AndHits)
		}
	})
	t.Run("a small devguide AND set still lists the manual", func(t *testing.T) {
		root := codeCorpus(t)
		more := map[string]string{}
		for i := range 4 {
			more[fmt.Sprintf("devguide-clean/p%d.txt", i)] = page(fmt.Sprintf("p%d", i), codeQuery)
		}
		addFiles(t, root, more)
		if got := keys(search(t, root, codeQuery)); !inTop5(got, manual) {
			t.Errorf("hits %q: the manual must stay in the top 5", got)
		}
	})
	t.Run("AND mode is kept when enough docs match every word", func(t *testing.T) {
		root := codeCorpus(t)
		more := map[string]string{}
		for i := range fts.MinAndHits {
			more[fmt.Sprintf("guides-clean/B/g%d.txt", i)] = page(fmt.Sprintf("g%d", i), codeQuery)
		}
		addFiles(t, root, more)
		if r := search(t, root, codeQuery); r.Fallback || r.ModeUsed != "and" {
			t.Errorf("mode %s fallback %v, want and without a fallback", r.ModeUsed, r.Fallback)
		}
	})
	t.Run("code counts when only code is searched", func(t *testing.T) {
		root := codeCorpus(t)
		more := map[string]string{}
		for i := 5; i < fts.MinAndHits; i++ {
			more[fmt.Sprintf("source/bacnet-rt/x/BDescriptor%d.java", i)] = "// " + codeQuery + "\n"
		}
		addFiles(t, root, more)
		r := search(t, root, codeQuery, "source", "devguide-rc")
		if r.Fallback || r.ModeUsed != "and" || len(r.Hits) < fts.MinAndHits {
			t.Errorf("code only: mode %s fallback %v hits %d, want and, no fallback, >= %d",
				r.ModeUsed, r.Fallback, len(r.Hits), fts.MinAndHits)
		}
		for _, h := range r.Hits {
			if h.Source != "source" && h.Source != "devguide-rc" {
				t.Errorf("code-only search returned %s", h.Key())
			}
		}
		// The same code hits do not keep AND mode once documentation is searched too.
		if mixed := search(t, root, codeQuery); !mixed.Fallback || mixed.ModeUsed != "or" {
			t.Errorf("every source: mode %s fallback %v, want an or fallback", mixed.ModeUsed, mixed.Fallback)
		}
	})
}

// TestRankingModuleExampleBelowDocPage: a module-jar template ranks like code,
// below a doc page with the same words (its markup repeats them in its title).
func TestRankingModuleExampleBelowDocPage(t *testing.T) {
	root := rankCorpus(t, map[string]string{
		"module-examples/web-rt/rc/passwordExpirationFormN4.vm": "TITLE: web-rt passwordExpirationFormN4.vm\n" +
			"MODULE: web-rt\nENTRY: rc/passwordExpirationFormN4.vm\n\n<form>password expiration form</form>\n",
		"guides-clean/Security/expiry.txt": page("User password configuration",
			"The password expiration form asks a user whose password expired for a new one."),
	})
	got := keys(search(t, root, "password expiration form"))
	if want := []string{"guides-clean/Security/expiry.txt", "module-examples/web-rt/rc/passwordExpirationFormN4.vm"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hits %q, want %q", got, want)
	}
}
