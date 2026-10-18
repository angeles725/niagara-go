package helpbuild

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

// The port of niagara_help_lib/guide_toc.py: the JavaHelp toc.xml of a
// guide folder as page -> (title, breadcrumb). ElementTree.parse is
// internal/pyxml in ETree mode (expat's verdict and error text, the encoding
// from the declaration).

// TocEntry is one page of a toc.xml: its TOC title and the titles of its
// ancestors, outermost first.
type TocEntry struct {
	Title string
	Trail []string
}

// Toc is parse_toc's dict: Pages keyed by the page path relative to the
// toc's folder, Order their insertion order.
type Toc struct {
	Pages map[string]TocEntry
	Order []string
}

// FindToc is guide_toc.find_toc: the folder's toc.xml in any letter case
// (the first in sorted order that is a file, links followed), "" when there
// is none or the folder cannot be listed.
func FindToc(folder string) string {
	names, err := sortedNames(folder)
	if err != nil {
		return ""
	}
	for _, name := range names {
		if p := pyJoin(folder, name); pycompat.Lower(name) == "toc.xml" && isRegular(p) {
			return p
		}
	}
	return ""
}

// parseTocTree is ET.parse(path).getroot(); the error text is what
// str(ParseError) or str(OSError) prints.
func parseTocTree(path string) (*pyxml.Element, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return nil, errors.New(pycompat.OSErrorText(&fs.PathError{Path: path, Err: pe.Err}))
		}
		return nil, err
	}
	return parseTocXML(data)
}

var tocDeclEncoding = regexp.MustCompile(`^<\?xml[^>]*?encoding[ \t\r\n]*=[ \t\r\n]*["']([^"']*)["']`)

// parseTocXML is ElementTree's parse of data through pyxml, with a
// workaround for a pyxml gap: after a UTF-8 BOM, expat applies the encoding
// the declaration names (a 4.3/4.15 toc.xml is BOM + ISO-8859-1) and counts
// the three BOM bytes as columns of line 1, where pyxml decodes the BOM as
// Latin-1 text and fails. The document after the BOM is parsed instead and a
// line-1 error column moved by 3.
func parseTocXML(data []byte) (*pyxml.Element, error) {
	mode := pyxml.Mode{ETree: true}
	rest, bom := bytes.CutPrefix(data, []byte("\xef\xbb\xbf"))
	m := tocDeclEncoding.FindSubmatch(rest)
	if !bom || m == nil || len(m[1]) == 0 || strings.EqualFold(string(m[1]), "utf-8") {
		return pyxml.Parse(data, mode)
	}
	root, err := pyxml.Parse(rest, mode)
	var xe *pyxml.Error
	if errors.As(err, &xe) {
		if strings.HasPrefix(xe.Msg, "unknown encoding") {
			return pyxml.Parse(data, mode) // not a gap this works around
		}
		if xe.Line == 1 {
			xe.Col += 3
		}
	}
	return root, err
}

// ParseToc is guide_toc.parse_toc. Anchors (#x) are dropped and
// backslashes normalized; the first entry for a page wins. A toc.xml that
// is not well-formed (or cannot be read) yields an empty Toc and the
// warning line Python prints on stderr.
func ParseToc(path string) (toc *Toc, warning string) {
	toc = &Toc{Pages: map[string]TocEntry{}}
	root, err := parseTocTree(path)
	if err != nil {
		return toc, "WARNING: toc not read, page titles fall back to <title>: " + path + ": " + err.Error()
	}
	var walk func(el *pyxml.Element, trail []string)
	walk = func(el *pyxml.Element, trail []string) {
		for _, item := range el.Children {
			if item.Tag != "tocitem" {
				continue
			}
			text, _ := item.Get("text")
			text = strings.Join(pycompat.Fields(text), " ")
			target, _ := item.Get("target")
			target, _, _ = strings.Cut(target, "#")
			target = strings.ReplaceAll(pycompat.Strip(target), `\`, "/")
			if _, seen := toc.Pages[target]; target != "" && text != "" && !seen {
				toc.Pages[target] = TocEntry{Title: text, Trail: append([]string(nil), trail...)}
				toc.Order = append(toc.Order, target)
			}
			sub := trail
			if text != "" {
				sub = append(append([]string(nil), trail...), text)
			}
			walk(item, sub)
		}
	}
	walk(root, nil)
	return toc, ""
}

// CountItems is guide_toc.count_items: the number of <tocitem> elements,
// the root included (Element.iter), 0 when the file is unreadable.
func CountItems(path string) int {
	root, err := parseTocTree(path)
	if err != nil {
		return 0
	}
	var count func(el *pyxml.Element) int
	count = func(el *pyxml.Element) int {
		n := 0
		if el.Tag == "tocitem" {
			n = 1
		}
		for _, c := range el.Children {
			n += count(c)
		}
		return n
	}
	return count(root)
}
