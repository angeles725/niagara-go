package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/helpbuild/htmltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The ports of tools/guides_parser.py (guides/ -> guides-clean/) and
// tools/devguide_parser.py (devguide/ -> devguide-clean/). Each page
// becomes one text file with a TITLE / [TOC / PAGE TITLE] / SOURCE header;
// the clean tree is rebuilt from scratch in a sibling directory swapped in
// at the end (atomicfile.RebuiltDir). Nothing is printed: Log holds the lines Python
// prints on stdout, Warnings the ones it prints on stderr.

// TextResult is what a text builder reports.
type TextResult struct {
	Files    int      // the builder's return value: pages written
	Log      []string // stdout lines
	Warnings []string // stderr lines
}

// readPage is open(path, encoding="utf-8", errors="replace").read().
func readPage(path string) (string, error) { return pycompat.ReadTextReplace(path) }

// cleanFileText is the header plus the text and a final newline when the
// text is not blank (a page with no text keeps its header-only entry).
func cleanFileText(header, text string) string {
	if pycompat.Strip(text) != "" {
		return header + text + "\n"
	}
	return header
}

// errNotUTF8Name is the UnicodeEncodeError write_text raises for a header
// naming a page whose path is not valid UTF-8 (os.walk decodes it with
// surrogateescape): the whole build fails and the old clean tree is kept.
func errNotUTF8Name(path string) error {
	return fmt.Errorf("%s: the page path is not valid UTF-8 (Python's write_text raises UnicodeEncodeError)", path)
}

// megabytes is "{:.1f}".format(chars / 1024 / 1024).
func megabytes(chars int) string { return fmt.Sprintf("%.1f", float64(chars)/1024/1024) }

// writeClean writes one clean file, its folders made like os.makedirs.
func writeClean(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return atomicfile.WriteText(path, text)
}

// mdClosing is the closing-sequence regex of _markdown_title.
var mdClosing = regexp.MustCompile(`(?:^|[` + pycompat.SpaceClass() + `]+)#+[` + pycompat.SpaceClass() + `]*$`)

// markdownTitle is guides_parser._markdown_title: the first non-empty
// level-1 ATX heading of a Markdown page without its optional closing
// sequence; "" (None) when there is none.
func markdownTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		rest, ok := strings.CutPrefix(line, "# ")
		if !ok {
			continue
		}
		if title := pycompat.Strip(mdClosing.ReplaceAllString(pycompat.Strip(rest), "")); title != "" {
			return title
		}
	}
	return ""
}

