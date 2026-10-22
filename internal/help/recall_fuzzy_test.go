package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fuzzyRoot is a data root holding only indexes/guides-index.json with body (the other
// indexes fuzzy reads are optional and stay missing).
func fuzzyRoot(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "indexes", "guides-index.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestFuzzyGuideTopicCounts: a guide folder shows "(<n> topics, <jar>)" when its
// total_topics, else its html_count, is truthy, and nothing when neither is; its topics
// are listed as folder/topic.
func TestFuzzyGuideTopicCounts(t *testing.T) {
	dir := fuzzyRoot(t, `{
 "zeta": {"total_topics": 3, "source_jar": "docZeta.jar", "topics": ["zeta-intro"]},
 "zetb": {"html_count": 7},
 "zetc": {"total_topics": 0, "html_count": 5, "source_jar": "c.jar"},
 "zetd": {}}`)
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).fuzzy("zet", 15); err != nil {
		t.Fatal(err)
	}
	want := `FUZZY SEARCH: 'zet' (5 total matches)
============================================================

GUIDES (5 matches, showing top 5):

  1.0  zeta (3 topics, docZeta.jar)
  1.0  zetb (7 topics, )
  1.0  zetc
  1.0  zetd
  1.0  zeta/zeta-intro
`
	if got := out.String(); got != want {
		t.Errorf("fuzzy:\n%s\nwant:\n%s", got, want)
	}
}

// TestFuzzyIndexError: an index fuzzy reads that is not valid JSON is its error, and
// nothing is printed.
func TestFuzzyIndexError(t *testing.T) {
	dir := fuzzyRoot(t, `{"zeta": `)
	var out bytes.Buffer
	err := (&Env{BaseDir: dir, Out: &out}).fuzzy("zeta", 15)
	if err == nil || !strings.Contains(err.Error(), "guides-index.json") || out.Len() != 0 {
		t.Errorf("err %v, stdout %q; want the guides-index.json load error and no output", err, out.String())
	}
}
