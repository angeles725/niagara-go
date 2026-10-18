package help

// Ground truth of the recall oracle (recall.py ground_truth): every item the
// raw extracted corpus declares, read straight from the .bajadoc XML, the
// javadoc HTML pages and the source, guide and devguide trees, never from
// an index.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
)

// classKey is a (package, simple name) pair; keys sort like Python tuples.
type classKey struct{ pkg, name string }

// memberKey is a (package, class, member) triple.
type memberKey struct{ pkg, class, member string }

func lessClassKey(a, b classKey) bool {
	if a.pkg != b.pkg {
		return a.pkg < b.pkg
	}
	return a.name < b.name
}

func lessMemberKey(a, b memberKey) bool {
	if a.pkg != b.pkg {
		return a.pkg < b.pkg
	}
	if a.class != b.class {
		return a.class < b.class
	}
	return a.member < b.member
}

// recallTruth is the ground_truth dict.
type recallTruth struct {
	xmlFiles  []string           // every .bajadoc under devguide/, guides/, jdk/ (sorted)
	xmlFailed map[string]bool    // the ones that stayed unparseable
	classes   []classKey         // sorted
	xml       []bajadocxml.Class // the Niagara .bajadoc classes, in walk order
	methods   []memberKey        // sorted
	slots     []memberKey        // sorted
	java      []string           // sorted
	guides    []string           // walk order
	devguide  []string           // walk order
	jdk       []bajadocxml.Class
	seconds   float64
}

