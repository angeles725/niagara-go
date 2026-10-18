package helpbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Ported from niagara-help tests: test_extraction.py
// TestRecursiveGuideExtraction, test_text_extraction.py TestTxtHtmlSiblings,
// test_round3_advisories.py test_upper_case_html_sibling_is_seen,
// test_t3a_advisories.py TestMarkdownTitle, test_n3_advisories.py
// TestMarkdownHashOnlyHeading and test_review_advisories.py
// TestTocFirstEntryWins; the method names are cited per case.

func guidePage(title, body string) []byte {
	return []byte("<html><head><title>" + title + "</title></head><body><p>" + body + "</p></body></html>")
}

// spyRoot is TestRecursiveGuideExtraction.setUp.
func spyRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	g := filepath.Join(root, "guides", "Spy")
	writeFile(t, filepath.Join(g, "top.html"), guidePage("Top Page", "top body"))
	writeFile(t, filepath.Join(g, "A", "Compile.html"), guidePage("Compile A", "alpha body"))
	writeFile(t, filepath.Join(g, "B", "Compile.html"), guidePage("Compile B", "beta body"))
	writeFile(t, filepath.Join(g, "B", "Deep", "legacy.htm"), guidePage("Legacy Htm", "gamma body"))
	return root
}

func mustParseGuides(t *testing.T, root string) *TextResult {
	t.Helper()
	res, err := ParseGuides(root)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func cleanText(t *testing.T, root string, parts ...string) string {
	t.Helper()
	return readText(t, filepath.Join(append([]string{root, "guides-clean", "Spy"}, parts...)...))
}

func TestParseGuidesNestedPages(t *testing.T) {
	root := spyRoot(t)
	// test_nested_and_same_basename_pages_are_extracted
	if res := mustParseGuides(t, root); res.Files != 4 {
		t.Fatalf("Files = %d, want 4", res.Files)
	}
	if a := cleanText(t, root, "A", "Compile.txt"); !strings.Contains(a, "alpha body") {
		t.Errorf("A/Compile.txt = %q", a)
	}
	if b := cleanText(t, root, "B", "Compile.txt"); !strings.Contains(b, "beta body") {
		t.Errorf("B/Compile.txt = %q", b)
	}
	// test_source_header_records_full_relative_path
	head := strings.Split(cleanText(t, root, "B", "Deep", "legacy.htm.txt"), "\n")[:2]
	if want := []string{"TITLE: Legacy Htm", "SOURCE: guides/Spy/B/Deep/legacy.htm"}; !reflect.DeepEqual(head, want) {
		t.Errorf("header = %q", head)
	}
}

// test_htm_and_html_with_same_stem_do_not_collide
func TestParseGuidesHtmTwin(t *testing.T) {
	root := spyRoot(t)
	writeFile(t, filepath.Join(root, "guides", "Spy", "top.htm"), guidePage("Top Htm", "htm twin body"))
	if res := mustParseGuides(t, root); res.Files != 5 {
		t.Fatalf("Files = %d, want 5", res.Files)
	}
	if s := cleanText(t, root, "top.htm.txt"); !strings.Contains(s, "SOURCE: guides/Spy/top.htm\n") {
		t.Errorf("top.htm.txt = %q", s)
	}
	if s := cleanText(t, root, "top.txt"); !strings.Contains(s, "SOURCE: guides/Spy/top.html\n") {
		t.Errorf("top.txt = %q", s)
	}
}

// test_page_without_text_still_gets_a_header_only_entry and
// test_rebuild_drops_stale_outputs
func TestParseGuidesHeaderOnlyAndStale(t *testing.T) {
	root := spyRoot(t)
	writeFile(t, filepath.Join(root, "guides", "Spy", "stub.html"),
		[]byte("<html><head><title></title></head><body></body></html>"))
	writeFile(t, filepath.Join(root, "guides-clean", "Spy", "gone.txt"), []byte("stale"))
	res := mustParseGuides(t, root)
	if got := cleanText(t, root, "stub.txt"); got != "TITLE: \nSOURCE: guides/Spy/stub.html\n\n" {
		t.Errorf("stub.txt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "guides-clean", "Spy", "gone.txt")); !os.IsNotExist(err) {
		t.Errorf("stale gone.txt kept: %v", err)
	}
	if want := []string{"Extracted 5 guide files to guides-clean/ (0.0 MB text, 0 errors, 0 titled from toc.xml)"}; !reflect.DeepEqual(res.Log, want) {
		t.Errorf("Log = %q", res.Log)
	}
}

// test_upper_case_html_sibling_is_seen
func TestParseGuidesUpperCaseSibling(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "guides", "F", "Page.HTML"), []byte("<html><title>Page</title><p>html words</p></html>"))
	writeFile(t, filepath.Join(root, "guides", "F", "Page.txt"), []byte("plain text words\n"))
	mustParseGuides(t, root)
	clean := filepath.Join(root, "guides-clean", "F")
	if s := readText(t, filepath.Join(clean, "Page.txt")); !strings.Contains(s, "html words") {
		t.Errorf("Page.txt = %q", s)
	}
	if s := readText(t, filepath.Join(clean, "Page.txt.txt")); !strings.Contains(s, "plain text words") {
		t.Errorf("Page.txt.txt = %q", s)
	}
}

