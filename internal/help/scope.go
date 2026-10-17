package help

// Honest-zero reporting (scope.py): when a lookup matches nothing it prints
// the scope it searched (version, data root, index entry counts), what that
// scope excludes on purpose and the nearest alternative commands, and counts
// a miss so the exit status becomes 1.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

const docTypesExcluded = "images, fonts/css, JavaHelp .dat search indexes, " +
	"bajadoc.index, raw js next to rendered jsdoc, " +
	"META-INF, licence texts (see coverage --doc-jars)"

type scopeDef struct {
	indexes      []string
	excluded     string
	alternatives []string
}

// scopes are the scope.py SCOPES that search indexes and unshared text
// directories. The scopes over shared text (pdf, fulltext: docs-text and
// course-text may come from another version) need text_sources.shared_root
// and arrive with the commands that report them.
var scopes = map[string]scopeDef{
	"guides": {[]string{"guides-index.json"}, docTypesExcluded,
		[]string{"guide-search", "find", "devguide-search"}},
	"devguide": {[]string{"devguide-toc.json"}, docTypesExcluded,
		[]string{"devguide-search", "find", "guide-search"}},
	"classes": {[]string{"class-index.json"}, "JDK API (opt-in: --jdk); " + docTypesExcluded,
		[]string{"find", "fuzzy", "class --jdk", "source-grep"}},
	"jdk": {[]string{"jdk-class-index.json"}, "Niagara classes (drop --jdk)",
		[]string{"class (without --jdk)", "find"}},
	"methods": {[]string{"method-index.json"}, "JDK members (class --jdk <Name> renders them)",
		[]string{"find", "fuzzy", "source-grep", "method-sig"}},
	"slots": {[]string{"slots-index.json"}, "slots of JDK classes (none exist)",
		[]string{"inherited-slots", "find", "source-grep"}},
	"source": {[]string{"source-index.json"},
		"classes shipped without source (use class/method for their API)",
		[]string{"class", "source-grep", "find"}},
	"inheritance": {[]string{"inheritance.json"}, "JDK types (class --jdk shows their extends)",
		[]string{"class", "find", "fuzzy"}},
	"xref": {[]string{"xref-index.json"}, "classes shipped without source (imports unknown)",
		[]string{"class", "source-grep"}},
	"calls": {[]string{"call-index.json"}, "calls from classes shipped without source",
		[]string{"source-grep", "find"}},
	"modules": {[]string{"source-index.json"},
		"modules shipped without source (module names come from the source files)",
		[]string{"module", "find", "search"}},
}

// scopeDirs are the text directories a scope counts after its indexes.
var scopeDirs = map[string][]string{
	"guides":   {"guides-clean"},
	"devguide": {"devguide-clean", "devguide-examples"},
}

// reportMiss prints the honest-zero block for a lookup that matched nothing
// and counts the miss (scope.report_miss).
func (e *Env) reportMiss(command, query, scope string) error {
	e.Miss()
	def := scopes[scope]
	line, err := e.scopeLine(def, scopeDirs[scope])
	if err != nil {
		return err
	}
	alts := make([]string, len(def.alternatives))
	for i, a := range def.alternatives {
		alts[i] = fmt.Sprintf("%s '%s'", a, query)
	}
	fmt.Fprintf(e.Out, "No match for '%s' (%s) in this scope.\n", query, command)
	fmt.Fprintf(e.Out, "  %s\n", line)
	fmt.Fprintf(e.Out, "  excluded from this scope: %s\n", def.excluded)
	fmt.Fprintf(e.Out, "  not proof of absence; try: %s\n", strings.Join(alts, ", "))
	return nil
}

// scopeLine is scope.scope_line for a scope without shared text.
func (e *Env) scopeLine(def scopeDef, dirs []string) (string, error) {
	vid := identify(e.BaseDir)
	if vid == "" {
		vid = "unregistered"
	}
	parts := make([]string, len(def.indexes))
	for i, name := range def.indexes {
		p, err := e.indexCount(name)
		if err != nil {
			return "", err
		}
		parts[i] = p
	}
	for _, d := range dirs {
		parts = append(parts, dirCount(e.BaseDir, d))
	}
	return fmt.Sprintf("searched: Niagara %s (%s) -- %s", vid, e.BaseDir, strings.Join(parts, "; ")), nil
}

// indexCount is scope._index_count: "<name>: absent", "<name>: N entries",
// or names / classes for the two class indexes.
func (e *Env) indexCount(name string) (string, error) {
	if st, err := os.Stat(index.Path(e.BaseDir, name)); err != nil || !st.Mode().IsRegular() {
		return name + ": absent", nil
	}
	data, err := e.loadIndex(name, false)
	if err != nil {
		return "", err
	}
	if name == "class-index.json" || name == "jdk-class-index.json" {
		total := 0
		for _, k := range data.Keys() {
			total += 1 + listLen(data.Object(k), "others")
		}
		return fmt.Sprintf("%s: %s names / %s classes", name, thousands(data.Len()), thousands(total)), nil
	}
	return fmt.Sprintf("%s: %s entries", name, thousands(data.Len())), nil
}

// dirCount is scope._dir_count for a directory no other version shares:
// "<name>/: absent" or "<name>/: N files" (every file os.walk lists).
func dirCount(baseDir, name string) string {
	path := filepath.Join(baseDir, name)
	if !isDir(path) {
		return name + "/: absent"
	}
	n := 0
	pyWalkFiles(path, func(string) { n++ })
	return fmt.Sprintf("%s/: %s files", name, thousands(n))
}

// identify is versions.identify: the registered version whose data root is
// baseDir, looked up in the registry at baseDir and two levels up
// (versions/<id>/ -> repository root); "" when none matches.
func identify(baseDir string) string {
	target, err := filepath.Abs(baseDir)
	if err != nil {
		return ""
	}
	for _, repo := range []string{target, filepath.Dir(filepath.Dir(target))} {
		reg, err := index.LoadRegistry(repo)
		if err != nil || reg == nil {
			continue // Python would raise on a malformed registry; resolution already read it
		}
		for _, vid := range reg.Versions.Keys() {
			if root, err := index.DataRoot(repo, vid); err == nil && root == target {
				return vid
			}
		}
	}
	return ""
}
