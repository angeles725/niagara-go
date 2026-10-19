package help

// Class-index browsing (class_search.py): search by glob and package.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// lowerGlob is fnmatch(name.lower(), pat.lower()).
func lowerGlob(name, pat string) bool {
	return fnmatch.Fnmatch(textfold.Lower(name), textfold.Lower(pat))
}

// SearchParams are the arguments of `search`.
type SearchParams struct {
	Pattern string
	JDK     bool
}

// Search lists the classes whose simple name matches a glob, then retries
// case-insensitively, then against package.name (cmd_search).
func (e *Env) Search(p SearchParams) error {
	idx, err := e.loadIndex(classIndexName(p.JDK), false)
	if err != nil {
		return err
	}
	var expandErr error
	tries := []func(key string) bool{
		func(k string) bool { return fnmatch.Fnmatch(k, p.Pattern) },
		func(k string) bool { return lowerGlob(k, p.Pattern) },
		func(k string) bool {
			all, err := expand(k, idx.Object(k))
			if err != nil && expandErr == nil {
				expandErr = err
			}
			for _, c := range all {
				if fnmatch.Fnmatch(c.qualified(), p.Pattern) {
					return true
				}
			}
			return false
		},
	}
	var matches []string
	for _, match := range tries {
		for _, k := range idx.Keys() {
			if match(k) {
				matches = append(matches, k)
			}
		}
		if expandErr != nil {
			return expandErr
		}
		if len(matches) > 0 {
			break
		}
	}
	if len(matches) == 0 {
		return e.reportMiss("search", p.Pattern, classScope(p.JDK))
	}
	sort.Strings(matches)
	var rows []string
	for _, name := range matches {
		all, err := expand(name, idx.Object(name))
		if err != nil {
			return err
		}
		for _, c := range all {
			rows = append(rows, fmt.Sprintf("  %s (%s) - %s\n", name, c.Type, c.Package))
		}
	}
	fmt.Fprintf(e.Out, "%d classes matching '%s':\n\n%s", len(matches), p.Pattern, strings.Join(rows, ""))
	return nil
}

// PackageParams are the arguments of `package`.
type PackageParams struct {
	Name string
	All  bool // also implied by an empty Name
}

type pkgClass struct{ name, typ string }

// Package lists all packages, or the classes of the packages matching Name
// exactly (case-insensitive) or else as a substring (cmd_package).
func (e *Env) Package(p PackageParams) error {
	idx, err := e.loadIndex("class-index.json", false)
	if err != nil {
		return err
	}
	var order []string // packages in first-seen order (dict insertion)
	packages := map[string][]pkgClass{}
	for _, key := range idx.Keys() {
		all, err := expand(key, idx.Object(key))
		if err != nil {
			return err
		}
		for _, c := range all {
			if _, seen := packages[c.Package]; !seen {
				order = append(order, c.Package)
			}
			packages[c.Package] = append(packages[c.Package], pkgClass{key, c.Type})
		}
	}
	if p.All || p.Name == "" {
		sorted := append([]string(nil), order...)
		sort.Strings(sorted)
		fmt.Fprintf(e.Out, "%d packages:\n\n", len(sorted))
		for _, pkg := range sorted {
			fmt.Fprintf(e.Out, "  %s (%d classes)\n", pkg, len(packages[pkg]))
		}
		return nil
	}
	want := textfold.Lower(p.Name)
	var matched []string
	for _, pkg := range order {
		if textfold.Lower(pkg) == want {
			matched = append(matched, pkg)
		}
	}
	if len(matched) == 0 {
		for _, pkg := range order {
			if strings.Contains(textfold.Lower(pkg), want) {
				matched = append(matched, pkg)
			}
		}
		sort.Strings(matched)
	}
	if len(matched) == 0 {
		return e.reportMiss("package", p.Name, "classes")
	}
	for _, pkg := range matched {
		classes := append([]pkgClass(nil), packages[pkg]...)
		sort.SliceStable(classes, func(i, j int) bool { return classes[i].name < classes[j].name })
		fmt.Fprintf(e.Out, "%s (%d classes):\n\n", pkg, len(classes))
		for _, c := range classes {
			fmt.Fprintf(e.Out, "  %s (%s)\n", c.name, c.typ)
		}
		fmt.Fprintln(e.Out)
	}
	return nil
}
