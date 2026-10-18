package helpbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// S3 parity: pdf_extractor.main() (testdata/parity/textdriver.py pdf) and
// ExtractPDFs on twin data roots, the same docs dir and the same pdftotext
// (the fake of pdffixtures_test.go in front of the real one, resolved on
// PATH by both sides). Compared: stdout (data root relabeled, the elapsed
// time normalized), the exit status, and both data-root trees byte for
// byte.

// runTextDriver runs textdriver.py; the environment (PATH,
// HB_PDF_TIMEOUT) is the test's.
func runTextDriver(t *testing.T, args ...string) [][]any {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "textdriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("textdriver %v: %v\n%s", args, err, stderr.String())
	}
	var rows [][]any
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		var row []any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("textdriver line %q: %v", l, err)
		}
		rows = append(rows, row)
	}
	return rows
}

var elapsedLine = regexp.MustCompile(`(?m)^Time:              \d+\.\d s$`)

// toolLines turns a tool's streams and outcome into the compared lines:
// each stream's lines, then "exit N" or "raise" (Go has no exception
// classes: the decision is compared).
func toolLines(root string, streams map[string]string, names []string, outcome string) []string {
	var out []string
	for _, n := range names {
		s := elapsedLine.ReplaceAllString(streams[n], "Time:              <t> s")
		for _, l := range strings.Split(strings.ReplaceAll(s, root, "<root>"), "\n") {
			out = append(out, fmt.Sprintf("%s %q", n, l))
		}
	}
	return append(out, outcome)
}

// pyToolLines is toolLines of a textdriver.py report.
func pyToolLines(t *testing.T, root string, rows [][]any) []string {
	streams := map[string]string{}
	var names []string
	outcome := ""
	for _, r := range rows {
		switch r[0] {
		case "stdout", "stderr":
			names = append(names, r[0].(string))
			streams[r[0].(string)] = r[1].(string)
		case "exit":
			outcome = fmt.Sprintf("exit %v", r[1])
		case "raise":
			t.Logf("python raised %v", r[1])
			outcome = "raise"
		}
	}
	return toolLines(root, streams, names, outcome)
}

// pdfStep runs one pdf_extractor on both data roots and compares.
func pdfStep(t *testing.T, what, pyRoot, goRoot, pyBase, goBase, docs string) {
	t.Helper()
	args := []string{"pdf", pyBase}
	if docs != "" {
		args = append(args, docs)
	}
	py := pyToolLines(t, pyRoot, runTextDriver(t, args...))
	res, err := ExtractPDFs(goBase, PDFOptions{DocsDir: docs})
	outcome := fmt.Sprintf("exit %d", res.Status)
	if err != nil {
		t.Logf("go error: %v", err)
		outcome = "raise"
	}
	gol := toolLines(goRoot, map[string]string{"stdout": res.Stdout()}, []string{"stdout"}, outcome)
	diffLines(t, what+" stdout", py, gol)
	diffLines(t, what+" tree", pyPDFTreeLines(t, pyRoot, pyBase), treeLines(t, goRoot))
}

// pyPDFTreeLines is treeLines of the Python tree root with the intended
// page-count difference taken out: in base's pdf-index.json every entry
// whose docs-text/<stem>.txt ends with a form feed has one page less
// (Python counts that trailing form feed as a page, Go does not). The file
// on disk is left as Python wrote it, since the next step reads it back.
func pyPDFTreeLines(t *testing.T, root, base string) []string {
	t.Helper()
	lines := treeLines(t, root)
	path := filepath.Join(base, "indexes", "pdf-index.json")
	v, err := loadJSON(path)
	if err != nil || v.Kind != pyjson.Object {
		return lines // missing or not an index: compared as it is
	}
	changed := false
	for _, stem := range v.Keys {
		e := v.M[stem]
		p, ok := e.Get("pages")
		if !ok || p.Kind != pyjson.Int || p.I.Sign() <= 0 {
			continue
		}
		text, err := os.ReadFile(filepath.Join(base, "docs-text", stem+".txt"))
		if err != nil || !bytes.HasSuffix(text, []byte("\f")) {
			continue
		}
		e.Set("pages", pyjson.IntValue(p.I.Int64()-1))
		changed = true
	}
	if !changed {
		return lines // byte for byte: also an index the test wrote itself
	}
	data := []byte(pyjson.Dumps(v, DumpPDFIndex))
	sum := sha256.Sum256(data)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range lines {
		if f := strings.Fields(l); len(f) == 5 && f[0] == filepath.ToSlash(rel) {
			lines[i] = fmt.Sprintf("%s %s %s %d %s", f[0], f[1], f[2], len(data), hex.EncodeToString(sum[:]))
		}
	}
	return lines
}

