package help

// Class lookups over class-index.json and bajadoc-clean/ (class_search.py):
// class, and the entry model that search and package share.

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
)

// classEntry is one class behind a class-index key (_expand): the primary
// entry or one of its "others", each named by the simple-name key.
type classEntry struct {
	Name, Package, Type, Path string
	Module                    any // printed only when truthy
}

func newClassEntry(key string, o *index.Object) classEntry {
	return classEntry{Name: key, Package: getStr(o, "package", ""), Type: getStr(o, "type", ""),
		Path: getStr(o, "path", ""), Module: get(o, "module", nil)}
}

// expand lists every class behind one key: primary first, then others.
// "others" must be a list of objects; anything else is a malformed index
// (Python raises a TypeError there; nh reports it as an error, exit 2).
func expand(key string, entry *index.Object) ([]classEntry, error) {
	out := []classEntry{newClassEntry(key, entry)}
	raw, present := entry.Get("others")
	if !present {
		return out, nil
	}
	others, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("malformed class index: \"others\" of %s is not a list", key)
	}
	for i, v := range others {
		o, ok := v.(*index.Object)
		if !ok {
			return nil, fmt.Errorf("malformed class index: \"others\"[%d] of %s is not an object", i, key)
		}
		out = append(out, newClassEntry(key, o))
	}
	return out, nil
}

func (c classEntry) qualified() string { return c.Package + "." + c.Name }

// classMatches is class_search.class_matches: every class a simple, nested
// or qualified name refers to, falling back to a case-insensitive simple name.
func classMatches(idx *index.Object, name string) ([]classEntry, error) {
	if idx.Has(name) {
		return expand(name, idx.Object(name))
	}
	parts := strings.Split(name, ".")
	for i := 1; i < len(parts); i++ {
		pkg, simple := strings.Join(parts[:i], "."), strings.Join(parts[i:], ".")
		if !idx.Has(simple) {
			continue
		}
		all, err := expand(simple, idx.Object(simple))
		if err != nil {
			return nil, err
		}
		var hit []classEntry
		for _, e := range all {
			if e.Package == pkg {
				hit = append(hit, e)
			}
		}
		if len(hit) > 0 {
			return hit, nil
		}
	}
	var out []classEntry
	want := lowerStr(name)
	for _, key := range idx.Keys() {
		if lowerStr(key) == want {
			all, err := expand(key, idx.Object(key))
			if err != nil {
				return nil, err
			}
			out = append(out, all...)
		}
	}
	return out, nil
}

// sourceLabel is _source_label: a sibling-tree .bajadoc XML path or the
// javadoc HTML page under bajadoc/.
func sourceLabel(path string) string {
	if rel, ok := strings.CutPrefix(path, "../"); ok {
		return "XML: " + rel
	}
	return "HTML: bajadoc/" + path
}

var typeParams = regexp.MustCompile(`<.*?>`)

// cleanClassName is doc_types.clean_class_name: entities decoded and type
// parameters dropped ("Cursor&lt;E&gt;" -> "Cursor").
func cleanClassName(name string) string {
	return pycompat.Strip(typeParams.ReplaceAllString(pyhtml.Unescape(name), ""))
}

// readCleanClass is class_search.read_clean_class: the bajadoc-clean text
// of a class, ok false when there is none.
func (e *Env) readCleanClass(pkg, name string) (string, bool, error) {
	path := filepath.Join(e.BaseDir, "bajadoc-clean", filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")),
		cleanClassName(name)+".txt")
	return readText(path)
}

func classIndexName(jdk bool) string {
	if jdk {
		return "jdk-class-index.json"
	}
	return "class-index.json"
}

func classScope(jdk bool) string {
	if jdk {
		return "jdk"
	}
	return "classes"
}

// ClassParams are the arguments of `class`.
type ClassParams struct {
	Name    string
	Verbose bool
	JDK     bool // the separate JDK index; members rendered from the .bajadoc XML
}

// Class looks up a class by simple, nested or qualified name (cmd_class):
// every class sharing the name is listed and the first is shown in detail.
func (e *Env) Class(p ClassParams) error {
	idx, err := e.loadIndex(classIndexName(p.JDK), false)
	if err != nil {
		return err
	}
	hits, err := classMatches(idx, p.Name)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		simple := p.Name[strings.LastIndex(p.Name, ".")+1:]
		if close := suggest(idx.Keys(), simple, 5); len(close) > 0 {
			fmt.Fprintf(e.Out, "Did you mean: %s\n", strings.Join(close, ", "))
		}
		return e.reportMiss("class", p.Name, classScope(p.JDK))
	}
	if len(hits) > 1 {
		fmt.Fprintf(e.Out, "%d classes named '%s':\n", len(hits), p.Name)
		for _, h := range hits {
			mod := ""
			if index.Truthy(h.Module) {
				mod = ", " + pyStr(h.Module)
			}
			fmt.Fprintf(e.Out, "  %s (%s%s)\n", h.qualified(), h.Type, mod)
		}
		fmt.Fprintln(e.Out, "Showing the first; pass a qualified name for another.")
		fmt.Fprintln(e.Out)
	}
	h := hits[0]
	fmt.Fprintf(e.Out, "CLASS: %s\nPACKAGE: %s\nTYPE: %s\n", h.Name, h.Package, h.Type)
	if index.Truthy(h.Module) {
		fmt.Fprintf(e.Out, "MODULE: %s\n", pyStr(h.Module))
	}
	fmt.Fprintln(e.Out, sourceLabel(h.Path))
	if p.JDK {
		e.printJDKMembers(h)
		return nil
	}
	clean, _, err := e.readCleanClass(h.Package, h.Name)
	switch {
	case err != nil:
		return err
	case clean != "":
		fmt.Fprintf(e.Out, "\n%s\n", clean)
	case p.Verbose:
		fmt.Fprintln(e.Out, "\n(No bajadoc-clean text found)")
	}
	return nil
}

// printJDKMembers is _print_jdk_members: the members of a JDK class,
// rendered on demand from its .bajadoc file (nothing when the file has no
// class of that name or cannot be parsed).
func (e *Env) printJDKMembers(h classEntry) {
	rel, _ := strings.CutPrefix(h.Path, "../")
	classes, _ := bajadocxml.ParseFile(pycompat.Join(e.BaseDir, rel))
	for _, c := range classes {
		if c.Name == h.Name {
			fmt.Fprintf(e.Out, "\n%s\n", bajadocxml.FormatCleanInfo(bajadocxml.ToCleanInfo(c)))
			return
		}
	}
}
