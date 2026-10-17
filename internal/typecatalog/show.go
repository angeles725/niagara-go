package typecatalog

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/slotomatic"
	"github.com/angeles725/niagara-go/internal/textfold"
)

var partSuffixes = []string{"", "-rt", "-wb", "-ux", "-se", "-doc"}

var sepRE = regexp.MustCompile(`[\\/]`)

// layoutModule is _layout_module: the N4 module named by an
// organized/<module>/<module>[-rt|-wb|-ux|-se|-doc]/ source path (lower-cased), or "" and false.
func layoutModule(source string) (string, bool) {
	var segs []string
	for _, s := range sepRE.Split(source, -1) {
		if s != "" {
			segs = append(segs, textfold.Lower(s))
		}
	}
	for i := 0; i+1 < len(segs); i++ {
		for _, suf := range partSuffixes {
			if segs[i+1] == segs[i]+suf {
				return segs[i], true
			}
		}
	}
	return "", false
}

// inModule is _in_module: whether catalog entry key, built from source, belongs to N4 module
// module (ignoring case). The organized layout of the source path decides alone when present;
// otherwise a package segment other than the first and the class equals the module.
func inModule(key, source, module string) bool {
	mod := textfold.Lower(module)
	if layout, ok := layoutModule(source); ok {
		return layout == mod
	}
	segs := strings.Split(key, ".")
	if len(segs) < 2 {
		return false
	}
	for _, s := range segs[1 : len(segs)-1] {
		if textfold.Lower(s) == mod {
			return true
		}
	}
	return false
}

func lastSegment(key string) string { return key[strings.LastIndexByte(key, '.')+1:] }

// source is the entry's source path for module scoping.
func (c *Catalog) source(key string) (string, error) {
	if r := c.m[key]; r != nil {
		return r.Source, nil
	}
	return sourceOf(key, c.raw[key])
}

// view is what show prints of an entry.
func (c *Catalog) view(key string) (entryView, error) {
	r := c.m[key]
	if r == nil {
		return viewLoaded(key, c.raw[key])
	}
	ev := entryView{extends: r.Extends, source: r.Source}
	for g, ds := range [3][]slotomatic.Decl{r.Properties, r.Actions, r.Topics} {
		for _, d := range ds {
			ev.groups[g] = append(ev.groups[g], declView{d.Name, d.Flags.String(), d.FlagLetters, d.Default, d.Facets})
		}
	}
	return ev, nil
}

// show is _show: print every type matching pkg.BClass, BClass, module:Type or Type; exit 1 on a
// miss, 2 on an entry Python could not print (intended difference: Python raised a traceback).
func show(out, errw io.Writer, cat *Catalog, path, name string) int {
	malformed := func(err error) int {
		fmt.Fprintf(errw, "%s: malformed catalog %s: %v\n", tool, path, err)
		return cli.ExitCannotSearch
	}
	module, simple := "", name
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		module, simple = name[:i], name[i+1:]
	}
	wanted := map[string]bool{name: true, simple: true, "B" + simple: true}
	var hits []string
	for _, k := range cat.Keys {
		if wanted[k] || wanted[lastSegment(k)] || strings.HasSuffix(k, "."+name) {
			hits = append(hits, k)
		}
	}
	if module != "" {
		var inside, outside []string
		for _, k := range hits {
			src, err := cat.source(k)
			if err != nil {
				return malformed(err)
			}
			if inModule(k, src, module) {
				inside = append(inside, k)
			} else {
				outside = append(outside, k)
			}
		}
		hits = inside
		if len(hits) == 0 && len(outside) > 0 {
			sort.Strings(outside)
			fmt.Fprintf(errw, "no type matching %s; %d type(s) named %s exist outside module '%s': %s\n",
				name, len(outside), simple, module, strings.Join(outside, ", "))
			return 1
		}
	}
	if len(hits) == 0 {
		tried := make([]string, 0, len(wanted))
		for w := range wanted {
			tried = append(tried, w)
		}
		sort.Strings(tried)
		fmt.Fprintf(errw, "no type matching %s (tried %s); searched %d type(s) in the catalog\n",
			name, strings.Join(tried, ", "), cat.Len())
		return 1
	}
	views := make([]entryView, len(hits))
	for i, k := range hits {
		v, err := cat.view(k)
		if err != nil {
			return malformed(err)
		}
		views[i] = v
	}
	labels := [3]string{"property", "action", "topic"}
	for i, k := range hits {
		fmt.Fprintf(out, "%s  extends %s   (%s)\n", k, views[i].extends, views[i].source)
		for g, decls := range views[i].groups {
			for _, d := range decls {
				fmt.Fprintf(out, "  %s %s flags=%s %s default=%s facets=%s\n", ljust(labels[g], 8),
					ljust(d.name, 24), ljust(d.flags, 6), ljust(d.letters, 8), d.def, d.facets)
			}
		}
	}
	return 0
}

type declView struct{ name, flags, letters, def, facets string }

// ljust is Python's "%-Ns": s padded with spaces to n code points.
func ljust(s string, n int) string {
	if k := utf8.RuneCountInString(s); k < n {
		return s + strings.Repeat(" ", n-k)
	}
	return s
}
