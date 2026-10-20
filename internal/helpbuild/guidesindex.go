package helpbuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The port of tools/build_guides_index.py: indexes/guides-index.json, per
// guides/<Folder>/ its html_count, source_jar, topics (every page, the
// .html/.htm extension dropped), total_topics and, with a toc.xml, toc
// {file, items, titled} and titles {topic: TOC title}.

// TopicName is build_guides_index.topic_name: "sub/Page.html" -> "sub/Page".
func TopicName(rel string) string {
	low := textfold.Lower(rel)
	for _, ext := range []string{".html", ".htm"} {
		if strings.HasSuffix(low, ext) {
			return rel[:len(rel)-len(ext)]
		}
	}
	return rel
}

// fsdecode is os.fsdecode(name) as WTF-8: each byte that is not valid
// UTF-8 becomes the lone surrogate U+DC80+byte, as Python's
// surrogateescape file-system decoding gives it (json.dumps then writes
// "\udcXX"). WTF-8 sorts bytewise in code-point order, like sorted() on
// the decoded names.
func fsdecode(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	var b []byte
	for i := 0; i < len(name); {
		r, size := utf8.DecodeRuneInString(name[i:])
		if r == utf8.RuneError && size == 1 {
			b = pyjson.AppendRune(b, 0xDC00+rune(name[i]))
		} else {
			b = append(b, name[i:i+size]...)
		}
		i += size
	}
	return string(b)
}

// sortedDecoded returns raw names sorted by their decoded form, and that
// form, as sorted(os.listdir()) orders and spells them.
func sortedDecoded(raw []string) (names, decoded []string) {
	names = append([]string(nil), raw...)
	sort.SliceStable(names, func(i, j int) bool { return fsdecode(names[i]) < fsdecode(names[j]) })
	decoded = make([]string, len(names))
	for i, n := range names {
		decoded[i] = fsdecode(n)
	}
	return names, decoded
}

// JarSources are the facts source_jar is decided from: the doc jar names
// of the version manifest (Unknown: there is no manifest, Python's None)
// and the install's modules/ folder ("" when unregistered or not mounted).
type JarSources struct {
	Names      []string
	Unknown    bool
	ModulesDir string
}

// KnownJarNames is build_guides_index.known_jar_names: the sorted doc jar
// base names of versions/<id>.manifest.json; ok is false when baseDir is
// no registered data root or has no manifest. A manifest of another shape
// is an error, as Python raises.
func KnownJarNames(baseDir string) (names []string, ok bool, err error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, false, err
	}
	vid, err := registry.Identify(abs)
	if err != nil || vid == "" {
		return nil, false, err
	}
	for _, repo := range []string{baseDir, filepath.Dir(filepath.Dir(abs))} {
		manifest := pycompat.Join(repo, "versions", vid+".manifest.json")
		if !pycompat.IsFile(manifest) {
			continue
		}
		data, err := pyfs.LoadJSON(manifest)
		if err != nil {
			return nil, false, err
		}
		if data.Kind != pyjson.Object {
			return nil, false, fmt.Errorf("%s: the manifest is not a JSON object", manifest)
		}
		jars, has := data.Get("jars")
		if !has || !jars.Truthy() && (jars.Kind == pyjson.Object || jars.Kind == pyjson.Str) {
			return []string{}, true, nil // nothing to iterate
		}
		if jars.Kind != pyjson.Array {
			return nil, false, fmt.Errorf("%s: \"jars\" is not a list", manifest)
		}
		names = []string{}
		for _, j := range jars.A {
			jar, _ := j.Get("jar")
			if jar.Kind != pyjson.Str {
				return nil, false, fmt.Errorf("%s: a jar record has no \"jar\" string", manifest)
			}
			names = append(names, jar.S[strings.LastIndex(jar.S, "/")+1:])
		}
		sort.Strings(names)
		return names, true, nil
	}
	return nil, false, nil
}

// ModulesDirOf is build_guides_index.modules_dir_of: <install>/modules of
// the version whose data root is baseDir, "" when unregistered or not a
// folder.
func ModulesDirOf(baseDir string) (string, error) {
	install, err := registry.InstallDir(baseDir)
	if err != nil || install == "" {
		return "", err
	}
	if modules := pycompat.Join(install, "modules"); pycompat.IsDir(modules) {
		return modules, nil
	}
	return "", nil
}

// DefaultJarSources are the sources build() reads when it is given none.
func DefaultJarSources(baseDir string) (JarSources, error) {
	names, ok, err := KnownJarNames(baseDir)
	if err != nil {
		return JarSources{}, err
	}
	modules, err := ModulesDirOf(baseDir)
	if err != nil {
		return JarSources{}, err
	}
	return JarSources{Names: names, Unknown: !ok, ModulesDir: modules}, nil
}

// jarForFolder is jar_for_folder: the doc jar whose pages land in
// guides/<folder>/, else the module jar <folder>.jar when modules/ holds
// it; null when the mounted install has none; verified false when it
// cannot be checked.
func jarForFolder(folder, rawFolder string, src JarSources, rules *Rules) (jar pyjson.Value, verified bool) {
	if src.Unknown {
		return pyjson.Value{}, false
	}
	want := "guides/" + folder + "/x.html"
	for _, name := range src.Names {
		if rel, ok := DestPath(name, "doc/x.html", rules); ok && rel == want {
			return pyjson.String(name), true
		}
	}
	if src.ModulesDir == "" {
		return pyjson.Value{}, false
	}
	if pycompat.IsFile(pycompat.Join(src.ModulesDir, rawFolder+".jar")) {
		return pyjson.String(folder + ".jar"), true
	}
	return pyjson.Value{}, true
}

