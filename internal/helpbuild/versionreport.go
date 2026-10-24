package helpbuild

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The gates and the summary `versions add` prints (version_report.py):
// runGates runs `coverage --doc-jars` and `coverage --sweep` on the new
// version, both full reports in <data root>/gates.log; printSummary lists,
// per full-text source kind, how many files are searchable and the
// fallbacks the version uses.

// gatesLog is GATES_LOG, the file of both gate reports in the data root.
const gatesLog = "gates.log"

// gateRun is one gate: its exit status and the text of its own last
// "RESULT: " line ("(no RESULT line)" when it printed none).
type gateRun struct {
	Status int
	Result string
}

// Gates is run_gates' {"doc-jars": ..., "sweep": ...}.
type Gates struct{ DocJars, Sweep gateRun }

// summaryKinds is KINDS: full-text source label and what it holds.
var summaryKinds = [][2]string{
	{"bajadoc-clean", "API class pages (javadoc HTML + .bajadoc XML)"},
	{"devguide-clean", "Developer Guide pages"},
	{"guides-clean", "guide and module help pages"},
	{"source", "Java source files"},
	{"devguide-rc", "devguide rc/ example sources"},
	{"devguide-examples", "developer example sources"},
	{"docs-text", "PDF manuals"},
	{"course-text", "developer course pages"},
	{"module-examples", "module jar templates (.px .hbs .vm .bog)"},
}

// runGates is run_gates: DocJarCoverage, then RunSweep, each into a buffer
// of its own (its RESULT line is read from it alone); the two reports,
// joined by a newline, are root/gates.log (a plain write, as Python's
// open("w"); LF on every OS, as build.log). docs reads the full-text index
// (nil: FreshFulltextDocs). An error is what a gate raises: gates.log is
// not written.
func runGates(repo, root, vid string, docs FulltextDocs) (*Gates, error) {
	if docs == nil {
		docs = FreshFulltextDocs
	}
	var dj, sw bytes.Buffer
	djStatus, err := DocJarCoverage(&dj, repo, root, vid, docs)
	if err != nil {
		return nil, err
	}
	swStatus, err := RunSweep(repo, root, vid, &sw, SweepOptions{Fulltext: docs})
	if err != nil {
		return nil, err
	}
	g := &Gates{DocJars: gateRun{djStatus, resultLine(dj.String())}, Sweep: gateRun{swStatus, resultLine(sw.String())}}
	if err := os.WriteFile(pycompat.Join(root, gatesLog), []byte(dj.String()+"\n"+sw.String()), 0o666); err != nil {
		return nil, err
	}
	return g, nil
}

// resultLine is _result_line: the last "RESULT: " line of text
// (str.splitlines), without the tag.
func resultLine(text string) string {
	found := "(no RESULT line)"
	for _, line := range pycompat.SplitLines(text, false) {
		if rest, ok := strings.CutPrefix(line, "RESULT: "); ok {
			found = rest
		}
	}
	return found
}

// sourceCounts is _counts: {source label: indexed files} of the full-text
// index (docs builds it when absent, so the first search is warm).
func sourceCounts(root string, docs FulltextDocs) (map[string]int, error) {
	if docs == nil {
		docs = FreshFulltextDocs
	}
	keys, err := docs(root)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for k := range keys {
		src, _, _ := strings.Cut(k, "/") // a source label holds no "/"
		out[src]++
	}
	return out, nil
}

// summaryClasses is sum(1 + len(v.get("others", []))) over class-index.json
// ({} when the file is absent).
func summaryClasses(root string) (int, error) {
	const name = "class-index.json"
	if !pycompat.IsFile(pycompat.Join(root, "indexes", name)) {
		return 0, nil
	}
	idx, err := loadReportIndex(root, name, false)
	if err != nil {
		return 0, err
	}
	n := 0
	for i, v := range idx.Values() {
		rec, ok := v.(*index.Object)
		if !ok {
			return 0, &indexShapeError{name, fmt.Sprintf("the record of %s is not an object", idx.Keys()[i])}
		}
		n++
		others, ok := rec.Get("others")
		if !ok {
			continue
		}
		switch o := others.(type) {
		case []any:
			n += len(o)
		case *index.Object:
			n += o.Len()
		case string:
			n += len([]rune(o))
		default:
			return 0, &indexShapeError{name, fmt.Sprintf("\"others\" of %s has no length", idx.Keys()[i])}
		}
	}
	return n, nil
}

