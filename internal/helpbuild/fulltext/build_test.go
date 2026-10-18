package fulltext

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// syntheticTree is a data root that exercises every rule of the Python
// builder: header forms, titles per source, form-feed pages, os.walk order,
// universal newlines, invalid UTF-8, a BOM, Unicode whitespace, accents,
// camelCase and the devguide-examples skips. It lives in code, not in
// testdata/, so git never normalises its bytes.
var syntheticTree = map[string]string{
	"bajadoc-clean/javax/baja/sys/BObject.txt":  "CLASS: BObject\nPACKAGE: javax.baja.sys\n\nThe BObject is the root.\nalarmAck configuring events HTTPServer\n",
	"bajadoc-clean/javax/baja/sys/BAbsTime.txt": "CLASS: BAbsTime\n\nTime values 1 x 22 3rd.\n",
	"bajadoc-clean/Zeta.txt":                    "TITLE: Zeta  \nTITLE: second\nCLASS: Ignored\nbody right after the header\n",
	"bajadoc-clean/Upper.txt":                   "TITLE:Upper\n\nupper body\n",
	"bajadoc-clean/_under.txt":                  "under body\n",
	"bajadoc-clean/a.txt":                       "TITLE: \n\nempty title value\n",
	"bajadoc-clean/notes.md":                    "not indexed\n",
	"devguide-clean/intro.txt":                  "TITLE: Introducción a la configuración\n\nConfiguración 72 °F naïve café ﬁle ²\n",
	"devguide-clean/crlf.txt":                   "TITLE: CRLF page\r\n\r\nline one\r\nline two\rline three\r\n",
	"devguide-clean/bad.txt":                    "TITLE: Bad \xe2\x82 bytes \xff\xfe end\n\nbody \xc3\x28 text \xf0\x9f\x98\n",
	"devguide-clean/bom.txt":                    "\xef\xbb\xbfTITLE: Bom\n\nbody\n",
	"devguide-clean/spaces.txt":                 "TITLE: Spaced \x1f\n　\nbody after an ideographic-space line\n",
	"devguide-clean/empty.txt":                  "",
	"devguide-clean/onlyheader.txt":             "TITLE: Only",
	"guides-clean/alarm/alarm-intro.txt":        "TITLE: Alarm intro\n\nThe AlarmService routes alarms.\n",
	"guides-clean/noheader.txt":                 "Plain text without header\nsecond line\n",
	"guides-clean/lower.txt":                    "title: lower key\n\nbody\n",
	"guides-clean/Key Space.txt":                "MY KEY: value\n \nbody\nTITLE: not a header\n",
	"guides-clean/.hidden.txt":                  "TITLE: Hidden\n\ndot file\n",
	"docs-text/Modbus_Guide.txt":                "TITLE: Modbus Guide\n\npage one text\n\fpage two modbus\nmore\n\f\n  \n\fpage four\n\f\x1c\x1f\n\fpage six",
	"docs-text/CR_Pages.txt":                    "one\rtwo\r\fthree\r\nfour\r\f\rfive",
	"docs-text/NoFF_Manual.txt":                 "no form feed here\n",
	"course-text/day1/lab_1.txt":                "slide one\n\fslide two\n\f",
	"course-text/readme.txt":                    "TITLE: Course readme\n\nno pages\n",
	"source/javax/baja/Foo.java":                "package javax.baja;\npublic class FooBar { int getValue(); }\n",
	"devguide-rc/Example.java":                  "CLASS: Example\n\nclass Example {}\n",
	"devguide-rc/notes.txt":                     "TITLE: RC notes\n\nnotes\n",
	"devguide-examples/web/app.js":              "function initApp() { return lexOnly; }\n",
	"devguide-examples/web/app.min.js":          "minified\n",
	"devguide-examples/ext/lib.js":              "third party\n",
	"devguide-examples/sub/ext/inner.js":        "not the top-level ext\n",
	"devguide-examples/view.hbs":                "{{title}}\n",
	"devguide-examples/page.html":               "<p>Page</p>\n",
	"devguide-examples/Thing.px":                "<px/>\n",
	"devguide-examples/data.json":               "{}\n",
}

// writeTree materialises syntheticTree under a temp dir with fixed mtimes
// (the fingerprint hashes them) and returns its path.
func writeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	base := time.Unix(1700000000, 123456789)
	for rel, text := range syntheticTree {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		// One mtime per path (map order is random; the mtime must not be).
		mtime := base.Add(time.Duration(len(rel)) * time.Second)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBuildDefaultPathAndFailure(t *testing.T) {
	root := writeTree(t)
	res, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "indexes", fts.DBName); res.Path != want {
		t.Errorf("Path = %q, want %q", res.Path, want)
	}
	wantLog := fmt.Sprintf("full-text index: %d documents -> %s", res.Documents, res.Path)
	if len(res.Log) != 1 || res.Log[0] != wantLog || res.Documents == 0 {
		t.Errorf("Log = %q (%d documents), want [%q]", res.Log, res.Documents, wantLog)
	}
	// The reader accepts the database as current and finds paged text in it.
	found, err := fts.Search(root, "", "modbus", []string{"docs-text"}, "and")
	if err != nil {
		t.Fatalf("reader rejects the Go-built index: %v", err)
	}
	var pages []int
	for _, h := range found.Hits {
		pages = append(pages, h.Page)
	}
	if len(found.Hits) != 1 || found.Hits[0].Path != "Modbus_Guide.txt" {
		t.Errorf("search modbus: %d hits (pages %v), want Modbus_Guide.txt", len(found.Hits), pages)
	}
	// No temp file is left beside it.
	entries, _ := os.ReadDir(filepath.Join(root, "indexes"))
	if len(entries) != 1 {
		t.Errorf("indexes/ holds %d entries, want only %s", len(entries), fts.DBName)
	}

	// A target whose directory cannot be created fails without output.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root, Options{Output: filepath.Join(blocker, "sub", fts.DBName)}); err == nil {
		t.Error("Build under a regular file succeeded")
	}
}

func TestBuildFailureKeepsThePreviousIndex(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := writeTree(t)
	res, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(res.Path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755) //nolint:errcheck // lets TempDir clean up
	if _, err := Build(root, Options{}); err == nil {
		t.Fatal("Build into a read-only indexes/ succeeded")
	}
	after, err := os.ReadFile(res.Path)
	if err != nil || string(after) != string(before) {
		t.Errorf("the previous index changed (%v)", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("indexes/ holds %d entries after a failed build, want 1", len(entries))
	}
}
