package bognav

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/slotomatic"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Effective slot defaults: what a component's type declares for a slot the bog does not store
// (bognav/defaults.py). A station .bog stores only non-default values, so an absent slot holds
// the default of the type that declares it. This answers it offline from the decompiled module
// sources under organized/ (slotomatic.OrganizedRoot), parsed by slotomatic.ParseSource:
//
//	bog prefix -> module   the bog's m= declarations (c:BooleanWritable is looked up in "control")
//	module -> classes      organized/<module>/<part>/{vineflower|decompiled}/**/*.java, parsed once
//	                       per module part, cached on disk between runs (catalogcache.go)
//	type -> class          B<Type> (the N4 naming rule), else <Type>, inside that module
//	extends walk           resolved by the class's imports / package, else a unique class of that
//	                       simple name among the loaded ones, else the one source file
//	                       organized/*/*/{vineflower|decompiled}/<package path>/<Class>.java names
//
// Values stay the declaration's Java expression, never evaluated.

var decompilers = []string{"vineflower", "decompiled"}

var importRE = slotomatic.Compile(`(?m)^\s*import\s+([\w.]+)\s*;`)

// classRec is one parsed class with the imports and organized-relative source path the walk needs.
type classRec struct {
	slotomatic.Type
	imports []string
	source  string
}

// defaultRec is one declared property of a type's chain.
type defaultRec struct {
	Default, Flags, Owner, Source string
}

