package help

// Class-index browsing (class_search.py): search by glob and package.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"
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
	qualifiedMatch := func(c classEntry) bool { return fnmatch.Fnmatch(c.qualified(), p.Pattern) }
	tries := []func(key string) bool{
		func(k string) bool { return fnmatch.Fnmatch(k, p.Pattern) },
		func(k string) bool { return lowerGlob(k, p.Pattern) },
		func(k string) bool {
			all, err := expand(k, idx.Object(k))
			if err != nil && expandErr == nil {
				expandErr = err
			}
			for _, c := range all {
				if qualifiedMatch(c) {
					return true
				}
			}
			return false
		},
	}
	var matches []string
	keep := func(classEntry) bool { return true } // a simple-name match lists every class of the name
	for i, match := range tries {
		if i == len(tries)-1 {
			keep = qualifiedMatch // a package.name match lists only the classes it matched
		}
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
			if keep(c) {
				rows = append(rows, fmt.Sprintf("  %s (%s) - %s\n", name, c.Type, c.Package))
			}
		}
	}
	// The header counts the rows printed: a name held by several packages is one
	// row per class, and then the name count follows.
	count := fmt.Sprintf("%d classes", len(rows))
	if len(rows) != len(matches) {
		names := "names"
		if len(matches) == 1 {
			names = "name"
		}
		count += fmt.Sprintf(" (%d %s)", len(matches), names)
	}
	fmt.Fprintf(e.Out, "%s matching '%s':\n\n%s", count, fts.Echo(p.Pattern), strings.Join(rows, ""))
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
		if desc := packageDescription(e.BaseDir, pkg); desc != nil {
			for _, l := range desc {
				fmt.Fprintln(e.Out, strings.TrimRight("  "+l, " "))
			}
			fmt.Fprintf(e.Out, "  (package-text/%s.txt)\n\n", pkg)
		}
		for _, c := range classes {
			fmt.Fprintf(e.Out, "  %s (%s)\n", c.name, c.typ)
		}
		fmt.Fprintln(e.Out)
	}
	return nil
}

// packageDescription is the description block of package-text/<pkg>.txt
// under baseDir (the page helpbuild's package-text builder writes), the
// lines after its header; nil when the package has no description page.
func packageDescription(baseDir, pkg string) []string {
	text, err := pycompat.ReadTextReplace(pycompat.Join(baseDir, "package-text", pkg+".txt"))
	if err != nil {
		return nil
	}
	_, body, _ := strings.Cut(text, "\n\n")
	return strings.Split(strings.TrimRight(body, "\n"), "\n")
}
