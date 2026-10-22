package indexes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The port of tools/bajadoc_xml_parser.py (S7b): every class described by
// a .bajadoc file under devguide/ or guides/ gets its bajadoc-clean/ page,
// unless a javadoc HTML page owns it (S7a's ParseBajadoc writes those).
// The class model and the page text are internal/bajadocxml's (the reader
// nh's class --jdk and selftest --recall share); this file is the walk,
// the skip rules, the SUBCLASSES derivation and the writes.

// bajadocXMLDirs is XML_DIRS, in walk order.
var bajadocXMLDirs = []string{"devguide", "guides"}

// BajadocXMLResult is what bajadoc_xml_parser.main() reports.
type BajadocXMLResult struct {
	Classes int      // pages written
	Log     []string // stdout lines (the one summary line)
}

// ParseBajadocXML is bajadoc_xml_parser.main() with baseDir as its argument
// (a version data root; the script's default is the niagara-help folder).
// err != nil is where Python raises, with no line printed: a .bajadoc file
// that cannot be read stops the walk before any page is written (pages are
// written after the whole walk), a failed write leaves the pages before it.
func ParseBajadocXML(baseDir string) (*BajadocXMLResult, error) {
	res := &BajadocXMLResult{}
	outDir := pycompat.Join(baseDir, "bajadoc-clean")
	var infos []bajadocxml.CleanInfo
	seen := map[[2]string]bool{}
	for _, d := range bajadocXMLDirs {
		root := pycompat.Join(baseDir, d)
		if !pycompat.IsDir(root) {
			continue
		}
		classes, err := bajadocXMLClasses(root)
		if err != nil {
			return res, err
		}
		for _, c := range classes {
			q := [2]string{c.Pkg, c.Name}
			if seen[q] || hasHTMLPage(baseDir, c.Pkg, c.Name) {
				continue
			}
			seen[q] = true
			infos = append(infos, bajadocxml.ToCleanInfo(c))
		}
	}
	subs := map[string][]string{}
	for _, info := range infos {
		if info.Extends != "" {
			subs[info.Extends] = append(subs[info.Extends], info.ClassName)
		}
	}
	for _, info := range infos {
		info.Subclasses = pycompat.SortedSet(subs[info.ClassName])
		if err := writeCleanClass(outDir, info.Pkg, info.ClassName, bajadocxml.FormatCleanInfo(info)); err != nil {
			return res, err
		}
	}
	res.Classes = len(infos)
	res.Log = append(res.Log, fmt.Sprintf("bajadoc-clean/ from .bajadoc XML: %d classes "+
		"(javadoc HTML pages keep their own text)", len(infos)))
	return res, nil
}

// bajadocXMLClasses is bajadoc_xml.iter_classes(root): the classes of every
// *.bajadoc file but module-index.bajadoc, folders and files in sorted
// order (os.walk, links to folders not entered), each carrying its File. A
// file that does not parse even sanitized adds nothing; one that cannot be
// read is the error ET.parse raises.
func bajadocXMLClasses(root string) ([]bajadocxml.Class, error) {
	var out []bajadocxml.Class
	var firstErr error
	pyfs.Walk(root, func(dir string, dirs, files []string) []string {
		if firstErr != nil {
			return nil
		}
		sort.Strings(dirs)
		sort.Strings(files)
		for _, name := range files {
			if !strings.HasSuffix(name, ".bajadoc") || name == "module-index.bajadoc" {
				continue
			}
			path := pycompat.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				firstErr = err
				return nil
			}
			classes, _ := bajadocxml.Parse(data)
			for _, c := range classes {
				c.File = path
				out = append(out, c)
			}
		}
		return dirs
	})
	return out, firstErr
}

// hasHTMLPage is has_html_page: bajadoc/<package path>/<Name>.html is a
// file (os.path.isfile, links followed).
func hasHTMLPage(baseDir, pkg, name string) bool {
	return pycompat.IsFile(pycompat.Join(baseDir, "bajadoc",
		strings.ReplaceAll(pkg, ".", string(filepath.Separator)), name+".html"))
}