// sourceDirs is _source_dirs: one decompiler output dir per module part.
func sourceDirs(moduleDir string) []string {
	parts, err := pycompat.SortedListdir(moduleDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, part := range parts {
		for _, dec := range decompilers {
			if d := slotomatic.Join(slotomatic.Join(moduleDir, part), dec); pycompat.IsDir(d) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

// javaFiles is every .java file under src, in sorted walk order.
func javaFiles(src string) []string {
	var out []string
	pycompat.Walk(src, func(dir string, dirs, files []string) []string {
		for _, fn := range files {
			if strings.HasSuffix(fn, ".java") {
				out = append(out, slotomatic.Join(dir, fn))
			}
		}
		return dirs
	})
	return out
}

// parseFiles is _parse_dir over the .java files of decompiler dir src that javaFiles lists: every
// public class (classes without slots too: their extends link is needed), keyed pkg.Class, in
// sorted walk order (first file wins).
func parseFiles(src string, paths []string) (keys []string, recs map[string]*classRec) {
	recs = map[string]*classRec{}
	base := filepath.Dir(filepath.Dir(filepath.Dir(src)))
	for _, path := range paths {
		text, err := slotomatic.ReadSource(path)
		if err != nil {
			continue
		}
		t := slotomatic.ParseSource(text)
		if t.Class == "" || t.Class != strings.TrimSuffix(filepath.Base(path), ".java") {
			continue // inner/secondary declarations: the file's public class only
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			rel = path
		}
		key := t.Class
		if t.Package != "" {
			key = t.Package + "." + t.Class
		}
		if _, ok := recs[key]; ok {
			continue
		}
		var imports []string
		for _, m := range importRE.FindAllStringSubmatch(text, -1) {
			imports = append(imports, m[1])
		}
		recs[key] = &classRec{Type: t, imports: imports, source: rel}
		keys = append(keys, key)
	}
	return keys, recs
}

// typeCatalog is TypeCatalog: lazily loaded per-module class records with an extends walk.
type typeCatalog struct {
	root     string
	classes  map[string]*classRec
	byModule map[string]map[string]bool
	dirKeys  map[string]map[string]bool
}

var (
	catalogMu sync.Mutex
	catalogs  = map[string]*typeCatalog{}
)

// catalog is the process-wide catalog over slotomatic.OrganizedRoot(), nil without organized/.
func catalog() *typeCatalog {
	root := slotomatic.OrganizedRoot()
	if root == "" {
		return nil
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	c := catalogs[root]
	if c == nil {
		c = &typeCatalog{root: root, classes: map[string]*classRec{}, byModule: map[string]map[string]bool{},
			dirKeys: map[string]map[string]bool{}}
		catalogs[root] = c
	}
	return c
}

func (c *typeCatalog) loadModule(module string) map[string]bool {
	mod := textfold.Lower(module)
	if keys, ok := c.byModule[mod]; ok || mod == "" {
		return keys
	}
	keys := map[string]bool{}
	for _, d := range sourceDirs(slotomatic.Join(c.root, module)) {
		for k := range c.addDir(d) {
			keys[k] = true
		}
	}
	c.byModule[mod] = keys
	return keys
}

func (c *typeCatalog) addDir(d string) map[string]bool {
	if keys, ok := c.dirKeys[d]; ok {
		return keys
	}
	order, recs := cachedParseDir(d)
	keys := map[string]bool{}
	for _, k := range order {
		if _, ok := c.classes[k]; !ok {
			c.classes[k] = recs[k]
		}
		keys[k] = true
	}
	c.dirKeys[d] = keys
	return keys
}

func simpleName(key string) string { return key[strings.LastIndexByte(key, '.')+1:] }

func sortedHits(keys func(func(string)), simple string) []string {
	var hits []string
	keys(func(k string) {
		if simpleName(k) == simple {
			hits = append(hits, k)
		}
	})
	sort.Strings(hits)
	return hits
}

// resolveType is resolve_type: pkg.Class of N4 type module:Type (B<Type> first, then <Type>).
func (c *typeCatalog) resolveType(module, typeName string) string {
	keys := c.loadModule(module)
	for _, simple := range []string{"B" + typeName, typeName} {
		hits := sortedHits(func(f func(string)) {
			for k := range keys {
				f(k)
			}
		}, simple)
		if len(hits) > 0 {
			return hits[0]
		}
	}
	return ""
}

func (c *typeCatalog) parent(key string) string {
	t := c.classes[key]
	if t == nil {
		return ""
	}
	ext, _, _ := strings.Cut(t.Extends, "<")
	if ext == "" {
		return ""
	}
	if strings.Contains(ext, ".") {
		if c.ensure(ext) {
			return ext
		}
		return ""
	}
	for _, imp := range t.imports {
		if simpleName(imp) == ext && c.ensure(imp) {
			return imp
		}
	}
	samePkg := ext
	if t.Package != "" {
		samePkg = t.Package + "." + ext
	}
	if c.ensure(samePkg) {
		return samePkg
	}
	hits := sortedHits(func(f func(string)) {
		for k := range c.classes {
			f(k)
		}
	}, ext)
	if len(hits) == 1 {
		return hits[0]
	}
	return ""
}

// ensure is _ensure: whether pkg.Class is (or can be) loaded; an unloaded module is found by
// the source path its package names.
func (c *typeCatalog) ensure(key string) bool {
	if _, ok := c.classes[key]; ok {
		return true
	}
	rel := strings.ReplaceAll(key, ".", string(filepath.Separator)) + ".java"
	modules, err := pycompat.SortedListdir(c.root)
	if err != nil {
		return false
	}
	for _, m := range modules {
		for _, d := range sourceDirs(slotomatic.Join(c.root, m)) {
			if st, err := os.Stat(slotomatic.Join(d, rel)); err == nil && st.Mode().IsRegular() {
				c.addDir(d)
				_, ok := c.classes[key]
				return ok
			}
		}
	}
	return false
}

// chain is [key, parent, grandparent, ...] as far as the sources resolve (cycle-safe, 40 deep).
func (c *typeCatalog) chain(key string) []string {
	var out []string
	seen := map[string]bool{}
	for key != "" && !seen[key] && len(out) < 40 {
		out = append(out, key)
		seen[key] = true
		key = c.parent(key)
	}
	return out
}

// defaults is TypeCatalog.defaults: the class key ("" when unresolved) and every property the
// type's chain declares, in Python dict order; the most derived declaration of a name wins.
func (c *typeCatalog) defaults(module, typeName string) (string, *bog.Ordered[defaultRec]) {
	props := &bog.Ordered[defaultRec]{}
	key := c.resolveType(module, typeName)
	if key == "" {
		return "", props
	}
	ch := c.chain(key)
	for i := len(ch) - 1; i >= 0; i-- {
		rec := c.classes[ch[i]]
		for _, p := range rec.Properties {
			props.Set(p.Name, defaultRec{Default: p.Default, Flags: p.FlagLetters, Owner: simpleName(ch[i]), Source: rec.source})
		}
	}
	return key, props
}

// componentDefaults is component_defaults: the class key and declared defaults of a component's
// type ("" and none without a catalog or a typed component).
func componentDefaults(b *bog.Bog, comp *bog.Comp) (string, *bog.Ordered[defaultRec]) {
	cat := catalog()
	if cat == nil || !strings.Contains(comp.Type, ":") {
		return "", &bog.Ordered[defaultRec]{}
	}
	module, _ := b.Prefixes.Get(comp.Pfx)
	if module == "" {
		module = comp.Module
	}
	return cat.defaults(module, comp.SimpleType())
}

// defaultLiteral is default_literal: a Java string literal "x" -> x, any other expression as is.
func defaultLiteral(expr string) string {
	if len(expr) >= 2 && expr[0] == '"' && expr[len(expr)-1] == '"' {
		return expr[1 : len(expr)-1]
	}
	return expr
}
