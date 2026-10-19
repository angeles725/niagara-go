package helpbuild

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/procgroup"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// pdf_extractor.main() as a library call: docs-text/<stem>.txt per PDF and
// indexes/pdf-index.json under a data root.

// DumpPDFIndex is json.dumps(indent=2, ensure_ascii=False), the
// pdf-index.json format (indent implies the (",", ": ") separators).
var DumpPDFIndex = pyjson.DumpOpts{EnsureASCII: false, Indent: 2, ItemSep: ",", KeySep: ": "}

// PDFOptions are the inputs of ExtractPDFs.
type PDFOptions struct {
	// DocsDir is --docs-dir; "" is <install_dir>/docs of the version
	// registered for the data root (none: no docs dir).
	DocsDir string
	// Pdftotext is the program to run; "" is Pdftotext, looked up on PATH.
	Pdftotext string
	// Timeout of one PDF; 0 is PDFTimeout.
	Timeout time.Duration
	// Emit, when set, receives every Log entry as it is produced (the
	// progress display of a long run).
	Emit func(entry string)
}

// PDFStats are the counters of the summary.
type PDFStats struct{ Total, Extracted, Empty, Failed, Software, Hardware int }

// PDFResult is what main() printed and returned. A Log entry is one
// print() line, except an entry starting with "\r", which is a progress
// write without a newline (sys.stdout.write); the tests' Stdout joins them
// back into the printed text.
type PDFResult struct {
	Log           []string
	Status        int // main()'s return value, the exit code
	Stats         PDFStats
	Empty, Failed []string // PDF file names, in run order
	emit          func(string)
}

func (r *PDFResult) add(entry string) {
	r.Log = append(r.Log, entry)
	if r.emit != nil {
		r.emit(entry)
	}
}

func (r *PDFResult) out(format string, a ...any) { r.add(fmt.Sprintf(format, a...)) }

// writePDFIndex saves pdf-index.json; the tests replace it to fail the
// save. Text that is not UTF-8 (a lone surrogate kept from the previous
// index) is refused as Python's encode refuses it.
var writePDFIndex = func(path string, v pyjson.Value) error {
	text := pyjson.Dumps(v, DumpPDFIndex)
	if !utf8.ValidString(text) {
		return fmt.Errorf("%s: 'utf-8' codec can't encode a surrogate of the index", path)
	}
	return atomicfile.WriteText(path, text)
}

// pyRound1 is round(x, 1): correctly rounded to one decimal, ties to even
// on the exact binary value, as strconv's 'f' formatting rounds.
func pyRound1(x float64) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	return f
}

// firstRunes is s[:n] in code points.
func firstRunes(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}

// docsDirOf is parse_args' docs dir: the option, else <install_dir>/docs.
func docsDirOf(base, docs string) (string, error) {
	if docs != "" {
		return docs, nil
	}
	install, err := installDir(base)
	if err != nil || install == "" {
		return "", err
	}
	return pycompat.Join(install, "docs"), nil
}

// loadPDFIndex reads the previous pdf-index.json: the dict, or why it
// cannot be used (corrupt, Python's str of the error); err is a failure
// Python does not catch.
func loadPDFIndex(path string) (prev pyjson.Value, corrupt string, isCorrupt bool, err error) {
	prev = pyjson.NewObject()
	if !pycompat.IsFile(path) {
		return prev, "", false, nil
	}
	v, err := loadJSON(path)
	var le *JSONLoadError
	switch {
	case errors.As(err, &le):
		return prev, le.Msg, true, nil
	case err != nil:
		return prev, "", false, err
	case v.Kind != pyjson.Object:
		return prev, "not a JSON object", true, nil
	}
	return v, "", false, nil
}

// probePdftotext is the "pdftotext -v" check: notFound when the program
// cannot be found (FileNotFoundError); any other start failure or the
// probe's timeout is an error, as Python does not catch it.
func probePdftotext(prog string) (notFound bool, err error) {
	_, err = runCapture([]string{prog, "-v"}, PDFProbeTimeout)
	var exit *exec.ExitError
	switch {
	case err == nil || errors.As(err, &exit):
		return false, nil // the exit status is not checked
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist):
		return true, nil
	case errors.Is(err, errTimedOut):
		return false, fmt.Errorf("%s -v timed out after %v", prog, PDFProbeTimeout)
	}
	return false, err
}

