package lookup

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// cmdPackage is cmd_package: the top-level classes of one package, the
// sub-packages of a partial name, or every package with --all.
func cmdPackage(env *registry.Env, name string, showAll bool, limit int) {
	data := env.LoadClassIndex()
	if data == nil || data.Len() == 0 {
		return
	}
	w := env.Out
	classes := data.Object("classes")
	if showAll {
		var order []string
		counts := map[string]int64{}
		for _, v := range classes.Values() {
			for _, e := range core.Objects(v) {
				if core.Truthy(e, "outer_class") {
					continue
				}
				pkg := core.Str(e, "package")
				if _, ok := counts[pkg]; !ok {
					order = append(order, pkg)
				}
				counts[pkg]++
			}
		}
		sort.Strings(order)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  All packages: %d unique\n", len(order))
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %-55s %6s\n", "PACKAGE", "COUNT")
		fmt.Fprintln(w, "  "+strings.Repeat("-", 63))
		var total int64
		for _, pkg := range order {
			fmt.Fprintf(w, "  %s %s\n", core.LJust(pkg, 55), core.RJust(core.Commas(counts[pkg]), 6))
			total += counts[pkg]
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  Total: %d packages, %s top-level classes\n", len(order), core.Commas(total))
		fmt.Fprintln(w)
		return
	}
	if name == "" {
		fmt.Fprintln(w, "  Usage: package <name> or package --all")
		return
	}
	var matches []match
	for i, cname := range classes.Keys() {
		for _, e := range core.Objects(classes.Values()[i]) {
			if core.Str(e, "package") == name && !core.Truthy(e, "outer_class") {
				matches = append(matches, match{cname, e})
			}
		}
	}
	if len(matches) == 0 {
		subPackages(env, data, name)
		return
	}
	sort.SliceStable(matches, func(a, b int) bool { return matches[a].name < matches[b].name })
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Package %s: %d classes\n", name, len(matches))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %35s  %-9s %6s  %-20s %s\n", "CLASS", "KIND", "LINES", "MODULE", "INNER")
	fmt.Fprintln(w, "  "+strings.Repeat("-", 80))
	for shown, m := range matches {
		if shown >= limit {
			break
		}
		inner := ""
		if n := len(core.List(m.e, "inner_classes")); n > 0 {
			inner = strconv.Itoa(n)
		}
		fmt.Fprintf(w, "  %s  %s %s  %s %s\n", core.RJust(m.name, 35), core.LJust(core.Str(m.e, "kind"), 9),
			core.RJust(core.Commas(core.Int(m.e, "lines")), 6), core.LJust(core.Str(m.e, "module"), 20), inner)
	}
	if len(matches) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %d more (use -n to show more)\n", len(matches)-limit)
	}
	fmt.Fprintln(w)
}

// subPackages lists the packages starting with name (no exact package), or
// reports the miss.
func subPackages(env *registry.Env, data *index.Object, name string) {
	classes := data.Object("classes")
	partial := map[string]bool{}
	for _, v := range classes.Values() {
		for _, e := range core.Objects(v) {
			if core.Truthy(e, "package") && strings.HasPrefix(core.Str(e, "package"), name) && !core.Truthy(e, "outer_class") {
				partial[core.Str(e, "package")] = true
			}
		}
	}
	if len(partial) == 0 {
		env.Miss(fmt.Sprintf("package '%s'", name), core.ClassScope(data), "")
		return
	}
	names := make([]string, 0, len(partial))
	for p := range partial {
		names = append(names, p)
	}
	sort.Strings(names)
	w := env.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  No exact match for '%s'. Sub-packages:\n", name)
	for _, p := range names[:min(30, len(names))] {
		fmt.Fprintf(w, "    %s\n", p)
	}
	if len(names) > 30 {
		fmt.Fprintf(w, "    ... and %d more\n", len(names)-30)
	}
	fmt.Fprintln(w)
}
