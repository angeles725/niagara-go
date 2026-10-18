package helpbuild

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures of the S3 tests: hand-written PDFs real pdftotext reads, and a
// fake pdftotext that hands every other PDF to the real one.

// makePDF builds a PDF with one page per element of pages, each line of a
// page drawn below the previous one in Helvetica with WinAnsiEncoding
// (byte 0xB0 is the degree sign). Byte 0x80 maps to U+FFFD through the
// font's ToUnicode CMap: the unmapped bullet glyph pdftotext reports as
// U+FFFD in real manuals. A page without lines has no text at all. This is
// test_versions_add.make_pdf grown to several pages and lines.
func makePDF(pages ...[]string) []byte {
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap /CMapName /X def " +
		"1 begincodespacerange <00> <FF> endcodespacerange 1 beginbfchar <80> <FFFD> endbfchar " +
		"endcmap CMapName currentdict /CMap defineresource pop end end"
	stream := func(body string) string {
		return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(body), body)
	}
	esc := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
	n := len(pages)
	var kids []string
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+2*i))
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding /ToUnicode 4 0 R >>",
		stream(cmap),
	}
	for i, lines := range pages {
		var b strings.Builder
		if len(lines) > 0 {
			b.WriteString("BT /F1 12 Tf 72 720 Td")
			for j, l := range lines {
				if j > 0 {
					b.WriteString(" 0 -14 Td")
				}
				fmt.Fprintf(&b, " (%s) Tj", esc.Replace(l))
			}
			b.WriteString(" ET")
		}
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R "+
				"/Resources << /Font << /F1 3 0 R >> >> >>", 6+2*i),
			stream(b.String()))
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	var offs []int
	for i, body := range objs {
		offs = append(offs, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

// realPdftotext is the pdftotext both sides of a parity test run:
// $HELPBUILD_PDFTOTEXT, else the first on PATH; "" when there is none.
func realPdftotext() string {
	if p := os.Getenv("HELPBUILD_PDFTOTEXT"); p != "" {
		return p
	}
	p, err := exec.LookPath("pdftotext")
	if err != nil {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	return abs
}

// needPdftotext skips a test without a real pdftotext.
func needPdftotext(t *testing.T) string {
	t.Helper()
	p := realPdftotext()
	if p == "" {
		t.Skip("pdftotext not on PATH ($HELPBUILD_PDFTOTEXT unset): not checked")
	}
	return p
}

// fakeScript is a fake pdftotext for one PDF argument ($4): "-v" answers;
// <pdf>.sleep execs a long sleep (the timeout case), <pdf>.fail exits 1,
// <pdf>.out prints that file's bytes (any output, also not UTF-8);
// anything else goes to real (exit 9 without one). Every call's arguments
// are appended to <script>.argv.
func fakeScript(real string) string {
	tail := "exit 9\n"
	if real != "" {
		tail = "exec '" + real + "' \"$@\"\n"
	}
	return "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$0.argv\"\n" +
		"[ \"$1\" = -v ] && { echo 'fake pdftotext' >&2; exit 0; }\n" +
		"[ -e \"$4.sleep\" ] && exec sleep 30\n" +
		"[ -e \"$4.fail\" ] && exit 1\n" +
		"[ -e \"$4.out\" ] && exec cat \"$4.out\"\n" + tail
}

// fakeBin writes the fake pdftotext as <dir>/bin/pdftotext and returns its
// folder.
func fakeBin(t *testing.T, dir, real string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh for the fake pdftotext")
	}
	bin := filepath.Join(dir, "bin")
	writeFile(t, filepath.Join(bin, "pdftotext"), []byte(fakeScript(real)))
	if err := os.Chmod(filepath.Join(bin, "pdftotext"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}
