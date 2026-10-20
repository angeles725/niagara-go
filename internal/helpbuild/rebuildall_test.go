package helpbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// rebuildRun runs RebuildAll on root and returns its stdout and stderr.
func rebuildRun(t *testing.T, root string, opt RebuildOptions) (res *RebuildResult, stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	opt.Stdout, opt.Stderr = &out, &errb
	res, err = RebuildAll(root, opt)
	return res, out.String(), errb.String(), err
}

// sourceOnlyRoot is a data root with one source/ class and nothing else.
func sourceOnlyRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	testkit.WriteFile(t, filepath.Join(root, "source", "m-rt", "a", "A.java"), []byte("package a;\npublic class A {}\n"))
	return root
}

// The dry run prints the plan in builder order: a builder whose source
// folder is absent is skipped, the two that find their own input are
// planned, and nothing is written.
func TestRebuildAllDryRunPlan(t *testing.T) {
	root := sourceOnlyRoot(t)
	res, out, errs, err := rebuildRun(t, root, RebuildOptions{DryRun: true})
	if err != nil || errs != "" {
		t.Fatalf("%v %q", err, errs)
	}
	want := `REBUILD-ALL (dry run)
============================================================
  SKIP  bajadoc_parser.py          -> bajadoc-clean/  (source 'bajadoc/' absent)
  SKIP  bajadoc_xml_parser.py      -> bajadoc-clean/  (source 'devguide/' absent)
  SKIP  build_class_index.py       -> class-index.json, jdk-class-index.json (when jdk/ exists)  (source 'devguide/' absent)
  SKIP  inheritance_builder.py     -> inheritance.json  (source 'bajadoc-clean/' absent)
  SKIP  method_indexer.py          -> method-index.json  (source 'bajadoc-clean/' absent)
  PLAN  source_indexer.py          -> source-index.json
  PLAN  slots_indexer.py           -> slots-index.json
  PLAN  xref_builder.py            -> xref-index.json
  SKIP  guides_parser.py           -> guides-clean/  (source 'guides/' absent)
  SKIP  build_guides_index.py      -> guides-index.json  (source 'guides/' absent)
  SKIP  devguide_parser.py         -> devguide-clean/  (source 'devguide/' absent)
  SKIP  build_devguide_toc.py      -> devguide-toc.json  (source 'devguide/' absent)
  PLAN  pdf_extractor.py           -> pdf-index.json, docs-text/
  PLAN  course_extractor.py        -> course-text/

Dry run — nothing executed. Drop --dry-run to build.
`
	if out != want {
		t.Fatalf("stdout\n%s\nwant\n%s", out, want)
	}
	if res.Ran != 0 || res.Skipped != 9 || res.Failed != 0 || res.Stamped != "" {
		t.Errorf("result %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "indexes")); !os.IsNotExist(err) {
		t.Errorf("a dry run wrote indexes/: %v", err)
	}
}

// --only keeps the builders whose script name starts with it; one that
// matches nothing runs nothing and still stamps the manifest, as Python.
func TestRebuildAllOnly(t *testing.T) {
	root := sourceOnlyRoot(t)
	_, out, _, err := rebuildRun(t, root, RebuildOptions{DryRun: true, Only: "build_"})
	if err != nil {
		t.Fatal(err)
	}
	var scripts []string
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) > 1 && (f[0] == "PLAN" || f[0] == "SKIP") {
			scripts = append(scripts, f[1])
		}
	}
	if got := strings.Join(scripts, " "); got != "build_class_index.py build_guides_index.py build_devguide_toc.py" {
		t.Errorf("--only build_: %s", got)
	}
	res, out, _, err := rebuildRun(t, root, RebuildOptions{Only: "zzz", Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	want := "REBUILD-ALL \n" + strings.Repeat("=", 60) + "\n\nRan 0, skipped 0, failed 0.\n" +
		"Manifest re-stamped. Run 'freshness' to confirm FRESH.\n"
	if out != want || res.Stamped != "2026-10-17T15:08:07.000654+00:00" {
		t.Fatalf("stdout %q, stamped %q", out, res.Stamped)
	}
}

// --stamp-only writes indexes/.manifest.json (json.dumps indent=2, sources
// in SOURCE_DIRS order, null for an absent folder) and runs nothing.
func TestRebuildAllStampOnly(t *testing.T) {
	root := sourceOnlyRoot(t)
	testkit.WriteFile(t, filepath.Join(root, "source", "B.java"), []byte("0123456789"))
	testkit.WriteFile(t, filepath.Join(root, "docs", "report.md"), []byte("not a manual"))
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	res, out, _, err := rebuildRun(t, root, RebuildOptions{StampOnly: true, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Manifest stamped at 2026-01-02T03:04:05+00:00 (baseline = current corpus).\n" || res.Stamped == "" {
		t.Fatalf("stdout %q", out)
	}
	want := `{
  "stamped_utc": "2026-01-02T03:04:05+00:00",
  "sources": {
    "bajadoc": null,
    "source": {
      "files": 2,
      "bytes": 39
    },
    "guides": null,
    "devguide": null,
    "docs": null
  }
}`
	if got := testkit.ReadText(t, filepath.Join(root, "indexes", ".manifest.json")); got != want {
		t.Fatalf("manifest\n%s\nwant\n%s", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, "indexes", "source-index.json")); !os.IsNotExist(err) {
		t.Errorf("--stamp-only ran a builder: %v", err)
	}
}

// stamped_utc is datetime.isoformat(): microseconds only when not zero.
func TestPyISOFormatUTC(t *testing.T) {
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 10, 18, 1, 2, 3, 0, time.UTC), "2026-10-18T01:02:03+00:00"},
		{time.Date(2026, 10, 18, 1, 2, 3, 999, time.UTC), "2026-10-18T01:02:03+00:00"},
		{time.Date(2026, 10, 18, 1, 2, 3, 1000, time.UTC), "2026-10-18T01:02:03.000001+00:00"},
		{time.Date(2026, 10, 17, 20, 2, 3, 120000000, time.FixedZone("CST", -6*3600)), "2026-10-18T02:02:03.120000+00:00"},
	} {
		if got := pyISOFormatUTC(c.t); got != c.want {
			t.Errorf("%v: %s, want %s", c.t, got, c.want)
		}
	}
}

