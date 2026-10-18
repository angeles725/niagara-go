package security

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/license"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// license-feature and license-features (license_features.py): the CLI view of
// the nav/license scan.

// report is license_report with its outcomes reported: nil after an exit-2
// error; a partial read warns on stderr.
func report(env *registry.Env) *license.Report {
	rep, f := license.Scan(env.BaseDir)
	if f != nil {
		env.Error(f.Message, f.Hint)
		return nil
	}
	if rep.Warning != "" {
		fmt.Fprintf(env.Err, "WARNING: %s\n", rep.Warning)
	}
	return rep
}

// miss is _miss: exit 1 over a fully read corpus, exit 2 when candidate
// files could not be read (the answer is not known).
func miss(env *registry.Env, rep *license.Report, subject, hint string) {
	env.Miss(subject, rep.Scope(), hint)
	if n := len(rep.Unreadable); n > 0 {
		env.Error(fmt.Sprintf("no conclusive answer for %s: %s candidate files could not be read",
			subject, core.Commas(int64(n))), "")
	}
}

// limitReached is the "... limit reached" line.
func limitReached(env *registry.Env, limit int) {
	fmt.Fprintf(env.Out, "    ... limit reached (%d) — use -n to increase\n", limit)
}

// first is items[:limit] with Python's slice rule (a negative limit counts from the end).
func first[T any](items []T, limit int) []T {
	if limit < 0 {
		return items[:max(0, len(items)+limit)]
	}
	return items[:min(limit, len(items))]
}

