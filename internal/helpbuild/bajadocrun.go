package helpbuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
)

// The run half of the bajadoc_parser.py port: main() (walk, parse, write,
// report) and the bajadoc_xml.write_clean_class it calls (format_output is
// internal/bajadocxml's).
// bajadoc-clean/ is written in place, page by page with atomic writes (it
// also holds the classes bajadoc_xml_parser.py renders from .bajadoc XML),
// never rebuilt as a whole.

// BajadocResult is what bajadoc_parser.main() reports.
type BajadocResult struct {
	Parsed, Skipped, Errors int
	Log                     []string // stdout lines; print("\nDone ...") is two
	Status                  int      // exit status: 1 when bajadoc/javax/baja is missing
}

func (r *BajadocResult) out(format string, args ...any) {
	r.Log = append(r.Log, fmt.Sprintf(format, args...))
}

// ErrUnsafeClassName is returned when a class name would put its page
// outside bajadoc-clean/ (an absolute name or ".." segments).
var ErrUnsafeClassName = errors.New("class name could leave bajadoc-clean")

// bdIndexPages are the javadoc index pages main() never reads.
var bdIndexPages = map[string]bool{"package-summary.html": true, "package-tree.html": true,
	"package-use.html": true, "package-frame.html": true}

// bajadocPages is main()'s file list: every *.html under dir (os.walk, links
// to folders not entered) but the index pages and the files of a folder
// whose path holds "<sep>class-use<sep>" (so class-use/ itself is read, only
// its subfolders are skipped), sorted.
func bajadocPages(dir string) []string {
	sep := string(filepath.Separator)
	var files []string
	pyWalk(dir, func(root string, dirs, names []string) []string {
		for _, f := range names {
			if !strings.HasSuffix(f, ".html") || bdIndexPages[f] {
				continue
			}
			if strings.Contains(root, sep+"class-use"+sep) || strings.Contains(root, "/class-use/") {
				continue
			}
			files = append(files, pycompat.Join(root, f))
		}
		return dirs
	})
	sort.Strings(files) // byte order is code-point order for UTF-8 paths
	return files
}

// ParseBajadoc is bajadoc_parser.main() with baseDir as its argument (a
// version data root; the script's default is the niagara-help folder):
// bajadoc/javax/baja/**.html -> bajadoc-clean/<package path>/<Class>.txt.
// A page that cannot be read is counted and reported, as Python catches
// it; err != nil is where Python raises (a failed write: traceback, exit
// 1), with the lines printed before it in the result.
func ParseBajadoc(baseDir string) (*BajadocResult, error) {
	res := &BajadocResult{}
	bajadocDir := pycompat.Join(baseDir, "bajadoc", "javax", "baja")
	outputDir := pycompat.Join(baseDir, "bajadoc-clean")
	if !pycompat.IsDir(bajadocDir) {
		res.out("ERROR: bajadoc directory not found: %s", bajadocDir)
		res.Status = 1
		return res, nil
	}
	files := bajadocPages(bajadocDir)
	res.out("Found %d HTML files to parse", len(files))
	t0 := time.Now()
	for _, path := range files {
		page, err := readPage(path)
		if err != nil {
			res.Errors++
			res.out("  ERROR parsing %s: %s", path, pycompat.OSErrorText(err))
			continue
		}
		c := parseBajadocPage(page)
		if c == nil {
			res.Skipped++
			continue
		}
		if err := writeCleanClass(outputDir, c.Package, c.Name, formatBajadoc(c)); err != nil {
			return res, err
		}
		res.Parsed++
	}
	res.Log = append(res.Log, "")
	res.out("Done in %.1fs", time.Since(t0).Seconds())
	res.out("  Parsed: %d", res.Parsed)
	res.out("  Skipped: %d", res.Skipped)
	res.out("  Errors: %d", res.Errors)
	res.out("  Output: %s", outputDir)
	return res, nil
}

var bdTypeParams = regexp.MustCompile(`<.*?>`)

// cleanClassName is doc_types.clean_class_name: entities decoded, type
// parameters dropped ("Cursor&lt;E&gt;" -> "Cursor"). RE2's "." skips "\n"
// as Python's does.
func cleanClassName(name string) string {
	return pycompat.Strip(bdTypeParams.ReplaceAllString(pyhtml.Unescape(name), ""))
}

// writeCleanClass is bajadoc_xml.write_clean_class: the page goes to
// <package path>/<clean name>.txt and a page left under the escaped name
// ("Cursor&lt;E&gt;.txt") is removed.
func writeCleanClass(outDir, pkg, className, text string) error {
	pkgDir := pycompat.Join(outDir, strings.ReplaceAll(pkg, ".", string(filepath.Separator)))
	path := pycompat.Join(pkgDir, cleanClassName(className)+".txt")
	legacy := pycompat.Join(pkgDir, className+".txt")
	for _, p := range []string{path, legacy} {
		if rel, err := filepath.Rel(filepath.Clean(outDir), filepath.Clean(p)); err != nil ||
			rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("%w: %q", ErrUnsafeClassName, className)
		}
	}
	if err := atomicfile.WriteText(path, text); err != nil {
		return err
	}
	if _, err := os.Stat(legacy); legacy != path && err == nil {
		return removeFile(legacy)
	}
	return nil
}

// formatBajadoc is bajadoc_xml.format_output on the class of a javadoc
// page (bajadocxml.FormatCleanInfo, the port S7b and nh's class --jdk use).
func formatBajadoc(c *BajadocClass) string {
	return bajadocxml.FormatCleanInfo(bajadocxml.CleanInfo{ClassName: c.Name, Kind: c.Kind, Pkg: c.Package,
		Since: c.Since, Extends: c.Extends, Desc: c.Description, Implements: c.Implements,
		Subclasses: c.Subclasses, Fields: c.Fields, Ctors: c.Constructors, Methods: c.Methods})
}
