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
// excludes on purpose and the nearest alternative commands. An alternative is
// always a command nh still carries (TestMissHintsAreLiveCommands): the
// retired fuzzy, method-sig, inherited-slots and module exit 2.
var scopes = map[string]def{
	"guides": {[]string{"guides-index.json"}, docTypesExcluded,
		[]string{"guide-search", "find", "devguide-search"}},
	"devguide": {[]string{"devguide-toc.json"}, docTypesExcluded,
		[]string{"devguide-search", "find", "guide-search"}},
	"classes": {[]string{"class-index.json"}, "JDK API (opt-in: --jdk); " + docTypesExcluded,
		[]string{"find", "class --jdk", "source-grep"}},
	"class": {[]string{"class-index.json", "bytecode-index.json"}, "JDK API (opt-in: --jdk); " + docTypesExcluded,
		[]string{"find", "class --jdk", "source-grep"}},
	"bytecode": {[]string{"bytecode-index.json"}, "classes no module.xml registers as a type (only registered " +
		"classes are read); a class is matched by qualified or simple name, exactly",
		[]string{"class (without --bytecode)", "source-grep", "find"}},
	"jdk": {[]string{"jdk-class-index.json"}, "Niagara classes (drop --jdk)",
		[]string{"class (without --jdk)", "find"}},
	"methods": {[]string{"method-index.json"}, "JDK members (class --jdk <Name> renders them)",
		[]string{"find", "source-grep"}},
	"slots": {[]string{"slots-index.json"}, "slots of JDK classes (none exist)",
		[]string{"slots --inherited", "find", "source-grep"}},
	"source": {[]string{"source-index.json"},
		"classes not in nh's source index (docSource-doc.jar and the vendor .java of module jars are indexed, third-party packages excluded; use class/method for their API)",
		[]string{"class", "source-grep", "find"}},
	"inheritance": {[]string{"inheritance.json"}, "JDK types (class --jdk shows their extends)",
		[]string{"class", "find"}},
	"xref": {[]string{"xref-index.json"}, "classes not in nh's source index (docSource-doc.jar and the vendor .java of module jars are indexed, third-party packages excluded; imports unknown)",
		[]string{"class", "source-grep"}},
	"modules": {[]string{"source-index.json"},
		"modules not in nh's source index (docSource-doc.jar and the vendor .java of module jars are indexed, third-party packages excluded; module names come from it)",
		[]string{"package", "find", "search"}},
	"module-xml": {[]string{"module-index.json"}, "jars without META-INF/module.xml; module names are matched " +
		"exactly, in any letter case, by part name or as a glob",
		[]string{"type", "search", "find"}},
	"types": {[]string{"module-index.json"}, "types of jars without META-INF/module.xml; a type is matched as " +
		"module:Type or Type, exactly, in any letter case or as a glob",
		[]string{"class", "search", "find"}},
	"palettes": {[]string{"palette-index.json"}, "modules whose jars ship no *.palette; a palette entry is " +
		"matched by Type, module:Type, entry name or folder, exactly, in any letter case or as a glob, and by the types it holds",
		[]string{"type", "module", "find"}},
	"tags": {[]string{"tag-index.json"}, "tag dictionaries outside haystack*.jar and tagdictionary*.jar (a station's own " +
		"dictionaries are not shipped); a name is matched as dict:name or name, exactly, in any letter case or as a glob",
		[]string{"find", "source-grep", "type"}},
	"lexicons": {[]string{"lexicon-index.json"}, "lexicon files outside the module jars (a station's own lexicons " +
		"are not shipped); a key is matched exactly, in any letter case or as a glob, within --module and --lang (default all); " +
		"--text needs every word in a value (base files unless --lang)",
		[]string{"lexicon --text", "find", "source-grep"}},
	"pdf": {[]string{"pdf-index.json"}, "manual pages without a text layer (scanned images)",
		[]string{"find", "guide-search", "pdf --no-fulltext"}},
	"fulltext": {nil, "JDK API (class --jdk); " + docTypesExcluded,
		[]string{"find --or", "class --jdk", "source-grep"}},
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
	query = fts.Echo(query)
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
	if name == "module-index.json" {
		return moduleCount(data), nil
	}
	if name == "palette-index.json" {
		return paletteCount(data), nil
	}
	if name == "tag-index.json" {
		return tagCount(data), nil
	}
	if name == "lexicon-index.json" {
		return lexiconCount(data), nil
	}
	if name == "bytecode-index.json" {
		classes, _ := getAny(data, "classes").([]any)
		return fmt.Sprintf("bytecode-index.json: %s classes", pycompat.Commas(len(classes))) + unreadable(data), nil
	}
	return fmt.Sprintf("%s: %s entries", name, pycompat.Commas(data.Len())), nil
}

