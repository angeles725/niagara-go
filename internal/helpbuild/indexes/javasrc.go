package indexes

import (
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The Java source helpers source_indexer.py exports and xref_builder.py and
// slots_indexer.py import: java_package, java_declaration, iter_java_roots,
// and the text of a .java file as open(errors="replace") reads it.

// rcDir is RC_DIR: the docDeveloper example sources, indexed as one more
// module whose paths are relative to source/ ("../devguide-rc/...").
const rcDir = "devguide-rc"

// SourceOriginsFile is source/<SourceOriginsFile>: the Java sources the
// extraction took from module jars (not docSource), as a JSON object of
// path under source/ -> jar name. The builders walk only the folders of
// source/, so this file is never read as a module.
const SourceOriginsFile = "module-jar-sources.json"

// javaPackageRe is _PACKAGE: only the name is matched, as the ";" of a
// package statement may sit on the next line.
var javaPackageRe = pyre.MustCompile(`package\s+([\w.]+)`, 0)

// javaDeclarationRe is _DECLARATION: any modifiers, a type keyword, then a
// name followed by the end of the line or what may follow a type name.
var javaDeclarationRe = pyre.MustCompile(`(?:(?:public|protected|private|abstract|final|static|strictfp|sealed|`+
	`non-sealed)\s+)*(?:class|interface|enum|record|@\s*interface)\s+(\w+)`+
	`(?=\s*(?:$|[{<(]|//|/\*|extends\b|implements\b|permits\b))`, 0)

// javaPackage is java_package: the package a line declares, ok false for
// Python's None (a line not starting with "package ", or no name after it).
func javaPackage(line string) (string, bool) {
	if !strings.HasPrefix(line, "package ") {
		return "", false
	}
	m := javaPackageRe.MatchRunes([]rune(line), 0)
	if m == nil {
		return "", false
	}
	return m.Group(1), true
}

// javaDeclaration is java_declaration: the type name a stripped line
// declares, "" for None. A line starting like a comment never declares.
func javaDeclaration(line string) string {
	if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "//") {
		return ""
	}
	// the keyword must come first (after modifiers): skip the regex for
	// the many lines that cannot match
	if line == "" || !strings.ContainsAny(line[:1], "pacfisner@") {
		return ""
	}
	m := javaDeclarationRe.MatchRunes([]rune(line), 0)
	if m == nil {
		return ""
	}
	return m.Group(1)
}

// javaRoot is one (module, dir) pair of iter_java_roots.
type javaRoot struct{ module, dir string }

// iterJavaRoots is iter_java_roots: every folder under source/ (sorted,
// links followed) then devguide-rc/. A root without source/ gets the
// warning line Python prints on stderr; a source/ that cannot be listed
// is the OSError Python raises.
func iterJavaRoots(baseDir string, res *IndexResult) ([]javaRoot, error) {
	var roots []javaRoot
	sourceDir := pycompat.Join(baseDir, "source")
	if !pycompat.IsDir(sourceDir) {
		res.Stderr = append(res.Stderr, "WARNING: "+baseDir+" has no source/ directory; indexing devguide-rc/ only")
	} else {
		names, err := pyListdir(sourceDir)
		if err != nil {
			return nil, err
		}
		sort.Strings(names)
		for _, name := range names {
			if path := pycompat.Join(sourceDir, name); pycompat.IsDir(path) {
				roots = append(roots, javaRoot{name, path})
			}
		}
	}
	if rc := pycompat.Join(baseDir, rcDir); pycompat.IsDir(rc) {
		roots = append(roots, javaRoot{rcDir, rc})
	}
	return roots, nil
}

// pyListdir is os.listdir: the names, an OSError naming the path when the
// folder cannot be listed.
func pyListdir(dir string) ([]string, error) {
	names, err := pycompat.Listdir(dir)
	if err != nil {
		return nil, pyOSError(dir, err)
	}
	return names, nil
}

// readJavaText is open(path, encoding="utf-8", errors="replace").read():
// ok false where Python's open or read raises the OSError the builders
// catch.
func readJavaText(path string) (string, bool) {
	text, err := pycompat.ReadTextReplace(path)
	return text, err == nil
}

// javaFileLines is iterating such a file: lines cut after each "\n"
// (universal newlines already applied), ends kept.
func javaFileLines(text string, visit func(line string) bool) {
	for text != "" {
		i := strings.IndexByte(text, '\n') + 1
		if i == 0 {
			i = len(text)
		}
		if !visit(text[:i]) {
			return
		}
		text = text[i:]
	}
}
