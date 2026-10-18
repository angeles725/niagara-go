package helpbuild

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Unit tests of pdftext.go, ported from niagara-help
// tests/test_source_extraction.py TestPdfText (named in each comment).

// test_unmapped_bullet_glyph_becomes_a_bullet, plus the corners of the
// (?m)^([ \t\f]*)\ufffd(?= ) substitution.
func TestFixUnmappedBullets(t *testing.T) {
	for in, want := range map[string]string{
		"Steps:\n   \ufffd Disconnect the LAN.\n": "Steps:\n   \u2022 Disconnect the LAN.\n",
		"\ufffd first line":                       "\u2022 first line",
		"\f\t\ufffd after a page break":           "\f\t\u2022 after a page break",
		"a \ufffd mid-line\n":                     "a \ufffd mid-line\n",
		"\ufffdx no space after":                  "\ufffdx no space after",
		"\ufffd\ufffd two":                        "\ufffd\ufffd two",
		"\r\ufffd cr is not a line start":         "\r\ufffd cr is not a line start",
		"x\n\ufffd \n\ufffd \ufffd y":             "x\n\u2022 \n\u2022 \ufffd y",
		"\ufffd":                                  "\ufffd",
		"":                                        "",
	} {
		if got := FixUnmappedBullets(in); got != want {
			t.Errorf("FixUnmappedBullets(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizePDFName(t *testing.T) {
	for in, want := range map[string]string{
		"Getting Started EC-Net4_UG_11_EN.pdf": "Getting_Started_EC-Net4_UG_11_EN",
		"a<b>c:d\"e|f?g*h&i.PDF":               "a_b_c_d_e_f_g_h_i",
		"  __x \t\u00a0 y__.pdf":               "x_y",
		"x.pdf.pdf":                            "x.pdf",
		"x.pdf\n":                              "x",
		"Guía ñ.Pdf":                           "Guía_ñ",
		".pdf":                                 "",
		"no extension":                         "no_extension",
	} {
		if got := SanitizePDFName(in); got != want {
			t.Errorf("SanitizePDFName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyPDF(t *testing.T) {
	for in, want := range map[string]string{"docJ9Install": "hardware", "MtgWiring": "hardware", "docTR100x": "hardware",
		"docJ7": "software", "": "software", "User Guides": "software", "docj9": "software"} {
		if got := ClassifyPDF(in); got != want {
			t.Errorf("ClassifyPDF(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveName(t *testing.T) {
	seen := map[string]bool{}
	var got []string
	for _, s := range []string{"a", "a", "a_2", "a", "b"} {
		got = append(got, resolveName(s, seen))
	}
	if want := []string{"a", "a_2", "a_2_2", "a_3", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("resolveName = %q, want %q", got, want)
	}
}

// FindPDFs sorts full paths, matches .pdf in any case and does not enter a
// symlinked folder (os.walk without followlinks); a symlinked file counts.
func TestFindPDFs(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"b/z.pdf", "a/y.PDF", "x.pdf", "a/notes.txt", "c/d/e.Pdf", "outside/o.pdf"} {
		writeFile(t, filepath.Join(dir, "docs", rel), []byte("%PDF"))
	}
	docs := filepath.Join(dir, "docs")
	if err := os.Symlink(filepath.Join(docs, "outside"), filepath.Join(docs, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(docs, "x.pdf"), filepath.Join(docs, "b", "alias.pdf")); err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, p := range FindPDFs(docs) {
		rels = append(rels, strings.TrimPrefix(p, docs+"/"))
	}
	want := []string{"a/y.PDF", "b/alias.pdf", "b/z.pdf", "c/d/e.Pdf", "outside/o.pdf", "x.pdf"}
	if !reflect.DeepEqual(rels, want) {
		t.Errorf("FindPDFs = %q, want %q", rels, want)
	}
	if got := FindPDFs(filepath.Join(dir, "missing")); got != nil {
		t.Errorf("FindPDFs(missing) = %q", got)
	}
}

func fakePdftotext(t *testing.T) (script, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake pdftotext is a shell script")
	}
	dir = t.TempDir()
	return filepath.Join(fakeBin(t, dir, ""), "pdftotext"), dir
}

// test_utf8_output_keeps_the_degree_sign: -enc UTF-8 is passed and the
// UTF-8 output is kept; two pages are counted from the form feed.
func TestExtractPDFTextUTF8KeepsDegreeSign(t *testing.T) {
	script, dir := fakePdftotext(t)
	pdf := filepath.Join(dir, "x.pdf")
	writeFile(t, pdf+".out", []byte("72 °F\fpage 2"))
	text, pages, err := ExtractPDFText(script, pdf, PDFTimeout)
	if err != nil || text != "72 °F\fpage 2" || pages != 2 {
		t.Fatalf("ExtractPDFText = %q, %d, %v", text, pages, err)
	}
	argv := string(readBytes(t, script+".argv"))
	if want := "-layout -enc UTF-8 " + pdf + " -\n"; argv != want {
		t.Errorf("argv %q, want %q", argv, want)
	}
}

// test_invalid_utf8_is_a_failure_not_replacement_chars; the other
// failures extract_text catches are typed too.
func TestExtractPDFTextFailures(t *testing.T) {
	script, dir := fakePdftotext(t)
	pdf := filepath.Join(dir, "x.pdf")
	check := func(what string, prog string, timeout time.Duration, want PDFFailure) {
		t.Helper()
		text, pages, err := ExtractPDFText(prog, pdf, timeout)
		var pe *PDFError
		if !errors.As(err, &pe) || pe.Failure != want || text != "" || pages != 0 {
			t.Errorf("%s: ExtractPDFText = %q, %d, %v; want failure %d", what, text, pages, err, want)
		}
	}
	writeFile(t, pdf+".out", []byte("72 \xb0F"))
	check("latin-1 output", script, PDFTimeout, PDFNotUTF8)
	writeFile(t, pdf+".fail", nil)
	check("exit 1", script, PDFTimeout, PDFExitStatus)
	check("missing program", filepath.Join(dir, "no-such-pdftotext"), PDFTimeout, PDFOSError)
	writeFile(t, pdf+".sleep", nil)
	start := time.Now()
	check("timeout", script, 300*time.Millisecond, PDFTimedOut)
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the timeout took %v", d)
	}
}

// A run is a timeout only when it failed and its context is done: one that
// succeeded as the deadline passed keeps its output.
func TestRunFailure(t *testing.T) {
	expired, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	<-expired.Done()
	exitErr := errors.New("exit status 1")
	for _, c := range []struct {
		ctx      context.Context
		run, err error
	}{
		{expired, nil, nil},
		{expired, exitErr, errTimedOut},
		{context.Background(), exitErr, exitErr},
		{context.Background(), nil, nil},
	} {
		if got := runFailure(c.ctx, c.run); got != c.err {
			t.Errorf("runFailure(ctx err %v, %v) = %v, want %v", c.ctx.Err(), c.run, got, c.err)
		}
	}
}

// Text that is only whitespace has no pages; the page count is the form
// feeds + 1 otherwise.
func TestExtractPDFTextPages(t *testing.T) {
	script, dir := fakePdftotext(t)
	pdf := filepath.Join(dir, "x.pdf")
	for out, want := range map[string]int{"\f": 0, " \n\f\u00a0": 0, "a\f\f": 3, "a": 1} {
		writeFile(t, pdf+".out", []byte(out))
		if _, pages, err := ExtractPDFText(script, pdf, PDFTimeout); err != nil || pages != want {
			t.Errorf("pages of %q = %d, %v; want %d", out, pages, err, want)
		}
	}
}

// makePDF fixtures read through the real pdftotext: the page text (the
// make_pdf case of test_versions_add), the unmapped bullet and the empty
// page.
func TestExtractPDFTextRealPdftotext(t *testing.T) {
	real := needPdftotext(t)
	dir := t.TempDir()
	pdf := filepath.Join(dir, "m.pdf")
	writeFile(t, pdf, makePDF([]string{"Gizmo manual: snarfblatt wiring", "\x80 Disconnect the LAN."}, nil, []string{"72 \xb0F"}))
	text, pages, err := ExtractPDFText(real, pdf, PDFTimeout)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"snarfblatt", "\u2022 Disconnect", "72 °F"} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q lacks %q", text, want)
		}
	}
	// pdftotext ends every page with a form feed, the last one too, so
	// extract_text's "form feeds + 1" counts one page more than the PDF
	// has (its comment says the last page has none): kept for parity.
	if pages != 4 || strings.Contains(text, "\ufffd") {
		t.Errorf("pages %d, text %q", pages, text)
	}
}
