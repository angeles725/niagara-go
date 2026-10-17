package lookup

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The class lookups of class_search.py: class, search and package.

func runClass(env *registry.Env, ns *argparse.Namespace) int {
	cmdClass(env, ns.String("name"))
	return env.ExitCode()
}

func runSearch(env *registry.Env, ns *argparse.Namespace) int {
	cmdSearch(env, ns.String("pattern"), ns.Int("n"))
	return env.ExitCode()
}

func runPackage(env *registry.Env, ns *argparse.Namespace) int {
	cmdPackage(env, ns.String("name"), ns.Bool("show_all"), ns.Int("n"))
	return env.ExitCode()
}

func printClassDetail(w io.Writer, name string, e *index.Object) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Class:       %s\n", name)
	fmt.Fprintf(w, "  Package:     %s\n", core.Str(e, "package"))
	fmt.Fprintf(w, "  Module:      %s\n", core.Str(e, "module"))
	fmt.Fprintf(w, "  Kind:        %s\n", core.Str(e, "kind"))
	mods := "-"
	if core.Truthy(e, "modifiers") {
		mods = strings.Join(core.Strs(e, "modifiers"), " ")
	}
	fmt.Fprintf(w, "  Modifiers:   %s\n", mods)
	if core.Truthy(e, "extends") {
		fmt.Fprintf(w, "  Extends:     %s\n", core.Str(e, "extends"))
	}
	if core.Truthy(e, "implements") {
		fmt.Fprintf(w, "  Implements:  %s\n", strings.Join(core.Strs(e, "implements"), ", "))
	}
	fmt.Fprintf(w, "  Lines:       %s\n", core.Commas(core.Int(e, "lines")))
	fmt.Fprintf(w, "  Path:        %s\n", core.Str(e, "path"))
	fmt.Fprintf(w, "  ZKM:         %s\n", yesNo(core.Truthy(e, "zkm"), "YES", "no"))
	if core.Truthy(e, "outer_class") {
		fmt.Fprintf(w, "  Outer class: %s\n", core.Str(e, "outer_class"))
	}
	if core.Truthy(e, "inner_classes") {
		inner := core.Strs(e, "inner_classes")
		fmt.Fprintf(w, "  Inner classes (%d):\n", len(inner))
		for _, ic := range inner {
			fmt.Fprintf(w, "    %s\n", ic)
		}
	}
	fmt.Fprintln(w)
}