// ParseGuides is guides_parser.parse_guides(baseDir).
func ParseGuides(baseDir string) (*TextResult, error) {
	res := &TextResult{}
	guidesDir := pycompat.Join(baseDir, "guides")
	err := atomicfile.RebuiltDir(pycompat.Join(baseDir, "guides-clean"), func(out string) error {
		return parseGuidesInto(res, guidesDir, out)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func parseGuidesInto(res *TextResult, guidesDir, outDir string) error {
	folders, err := sortedNames(guidesDir)
	if err != nil {
		return err
	}
	chars, errs, titled := 0, 0, 0
	var werr error
	for _, folder := range folders {
		folderPath := pycompat.Join(guidesDir, folder)
		if !pycompat.IsDir(folderPath) {
			continue
		}
		toc := &Toc{}
		if tocPath := FindToc(folderPath); tocPath != "" {
			var warning string
			if toc, warning = ParseToc(tocPath); warning != "" {
				res.Warnings = append(res.Warnings, warning)
			}
		}
		pyfs.Walk(folderPath, func(dir string, dirs, files []string) []string {
			for _, fname := range files {
				if werr != nil || !IsGuidePage(fname) {
					continue
				}
				pagePath := pycompat.Join(dir, fname)
				rel, err := filepath.Rel(guidesDir, pagePath)
				if err != nil {
					werr = err
					continue
				}
				rel = filepath.ToSlash(rel)
				raw, err := readPage(pagePath)
				if err != nil {
					errs++
					continue
				}
				if !utf8.ValidString(rel) {
					werr = errNotUTF8Name(pagePath)
					continue
				}
				title, text := guidePageText(fname, raw)
				_, inFolder, _ := strings.Cut(rel, "/")
				var header string
				if e, ok := toc.Pages[inFolder]; ok {
					crumbs := append(append([]string(nil), e.Trail...), e.Title)
					header = "TITLE: " + e.Title + "\nTOC: " + strings.Join(crumbs, " > ") + "\n"
					if title != "" && title != e.Title {
						header += "PAGE TITLE: " + title + "\n"
					}
					titled++
				} else {
					header = "TITLE: " + title + "\n"
				}
				header += "SOURCE: guides/" + rel + "\n\n"
				if werr = writeClean(pycompat.Join(outDir, CleanTextName(rel, guidesDir)), cleanFileText(header, text)); werr != nil {
					continue
				}
				res.Files++
				chars += utf8.RuneCountInString(text)
			}
			return dirs
		})
		if werr != nil {
			return werr
		}
	}
	res.Log = append(res.Log, fmt.Sprintf("Extracted %d guide files to guides-clean/ (%s MB text, %d errors, %d titled from toc.xml)",
		res.Files, megabytes(chars), errs, titled))
	return nil
}

// guidePageText is the (title, text) of one guide page: an HTML page
// through HTMLToText, a plain-text page as it is (a Markdown page titled by
// its first heading), else titled by its file name.
func guidePageText(fname, raw string) (title, text string) {
	low := textfold.Lower(fname)
	if strings.HasSuffix(low, ".html") || strings.HasSuffix(low, ".htm") {
		return htmltext.HTMLToText(raw)
	}
	text = pycompat.Strip(strings.ReplaceAll(raw, "\r\n", "\n"))
	if strings.HasSuffix(low, ".md") {
		title = markdownTitle(text)
	}
	if title == "" {
		title = fname
	}
	return title, text
}

// ParseDevguide is devguide_parser.parse_devguide(baseDir): every .html
// page and every .txt page that is not a licence text; the licence texts
// are listed as excluded.
func ParseDevguide(baseDir string) (*TextResult, error) {
	res := &TextResult{}
	devguideDir := pycompat.Join(baseDir, "devguide")
	err := atomicfile.RebuiltDir(pycompat.Join(baseDir, "devguide-clean"), func(out string) error {
		return parseDevguideInto(res, baseDir, devguideDir, out)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func parseDevguideInto(res *TextResult, baseDir, devguideDir, outDir string) error {
	chars := 0
	var excluded []string
	var werr error
	pyfs.Walk(devguideDir, func(dir string, dirs, files []string) []string {
		for _, fname := range files {
			if werr != nil {
				break
			}
			low := textfold.Lower(fname)
			isHTML := strings.HasSuffix(low, ".html")
			if !isHTML {
				isTxt := strings.HasSuffix(low, ".txt")
				if isTxt && IsLicence(fname) {
					rel, err := filepath.Rel(baseDir, pycompat.Join(dir, fname))
					if werr = err; err != nil {
						break
					}
					excluded = append(excluded, rel)
				}
				if !isTxt || IsLicence(fname) {
					continue
				}
			}
			werr = devguidePage(res, &chars, baseDir, devguideDir, outDir, dir, fname, isHTML)
		}
		return dirs
	})
	if werr != nil {
		return werr
	}
	for _, rel := range excluded {
		res.Log = append(res.Log, "  excluded (licence text): "+rel)
	}
	res.Log = append(res.Log, fmt.Sprintf("Extracted %d devguide files to devguide-clean/ (%s MB text)", res.Files, megabytes(chars)))
	return nil
}

func devguidePage(res *TextResult, chars *int, baseDir, devguideDir, outDir, dir, fname string, isHTML bool) error {
	pagePath := pycompat.Join(dir, fname)
	relDir, err := filepath.Rel(devguideDir, dir)
	if err != nil {
		return err
	}
	raw, err := readPage(pagePath)
	if err != nil {
		return nil // skipped silently, like Python's bare continue
	}
	var title, text string
	if isHTML {
		title, text = htmltext.HTMLToText(raw)
	} else {
		title, text = fname, pycompat.Strip(strings.ReplaceAll(raw, "\r\n", "\n"))
	}
	outSub := outDir
	if relDir != "." {
		outSub = pycompat.Join(outDir, relDir)
	}
	relSource, err := filepath.Rel(baseDir, pagePath)
	if err != nil {
		return err
	}
	if !utf8.ValidString(relSource) {
		return errNotUTF8Name(pagePath)
	}
	header := "TITLE: " + title + "\nSOURCE: " + strings.ReplaceAll(relSource, `\`, "/") + "\n\n"
	if err := writeClean(pycompat.Join(outSub, CleanTextName(fname, dir)), cleanFileText(header, text)); err != nil {
		return err
	}
	res.Files++
	*chars += utf8.RuneCountInString(text)
	return nil
}
