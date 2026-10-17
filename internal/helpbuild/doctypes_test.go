package helpbuild

import (
	"path/filepath"
	"testing"
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
		if got := CleanTextName(rel, ""); got != want {
			t.Errorf("CleanTextName(%q) = %q, want %q", rel, got, want)
		}
	}
}

// test_text_extraction.TestTxtHtmlSiblings.test_both_pages_keep_their_own_clean_file
// (the clean_text_name half; the parser half belongs to slice S5).
func TestBothPagesKeepTheirOwnCleanFile(t *testing.T) {
	d := filepath.Join(t.TempDir(), "devguide")
	writeFile(t, filepath.Join(d, "changes.html"), []byte("<p>html page words</p>"))
	writeFile(t, filepath.Join(d, "changes.txt"), []byte("plain text page words\n"))
	for rel, want := range map[string]string{
		"changes.txt":  "changes.txt.txt",
		"changes.html": "changes.txt",
		"other.txt":    "other.txt",
	} {
		if got := CleanTextName(rel, d); got != want {
			t.Errorf("CleanTextName(%q) = %q, want %q", rel, got, want)
		}
	}
}

// test_round3_advisories.TestCleanTextNameSibling: an empty src_dir (None)
// skips the sibling check; an X.HTML sibling is seen case-insensitively.
func TestUpperCaseHTMLSiblingIsSeen(t *testing.T) {
	if got := CleanTextName("a.txt", ""); got != "a.txt" {
		t.Errorf("no src dir: %q", got)
	}
	guides := filepath.Join(t.TempDir(), "guides")
	writeFile(t, filepath.Join(guides, "F", "Page.HTML"), []byte("<p>html words</p>"))
	writeFile(t, filepath.Join(guides, "F", "Page.txt"), []byte("plain text words\n"))
	if got := CleanTextName("F/Page.txt", guides); got != "F/Page.txt.txt" {
		t.Errorf("upper-case sibling: %q", got)
	}
	if got := CleanTextName("G/Page.txt", guides); got != "G/Page.txt" {
		t.Errorf("missing folder: %q", got)
	}
}

func TestDocTypePredicates(t *testing.T) {
	checks := []struct {
		name string
		got  bool
		want bool
	}{
		{"guide .html", IsGuidePage("a/B.HTML"), true},
		{"guide .htm", IsGuidePage("a/b.htm"), true},
		{"guide .md", IsGuidePage("a/b.md"), true},
		{"guide .txt", IsGuidePage("a/notes.txt"), true},
		{"licence .txt is not a guide page", IsGuidePage("doc/lgpl-2.1.txt"), false},
		{"licence .html stays a guide page", IsGuidePage("doc/LICENSE.html"), true},
		{"guide .css", IsGuidePage("a/b.css"), false},
		{"licence prefix", IsLicence("doc/LICENSE.txt"), true},
		{"licence COPYING", IsLicence("Copying"), true},
		{"licence notice", IsLicence("x/NOTICE.md"), true},
		{"licence marker", IsLicence("x/foo-license.txt"), true},
		{"licence gpl-", IsLicence("x/gpl-3.0.txt"), true},
		{"licence lgpl", IsLicence("x/LGPL.txt"), true},
		{"not a licence", IsLicence("licensing/readme.txt"), false},
		{"İ is not I (full lower-case)", IsLicence("LİCENSE.txt"), false},
		{"source archive", IsSourceArchive("guides/x/libfoo-1.0.TAR.XZ"), true},
		{"source archive tgz", IsSourceArchive("guides/a.tgz"), true},
		{"source archive outside guides", IsSourceArchive("devguide/a.tar.gz"), false},
		{"not an archive", IsSourceArchive("guides/a.zip"), false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
