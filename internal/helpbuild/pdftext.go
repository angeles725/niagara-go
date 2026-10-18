package helpbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// PDF text through pdftotext (pdf_extractor.py, the per-PDF part): the
// classification, the text-file stem, the PDF walk, the bullet fix and one
// pdftotext run. ExtractPDFs (pdfindex.go) is the tool's main().

// PDFTimeout is the limit of one pdftotext run, as in extract_text; the
// version probe ("pdftotext -v") gets PDFProbeTimeout.
const (
	PDFTimeout      = 120 * time.Second
	PDFProbeTimeout = 10 * time.Second
)

// Pdftotext is the program name pdf_extractor runs: resolved on PATH when
// the run starts, as subprocess does without a shell.
const Pdftotext = "pdftotext"

// HardwarePrefixes are the docs/ folder prefixes of the hardware manuals
// (HARDWARE_PREFIXES). SOFTWARE_HIGH is defined in pdf_extractor.py but
// never read, so it is not ported.
var HardwarePrefixes = []string{"docCIPerModel", "docJ8", "docJ9", "docTR50", "docTR100", "MtgWiring"}

// ClassifyPDF is classify: "hardware" or "software" from the top-level
// docs/ folder name.
func ClassifyPDF(folder string) string {
	for _, p := range HardwarePrefixes {
		if strings.HasPrefix(folder, p) {
			return "hardware"
		}
	}
	return "software"
}

// hasSuffixFold is s.lower().endswith(suffix) for an ASCII lower-case
// suffix: no non-ASCII character lowers to an ASCII letter or dot of the
// suffixes used here (".pdf", ".txt"), so comparing the last bytes ASCII-
// folded is exact.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// SanitizePDFName is sanitize: the flat docs-text/ stem of a PDF file name.
func SanitizePDFName(name string) string {
	// re.sub(r'\.pdf$', '', name, flags=re.I): $ also matches before a
	// trailing newline.
	switch {
	case hasSuffixFold(name, ".pdf"):
		name = name[:len(name)-4]
	case strings.HasSuffix(name, "\n") && hasSuffixFold(name[:len(name)-1], ".pdf"):
		name = name[:len(name)-5] + "\n"
	}
	var b strings.Builder
	under := false // the last rune written is "_" from a run being collapsed
	for _, r := range name {
		if strings.ContainsRune(`<>:"/\|?*&`, r) || pycompat.IsSpace(r) {
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

// FindPDFs is find_pdfs: every *.pdf (any case) under docsDir, walked like
// os.walk (symlinked folders are not entered, unreadable ones skipped),
// sorted by full path.
func FindPDFs(docsDir string) []string {
	var out []string
	pyWalk(docsDir, func(dir string, dirs, files []string) []string {
		for _, f := range files {
			if hasSuffixFold(f, ".pdf") {
				out = append(out, pyJoin(dir, f))
			}
		}
		return dirs
	})
	sort.Strings(out)
	return out
}

// FixUnmappedBullets is fix_unmapped_bullets: U+FFFD (a bullet glyph
// without a Unicode mapping) after the indent of a line and before a space
// becomes U+2022, re.sub(r"(?m)^([ \t\f]*)\ufffd(?= )", "\\1\u2022").
func FixUnmappedBullets(text string) string {
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

// PDFFailure says why a pdftotext run gave no text; extract_text returns
// (None, 0) for all of them.
type PDFFailure int

// The failures extract_text catches.
const (
	PDFExitStatus PDFFailure = iota // returncode != 0 (a signal included)
	PDFTimedOut                     // subprocess.TimeoutExpired
	PDFNotUTF8                      // UnicodeDecodeError: the output is not UTF-8
	PDFOSError                      // OSError: the program could not be started
)

// PDFError is a failed text extraction of one PDF.
type PDFError struct {
	Path    string
	Failure PDFFailure
	Err     error
}

func (e *PDFError) Error() string {
	what := map[PDFFailure]string{PDFExitStatus: "pdftotext failed", PDFTimedOut: "pdftotext timed out",
		PDFNotUTF8: "pdftotext output is not valid UTF-8", PDFOSError: "pdftotext could not run"}[e.Failure]
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Path, what, e.Err)
	}
	return e.Path + ": " + what
}

func (e *PDFError) Unwrap() error { return e.Err }

// errTimedOut marks a run stopped by its timeout.
var errTimedOut = errors.New("timed out")

// interruptError is a SIGINT or SIGTERM that came while pdftotext ran (on
// Unix, see relayInterrupts): Python's KeyboardInterrupt, which
// pdf_extractor does not catch, so the run stops instead of counting a
// failed PDF.
type interruptError struct{ sig os.Signal }

func (e *interruptError) Error() string { return "interrupted by " + e.sig.String() }

// runCapture runs argv (no shell) and returns its stdout; stderr is
// dropped (Python captures it and never reads it) and stdin is the null
// device (Python lets the child inherit it; pdftotext never reads it). On
// the timeout the child's process group is killed (killGroupOnCancel) and
// errTimedOut returned. A SIGINT or SIGTERM meanwhile kills the group too,
// then is raised again on this process (reraise) and, should it survive,
// returned as an *interruptError.
func runCapture(argv []string, timeout time.Duration) ([]byte, error) {
	base, interrupt := context.WithCancelCause(context.Background())
	defer interrupt(nil)
	ctx, cancel := context.WithTimeout(base, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	killGroupOnCancel(cmd)
	cmd.WaitDelay = time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	stop := relayInterrupts(interrupt)
	err := cmd.Run()
	stop()
	var in *interruptError
	if errors.As(context.Cause(base), &in) {
		reraise(in.sig)
		return nil, in
	}
	if err := runFailure(ctx, err); err != nil {
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

// ExtractPDFText is extract_text: `<pdftotext> -layout -enc UTF-8 <pdf> -`
// with the timeout, the output decoded strictly as UTF-8, the bullet fix
// applied, and the page count (pageCount, 0 for text that is only
// whitespace). A failure is a *PDFError.
func ExtractPDFText(pdftotext, pdfPath string, timeout time.Duration) (text string, pages int, err error) {
	out, err := runCapture([]string{pdftotext, "-layout", "-enc", "UTF-8", pdfPath, "-"}, timeout)
	if err != nil {
		var exit *exec.ExitError
		var in *interruptError
		switch {
		case errors.As(err, &in):
			return "", 0, err // not a failed PDF: the run stops
		case errors.Is(err, errTimedOut):
			return "", 0, &PDFError{Path: pdfPath, Failure: PDFTimedOut}
		case errors.As(err, &exit):
			return "", 0, &PDFError{Path: pdfPath, Failure: PDFExitStatus, Err: err}
		}
		return "", 0, &PDFError{Path: pdfPath, Failure: PDFOSError, Err: err}
	}
	if !utf8.Valid(out) {
		return "", 0, &PDFError{Path: pdfPath, Failure: PDFNotUTF8}
	}
	text = FixUnmappedBullets(string(out))
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