// summaryFolders is len(guides-index.json) (0 when the file is absent).
func summaryFolders(root string) (int, error) {
	const name = "guides-index.json"
	if !pycompat.IsFile(pycompat.Join(root, "indexes", name)) {
		return 0, nil
	}
	idx, err := loadReportIndex(root, name, false)
	if err != nil {
		return 0, err
	}
	return idx.Len(), nil
}

// relTo is os.path.relpath(path, start).
func relTo(path, start string) string {
	a, err1 := pyAbspath(path)
	b, err2 := pyAbspath(start)
	if err1 != nil || err2 != nil {
		return path
	}
	rel, err := filepath.Rel(b, a)
	if err != nil {
		return path
	}
	return rel
}

// printSummary is print_summary: the SUMMARY block and the RESULT line of
// `versions add` for vid at root, from the extraction result and the
// gates. docs reads the full-text index (nil: FreshFulltextDocs).
func printSummary(w io.Writer, repo, vid, root string, res *SetupResult, gates *Gates, docs FulltextDocs) error {
	counts, err := sourceCounts(root, docs)
	if err != nil {
		return err
	}
	classes, err := summaryClasses(root)
	if err != nil {
		return err
	}
	folders, err := summaryFolders(root)
	if err != nil {
		return err
	}
	out := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	out("")
	out("SUMMARY %s", vid)
	out("  install:   %s", res.Install)
	out("  data root: %s", relTo(root, repo))
	fid := res.Fidelity
	if ext := res.Extracted; ext != nil {
		jars, matched, checked := 0, 0, 0
		if fid != nil {
			jars, matched, checked = fid.Jars, fid.Matched, fid.Checked
		}
		out("  extracted: %s files (%s bytes) from %d doc jars (sha256-exact %s of %s)",
			commas(ext.Files), commas(ext.Bytes), jars, commas(matched), commas(checked))
	}
	out("  classes:   %s in class-index.json", commas(classes))
	out("  guide folders: %s", commas(folders))
	out("  searchable full-text files per source kind:")
	_, pdfFrom := fts.DocsTextRoot(root)
	known := map[string]bool{}
	for _, k := range summaryKinds {
		label, what := k[0], k[1]
		known[label] = true
		n := counts[label]
		note := ""
		switch label {
		case "course-text":
			if _, owner := fts.SharedRoot(root, label); owner != "" {
				note = "  (shared: the course bound to Niagara " + owner + ")"
			}
		case "docs-text":
			switch {
			case pdfFrom != "":
				note = "  (fallback: this install ships no PDF manuals; these are the manuals of Niagara " + pdfFrom + ")"
			case n == 0:
				note = "  (none: no docs/ PDFs here and no registered version has any)"
			}
		}
		out("    %s %s  %s%s", pyFmt(label, "18s"), pyFmt(n, ">7,"), what, note)
	}
	var extra []string
	for label := range counts {
		if !known[label] {
			extra = append(extra, label)
		}
	}
	sort.Strings(extra)
	for _, label := range extra {
		out("    %s %s", pyFmt(label, "18s"), pyFmt(counts[label], ">7,"))
	}
	out("  gates (full reports: %s):", relTo(pycompat.Join(root, gatesLog), repo))
	out("    coverage --doc-jars  exit %d  %s", gates.DocJars.Status, gates.DocJars.Result)
	out("    coverage --sweep     exit %d  %s", gates.Sweep.Status, gates.Sweep.Result)
	ok := gates.DocJars.Status == 0 && gates.Sweep.Status == 0 &&
		(fid == nil || len(fid.Mismatched) == 0 && len(fid.Missing) == 0)
	if ok {
		out("RESULT: READY - every page is searchable and both gates pass; query it with --niagara-version %s", vid)
	} else {
		out("RESULT: NOT READY - see the gate reports above")
	}
	return nil
}