// relSlash is os.path.relpath(path, base) with "/" separators.
func relSlash(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// walkMatching is recall._walk: the files under root whose name passes pred,
// in sorted walk order (nothing when root is not a directory).
func walkMatching(root string, pred func(name string) bool) []string {
	var out []string
	if !isDir(root) {
		return out
	}
	walkSorted(root, func(path, name string) {
		if pred(name) {
			out = append(out, path)
		}
	})
	return out
}

// javadocNavPages are recall._NAV_PAGES.
var javadocNavPages = map[string]bool{"package-summary.html": true, "package-frame.html": true,
	"package-tree.html": true, "package-use.html": true}

// subTitle is recall._SUBTITLE; [\p{L}\p{N}_] stands for Python's Unicode \w.
var subTitle = regexp.MustCompile(`<div class="subTitle">([\p{L}\p{N}_.]+)</div>`)

// htmlClassPages is recall.html_class_pages: (package, simple) of every
// javadoc HTML class page, the package being the one the page declares.
func htmlClassPages(baseDir string) []classKey {
	root := filepath.Join(baseDir, "bajadoc")
	var out []classKey
	for _, path := range walkMatching(root, func(n string) bool { return strings.HasSuffix(n, ".html") }) {
		parts := strings.Split(relSlash(path, root), "/")
		if len(parts) < 2 || javadocNavPages[parts[len(parts)-1]] || contains(parts, "class-use") ||
			contains(parts, "doc-files") || parts[0] == "index-files" {
			continue
		}
		pkg := ""
		if data, err := os.ReadFile(path); err == nil {
			if m := subTitle.FindSubmatch(data); m != nil {
				pkg = string(m[1])
			}
		}
		if pkg == "" {
			pkg = strings.Join(parts[:len(parts)-1], ".")
		}
		last := parts[len(parts)-1]
		out = append(out, classKey{pkg, last[:len(last)-len(".html")]})
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// guidePageExt is doc_types.GUIDE_PAGE_EXT.
var guidePageExt = []string{".html", ".htm", ".txt", ".md"}

// isLicence is doc_types.is_licence.
func isLicence(name string) bool {
	low := lowerStr(filepath.Base(name))
	for _, p := range []string{"license", "licence", "copying", "notice"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	for _, m := range []string{"-license", "lgpl", "gpl-"} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// isGuidePage is doc_types.is_guide_page.
func isGuidePage(name string) bool {
	low := lowerStr(name)
	for _, ext := range guidePageExt {
		if strings.HasSuffix(low, ext) {
			return !(strings.HasSuffix(low, ".txt") && isLicence(name))
		}
	}
	return false
}

// isDevguidePage is recall.is_devguide_page.
func isDevguidePage(name string) bool {
	low := lowerStr(name)
	if strings.HasSuffix(low, ".html") {
		return true
	}
	return strings.HasSuffix(low, ".txt") && !isLicence(name)
}

// cleanTextName is doc_types.clean_text_name: the guides-clean /
// devguide-clean path of a page path relative to srcDir.
func cleanTextName(rel, srcDir string) string {
	low := lowerStr(rel)
	switch {
	case strings.HasSuffix(low, ".html"):
		return rel[:len(rel)-len(".html")] + ".txt"
	case strings.HasSuffix(low, ".txt"):
		if srcDir != "" && hasHTMLSibling(srcDir, rel[:len(rel)-len(".txt")]) {
			return rel + ".txt"
		}
		return rel
	}
	return rel + ".txt"
}

// hasHTMLSibling is doc_types._has_html_sibling: srcDir holds <stem>.html
// in any letter case.
func hasHTMLSibling(srcDir, stem string) bool {
	folder, base := filepath.Split(filepath.Join(srcDir, stem))
	want := lowerStr(base + ".html")
	entries, err := os.ReadDir(folder)
	if err != nil {
		return false
	}
	for _, d := range entries {
		if lowerStr(d.Name()) == want {
			return true
		}
	}
	return false
}

// groundTruth is recall.ground_truth.
func groundTruth(baseDir string) recallTruth {
	start := time.Now()
	t := recallTruth{xmlFailed: map[string]bool{}}
	failed := func(path string) { t.xmlFailed[relSlash(path, baseDir)] = true }
	for _, d := range []string{"devguide", "guides"} {
		if root := filepath.Join(baseDir, d); isDir(root) {
			t.xml = append(t.xml, iterBajadocClasses(root, failed)...)
		}
	}
	seen := map[classKey]bool{}
	for _, c := range t.xml {
		if k := (classKey{c.Pkg, c.Name}); !seen[k] {
			seen[k] = true
			t.classes = append(t.classes, k)
		}
	}
	for _, k := range htmlClassPages(baseDir) {
		if !seen[k] {
			seen[k] = true
			t.classes = append(t.classes, k)
		}
	}
	sort.Slice(t.classes, func(i, j int) bool { return lessClassKey(t.classes[i], t.classes[j]) })
	methods, slots := map[memberKey]bool{}, map[memberKey]bool{}
	for _, c := range t.xml {
		for _, m := range c.Members {
			k := memberKey{c.Pkg, c.Name, m.Name}
			switch m.Kind {
			case "method":
				methods[k] = true
			case "property", "action", "topic":
				slots[k] = true
			}
		}
	}
	t.methods, t.slots = sortedMembers(methods), sortedMembers(slots)
	for _, d := range []string{"source", "devguide-rc"} {
		for _, p := range walkMatching(filepath.Join(baseDir, d), func(n string) bool { return strings.HasSuffix(n, ".java") }) {
			t.java = append(t.java, relSlash(p, baseDir))
		}
	}
	sort.Strings(t.java)
	groot := filepath.Join(baseDir, "guides")
	for _, p := range walkMatching(groot, isGuidePage) {
		t.guides = append(t.guides, relSlash(p, groot))
	}
	droot := filepath.Join(baseDir, "devguide")
	for _, p := range walkMatching(droot, isDevguidePage) {
		t.devguide = append(t.devguide, relSlash(p, droot))
	}
	if root := filepath.Join(baseDir, "jdk"); isDir(root) {
		t.jdk = iterBajadocClasses(root, failed)
	}
	for _, d := range []string{"devguide", "guides", "jdk"} {
		for _, p := range walkMatching(filepath.Join(baseDir, d), func(n string) bool { return strings.HasSuffix(n, ".bajadoc") }) {
			t.xmlFiles = append(t.xmlFiles, relSlash(p, baseDir))
		}
	}
	sort.Strings(t.xmlFiles)
	t.seconds = time.Since(start).Seconds()
	return t
}

func sortedMembers(set map[memberKey]bool) []memberKey {
	out := make([]memberKey, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return lessMemberKey(out[i], out[j]) })
	return out
}

// walkSorted calls fn for every file under root in os.walk order with
// sorted directories and sorted names (the _walk / iter_classes order).
func walkSorted(root string, fn func(path, name string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var dirs, files []string
	for _, d := range entries {
		p := filepath.Join(root, d.Name())
		if d.IsDir() || (d.Type()&os.ModeSymlink != 0 && isDir(p)) {
			dirs = append(dirs, d.Name())
		} else {
			files = append(files, d.Name())
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	for _, n := range files {
		fn(filepath.Join(root, n), n)
	}
	for _, d := range dirs {
		p := filepath.Join(root, d)
		if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
			continue // os.walk lists a link to a directory but does not enter it
		}
		walkSorted(p, fn)
	}
}

// iterBajadocClasses is iter_classes: every class of the .bajadoc files
// under root (module-index.bajadoc excluded), each with its file. failed
// receives every file that could not be parsed.
func iterBajadocClasses(root string, failed func(path string)) []bajadocxml.Class {
	var out []bajadocxml.Class
	walkSorted(root, func(path, name string) {
		if !strings.HasSuffix(name, ".bajadoc") || name == "module-index.bajadoc" {
			return
		}
		classes, ok := bajadocxml.ParseFile(path)
		if !ok && failed != nil {
			failed(path)
		}
		for _, c := range classes {
			c.File = path
			out = append(out, c)
		}
	})
	return out
}