// syntheticDocs writes the docs/ tree of the synthetic install: text over
// two pages with an unmapped bullet, short and blank PDFs, a stem
// collision, hardware, top-level, nested, non-ASCII and punctuated names,
// a corrupt PDF, one whose text is not UTF-8 and one that times out (fake
// pdftotext side files), a symlinked folder and a file that is not a PDF.
func syntheticDocs(t *testing.T, docs string) {
	long := "Alarm classes route every alarm to its recipients and consoles."
	pdfs := map[string][]byte{
		"docAlarms/Alarms Guide.pdf":   makePDF([]string{long, "72 \xb0F is the limit"}, []string{"Steps:", "\x80 Disconnect the LAN.", " \x80x stays"}),
		"docAlarms/A&B: notes.pdf":     makePDF([]string{long}),
		"docX/Same Name.pdf":           makePDF([]string{long, "first"}),
		"docY/Same_Name.PDF":           makePDF([]string{long, "second"}),
		"docJ9Install/J9 Mounting.pdf": makePDF([]string{"Mount the J9 on a DIN rail; wire the 24 V supply first."}),
		"Top Level.pdf":                makePDF([]string{long, "top"}),
		"docNested/sub/deep.pdf":       makePDF([]string{long, "deep"}),
		"docGuía/Guía ñ.pdf":           makePDF([]string{"Gu\xeda del m\xf3dulo: se\xf1ales y alarmas del sistema completo."}),
		"docShort/short.pdf":           makePDF([]string{"tiny"}),
		"docShort/blank.pdf":           makePDF(nil),
		"docBad/corrupt.pdf":           []byte("%PDF-1.4\nnot really a PDF\n"),
		"docBad/latin1.pdf":            makePDF([]string{"never read"}),
		"docBad/slow.pdf":              makePDF([]string{"never read"}),
	}
	for rel, data := range pdfs {
		writeFile(t, filepath.Join(docs, rel), data)
	}
	writeFile(t, filepath.Join(docs, "docBad", "latin1.pdf.out"), []byte("72 \xb0F"))
	writeFile(t, filepath.Join(docs, "docBad", "slow.pdf.sleep"), nil)
	writeFile(t, filepath.Join(docs, "docAlarms", "readme.txt"), []byte("not a PDF"))
	if err := os.Symlink(filepath.Join(docs, "docAlarms"), filepath.Join(docs, "linked")); err != nil {
		t.Fatal(err)
	}
}