func TestParseGuidesTocHeader(t *testing.T) {
	root := t.TempDir()
	g := filepath.Join(root, "guides", "W")
	writeFile(t, filepath.Join(g, "toc.xml"), []byte(`<toc><tocitem target="index.html" text="Web  Charts">`+
		`<tocitem target="sub\Over.html#x" text="Overview"/></tocitem></toc>`))
	writeFile(t, filepath.Join(g, "index.html"), guidePage("Web Charts", "i"))
	writeFile(t, filepath.Join(g, "sub", "Over.html"), guidePage("Other title", "o"))
	writeFile(t, filepath.Join(root, "guides", "Bad", "TOC.xml"), []byte("<toc><tocitem></toc>"))
	writeFile(t, filepath.Join(root, "guides", "Bad", "a.md"), []byte("# A title ##\nbody"))
	res := mustParseGuides(t, root)
	want := "TITLE: Overview\nTOC: Web Charts > Overview\nPAGE TITLE: Other title\nSOURCE: guides/W/sub/Over.html\n\no\n"
	if got := readText(t, filepath.Join(root, "guides-clean", "W", "sub", "Over.txt")); got != want {
		t.Errorf("Over.txt = %q", got)
	}
	if got := readText(t, filepath.Join(root, "guides-clean", "Bad", "a.md.txt")); got != "TITLE: A title\nSOURCE: guides/Bad/a.md\n\n# A title ##\nbody\n" {
		t.Errorf("a.md.txt = %q", got)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "toc not read") {
		t.Errorf("Warnings = %q", res.Warnings)
	}
	if !strings.HasSuffix(res.Log[0], "2 titled from toc.xml)") {
		t.Errorf("Log = %q", res.Log)
	}
}

// test_both_pages_keep_their_own_clean_file
func TestParseDevguideSiblings(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "devguide")
	writeFile(t, filepath.Join(d, "changes.html"), []byte("<html><title>Changes</title><p>html page words</p>"))
	writeFile(t, filepath.Join(d, "changes.txt"), []byte("plain text page words\n"))
	writeFile(t, filepath.Join(d, "sub", "lgpl-2.1.txt"), []byte("GNU"))
	res, err := ParseDevguide(root)
	if err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(root, "devguide-clean")
	if s := readText(t, filepath.Join(clean, "changes.txt")); !strings.Contains(s, "html page words") {
		t.Errorf("changes.txt = %q", s)
	}
	if s := readText(t, filepath.Join(clean, "changes.txt.txt")); s != "TITLE: changes.txt\nSOURCE: devguide/changes.txt\n\nplain text page words\n" {
		t.Errorf("changes.txt.txt = %q", s)
	}
	want := []string{"  excluded (licence text): devguide/sub/lgpl-2.1.txt", "Extracted 2 devguide files to devguide-clean/ (0.0 MB text)"}
	if !reflect.DeepEqual(res.Log, want) {
		t.Errorf("Log = %q", res.Log)
	}
}