func yesNo(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// formatClassRow is _format_class_row: one summary line for a class.
func formatClassRow(name string, e *index.Object) string {
	extra := ""
	if core.Truthy(e, "outer_class") {
		extra = "in:" + core.Str(e, "outer_class")
	} else if core.Truthy(e, "zkm") {
		extra = "ZKM"
	}
	lines := "-"
	if n := core.Int(e, "lines"); n > 0 {
		lines = core.Commas(n)
	}
	pkg := core.Str(e, "package")
	if extra != "" {
		pkg += "  " + extra
	}
	return fmt.Sprintf("  %s  %s %s  %s %s", core.RJust(core.Cut(name, 35), 35), core.LJust(core.Str(e, "kind"), 9),
		core.RJust(lines, 6), core.LJust(core.Cut(core.Str(e, "module"), 20), 20), pkg)
}

// qualifiedMatches is _qualified_matches: the entries for "pkg.Name" or
// "pkg.Outer.Inner".
func qualifiedMatches(classes *index.Object, name string) (string, []*index.Object) {
	prefix, simple := "", name
	if i := strings.LastIndex(name, "."); i >= 0 {
		prefix, simple = name[:i], name[i+1:]
	}
	var out []*index.Object
	for _, e := range core.Entries(classes, simple) {
		pkg := core.Str(e, "package")
		if prefix == pkg {
			out = append(out, e)
			continue
		}
		if core.Truthy(e, "outer_class") {
			outer := core.Str(e, "outer_class")
			if prefix == outer || prefix == pkg+"."+outer {
				out = append(out, e)
			}
		}
	}
	return simple, out
}

// cmdClass is cmd_class: a class by simple name, qualified name or any case.
func cmdClass(env *registry.Env, name string) {
	data := env.LoadClassIndex()
	if data == nil || data.Len() == 0 {
		return
	}
	w := env.Out
	classes := data.Object("classes")
	entries, found := []*index.Object(nil), classes.Has(name)
	if found {
		entries = core.Entries(classes, name)
	} else if strings.Contains(name, ".") {
		if simple, qm := qualifiedMatches(classes, name); len(qm) > 0 {
			name, entries, found = simple, qm, true
		}
	}
	if found {
		if len(entries) == 1 {
			printClassDetail(w, name, entries[0])
			return
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %d entries for '%s':\n", len(entries), name)
		fmt.Fprintln(w)
		for i, e := range entries {
			fqn := name
			if core.Truthy(e, "package") {
				fqn = core.Str(e, "package") + "." + name
			}
			fmt.Fprintf(w, "  [%d] %s\n", i+1, fqn)
			fmt.Fprintf(w, "      Module: %s  Kind: %s  Lines: %s\n",
				core.Str(e, "module"), core.Str(e, "kind"), core.Commas(core.Int(e, "lines")))
			if core.Truthy(e, "extends") {
				fmt.Fprintf(w, "      Extends: %s\n", core.Str(e, "extends"))
			}
			if core.Truthy(e, "outer_class") {
				fmt.Fprintf(w, "      Inner class of: %s\n", core.Str(e, "outer_class"))
			}
		}
		fmt.Fprintln(w)
		return
	}
	lower := textfold.Lower(name)
	var ci []string
	for _, k := range classes.Keys() {
		if textfold.Lower(k) == lower {
			ci = append(ci, k)
		}
	}
	if len(ci) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  No exact match for '%s'. Case-insensitive matches:\n", name)
		fmt.Fprintln(w)
		for _, k := range ci {
			for _, e := range core.Entries(classes, k) {
				fmt.Fprintf(w, "    %s (%s, %s)\n", k, core.Str(e, "package"), core.Str(e, "module"))
			}
		}
		fmt.Fprintln(w)
		return
	}
	simple := name[strings.LastIndex(name, ".")+1:]
	env.Miss(fmt.Sprintf("class '%s'", name), core.ClassScope(data), fmt.Sprintf("search '*%s*'", simple))
}

// match is one (class name, entry) pair of a listing.
type match struct {
	name string
	e    *index.Object
}

// cmdSearch is cmd_search: classes whose lower-cased name matches the
// lower-cased glob pattern.
func cmdSearch(env *registry.Env, pattern string, limit int) {
	data := env.LoadClassIndex()
	if data == nil || data.Len() == 0 {
		return
	}
	w := env.Out
	classes := data.Object("classes")
	pat := textfold.Lower(pattern)
	var matches []match
	for i, name := range classes.Keys() {
		if fnmatch.FnmatchCase(textfold.Lower(name), pat) {
			for _, e := range core.Objects(classes.Values()[i]) {
				matches = append(matches, match{name, e})
			}
		}
	}
	if len(matches) == 0 {
		env.Miss(fmt.Sprintf("pattern '%s'", pattern), core.ClassScope(data), "")
		return
	}
	sort.SliceStable(matches, func(a, b int) bool {
		la, lb := textfold.Lower(matches[a].name), textfold.Lower(matches[b].name)
		if la != lb {
			return la < lb
		}
		return core.Str(matches[a].e, "module") < core.Str(matches[b].e, "module")
	})
	kinds := map[string]int{}
	for _, m := range matches {
		kinds[core.Str(m.e, "kind")]++
	}
	kindNames := make([]string, 0, len(kinds))
	for k := range kinds {
		kindNames = append(kindNames, k)
	}
	sort.Strings(kindNames)
	parts := make([]string, len(kindNames))
	for i, k := range kindNames {
		parts[i] = fmt.Sprintf("%s=%d", k, kinds[k])
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Search '%s': %d results\n", pattern, len(matches))
	if len(parts) > 0 {
		fmt.Fprintf(w, "  (%s)\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %35s  %-9s %6s  %-20s %s\n", "CLASS", "KIND", "LINES", "MODULE", "PACKAGE")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 100))
	for shown, m := range matches {
		if shown >= limit {
			break
		}
		fmt.Fprintln(w, formatClassRow(m.name, m.e))
	}
	if len(matches) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %d more (use -n to show more)\n", len(matches)-limit)
	}
	fmt.Fprintln(w)
}