// _fingerprint_dir: every file of the tree (docs/ only *.pdf, any case),
// links to files counted at the target's size, dangling links and the
// files under a linked folder not counted; docs/ without a PDF is null.
func TestCorpusFingerprint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	root := t.TempDir()
	testkit.WriteFile(t, filepath.Join(root, "guides", "a.html"), []byte("12345"))
	testkit.WriteFile(t, filepath.Join(root, "guides", "sub", "b.txt"), []byte("123"))
	testkit.WriteFile(t, filepath.Join(root, "elsewhere", "big.bin"), make([]byte, 100))
	mustSymlink(t, filepath.Join(root, "elsewhere", "big.bin"), filepath.Join(root, "guides", "link.bin"))
	mustSymlink(t, filepath.Join(root, "elsewhere"), filepath.Join(root, "guides", "linkdir"))
	mustSymlink(t, filepath.Join(root, "nothing"), filepath.Join(root, "guides", "dangling"))
	testkit.WriteFile(t, filepath.Join(root, "docs", "Manual.PDF"), []byte("1234567"))
	testkit.WriteFile(t, filepath.Join(root, "docs", "notes.md"), []byte("x"))
	testkit.WriteFile(t, filepath.Join(root, "devguide", "notes.md"), nil)
	fp := CorpusFingerprint(root)
	got := map[string]string{}
	for _, d := range SourceDirs {
		v, _ := fp.Get(d)
		got[d] = testkit.DumpLine(v)
	}
	want := map[string]string{
		"bajadoc": "[null]", "source": "[null]",
		"guides":   `[{"files": 3, "bytes": 108}]`,
		"devguide": `[{"files": 1, "bytes": 0}]`,
		"docs":     `[{"files": 1, "bytes": 7}]`,
	}
	for d, w := range want {
		if got[d] != w {
			t.Errorf("%s: %s, want %s", d, got[d], w)
		}
	}
	if err := os.Remove(filepath.Join(root, "docs", "Manual.PDF")); err != nil {
		t.Fatal(err)
	}
	if v, _ := CorpusFingerprint(root).Get("docs"); testkit.DumpLine(v) != "[null]" {
		t.Errorf("docs without a PDF: %s", testkit.DumpLine(v))
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// A full run prints RUN before each builder and the builder's own lines
// after it, writes the indexes, counts and stamps.
func TestRebuildAllRunsAndStamps(t *testing.T) {
	root := sourceOnlyRoot(t)
	res, out, errs, err := rebuildRun(t, root, RebuildOptions{Now: fixedNow})
	if err != nil || errs != "" {
		t.Fatalf("%v %q", err, errs)
	}
	for _, want := range []string{
		"REBUILD-ALL \n",
		"\n  RUN   source_indexer.py          -> source-index.json\n",
		"\n  RUN   pdf_extractor.py           -> pdf-index.json, docs-text/\nSKIP: no PDF manuals for ",
		"\n  RUN   course_extractor.py        -> course-text/\ncourse: no course bound to this version; skipped\n",
		"\n\nRan 5, skipped 9, failed 0.\nManifest re-stamped. Run 'freshness' to confirm FRESH.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if res.Ran != 5 || res.Skipped != 9 || res.Failed != 0 || res.Stamped != "2026-10-17T15:08:07.000654+00:00" {
		t.Errorf("result %+v", res)
	}
	for _, name := range []string{"source-index.json", "slots-index.json", "xref-index.json", ".manifest.json"} {
		if !pycompat.IsFile(filepath.Join(root, "indexes", name)) {
			t.Errorf("indexes/%s not written", name)
		}
	}
	if pycompat.IsFile(filepath.Join(root, "indexes", "call-index.json")) {
		t.Error("call-index.json is retired (I3-5) but was written")
	}
}

// A builder that exits non-zero is a FAIL line and leaves the manifest of
// the last good build; a builder that raises (a Go error) prints what it
// printed before, an ERROR line on stderr, exits 1, and the run goes on.
func TestRebuildAllFailingBuilders(t *testing.T) {
	root := sourceOnlyRoot(t)
	testkit.WriteFile(t, filepath.Join(root, "bajadoc", "x.html"), nil) // no bajadoc/javax/baja: exit 1
	testkit.WriteFile(t, filepath.Join(root, "source", "m-rt", "b", "B.java"), []byte("package b;\nimport c.;\npublic class B {}\n"))
	if _, _, _, err := rebuildRun(t, root, RebuildOptions{StampOnly: true, Now: fixedNow}); err != nil {
		t.Fatal(err)
	}
	before := testkit.ReadText(t, filepath.Join(root, "indexes", ".manifest.json"))
	res, out, errs, err := rebuildRun(t, root, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"  RUN   bajadoc_parser.py          -> bajadoc-clean/\nERROR: ",
		"\n  FAIL  bajadoc_parser.py (exit 1)\n",
		"\n  RUN   xref_builder.py            -> xref-index.json\n  FAIL  xref_builder.py (exit 1)\n",
		"\n  RUN   course_extractor.py        -> course-text/\n",
		"\nRan 4, skipped 8, failed 2.\nBuilders failed — manifest NOT updated (still reflects last good build).\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if !strings.HasPrefix(errs, "ERROR: xref_builder.py raised: ") || strings.Count(errs, "\n") != 1 {
		t.Errorf("stderr %q", errs)
	}
	if res.Failed != 2 || res.Stamped != "" {
		t.Errorf("result %+v", res)
	}
	if got := testkit.ReadText(t, filepath.Join(root, "indexes", ".manifest.json")); got != before {
		t.Errorf("a failed build re-stamped the manifest")
	}
}

// A builder that panics is a crash of that builder only (Python's child
// process dying), not of the whole run.
func TestRebuildAllRecoversAPanic(t *testing.T) {
	root := sourceOnlyRoot(t)
	saved := rebuildBuilders
	t.Cleanup(func() { rebuildBuilders = saved })
	rebuildBuilders = append([]Builder(nil), saved...)
	for i, b := range rebuildBuilders {
		if b.Script == "slots_indexer.py" {
			rebuildBuilders[i].run = func(string, *rebuildIO, *RebuildOptions) (int, error) { panic("boom") }
		}
	}
	res, out, errs, err := rebuildRun(t, root, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "  FAIL  slots_indexer.py (exit 1)\n") || !strings.Contains(errs, "ERROR: slots_indexer.py raised: panic: boom") ||
		res.Failed != 1 || res.Ran != 4 {
		t.Fatalf("%+v\n%s\n%s", res, out, errs)
	}
}

type failWriter struct{ n int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		return 0, errors.New("broken pipe")
	}
	w.n--
	return len(p), nil
}

