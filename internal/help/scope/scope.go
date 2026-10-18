// Package scope is what nh prints when a lookup cannot give an answer, as a
// library (decision D29): it prints nothing, returns the exact lines and needs
// no nh Env, so another binary (nsearch) produces nh's text without exec'ing it.
//
//   - Miss is the honest-zero block of a lookup that matched nothing
//     (scope.report_miss): the scope it searched, what that scope excludes on
//     purpose and the nearest alternatives. nh exits 1 after it.
//   - Resolve and VersionLines are the version selection (app.py
//     resolve_base_dir) and its failure text on stdout. nh exits 2 after it.
//   - ErrorLine is the stderr line of a lookup that cannot search, such as a
//     full-text index that cannot be built (*UnwritableIndexError). nh exits 2.
//   - OpenFulltext rebuilds a missing or stale full-text index before a
//     search, printing Python's lines (fulltext.go, D38).
package scope

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

const docTypesExcluded = "images, fonts/css, JavaHelp .dat search indexes, " +
	"bajadoc.index, raw js next to rendered jsdoc, " +
	"META-INF, licence texts (see coverage --doc-jars)"

type def struct {
	indexes      []string
	excluded     string
	alternatives []string
}

// scopes are the scope.py SCOPES: the indexes each one searches, what it
// excludes on purpose and the nearest alternative commands.
var scopes = map[string]def{
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
	"modules": {[]string{"source-index.json"},
		"modules shipped without source (module names come from the source files)",
		[]string{"module", "find", "search"}},
	"pdf": {[]string{"pdf-index.json"}, "manual pages without a text layer (scanned images)",
		[]string{"find", "guide-search", "pdf --no-fulltext"}},
	"fulltext": {nil, "JDK API (class --jdk); " + docTypesExcluded,
		[]string{"find --or", "fuzzy", "class --jdk", "source-grep"}},
}

// dirs are the text directories a scope counts after its indexes.
var dirs = map[string][]string{
	"guides":   {"guides-clean"},
	"devguide": {"devguide-clean", "devguide-examples"},
	"pdf":      {"docs-text"},
	"fulltext": fts.Labels,
}

// Loader returns indexes/<name> of the data root being described; Miss and
// Line call it only for an index file that exists. nil reads it from disk;
// nh passes its per-run cache so an index is parsed once.
type Loader func(name string) (*index.Object, error)

// UnknownScopeError is a scope name scope.py SCOPES does not define.
type UnknownScopeError string

func (e UnknownScopeError) Error() string { return "unknown search scope '" + string(e) + "'" }

// Miss is scope.report_miss without the printing: the four lines of the
// honest-zero block for command run on query in the named scope of the data
// root baseDir ("fulltext" for find).
func Miss(baseDir, command, query, scope string, load Loader) ([]string, error) {
	d, ok := scopes[scope]
	if !ok {
		return nil, UnknownScopeError(scope)
	}
	line, err := Line(baseDir, scope, load)
	if err != nil {
		return nil, err
	}
	alts := make([]string, len(d.alternatives))
	for i, a := range d.alternatives {
		alts[i] = fmt.Sprintf("%s '%s'", a, query)
	}
	return []string{
		fmt.Sprintf("No match for '%s' (%s) in this scope.", query, command),
		"  " + line,
		"  excluded from this scope: " + d.excluded,
		"  not proof of absence; try: " + strings.Join(alts, ", "),
	}, nil
}

// Line is scope.scope_line: "searched: Niagara <id> (<root>) -- <counts>",
// the index entry counts then the text directory file counts of the scope.
func Line(baseDir, scope string, load Loader) (string, error) {
	d, ok := scopes[scope]
	if !ok {
		return "", UnknownScopeError(scope)
	}
	if load == nil {
		load = func(name string) (*index.Object, error) { return index.LoadObject(index.Path(baseDir, name)) }
	}
	vid := Identify(baseDir)
	if vid == "" {
		vid = "unregistered"
	}
	parts := make([]string, len(d.indexes))
	for i, name := range d.indexes {
		p, err := indexCount(baseDir, name, load)
		if err != nil {
			return "", err
		}
		parts[i] = p
	}
	if ds := dirs[scope]; len(ds) > 0 {
		sources := fts.SourceDirs(baseDir)
		for _, name := range ds {
			parts = append(parts, dirCount(baseDir, sources, name))
		}
	}
	return fmt.Sprintf("searched: Niagara %s (%s) -- %s", vid, baseDir, strings.Join(parts, "; ")), nil
}

// indexCount is scope._index_count: "<name>: absent", "<name>: N entries",
// or names / classes for the two class indexes. pdf-index.json is counted in
// the data root that owns the (maybe shared) manuals, read from disk.
func indexCount(baseDir, name string, load Loader) (string, error) {
	if name == "pdf-index.json" {
		if root, _ := fts.DocsTextRoot(baseDir); root != baseDir {
			baseDir = root
			load = func(name string) (*index.Object, error) { return index.LoadObject(index.Path(root, name)) }
		}
	}
	if st, err := os.Stat(index.Path(baseDir, name)); err != nil || !st.Mode().IsRegular() {
		return name + ": absent", nil
	}
	data, err := load(name)
	if err != nil {
		return "", err
	}
	if name == "class-index.json" || name == "jdk-class-index.json" {
		total := 0
		for _, k := range data.Keys() {
			others, _ := getAny(data.Object(k), "others").([]any)
			total += 1 + len(others)
		}
		return fmt.Sprintf("%s: %s names / %s classes", name, pycompat.Commas(data.Len()), pycompat.Commas(total)), nil
	}
	return fmt.Sprintf("%s: %s entries", name, pycompat.Commas(data.Len())), nil
}

// dirCount is scope._dir_count: "<name>/: absent" or "<name>/: N files"
// (every file os.walk lists) in the folder text_sources resolves, which for
// docs-text / course-text may belong to another version, named after the count.
func dirCount(baseDir string, sources []fts.SourceDir, name string) string {
	path := fts.DirOf(sources, name)
	if path == "" {
		path = filepath.Join(baseDir, name)
	}
	if !pycompat.IsDir(path) {
		return name + "/: absent"
	}
	shared := ""
	if _, ok := fts.Shared[name]; ok {
		if _, owner := fts.SharedRoot(baseDir, name); owner != "" {
			shared = " (shared from Niagara " + owner + ")"
		}
	}
	return fmt.Sprintf("%s/: %s files%s", name, pycompat.Commas(fts.CountFiles(path)), shared)
}

// Identify is versions.identify: the registered version whose data root is
// baseDir, looked up in the registry at baseDir and two levels up
// (versions/<id>/ -> repository root); "" when none matches.
func Identify(baseDir string) string {
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

// getAny is o.get(k); a nil object is an empty dict.
func getAny(o *index.Object, k string) any {
	if o == nil {
		return nil
	}
	v, _ := o.Get(k)
	return v
}
