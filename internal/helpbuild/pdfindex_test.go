package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Unit tests of ExtractPDFs, ported from niagara-help
// tests/test_source_extraction.py TestPdfExtractorRun: a failed PDF keeps
// its text and its index entry; docs-text/ is pruned from the stems
// actually written, only after pdf-index.json is saved; an unresolved
// install never looks like a successful refresh.

type pdfRun struct {
	root, docs, textDir, script string
	oldIndex                    pyjson.Value
}

// newPDFRun is TestPdfExtractorRun.setUp: docA/A.pdf, docB/B.pdf, old
// texts A, B, Gone and an index listing the three.
func newPDFRun(t *testing.T) *pdfRun {
	script, dir := fakePdftotext(t)
	r := &pdfRun{root: filepath.Join(dir, "root"), script: script}
	r.docs = filepath.Join(r.root, "install-docs")
	r.textDir = filepath.Join(r.root, "docs-text")
	for _, rel := range []string{"docA/A.pdf", "docB/B.pdf"} {
		writeFile(t, filepath.Join(r.docs, rel), []byte("%PDF"))
	}
	r.oldIndex = pyjson.NewObject()
	for _, s := range []string{"A", "B", "Gone"} {
		writeFile(t, filepath.Join(r.textDir, s+".txt"), []byte("old text of "+s))
		e := pyjson.NewObject()
		e.Set("source_pdf", pyjson.String("doc"+s+"/"+s+".pdf"))
		e.Set("pages", pyjson.IntValue(1))
		e.Set("category", pyjson.String("software"))
		r.oldIndex.Set(s, e)
	}
	writeFile(t, filepath.Join(r.root, "indexes", "pdf-index.json"), []byte(pyjson.Dumps(r.oldIndex, DumpDefault)))
	return r
}

// texts sets what the fake pdftotext prints per PDF ("" = it fails).
func (r *pdfRun) texts(t *testing.T, m map[string]string) {
	for rel, text := range m {
		pdf := filepath.Join(r.docs, rel)
		_ = os.Remove(pdf + ".fail")
		if text == "" {
			writeFile(t, pdf+".fail", nil)
		}
		writeFile(t, pdf+".out", []byte(text))
	}
}

func (r *pdfRun) main(t *testing.T, base, docs string) *PDFResult {
	t.Helper()
	res, err := ExtractPDFs(base, PDFOptions{DocsDir: docs, Pdftotext: r.script})
	if err != nil {
		t.Fatalf("ExtractPDFs: %v\n%s", err, res.Stdout())
	}
	return res
}

func (r *pdfRun) index(t *testing.T) pyjson.Value {
	v, err := loadJSON(filepath.Join(r.root, "indexes", "pdf-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func (r *pdfRun) text(t *testing.T, stem string) string {
	return string(readBytes(t, filepath.Join(r.textDir, stem+".txt")))
}

func TestPDFFailedKeepsTextAndIndexEntry(t *testing.T) { // test_a_failed_pdf_keeps_its_text_and_index_entry
	r := newPDFRun(t)
	r.texts(t, map[string]string{"docA/A.pdf": strings.Repeat("new text A ", 10), "docB/B.pdf": ""})
	res := r.main(t, r.root, r.docs)
	if res.Status != 1 || !reflect.DeepEqual(res.Failed, []string{"B.pdf"}) {
		t.Fatalf("status %d, failed %q", res.Status, res.Failed)
	}
	if got := r.text(t, "B"); got != "old text of B" {
		t.Errorf("B.txt = %q", got)
	}
	old, _ := r.oldIndex.Get("B")
	if got, _ := r.index(t).Get("B"); !pyjson.Equal(got, old) {
		t.Errorf("index B = %s", pyjson.Dumps(got, DumpDefault))
	}
	if !strings.Contains(r.text(t, "A"), "new text A") {
		t.Errorf("A.txt = %q", r.text(t, "A"))
	}
	if r.text(t, "Gone") == "" {
		t.Error("Gone.txt pruned after a failed run")
	}
}

// A failed PDF keeps the last good entry only when that entry came from
// the same PDF: "a b.pdf" and "a_b.pdf" sanitise to one stem, so once the
// first is removed the second takes its stem "a_b" and, when it fails,
// must not inherit the removed manual's text (Python matches the stem
// only; an intended difference).
func TestPDFFailedInheritsOnlyFromTheSamePDF(t *testing.T) {
	r := newPDFRun(t)
	r.texts(t, map[string]string{"docA/A.pdf": strings.Repeat("new text A ", 10), "docB/B.pdf": strings.Repeat("new text B ", 10)})
	for stem, src := range map[string]string{"a_b": "docA/a b.pdf", "a_b_2": "docA/a_b.pdf"} {
		writeFile(t, filepath.Join(r.textDir, stem+".txt"), []byte("old text of "+src))
		e := pyjson.NewObject()
		e.Set("source_pdf", pyjson.String(src))
		r.oldIndex.Set(stem, e)
	}
	writeFile(t, filepath.Join(r.root, "indexes", "pdf-index.json"), []byte(pyjson.Dumps(r.oldIndex, DumpDefault)))
	writeFile(t, filepath.Join(r.docs, "docA", "a_b.pdf"), []byte("%PDF"))
	r.texts(t, map[string]string{"docA/a_b.pdf": ""})
	res := r.main(t, r.root, r.docs)
	if res.Status != 1 || !reflect.DeepEqual(res.Failed, []string{"a_b.pdf"}) {
		t.Fatalf("status %d, failed %q", res.Status, res.Failed)
	}
	idx := r.index(t)
	if got, ok := idx.Get("a_b"); ok {
		t.Errorf("a_b.pdf inherited the entry of the removed a b.pdf: %s", pyjson.Dumps(got, DumpDefault))
	}
	if got := idx.Keys; !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Errorf("index keys %q, want A and B", got)
	}
}

func TestPDFPruneRunsAfterTheIndexIsSaved(t *testing.T) { // test_prune_runs_after_the_index_is_saved
	r := newPDFRun(t)
	r.texts(t, map[string]string{"docA/A.pdf": strings.Repeat("new text A ", 10), "docB/B.pdf": strings.Repeat("new text B ", 10)})
	saved := writePDFIndex
	t.Cleanup(func() { writePDFIndex = saved })
	writePDFIndex = func(string, pyjson.Value) error { return errors.New("disk full") }
	_, err := ExtractPDFs(r.root, PDFOptions{DocsDir: r.docs, Pdftotext: r.script})
	writePDFIndex = saved
	if err == nil || err.Error() != "disk full" {
		t.Fatalf("broken save: %v", err)
	}
	if got := r.text(t, "Gone"); got != "old text of Gone" {
		t.Fatalf("Gone.txt = %q after a failed save", got)
	}
	res := r.main(t, r.root, r.docs)
	if res.Status != 0 {
		t.Fatalf("status %d\n%s", res.Status, res.Stdout())
	}
	if _, err := os.Stat(filepath.Join(r.textDir, "Gone.txt")); !os.IsNotExist(err) {
		t.Errorf("Gone.txt not pruned: %v", err)
	}
	if got := r.index(t).Keys; !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Errorf("index keys %q", got)
	}
}

