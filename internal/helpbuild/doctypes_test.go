package helpbuild

import (
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// test_text_extraction.TestCleanNames.test_text_pages_keep_their_name.
func TestTextPagesKeepTheirName(t *testing.T) {
	for rel, want := range map[string]string{
		"changes37.txt": "changes37.txt",
		"a/Page.html":   "a/Page.txt",
		"a/Page.htm":    "a/Page.htm.txt",
		"a/Page.md":     "a/Page.md.txt",
		"a/Page.HTML":   "a/Page.txt",
	} {
		if got := cleanTextName(rel, ""); got != want {
			t.Errorf("CleanTextName(%q) = %q, want %q", rel, got, want)
		}
	}
}

// test_text_extraction.TestTxtHtmlSiblings.test_both_pages_keep_their_own_clean_file
// (the clean_text_name half; the parser half belongs to slice S5).
func TestBothPagesKeepTheirOwnCleanFile(t *testing.T) {
	d := filepath.Join(t.TempDir(), "devguide")
	testkit.WriteFile(t, filepath.Join(d, "changes.html"), []byte("<p>html page words</p>"))
	testkit.WriteFile(t, filepath.Join(d, "changes.txt"), []byte("plain text page words\n"))
	for rel, want := range map[string]string{
		"changes.txt":  "changes.txt.txt",
		"changes.html": "changes.txt",
		"other.txt":    "other.txt",
	} {
		if got := cleanTextName(rel, d); got != want {
			t.Errorf("CleanTextName(%q) = %q, want %q", rel, got, want)
		}
	}
}

// test_round3_advisories.TestCleanTextNameSibling: an empty src_dir (None)
// skips the sibling check; an X.HTML sibling is seen case-insensitively.
func TestUpperCaseHTMLSiblingIsSeen(t *testing.T) {
	if got := cleanTextName("a.txt", ""); got != "a.txt" {
		t.Errorf("no src dir: %q", got)
	}
	guides := filepath.Join(t.TempDir(), "guides")
	testkit.WriteFile(t, filepath.Join(guides, "F", "Page.HTML"), []byte("<p>html words</p>"))
	testkit.WriteFile(t, filepath.Join(guides, "F", "Page.txt"), []byte("plain text words\n"))
	if got := cleanTextName("F/Page.txt", guides); got != "F/Page.txt.txt" {
		t.Errorf("upper-case sibling: %q", got)
	}
	if got := cleanTextName("G/Page.txt", guides); got != "G/Page.txt" {
		t.Errorf("missing folder: %q", got)
	}
}

func TestDocTypePredicates(t *testing.T) {
	checks := []struct {
		name string
		got  bool
		want bool
	}{
		{"guide .html", isGuidePage("a/B.HTML"), true},
		{"guide .htm", isGuidePage("a/b.htm"), true},
		{"guide .md", isGuidePage("a/b.md"), true},
		{"guide .txt", isGuidePage("a/notes.txt"), true},
		{"licence .txt is not a guide page", isGuidePage("doc/lgpl-2.1.txt"), false},
		{"licence .html stays a guide page", isGuidePage("doc/LICENSE.html"), true},
		{"guide .css", isGuidePage("a/b.css"), false},
		{"licence prefix", isLicence("doc/LICENSE.txt"), true},
		{"licence COPYING", isLicence("Copying"), true},
		{"licence notice", isLicence("x/NOTICE.md"), true},
		{"licence marker", isLicence("x/foo-license.txt"), true},
		{"licence gpl-", isLicence("x/gpl-3.0.txt"), true},
		{"licence lgpl", isLicence("x/LGPL.txt"), true},
		{"not a licence", isLicence("licensing/readme.txt"), false},
		{"İ is not I (full lower-case)", isLicence("LİCENSE.txt"), false},
		{"source archive", isSourceArchive("guides/x/libfoo-1.0.TAR.XZ"), true},
		{"source archive tgz", isSourceArchive("guides/a.tgz"), true},
		{"source archive outside guides", isSourceArchive("devguide/a.tar.gz"), false},
		{"not an archive", isSourceArchive("guides/a.zip"), false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