// ExtractPDFs is pdf_extractor.main() for data root baseDir. The early
// exits print their ERROR/SKIP line and set Status (2, 0, or 1 when
// pdftotext is missing). Otherwise every PDF of the docs dir is converted:
// docs-text/<stem>.txt is written for each extracted one (even when nearly
// empty), a failed one keeps its last good text and index entry, then
// pdf-index.json is saved and, when no PDF failed, the .txt files this run
// did not write are removed. Status is 1 when a PDF failed. err is what
// Python raises (traceback); res holds the lines printed until then.
func ExtractPDFs(baseDir string, opt PDFOptions) (res *PDFResult, err error) {
	res = &PDFResult{emit: opt.Emit}
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return res, err
	}
	docs, err := docsDirOf(base, opt.DocsDir)
	if err != nil {
		return res, err
	}
	shown := docs // '%s' % None
	if docs == "" {
		shown = "None"
	}
	outputDir := filepath.Join(base, "docs-text")
	indexDir := filepath.Join(base, "indexes")
	indexPath := filepath.Join(indexDir, "pdf-index.json")
	previous, why, corrupt, err := loadPDFIndex(indexPath)
	if err != nil {
		return res, err
	}
	// One walk for the check and the run (Python walks twice).
	var pdfs []string
	if docs != "" && pycompat.IsDir(docs) {
		pdfs = FindPDFs(docs)
	}
	hasPDFs := len(pdfs) > 0
	nPrev := len(previous.Keys)
	switch {
	case corrupt && !hasPDFs:
		res.out("ERROR: %s is corrupt or unreadable (%s) and cannot be rebuilt: no PDF manual in the docs dir %s. "+
			"It is kept as found; mount the install or pass --docs-dir PATH.", indexPath, why, shown)
		res.Status = 2
		return res, nil
	case corrupt:
		res.out("WARNING: %s is corrupt or unreadable (%s); rebuilding it from the PDFs.", indexPath, why)
	}
	switch {
	case docs == "" && nPrev > 0:
		res.out("ERROR: no install registered for %s, so its %d PDF manuals cannot be refreshed; pass --docs-dir PATH.",
			base, nPrev)
		res.Status = 2
		return res, nil
	case nPrev > 0 && !hasPDFs:
		state := "is missing"
		if pycompat.IsDir(docs) {
			state = "holds no PDF"
		}
		res.out("ERROR: %s lists %d PDF manual(s) but the docs dir %s %s; pdf-index.json and docs-text/ are kept "+
			"unchanged. Mount the install or pass --docs-dir PATH.", indexPath, nPrev, docs, state)
		res.Status = 2
		return res, nil
	case !hasPDFs:
		res.out("SKIP: no PDF manuals for %s (docs dir: %s); search uses the shared manuals.", base, shown)
		return res, nil
	}
	prog := opt.Pdftotext
	if prog == "" {
		prog = Pdftotext
	}
	if notFound, err := probePdftotext(prog); err != nil {
		return res, err
	} else if notFound {
		res.out("ERROR: pdftotext not found on PATH. Install poppler-utils.")
		res.Status = 1
		return res, nil
	}
	for _, d := range []string{outputDir, indexDir} {
		if err := os.MkdirAll(d, 0o777); err != nil {
			return res, err
		}
	}
	return res, extractAll(res, docs, pdfs, outputDir, indexPath, previous, prog, opt.Timeout)
}

// pdfTimeout is the timeout of one PDF; the parity test shortens it for
// its timeout fixture only, as its Python driver does.
var pdfTimeout = func(pdfPath string, timeout time.Duration) time.Duration { return timeout }

