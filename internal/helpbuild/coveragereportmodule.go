package helpbuild

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The two halves of cmd_coverage: one module's report and the overview by
// module.

// classPackage is class_index.get(cls, {}).get("package", "") as str().
func classPackage(classIndex *index.Object, cls string) (string, error) {
	v, ok := classIndex.Get(cls)
	if !ok {
		return "", nil
	}
	o, ok := v.(*index.Object)
	if !ok {
		return "", &indexShapeError{"class-index.json", "the record of " + cls + " is not an object"}
	}
	pkg, ok := o.Get("package")
	if !ok {
		return "", nil
	}
	return pycompat.Str(pkg), nil
}

// moduleMethods is the "Total methods" count: the method-index.json entries
// whose class is one of classes and whose section is "methods" (Python
// visits every entry once per class; the classes are distinct keys, so
// counting each entry once against the set is the same number).
func moduleMethods(methods *index.Object, classes []string) (int, error) {
	in := make(map[string]bool, len(classes))
	for _, c := range classes {
		in[c] = true
	}
	n := 0
	for i, name := range methods.Keys() {
		var entries []any
		switch v := methods.Values()[i].(type) {
		case []any:
			entries = v
		case string:
			if v != "" {
				return 0, &indexShapeError{"method-index.json", "the entries of " + name + " are a string"}
			}
		case *index.Object:
			if v.Len() != 0 {
				return 0, &indexShapeError{"method-index.json", "the entries of " + name + " are an object"}
			}
		default:
			return 0, &indexShapeError{"method-index.json", "the entries of " + name + " are not a list"}
		}
		for _, e := range entries {
			o, ok := e.(*index.Object)
			if !ok {
				return 0, &indexShapeError{"method-index.json", "an entry of " + name + " is not an object"}
			}
			cls, _ := o.Get("class")
			section, _ := o.Get("section")
			if c, ok := cls.(string); ok && in[c] && section == "methods" {
				n++
			}
		}
	}
	return n, nil
}

func moduleCoverage(w io.Writer, x *coverageIndexes, hasDoc map[string]bool, module string) (CoverageResult, error) {
	classes, err := moduleClasses(x.source, module)
	if err != nil {
		return CoverageResult{}, err
	}
	if len(classes) == 0 {
		return CoverageResult{Miss: true}, nil
	}
	var documented, undocumented []string
	for _, c := range classes {
		if hasDoc[c] {
			documented = append(documented, c)
		} else {
			undocumented = append(undocumented, c)
		}
	}
	// Every value is computed before printing: an index of an unexpected
	// shape is an *indexShapeError before any output, where Python raises
	// after printing the lines before the failing lookup (an intended
	// difference, pinned by TestCoverageMalformedIndexes).
	pkgs := make([]string, len(undocumented))
	for i, c := range undocumented {
		if pkgs[i], err = classPackage(x.class, c); err != nil {
			return CoverageResult{}, err
		}
	}
	withSlots := 0
	for _, c := range classes {
		if x.slots.Has(c) {
			withSlots++
		}
	}
	methods, err := moduleMethods(x.method, classes)
	if err != nil {
		return CoverageResult{}, err
	}
	pct := float64(len(documented)) / float64(len(classes)) * 100
	out := &reportWriter{w: w}
	out.line("DOCUMENTATION COVERAGE: " + module)
	out.line(strings.Repeat("=", 60))
	out.line("")
	out.line(fmt.Sprintf("Classes: %d total, %d documented, %d undocumented",
		len(classes), len(documented), len(undocumented)))
	out.line("Coverage: " + pyFmt(pct, ".1f") + "%")
	if len(undocumented) > 0 {
		out.line("\nUNDOCUMENTED CLASSES:")
		for i, c := range undocumented {
			out.line(fmt.Sprintf("  %s (%s)", c, pkgs[i]))
		}
	}
	out.line(fmt.Sprintf("\nSlot coverage: %d/%d classes have @Niagara annotations", withSlots, len(classes)))
	out.line(fmt.Sprintf("Total methods: %d", methods))
	return CoverageResult{}, out.err
}

// moduleStat is one overview row.
type moduleStat struct {
	pct         float64
	total, docs int
	module      string
}

// moduleGroups are the source-index classes by module, modules in
// first-seen order (by_module).
type moduleGroups struct {
	order    []string
	byModule map[string][]string
}

// groupByModule is the by_module loop; a record of an unexpected shape is
// an *indexShapeError, returned before the report prints anything.
func groupByModule(source *index.Object) (*moduleGroups, error) {
	g := &moduleGroups{byModule: map[string][]string{}}
	for i, cls := range source.Keys() {
		mod, err := sourceModule(cls, source.Values()[i], "unknown")
		if err != nil {
			return nil, err
		}
		if _, seen := g.byModule[mod]; !seen {
			g.order = append(g.order, mod)
		}
		g.byModule[mod] = append(g.byModule[mod], cls)
	}
	return g, nil
}

func overviewCoverage(w io.Writer, source *index.Object, groups *moduleGroups, hasDoc map[string]bool) error {
	order, byModule := groups.order, groups.byModule
	documented := 0
	for _, cls := range source.Keys() {
		if hasDoc[cls] {
			documented++
		}
	}
	total := source.Len()
	overall := 0.0
	if total > 0 {
		overall = float64(documented) / float64(total) * 100
	}
	out := &reportWriter{w: w}
	out.line("DOCUMENTATION COVERAGE OVERVIEW")
	out.line(strings.Repeat("=", 60))
	out.line("")
	// The denominator is source-index.json, not every class nh knows: say so.
	out.line(fmt.Sprintf("Overall: %d/%d classes documented (%s%% of the %d classes with indexed source)",
		documented, total, pyFmt(overall, ".1f"), total))
	out.line("")
	out.line(fmt.Sprintf("%s  %s  %s  %s  %s", pyFmt("cover", ">6s"), pyFmt("total", ">5s"),
		pyFmt("docs", ">5s"), pyFmt("miss", ">5s"), "module"))
	out.line(fmt.Sprintf("%s  %s  %s  %s  %s", pyFmt("-----", ">6s"), pyFmt("-----", ">5s"),
		pyFmt("----", ">5s"), pyFmt("----", ">5s"), "------"))
	stats := make([]moduleStat, 0, len(order))
	for _, mod := range order {
		classes := byModule[mod]
		docs := 0
		for _, c := range classes {
			if hasDoc[c] {
				docs++
			}
		}
		stats = append(stats, moduleStat{float64(docs) / float64(len(classes)) * 100, len(classes), docs, mod})
	}
	sort.SliceStable(stats, func(i, j int) bool {
		if stats[i].pct != stats[j].pct {
			return stats[i].pct > stats[j].pct
		}
		return stats[i].module < stats[j].module
	})
	for _, s := range stats {
		out.line(fmt.Sprintf("%s%%  %s  %s  %s  %s", pyFmt(s.pct, ">5.0f"), pyFmt(s.total, ">5d"),
			pyFmt(s.docs, ">5d"), pyFmt(s.total-s.docs, ">5d"), s.module))
	}
	return out.err
}