// A stdout that cannot be written stops the run with that error (Python's
// print raises).
func TestRebuildAllWriteError(t *testing.T) {
	root := sourceOnlyRoot(t)
	_, err := RebuildAll(root, RebuildOptions{DryRun: true, Stdout: &failWriter{n: 2}})
	if err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Fatalf("err %v", err)
	}
}

// RebuildToLog is _rebuild's middle: rebuild-all with its stdout in
// root/build.log (stderr to the given writer), returning the SKIP, FAIL and
// Ran lines; through ExtractAndIndex's hook the EMPTY lines follow.
func TestRebuildToLog(t *testing.T) {
	root := sourceOnlyRoot(t)
	var errb bytes.Buffer
	tail, err := rebuild(root, RebuildToLog(&errb))
	if err != nil {
		t.Fatal(err)
	}
	log := testkit.ReadText(t, filepath.Join(root, "build.log"))
	if !strings.HasPrefix(log, "REBUILD-ALL \n") || !strings.HasSuffix(log, "Run 'freshness' to confirm FRESH.\n") {
		t.Fatalf("build.log\n%s", log)
	}
	var want []string
	for _, l := range strings.Split(log, "\n") {
		if strings.HasPrefix(l, "  SKIP") || strings.HasPrefix(l, "  FAIL") || strings.HasPrefix(l, "Ran ") {
			want = append(want, strings.TrimRight(l, " "))
		}
	}
	want = append(want, "  EMPTY class-index.json (its source is not in this install)",
		"  EMPTY inheritance.json (its source is not in this install)",
		"  EMPTY method-index.json (its source is not in this install)",
		"  EMPTY guides-index.json (its source is not in this install)",
		"  EMPTY devguide-toc.json (its source is not in this install)")
	if strings.Join(tail, "\n") != strings.Join(want, "\n") || len(tail) != 15 {
		t.Fatalf("tail\n%s\nwant\n%s", strings.Join(tail, "\n"), strings.Join(want, "\n"))
	}
	if got := testkit.ReadText(t, filepath.Join(root, "indexes", "pdf-index.json")); got != "{}\n" {
		t.Errorf("pdf-index.json stub %q", got)
	}
}

// The tail is read back as Python reads build.log: invalid UTF-8
// replaced, universal newlines, trailing whitespace stripped.
func TestBuildLogTail(t *testing.T) {
	got := buildLogTail([]byte("x\r  SKIP  a \xff \r\nRan 1, skipped 1, failed 0.\t\n  FAIL\r\n  RUN   b\n"))
	want := "  SKIP  a \ufffd|Ran 1, skipped 1, failed 0.|  FAIL"
	if strings.Join(got, "|") != want {
		t.Fatalf("%q", got)
	}
}