// previousJars is previous_jars: {folder: source_jar} of the index already
// under baseDir; empty when it is absent or unreadable (what Python
// catches: an OSError, a ValueError, a top level that is not a dict).
func previousJars(baseDir string) (map[string]pyjson.Value, error) {
	data, err := pyfs.LoadJSON(pycompat.Join(baseDir, "indexes", "guides-index.json"))
	var le *pyfs.JSONLoadError
	if errors.As(err, &le) {
		return nil, nil
	}
	if err != nil || data.Kind != pyjson.Object {
		return nil, err
	}
	out := map[string]pyjson.Value{}
	for _, k := range data.Keys {
		if v := data.M[k]; v.Kind == pyjson.Object {
			out[k], _ = v.Get("source_jar")
		}
	}
	return out, nil
}

// GuidesIndex is build_guides_index.build(baseDir, jar_names, modules_dir,
// kept): the index and the folders whose source_jar was kept from the
// previous index because it could not be checked (a truthy value only).
// The toc.xml warnings are what parse_toc prints on stderr.
func GuidesIndex(baseDir string, src JarSources) (index pyjson.Value, kept, warnings []string, err error) {
	rules, err := Default()
	if err != nil {
		return index, nil, nil, err
	}
	previous, err := previousJars(baseDir)
	if err != nil {
		return index, nil, nil, err
	}
	guides := pycompat.Join(baseDir, "guides")
	listed, err := sortedNames(guides)
	if err != nil {
		return index, nil, nil, err
	}
	folders, keys := sortedDecoded(listed)
	index = pyjson.NewObject()
	for fi, rawFolder := range folders {
		folder := keys[fi]
		path := pycompat.Join(guides, rawFolder)
		if !pycompat.IsDir(path) {
			continue
		}
		var pages []string
		htmlCount := 0
		var werr error
		pyfs.Walk(path, func(dir string, dirs, files []string) []string {
			for _, f := range files {
				if low := textfold.Lower(f); strings.HasSuffix(low, ".html") || strings.HasSuffix(low, ".htm") {
					htmlCount++
				}
				if IsGuidePage(f) {
					rel, err := filepath.Rel(path, pycompat.Join(dir, f))
					if err != nil {
						werr = err
					}
					pages = append(pages, filepath.ToSlash(rel))
				}
			}
			return dirs
		})
		if werr != nil {
			return index, nil, nil, werr
		}
		_, pages = sortedDecoded(pages)
		topics := make([]pyjson.Value, len(pages))
		for i, p := range pages {
			topics[i] = pyjson.String(TopicName(p))
		}
		jar, verified := jarForFolder(folder, rawFolder, src, rules)
		if !verified {
			jar = previous[folder]
			if jar.Truthy() {
				kept = append(kept, folder)
			}
		}
		entry := pyjson.NewObject()
		entry.Set("html_count", pyjson.IntValue(int64(htmlCount)))
		entry.Set("source_jar", jar)
		entry.Set("topics", pyjson.Value{Kind: pyjson.Array, A: topics})
		entry.Set("total_topics", pyjson.IntValue(int64(len(topics))))
		if tocPath := FindToc(path); tocPath != "" {
			toc, warning := ParseToc(tocPath)
			if warning != "" {
				warnings = append(warnings, warning)
			}
			titles := pyjson.NewObject()
			for _, p := range pages {
				if e, ok := toc.Pages[p]; ok {
					titles.Set(TopicName(p), pyjson.String(e.Title))
				}
			}
			info := pyjson.NewObject()
			info.Set("file", pyjson.String(fsdecode(pyBasename(tocPath))))
			info.Set("items", pyjson.IntValue(int64(CountItems(tocPath))))
			info.Set("titled", pyjson.IntValue(int64(len(titles.Keys))))
			entry.Set("toc", info)
			entry.Set("titles", titles)
		}
		index.Set(folder, entry)
	}
	return index, kept, warnings, nil
}

// BuildGuidesIndex is build_guides_index.main(): baseDir/indexes/
// guides-index.json written with indent=2 from the default jar sources.
func BuildGuidesIndex(baseDir string) (*TextResult, error) {
	src, err := DefaultJarSources(baseDir)
	if err != nil {
		return nil, err
	}
	index, kept, warnings, err := GuidesIndex(baseDir, src)
	if err != nil {
		return nil, err
	}
	out := pycompat.Join(baseDir, "indexes", "guides-index.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return nil, err
	}
	if err := pyfs.WriteJSON(out, index, pyfs.DumpIndent2); err != nil {
		return nil, err
	}
	res := &TextResult{Files: len(index.Keys), Warnings: warnings}
	res.Log = append(res.Log, fmt.Sprintf("guides-index.json: %d folders -> %s", len(index.Keys), out))
	if len(kept) > 0 {
		shown := strings.Join(kept[:min(len(kept), 10)], ", ")
		if len(kept) > 10 {
			shown += " ..."
		}
		res.Log = append(res.Log, fmt.Sprintf("  NOTE: %d source_jar value(s) kept from the previous index, "+
			"not re-verified (install not mounted or no version manifest): %s", len(kept), shown))
	}
	return res, nil
}
