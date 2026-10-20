package pdftext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/procgroup"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// PDF text through pdftotext (pdf_extractor.py, the per-PDF part): the
// classification, the text-file stem, the PDF walk, the bullet fix and one
// pdftotext run. ExtractPDFs (pdfindex.go) is the tool's main().

// PDFTimeout is the limit of one pdftotext run, as in extract_text; the
// version probe ("pdftotext -v") gets PDFProbeTimeout.
const (
	PDFTimeout      = 120 * time.Second
	pdfProbeTimeout = 10 * time.Second
)

// pdftotext is the program name pdf_extractor runs: resolved on PATH when
// the run starts, as subprocess does without a shell.
const pdftotext = "pdftotext"

// hardwarePrefixes are the docs/ folder prefixes of the hardware manuals
// (HARDWARE_PREFIXES). SOFTWARE_HIGH is defined in pdf_extractor.py but
// never read, so it is not ported.
var hardwarePrefixes = []string{"docCIPerModel", "docJ8", "docJ9", "docTR50", "docTR100", "MtgWiring"}

// classifyPDF is classify: "hardware" or "software" from the top-level
// docs/ folder name.
func classifyPDF(folder string) string {
	for _, p := range hardwarePrefixes {
		if strings.HasPrefix(folder, p) {
			return "hardware"
		}
	}
	return "software"
}