// moduleCount is the module-index.json part of a scope line: its modules and
// the jars whose module.xml could not be read (the first three named).
func moduleCount(data *index.Object) string {
	n := 0
	if mods, ok := getAny(data, "modules").(*index.Object); ok {
		n = mods.Len()
	}
	line := fmt.Sprintf("module-index.json: %s modules", pycompat.Commas(n))
	errs, _ := getAny(data, "errors").([]any)
	if len(errs) == 0 {
		return line
	}
	var jars []string
	for _, e := range errs[:min(3, len(errs))] {
		jar, _ := getAny(asObject(e), "jar").(string)
		jars = append(jars, jar)
	}
	if len(errs) > 3 {
		jars = append(jars, "...")
	}
	return fmt.Sprintf("%s (%d module.xml unreadable: %s)", line, len(errs), strings.Join(jars, ", "))
}

// paletteCount is the palette-index.json part of a scope line: its palettes
// and those it could not read.
func paletteCount(data *index.Object) string {
	pals, _ := getAny(data, "palettes").([]any)
	return fmt.Sprintf("palette-index.json: %s palettes", pycompat.Commas(len(pals))) + unreadable(data)
}

// tagCount is the tag-index.json part of a scope line: its distinct names
// (tags, tag groups, relations), the files read and those it could not read.
func tagCount(data *index.Object) string {
	names := map[string]bool{}
	for _, sec := range []string{"tags", "groups", "relations"} {
		rows, _ := getAny(data, sec).([]any)
		for _, r := range rows {
			d, _ := getAny(asObject(r), "dict").(string)
			n, _ := getAny(asObject(r), "name").(string)
			names[sec+"\x00"+d+":"+n] = true
		}
	}
	files, _ := getAny(data, "files").([]any)
	return fmt.Sprintf("tag-index.json: %s names from %s files", pycompat.Commas(len(names)), pycompat.Commas(len(files))) + unreadable(data)
}

// lexiconCount is the lexicon-index.json part of a scope line: its files
// (base and translations), their entries and the files it could not read.
func lexiconCount(data *index.Object) string {
	files, _ := getAny(data, "files").([]any)
	base, entries := 0, 0
	for _, f := range files {
		if lang, _ := getAny(asObject(f), "language").(string); lang == "" {
			base++
		}
		e, _ := getAny(asObject(f), "entries").([]any)
		entries += len(e)
	}
	return fmt.Sprintf("lexicon-index.json: %s lexicon files (%s base, %s translations), %s entries", pycompat.Commas(len(files)),
		pycompat.Commas(base), pycompat.Commas(len(files)-base), pycompat.Commas(entries)) + unreadable(data)
}

// unreadable is " (N unreadable: jar!path, ...)" for the errors rows of an
// index built from the install (the first three named), "" when none.
func unreadable(data *index.Object) string {
	errs, _ := getAny(data, "errors").([]any)
	if len(errs) == 0 {
		return ""
	}
	var where []string
	for _, e := range errs[:min(3, len(errs))] {
		jar, _ := getAny(asObject(e), "jar").(string)
		if path, _ := getAny(asObject(e), "path").(string); path != "" {
			jar += "!" + path
		}
		where = append(where, jar)
	}
	if len(errs) > 3 {
		where = append(where, "...")
	}
	return fmt.Sprintf(" (%d unreadable: %s)", len(errs), strings.Join(where, ", "))
}

func asObject(v any) *index.Object { o, _ := v.(*index.Object); return o }

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
