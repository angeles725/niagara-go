package license

import (
	"fmt"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The queries and row formatters of license-feature and license-features
// (license_features.py cmd_license_feature, cmd_license_features).

// orUnknown is `s or "?"`.
func orUnknown(s *string) string {
	if s == nil || *s == "" {
		return "?"
	}
	return *s
}

// ModuleMatches is _module_matches: the call is in submodule or module m.
func ModuleMatches(c Call, m string) bool {
	return c.Module == m || strings.SplitN(c.Path, "/", 2)[0] == m
}

// Scope is _scope: what a license lookup searched, for a miss line.
func (r *Report) Scope() string {
	resolved, names := 0, map[string]bool{}
	for _, c := range r.Calls {
		if c.Resolved() {
			resolved++
			names[*c.Feature] = true
		}
	}
	unreadable := ""
	if n := len(r.Unreadable); n > 0 {
		unreadable = fmt.Sprintf(" (%s unreadable)", core.Commas(int64(n)))
	}
	return fmt.Sprintf("%s licensing calls (%s resolved to %s feature names, %s unresolved) in %s "+
		"candidate files%s from %s submodules, token index built %s",
		commas(len(r.Calls)), commas(resolved), commas(len(names)), commas(len(r.Calls)-resolved),
		commas(r.Files), unreadable, commas(r.Modules), r.IndexBuilt)
}

func commas(n int) string { return core.Commas(int64(n)) }

// FeatureCalls is the license-feature match: the calls whose feature equals
// name ignoring case, in submodule or module m when m is not empty.
func (r *Report) FeatureCalls(name, m string) []Call {
	wanted := textfold.Lower(name)
	var out []Call
	for _, c := range r.Calls {
		f := ""
		if c.Feature != nil {
			f = *c.Feature
		}
		if textfold.Lower(f) == wanted && (m == "" || ModuleMatches(c, m)) {
			out = append(out, c)
		}
	}
	return out
}

// RelatedFeatures are the license-feature fallback names: up to 10 resolved
// feature names, sorted, that contain name or are contained in it (ignoring
// case) without being equal to it.
func (r *Report) RelatedFeatures(name string) []string {
	q := textfold.Lower(name)
	seen := map[string]bool{}
	var out []string
	for _, c := range r.Calls {
		if !c.Resolved() || seen[*c.Feature] {
			continue
		}
		seen[*c.Feature] = true
		f := textfold.Lower(*c.Feature)
		if q != f && (strings.Contains(f, q) || strings.Contains(q, f)) {
			out = append(out, *c.Feature)
		}
	}
	slices.Sort(out)
	return out[:min(10, len(out))]
}

// FeatureHeader and FeatureRule head the license-feature call-site table.
var (
	FeatureHeader = fmt.Sprintf("  %s %s  %s %s %s %s", core.LJust("CLASS", 36), core.RJust("LINE", 6),
		core.LJust("VENDOR", 9), core.LJust("FORM", 12), core.LJust("MODULE", 22), "PATH")
	FeatureRule = "  " + strings.Repeat("-", 110)
)

// FeatureRow is one license-feature call-site line.
func FeatureRow(c Call) string {
	return fmt.Sprintf("  %s %s  %s %s %s %s", core.LJust(core.Cut(c.Class, 36), 36),
		core.RJust(fmt.Sprint(c.Line), 6), core.LJust(core.Cut(orUnknown(c.Vendor), 9), 9),
		core.LJust(c.Form, 12), core.LJust(core.Cut(c.Module, 22), 22), c.Path)
}

// FeatureCount is one license-features aggregate row.
type FeatureCount struct {
	Feature string
	Vendors []string // lower-cased vendors ("?" when unresolved), sorted
	Hits    int
	Classes int // distinct classes
	Modules int // distinct submodules
}

// Aggregate is the license-features aggregate of the resolved calls, most
// hits first, then by name.
func Aggregate(calls []Call) []FeatureCount {
	type bucket struct {
		vendors, classes, modules map[string]bool
		hits                      int
	}
	byName := map[string]*bucket{}
	var names []string
	for _, c := range calls {
		if !c.Resolved() {
			continue
		}
		b := byName[*c.Feature]
		if b == nil {
			b = &bucket{vendors: map[string]bool{}, classes: map[string]bool{}, modules: map[string]bool{}}
			byName[*c.Feature] = b
			names = append(names, *c.Feature)
		}
		b.hits++
		b.vendors[textfold.Lower(orUnknown(c.Vendor))] = true
		b.classes[c.Class] = true
		b.modules[c.Module] = true
	}
	out := make([]FeatureCount, 0, len(names))
	for _, n := range names {
		b := byName[n]
		vendors := make([]string, 0, len(b.vendors))
		for v := range b.vendors {
			vendors = append(vendors, v)
		}
		slices.Sort(vendors)
		out = append(out, FeatureCount{n, vendors, b.hits, len(b.classes), len(b.modules)})
	}
	slices.SortStableFunc(out, func(a, b FeatureCount) int {
		if a.Hits != b.Hits {
			return b.Hits - a.Hits
		}
		return strings.Compare(a.Feature, b.Feature)
	})
	return out
}

// AggregateHeader and AggregateRule head the license-features aggregate table.
var (
	AggregateHeader = fmt.Sprintf("  %s %s %s  %s  %s", core.LJust("FEATURE", 30), core.LJust("VENDOR", 12),
		core.RJust("HITS", 5), core.RJust("CLS", 5), core.RJust("MOD", 5))
	AggregateRule = "  " + strings.Repeat("-", 70)
)

// Line is the aggregate row of f.
func (f FeatureCount) Line() string {
	return fmt.Sprintf("  %s %s %5d  %5d  %5d", core.LJust(core.Cut(f.Feature, 30), 30),
		core.LJust(core.Cut(strings.Join(f.Vendors, ","), 12), 12), f.Hits, f.Classes, f.Modules)
}

// ModuleFeatures are the sorted distinct features of the resolved calls.
func ModuleFeatures(calls []Call) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range calls {
		if c.Resolved() && !seen[*c.Feature] {
			seen[*c.Feature] = true
			out = append(out, *c.Feature)
		}
	}
	slices.Sort(out)
	return out
}

// ModuleRow is one `license-features --module` call line.
func ModuleRow(c Call) string {
	feature, vendor, arg := "(unresolved)", c.Args[0], c.Args[1]
	if c.Resolved() {
		feature, arg = *c.Feature, *c.Feature
	}
	if c.Vendor != nil && *c.Vendor != "" {
		vendor = *c.Vendor
	}
	return fmt.Sprintf("    %s %s:%d  %s(%s, %s)", core.LJust(feature, 22), c.Path, c.Line, c.Form, vendor, arg)
}

// UnresolvedCallRow is one `license-features --unresolved` call line.
func UnresolvedCallRow(c Call) string {
	return fmt.Sprintf("    %s:%d  %s(%s)", c.Path, c.Line, c.Form, strings.Join(c.Args, ", "))
}

// UnresolvedSiteRow is one `license-features --unresolved` getLicenseManager() line.
func UnresolvedSiteRow(s Site) string { return fmt.Sprintf("    %s:%d  %s", s.Path, s.Line, s.Reason) }