// extractAll is main()'s loop over pdfs (FindPDFs of docs), the index
// save, the prune and the summary.
func extractAll(res *PDFResult, docs string, pdfs []string, outputDir, indexPath string, previous pyjson.Value,
	prog string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = PDFTimeout
	}
	total := len(pdfs)
	res.out("Found %d PDFs in %s", total, docs)
	res.Stats.Total = total
	index := pyjson.NewObject()
	written, seen := map[string]bool{}, map[string]bool{}
	t0 := time.Now()
	for i, pdfPath := range pdfs {
		folder, err := topFolder(pdfPath, docs)
		if err != nil {
			return err
		}
		name := filepath.Base(pdfPath)
		category := ClassifyPDF(folder)
		stem := resolveName(SanitizePDFName(name), seen)
		res.add(fmt.Sprintf("\r[%3d%%] %d/%d  %s", (i+1)*100/total, i+1, total, firstRunes(name, 60)))
		textPath := filepath.Join(outputDir, stem+".txt")
		rel, err := pyRelSlash(pdfPath, docs)
		if err != nil {
			return err
		}
		text, pages, err := ExtractPDFText(prog, pdfPath, pdfTimeout(pdfPath, timeout))
		if procgroup.IsInterrupt(err) {
			return err // KeyboardInterrupt: nothing is saved or pruned
		}
		if err != nil {
			res.Stats.Failed++
			res.Failed = append(res.Failed, name)
			// Keep the last good text and its index entry: a failed run
			// must not make a manual unsearchable. Only an entry of this
			// same PDF is kept (Python matches the stem only): a PDF that
			// took the stem of a removed one must not inherit its text.
			if old, ok := previous.Get(stem); ok && sameSource(old, rel) && pycompat.IsFile(textPath) {
				index.Set(stem, old)
				written[stem] = true
			}
			continue
		}
		sizeKB := pyjson.IntValue(0)
		if st, err := os.Stat(pdfPath); err == nil {
			sizeKB = pyjson.Value{Kind: pyjson.Float, F: pyRound1(float64(st.Size()) / 1024.0)}
		}
		stripped := utf8.RuneCountInString(pycompat.Strip(text))
		if stripped < 50 { // less than 50 chars = effectively empty
			res.Stats.Empty++
			res.Empty = append(res.Empty, name)
		} else {
			res.Stats.Extracted++
		}
		if err := atomicfile.WriteText(textPath, text); err != nil {
			return err
		}
		written[stem] = true
		e := pyjson.NewObject()
		e.Set("source_pdf", pyjson.String(rel))
		e.Set("folder", pyjson.String(folder))
		e.Set("pages", pyjson.IntValue(int64(pages)))
		e.Set("size_kb", sizeKB)
		e.Set("text_chars", pyjson.IntValue(int64(stripped)))
		e.Set("category", pyjson.String(category))
		index.Set(stem, e)
		if category == "hardware" {
			res.Stats.Hardware++
		} else {
			res.Stats.Software++
		}
	}
	elapsed := time.Since(t0).Seconds()
	res.out("\n")
	if err := writePDFIndex(indexPath, index); err != nil {
		return err
	}
	// docs-text/ is derived: once the index is saved and the run is clean,
	// drop every .txt this run did not write.
	if len(res.Failed) == 0 {
		if err := pruneText(outputDir, written); err != nil {
			return err
		}
	}
	summary(res, elapsed, outputDir, indexPath)
	return nil
}

// sameSource reports whether the previous index entry old was made from
// the PDF at rel (its "source_pdf", relative to the docs dir).
func sameSource(old pyjson.Value, rel string) bool {
	src, ok := old.Get("source_pdf")
	return ok && src.Kind == pyjson.Str && src.S == rel
}

// pruneText removes the docs-text/*.txt files whose stem is not in written
// (os.listdir + os.remove: a directory named *.txt is an error).
func pruneText(outputDir string, written map[string]bool) error {
	ents, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if n := e.Name(); strings.HasSuffix(n, ".txt") && !written[strings.TrimSuffix(n, ".txt")] {
			if err := removeFile(filepath.Join(outputDir, n)); err != nil {
				return err
			}
		}
	}
	return nil
}

// summary prints main()'s closing block and sets the exit status.
func summary(res *PDFResult, elapsed float64, outputDir, indexPath string) {
	bar := strings.Repeat("=", 60)
	s := res.Stats
	res.out("%s", bar)
	res.out("Phase 4 - PDF Extraction Complete")
	res.out("%s", bar)
	res.out("Total PDFs:        %d", s.Total)
	res.out("Extracted (text):  %d", s.Extracted)
	res.out("Empty/minimal:     %d", s.Empty)
	res.out("Failed:            %d", s.Failed)
	res.out("Software:          %d", s.Software)
	res.out("Hardware:          %d", s.Hardware)
	res.out("Time:              %.1f s", elapsed)
	res.out("Output:            %s/", outputDir)
	res.out("Index:             %s", indexPath)
	for _, l := range []struct {
		title string
		names []string
	}{{"Empty/minimal text", res.Empty}, {"Failed extractions", res.Failed}} {
		if len(l.names) == 0 {
			continue
		}
		res.out("\n--- %s (%d) ---", l.title, len(l.names))
		sorted := append([]string(nil), l.names...)
		sort.Strings(sorted)
		for _, n := range sorted {
			res.out("  %s", n)
		}
	}
	res.out("\nDone.")
	if len(res.Failed) > 0 {
		res.Status = 1
	}
}