func TestParseGuidesRefusesNonUTF8Names(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "guides", "F", "a\xff.html"), guidePage("A", "a"))
	writeFile(t, filepath.Join(root, "guides-clean", "kept.txt"), []byte("old"))
	if _, err := ParseGuides(root); err == nil {
		t.Fatal("ParseGuides accepted a page name that is not UTF-8")
	}
	if s := readText(t, filepath.Join(root, "guides-clean", "kept.txt")); s != "old" {
		t.Errorf("old clean tree not kept: %q", s)
	}
}

func TestMarkdownTitle(t *testing.T) {
	for text, want := range map[string]string{
		// test_closing_sequence_only
		"# C#\n": "C#", "# Using F# ##\n": "Using F#", "# Title #\n": "Title", "# Plain\n": "Plain", "# A # B\n": "A # B",
		// test_hash_only_heading_is_empty
		"# #\n": "", "# ##\n": "", "#   ###   \n": "", "# ##\n# Real title\n": "Real title", "# Title ##\n": "Title",
		"no heading": "", "#NoSpace\n": "", "#  Title　# \n": "Title",
	} {
		if got := markdownTitle(text); got != want {
			t.Errorf("markdownTitle(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestParseToc(t *testing.T) {
	dir := t.TempDir()
	// test_backslash_duplicate_keeps_the_first_title
	p := filepath.Join(dir, "toc.xml")
	writeFile(t, p, []byte(`<toc><tocitem target="a\b.html" text="First"/><tocitem target="a/b.html" text="Second"/></toc>`))
	toc, warning := ParseToc(p)
	if warning != "" || toc.Pages["a/b.html"].Title != "First" || CountItems(p) != 2 {
		t.Errorf("toc = %+v, warning %q, items %d", toc, warning, CountItems(p))
	}
	// test_broken_toc_warns
	writeFile(t, p, []byte("<toc><tocitem></toc>"))
	toc, warning = ParseToc(p)
	if len(toc.Pages) != 0 || !strings.Contains(warning, "toc not read") || CountItems(p) != 0 {
		t.Errorf("broken toc: %+v, %q", toc, warning)
	}
	want := "WARNING: toc not read, page titles fall back to <title>: " + p + ": mismatched tag: line 1, column 16"
	if warning != want {
		t.Errorf("warning = %q, want %q", warning, want)
	}
	// Trails, anchors and a BOM before an ISO-8859-1 declaration.
	writeFile(t, p, []byte("\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><toc>"+
		"<tocitem target=\"i.html\" text=\" G  \xe9 \"><tocitem text=\"\"><tocitem target=\"s.html#a\" text=\"S\"/></tocitem></tocitem></toc>"))
	toc, warning = ParseToc(p)
	if warning != "" || !reflect.DeepEqual(toc.Order, []string{"i.html", "s.html"}) ||
		!reflect.DeepEqual(toc.Pages["s.html"], TocEntry{Title: "S", Trail: []string{"G é"}}) {
		t.Errorf("toc = %+v, warning %q", toc, warning)
	}
	if FindToc(filepath.Join(dir, "missing")) != "" || FindToc(dir) != p {
		t.Errorf("FindToc(%s) = %q", dir, FindToc(dir))
	}
}

// A page that cannot be opened (here a dangling symlink, which os.walk
// lists as a file) is counted in the errors of the summary line and
// skipped; the other pages are still extracted.
func TestParseGuidesCountsUnreadablePages(t *testing.T) {
	root := spyRoot(t)
	g := filepath.Join(root, "guides", "Spy")
	for _, name := range []string{"gone.html", "A/gone.md"} {
		if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(g, filepath.FromSlash(name))); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	res := mustParseGuides(t, root)
	if res.Files != 4 {
		t.Errorf("Files = %d, want 4", res.Files)
	}
	if want := "(0.0 MB text, 2 errors, 0 titled from toc.xml)"; len(res.Log) == 0 || !strings.HasSuffix(res.Log[len(res.Log)-1], want) {
		t.Errorf("Log = %q, want a line ending %q", res.Log, want)
	}
}