func TestPDFUnresolvedInstallIsAnError(t *testing.T) { // test_unresolved_install_with_existing_manuals_is_an_error
	r := newPDFRun(t)
	res := r.main(t, r.root, "")
	if res.Status != 2 || !strings.Contains(res.Stdout(), "install") {
		t.Errorf("status %d\n%s", res.Status, res.Stdout())
	}
	empty := filepath.Join(r.root, "fresh")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	res = r.main(t, empty, "")
	if res.Status != 0 || !strings.Contains(res.Stdout(), "SKIP") || !strings.Contains(res.Stdout(), "docs dir: None") {
		t.Errorf("status %d\n%s", res.Status, res.Stdout())
	}
}

// A missing pdftotext is exit 1 with Python's message, before any write.
func TestPDFPdftotextNotFound(t *testing.T) {
	r := newPDFRun(t)
	res, err := ExtractPDFs(filepath.Join(r.root, "other"), PDFOptions{DocsDir: r.docs, Pdftotext: filepath.Join(r.root, "nope")})
	if err != nil || res.Status != 1 || res.Stdout() != "ERROR: pdftotext not found on PATH. Install poppler-utils.\n" {
		t.Fatalf("status %d, %v\n%s", res.Status, err, res.Stdout())
	}
	if _, err := os.Stat(filepath.Join(r.root, "other")); !os.IsNotExist(err) {
		t.Errorf("the data root was created: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if res, err := ExtractPDFs(filepath.Join(r.root, "other"), PDFOptions{DocsDir: r.docs}); err != nil || res.Status != 1 {
		t.Fatalf("PATH lookup: status %d, %v", res.Status, err)
	}
}

// Emit sees every entry as it is produced; Stdout puts the progress writes
// back on one line, as the terminal shows them.
func TestPDFEmitAndStdout(t *testing.T) {
	r := newPDFRun(t)
	r.texts(t, map[string]string{"docA/A.pdf": "short", "docB/B.pdf": strings.Repeat("b", 60)})
	var emitted []string
	res, err := ExtractPDFs(r.root, PDFOptions{DocsDir: r.docs, Pdftotext: r.script, Emit: func(e string) { emitted = append(emitted, e) }})
	if err != nil || !reflect.DeepEqual(emitted, res.Log) {
		t.Fatalf("emitted %q, log %q, %v", emitted, res.Log, err)
	}
	out := res.Stdout()
	if !strings.Contains(out, " PDFs in "+r.docs+"\n\r[ 50%] 1/2  A.pdf\r[100%] 2/2  B.pdf\n\n=") {
		t.Errorf("stdout:\n%q", out)
	}
	if !strings.Contains(out, "\n--- Empty/minimal text (1) ---\n  A.pdf\n\nDone.\n") {
		t.Errorf("stdout:\n%q", out)
	}
}