func runLicenseFeature(env *registry.Env, ns *argparse.Namespace) int {
	name, limit, module := ns.String("name"), ns.Int("n"), ns.String("lf_module")
	rep := report(env)
	if rep == nil {
		return env.ExitCode()
	}
	w := env.Out
	matched := rep.FeatureCalls(name, module)
	if len(matched) == 0 {
		subject := fmt.Sprintf("license feature '%s'", name)
		if module != "" {
			subject += fmt.Sprintf(" in module '%s'", module)
		}
		miss(env, rep, subject, "license-features")
		fallbackHints(env, rep, name)
		fmt.Fprintln(w)
		return env.ExitCode()
	}
	modules := map[string]bool{}
	for _, c := range matched {
		modules[c.Module] = true
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  LICENSE FEATURE '%s': %d call site(s) in %d submodule(s)\n", name, len(matched), len(modules))
	fmt.Fprintln(w)
	fmt.Fprintln(w, license.FeatureHeader)
	fmt.Fprintln(w, license.FeatureRule)
	for _, c := range first(matched, limit) {
		fmt.Fprintln(w, license.FeatureRow(c))
	}
	if len(matched) > limit {
		limitReached(env, limit)
	}
	fmt.Fprintln(w)
	return env.ExitCode()
}

// fallbackHints is _print_fallback_hints: related feature, method and class
// names after a miss, and the commands to try next.
func fallbackHints(env *registry.Env, rep *license.Report, name string) {
	w := env.Out
	related := rep.RelatedFeatures(name)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Fallback hints for '%s':\n", name)
	fmt.Fprintf(w, "    Related feature names (%d):\n", len(related))
	for _, f := range related {
		fmt.Fprintf(w, "      - %s\n", f)
	}
	if len(related) == 0 {
		fmt.Fprintln(w, "      (none — try 'license-features' for the full list)")
	}
	q := textfold.Lower(name)
	found := map[string][]string{}
	for _, idx := range []struct{ label, file, key string }{
		{"method", "method-index.json", "methods"}, {"class", "class-index.json", "classes"},
	} {
		names := relatedNames(filepath.Join(env.BaseDir, "indexes", idx.file), idx.key, q)
		found[idx.label] = names
		fmt.Fprintf(w, "    Related %s names (%d):\n", idx.label, len(names))
		for _, n := range names {
			if idx.label == "method" {
				n += "()"
			}
			fmt.Fprintf(w, "      - %s\n", n)
		}
		if len(names) == 0 {
			fmt.Fprintln(w, "      (none)")
		}
	}
	fmt.Fprintln(w, "    Next commands to try:")
	if len(related) > 0 {
		fmt.Fprintf(w, "      license-feature %s\n", related[0])
	}
	fmt.Fprintln(w, "      license-features                     # full aggregate of feature names")
	if m := found["method"]; len(m) > 0 {
		fmt.Fprintf(w, "      method %s\n", m[0])
	}
	if c := found["class"]; len(c) > 0 {
		fmt.Fprintf(w, "      source %s\n", c[0])
	}
	fmt.Fprintf(w, "      grep '%s'\n", name)
}

// relatedNames is sorted(n for n in json.load(path).get(key, {}) if q in
// n.lower() and len(n) > 3)[:8]; nothing when the index cannot be read.
func relatedNames(path, key, q string) []string {
	v, err := index.LoadPython(path)
	if err != nil {
		return nil
	}
	data, _ := v.(*index.Object)
	if data == nil {
		return nil
	}
	table := data.Object(key)
	if table == nil {
		return nil
	}
	var out []string
	for _, n := range table.Keys() {
		if strings.Contains(textfold.Lower(n), q) && utf8.RuneCountInString(n) > 3 {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out[:min(8, len(out))]
}

func runLicenseFeatures(env *registry.Env, ns *argparse.Namespace) int {
	limit, module := ns.Int("n"), ns.String("lf_module")
	rep := report(env)
	if rep == nil {
		return env.ExitCode()
	}
	if ns.Bool("lf_unresolved") {
		printUnresolved(env, rep, limit)
		return env.ExitCode()
	}
	var calls []license.Call
	for _, c := range rep.Calls {
		if module == "" || license.ModuleMatches(c, module) {
			calls = append(calls, c)
		}
	}
	if module != "" {
		printModule(env, rep, module, calls, limit)
		return env.ExitCode()
	}
	w := env.Out
	unresolvedSites := 0
	for _, s := range rep.Sites {
		if s.Reason != "" {
			unresolvedSites++
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  LICENSE FEATURES — aggregate")
	fmt.Fprintf(w, "  %s\n", rep.Scope())
	fmt.Fprintf(w, "  getLicenseManager() sites: %s (%s lead to a feature, %s unresolved: "+
		"license-features --unresolved)\n", core.Commas(int64(len(rep.Sites))),
		core.Commas(int64(len(rep.Sites)-unresolvedSites)), core.Commas(int64(unresolvedSites)))
	fmt.Fprintln(w)
	ranked := license.Aggregate(calls)
	if len(ranked) == 0 {
		miss(env, rep, "resolvable license feature names", "")
		return env.ExitCode()
	}
	fmt.Fprintln(w, license.AggregateHeader)
	fmt.Fprintln(w, license.AggregateRule)
	for _, f := range first(ranked, limit) {
		fmt.Fprintln(w, f.Line())
	}
	if len(ranked) > limit {
		limitReached(env, limit)
	}
	fmt.Fprintln(w)
	return env.ExitCode()
}

// printModule is _print_module: the features a module requires and its calls.
func printModule(env *registry.Env, rep *license.Report, module string, calls []license.Call, limit int) {
	if len(calls) == 0 {
		miss(env, rep, fmt.Sprintf("licensing calls in module '%s'", module), "")
		return
	}
	w := env.Out
	features := strings.Join(license.ModuleFeatures(calls), ", ")
	if features == "" {
		features = "(none resolved)"
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  MODULE %s requires features: %s\n", module, features)
	fmt.Fprintln(w)
	for _, c := range first(calls, limit) {
		fmt.Fprintln(w, license.ModuleRow(c))
	}
	if len(calls) > limit {
		limitReached(env, limit)
	}
	fmt.Fprintln(w)
}

// printUnresolved is _print_unresolved: every call and site without a feature,
// and the candidate files that could not be read.
func printUnresolved(env *registry.Env, rep *license.Report, limit int) {
	w := env.Out
	var calls []license.Call
	for _, c := range rep.Calls {
		if !c.Resolved() {
			calls = append(calls, c)
		}
	}
	var sites []license.Site
	for _, s := range rep.Sites {
		if s.Reason != "" {
			sites = append(sites, s)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  UNRESOLVED licensing calls: %s\n", core.Commas(int64(len(calls))))
	for _, c := range first(calls, limit) {
		fmt.Fprintln(w, license.UnresolvedCallRow(c))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  getLicenseManager() sites without a feature: %s of %s\n",
		core.Commas(int64(len(sites))), core.Commas(int64(len(rep.Sites))))
	for _, s := range first(sites, limit) {
		fmt.Fprintln(w, license.UnresolvedSiteRow(s))
	}
	if len(rep.Unreadable) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  Candidate files that could not be read: %s\n", core.Commas(int64(len(rep.Unreadable))))
		for _, rel := range first(rep.Unreadable, limit) {
			fmt.Fprintf(w, "    %s  could not read\n", rel)
		}
	}
	if max(len(calls), len(sites), len(rep.Unreadable)) > limit {
		limitReached(env, limit)
	}
	fmt.Fprintln(w)
}