func TestParityPDFSynthetic(t *testing.T) {
	driver(t)
	real := needPdftotext(t)
	dir := t.TempDir()
	t.Setenv("PATH", fakeBin(t, dir, real)+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Only the timeout fixture (<pdf>.sleep) gets 1 s, on both sides;
	// every other PDF keeps the 120 s default.
	t.Setenv("HB_PDF_TIMEOUT", "1")
	saved := pdfTimeout
	t.Cleanup(func() { pdfTimeout = saved })
	pdfTimeout = func(pdf string, d time.Duration) time.Duration {
		if isRegular(pdf + ".sleep") {
			return time.Second
		}
		return d
	}
	install := filepath.Join(dir, "install")
	syntheticDocs(t, filepath.Join(install, "docs"))
	emptyDocs := filepath.Join(dir, "empty-docs")
	writeFile(t, filepath.Join(emptyDocs, "notes.txt"), []byte("no PDF here"))
	pyRepo, goRepo := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	repos := []string{pyRepo, goRepo}
	for _, repo := range repos {
		writeRegistry(t, repo, map[string][3]string{
			"1.0": {"synthetic", install, "versions/1.0"},
			"9.9": {"not mounted", install + "-missing", "versions/9.9"},
		})
		for _, b := range []string{"fresh", "fresh2", "versions/1.0", "versions/9.9"} {
			if err := os.MkdirAll(filepath.Join(repo, b), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	step := func(what, base, docs string) {
		t.Helper()
		pdfStep(t, what, pyRepo, goRepo, filepath.Join(pyRepo, base), filepath.Join(goRepo, base), docs)
	}
	index := func(base, text string) {
		onBoth(t, repos, func(r string) { writeFile(t, filepath.Join(r, base, "indexes", "pdf-index.json"), []byte(text)) })
	}
	step("unregistered root", "fresh", "")
	index("fresh", `{"x": {"pages": 1}}`)
	step("unregistered root with manuals", "fresh", "")
	index("fresh", `{"x": 1,`)
	step("corrupt index, nothing to rebuild from", "fresh", "")
	index("fresh", "[1]")
	step("index not an object", "fresh", "")
	index("fresh", "\xff{}")
	step("index not UTF-8", "fresh", "")
	index("versions/9.9", "{\"x\": 1}\r\n")
	step("install not mounted", "versions/9.9", "")
	index("fresh2", `{"x": 1, "y": 2}`)
	step("docs dir holds no PDF", "fresh2", emptyDocs)

	// A PATH holding python3 (the driver's lookup) but no pdftotext.
	python, _ := driver(t)
	noPdf := t.TempDir()
	if err := os.Symlink(python, filepath.Join(noPdf, "python3")); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	t.Setenv("PATH", noPdf)
	step("pdftotext not on PATH", "versions/1.0", "")
	t.Setenv("PATH", path)

	onBoth(t, repos, func(r string) {
		root := filepath.Join(r, "versions", "1.0")
		writeFile(t, filepath.Join(root, "docs-text", "corrupt.txt"), []byte("last good text"))
		writeFile(t, filepath.Join(root, "docs-text", "Gone.txt"), []byte("stale"))
		writeFile(t, filepath.Join(root, "docs-text", "notes.md"), []byte("not a .txt"))
	})
	index("versions/1.0", `{"corrupt": {"source_pdf": "docBad/corrupt.pdf", "size_kb": 1e2, "ñ": [1.5, null, true]},`+
		` "Gone": {"pages": 3}}`)
	step("failures keep the last good text", "versions/1.0", "")
	if err := os.Remove(filepath.Join(install, "docs", "docBad", "slow.pdf.sleep")); err != nil {
		t.Fatal(err)
	}
	index("versions/1.0", "{")
	step("corrupt index rebuilt", "versions/1.0", "")
	for _, gone := range []string{"corrupt.pdf", "latin1.pdf"} {
		if err := os.Remove(filepath.Join(install, "docs", "docBad", gone)); err != nil {
			t.Fatal(err)
		}
	}
	step("clean run prunes", "versions/1.0", "")
}

// realPDFs are five EC-Net 4.3.58.18 manuals: the largest, the one whose
// bullets come out of pdftotext as U+FFFD, the smallest, a hardware
// installation guide and the platform guide.
var realPDFs = []string{
	"User Guides/Getting Started EC-Net4_UG_11_EN.pdf",
	"User Guides/Station Security EC-Net4_UG_10_EN.pdf",
	"User Guides/05DI-UGSCIEN-10.pdf",
	"Hardware Installation Guides/EC-BOS-8 Quick Start_IG_10_EN.pdf",
	"User Guides/Platform Guide EC-Net4_UG_11_EN.pdf",
}

// TestParityPDFReal converts five real manuals (a copy; the install is
// only read) with the real pdftotext on both sides.
// $HELPBUILD_PARITY_PDF_DOCS names a docs dir to convert whole instead.
func TestParityPDFReal(t *testing.T) {
	if testing.Short() {
		t.Skip("real-data parity skipped with -short")
	}
	driver(t)
	real := needPdftotext(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(bin, "pdftotext")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	docs := os.Getenv("HELPBUILD_PARITY_PDF_DOCS")
	if docs == "" {
		src := "/mnt/c/Niagara/EC-Net4-4.3.58.18/docs"
		if !isDir(src) {
			t.Skip("EC-Net 4.3.58.18 docs not mounted: real PDF parity not checked")
		}
		docs = filepath.Join(dir, "docs")
		for _, rel := range realPDFs {
			copyFile(t, filepath.Join(src, rel), filepath.Join(docs, rel))
		}
	}
	pyRoot, goRoot := filepath.Join(dir, "py"), filepath.Join(dir, "go")
	for _, r := range []string{pyRoot, goRoot} {
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pdfStep(t, "real manuals", pyRoot, goRoot, pyRoot, goRoot, docs)
	logTreeSize(t, goRoot)
}

// logTreeSize logs the number of files and bytes under root.
func logTreeSize(t *testing.T, root string) {
	files, size := 0, int64(0)
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			files++
			size += info.Size()
		}
		return nil
	})
	t.Logf("%s: %d files, %d bytes", filepath.Base(root), files, size)
}