// HasSuffixFold is s.lower().endswith(suffix) for an ASCII lower-case
// suffix: no non-ASCII character lowers to an ASCII letter or dot of the
// suffixes used here (".pdf", ".txt"), so comparing the last bytes ASCII-
// folded is exact.
func HasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// sanitizePDFName is sanitize: the flat docs-text/ stem of a PDF file name.
func sanitizePDFName(name string) string {
	// re.sub(r'\.pdf$', '', name, flags=re.I): $ also matches before a
	// trailing newline.
	const ext = ".pdf"
	switch {
	case HasSuffixFold(name, ext):
		name = name[:len(name)-len(ext)]
	case strings.HasSuffix(name, "\n") && HasSuffixFold(name[:len(name)-len("\n")], ext):
		name = name[:len(name)-len(ext+"\n")] + "\n"
	}
	var b strings.Builder
	under := false // the last rune written is "_" from a run being collapsed
	for _, r := range name {
		if strings.ContainsRune(`<>:"/\|?*&`, r) || textfold.IsSpace(r) {
			r = '_'
		}
		// [<>...&] -> "_", \s+ -> "_", then _+ -> "_": every run of
		// underscores, whatever made it, becomes one.
		if r == '_' {
			if under {
				continue
			}
			under = true
		} else {
			under = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "_")
}

// pyRelSlash is os.path.relpath(path, start).replace('\\', '/') for a path
// under start (on POSIX a backslash in a name is replaced too).
func pyRelSlash(path, start string) (string, error) {
	ap, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	as, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(as, ap)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(rel, `\`, "/"), nil
}

// topFolder is top_folder: the docs/ child folder holding the PDF, "" for a
// PDF directly in docs/.
func topFolder(pdfPath, docsDir string) (string, error) {
	rel, err := pyRelSlash(pdfPath, docsDir)
	if err != nil {
		return "", err
	}
	first, _, nested := strings.Cut(rel, "/")
	if !nested {
		return "", nil
	}
	return first, nil
}

// findPDFs is find_pdfs: every *.pdf (any case) under docsDir, walked like
// os.walk (symlinked folders are not entered, unreadable ones skipped),
// sorted by full path.
func findPDFs(docsDir string) []string {
	var out []string
	pyfs.Walk(docsDir, func(dir string, dirs, files []string) []string {
		for _, f := range files {
			if HasSuffixFold(f, ".pdf") {
				out = append(out, pycompat.Join(dir, f))
			}
		}
		return dirs
	})
	sort.Strings(out)
	return out
}

// fixUnmappedBullets is fix_unmapped_bullets: U+FFFD (a bullet glyph
// without a Unicode mapping) after the indent of a line and before a space
// becomes U+2022, re.sub(r"(?m)^([ \t\f]*)\ufffd(?= )", "\\1\u2022").
func fixUnmappedBullets(text string) string {
	const fffd, bullet = "\ufffd", "\u2022"
	var b strings.Builder
	done := 0
	for line := 0; ; {
		j := line
		for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\f') {
			j++
		}
		if strings.HasPrefix(text[j:], fffd+" ") {
			b.WriteString(text[done:j])
			b.WriteString(bullet)
			done = j + len(fffd)
		}
		nl := strings.IndexByte(text[j:], '\n')
		if nl < 0 {
			break
		}
		line = j + nl + 1
	}
	b.WriteString(text[done:])
	return b.String()
}

// pdfFailure says why a pdftotext run gave no text; extract_text returns
// (None, 0) for all of them.
type pdfFailure int

// The failures extract_text catches.
const (
	pdfExitStatus pdfFailure = iota // returncode != 0 (a signal included)
	pdfTimedOut                     // subprocess.TimeoutExpired
	pdfNotUTF8                      // UnicodeDecodeError: the output is not UTF-8
	pdfOSError                      // OSError: the program could not be started
)

// pdfError is a failed text extraction of one PDF.
type pdfError struct {
	Path    string
	Failure pdfFailure
	Err     error
}

func (e *pdfError) Error() string {
	what := map[pdfFailure]string{pdfExitStatus: "pdftotext failed", pdfTimedOut: "pdftotext timed out",
		pdfNotUTF8: "pdftotext output is not valid UTF-8", pdfOSError: "pdftotext could not run"}[e.Failure]
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Path, what, e.Err)
	}
	return e.Path + ": " + what
}

func (e *pdfError) Unwrap() error { return e.Err }

// errTimedOut marks a run stopped by its timeout.
var errTimedOut = errors.New("timed out")

// runCapture runs argv (no shell) and returns its stdout; stderr is
// dropped (Python captures it and never reads it) and stdin is the null
// device (Python lets the child inherit it; pdftotext never reads it). The
// child runs through internal/procgroup: on the timeout its process group
// is killed and errTimedOut returned. A SIGINT or SIGTERM meanwhile kills
// the group too, then is raised again on this process and, should it
// survive, returned as a *procgroup.InterruptError: Python's
// KeyboardInterrupt, which pdf_extractor does not catch, so the run stops
// instead of counting a failed PDF.
func runCapture(argv []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := procgroup.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.WaitDelay = time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	runErr := cmd.Run()
	if procgroup.IsInterrupt(runErr) {
		return nil, runErr
	}
	if err := runFailure(ctx, runErr); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// runFailure is the error of a finished run: errTimedOut only when the run
// failed and its context is done (the cancel killed it); a run that
// succeeded as the deadline passed is a success.
func runFailure(ctx context.Context, runErr error) error {
	if runErr != nil && ctx.Err() != nil {
		return errTimedOut
	}
	return runErr
}

// pageCount is the pages of text that is not only whitespace: every form
// feed ends a page and text after the last one is one more page. pdftotext
// 26.10.0 ends every page with a form feed, the last one too; extract_text
// counts form feeds + 1, one page too many for it (an intended difference,
// docs/helpbuild.md). Text without the trailing form feed counts the same
// on both sides.
func pageCount(text string) int {
	n := strings.Count(text, "\f")
	if !strings.HasSuffix(text, "\f") {
		n++
	}
	return n
}

// extractPDFText is extract_text: `<pdftotext> -layout -enc UTF-8 <pdf> -`
// with the timeout, the output decoded strictly as UTF-8, the bullet fix
// applied, and the page count (pageCount, 0 for text that is only
// whitespace). A failure is a *PDFError.
func extractPDFText(pdftotext, pdfPath string, timeout time.Duration) (text string, pages int, err error) {
	out, err := runCapture([]string{pdftotext, "-layout", "-enc", "UTF-8", pdfPath, "-"}, timeout)
	if err != nil {
		var exit *exec.ExitError
		switch {
		case procgroup.IsInterrupt(err):
			return "", 0, err // not a failed PDF: the run stops
		case errors.Is(err, errTimedOut):
			return "", 0, &pdfError{Path: pdfPath, Failure: pdfTimedOut}
		case errors.As(err, &exit):
			return "", 0, &pdfError{Path: pdfPath, Failure: pdfExitStatus, Err: err}
		}
		return "", 0, &pdfError{Path: pdfPath, Failure: pdfOSError, Err: err}
	}
	if !utf8.Valid(out) {
		return "", 0, &pdfError{Path: pdfPath, Failure: pdfNotUTF8}
	}
	text = fixUnmappedBullets(string(out))
	if pycompat.Strip(text) != "" {
		pages = pageCount(text)
	}
	return text, pages, nil
}

// resolveName is resolve_name: stem, or stem_2, stem_3 ... when taken.
func resolveName(stem string, seen map[string]bool) string {
	name := stem
	for n := 2; seen[name]; n++ {
		name = fmt.Sprintf("%s_%d", stem, n)
	}
	seen[name] = true
	return name
}
