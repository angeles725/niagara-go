package helpbuild

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The `coverage [module]` report (niagara_help_lib/coverage_report.py): the
// guide extraction coverage (guides/ HTML pages vs guides-clean/ text, per
// folder) and the documentation coverage of the classes, overall or for one
// module. The printers write exactly what Python prints to stdout; the CLI
// decides the exit status (Coverage's GuideGaps, Miss).

// MissingIndexError is index_cache.MissingIndexError: a required index of
// the report is absent (the CLI exits 2 with "Cannot search:").
type MissingIndexError struct{ Name, BaseDir string }

func (e *MissingIndexError) Error() string {
	return fmt.Sprintf("index indexes/%s is missing under %s; rebuild the indexes "+
		"with: nh rebuild-all", e.Name, e.BaseDir)
}

// IndexShapeError is the AttributeError, TypeError or KeyError Python
// raises on an index of an unexpected shape (a list where a dict is
// expected, a record without its key).
type IndexShapeError struct{ Name, What string }

func (e *IndexShapeError) Error() string { return "indexes/" + e.Name + ": " + e.What }

// loadReportIndex is index_cache.load_index(baseDir, name, required): a
// missing file is an empty object, or a *MissingIndexError when required;
// the top level must be an object.
func loadReportIndex(baseDir, name string, required bool) (*index.Object, error) {
	path := pycompat.Join(baseDir, "indexes", name)
	if _, err := os.Stat(path); err != nil {
		if required {
			return nil, &MissingIndexError{Name: name, BaseDir: baseDir}
		}
		return index.NewObject(), nil
	}
	v, err := index.LoadPython(path)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*index.Object)
	if !ok {
		return nil, &IndexShapeError{name, "the top level is not an object"}
	}
	return o, nil
}

// GuidePageExtensions are coverage_report.GUIDE_PAGE_EXTENSIONS.
var GuidePageExtensions = []string{".html", ".htm"}

// countTree is _count_tree: the files under path (os.walk, no symlinked
// folder entered) whose lower-cased name ends with one of extensions.
func countTree(path string, extensions []string) int {
	n := 0
	pyWalk(path, func(_ string, dirs, files []string) []string {
		for _, f := range files {
			if hasAnySuffix(textfold.Lower(f), extensions) {
				n++
			}
		}
		return dirs
	})
	return n
}

// GuideFolder is one guide_coverage row: the HTML pages of guides/<Folder>
// and the text files of guides-clean/<Folder>, both counted recursively.
type GuideFolderCoverage struct {
	Folder          string
	OnDisk, Indexed int
}

// GuideCoverage is guide_coverage: one row per folder of guides/ (sorted
// by name), none when there is no guides/.
func GuideCoverage(baseDir string) ([]GuideFolderCoverage, error) {
	guides := pycompat.Join(baseDir, "guides")
	clean := pycompat.Join(baseDir, "guides-clean")
	if !pycompat.IsDir(guides) {
		return nil, nil
	}
	names, err := sortedNames(guides)
	if err != nil {
		return nil, err
	}
	var rows []GuideFolderCoverage
	for _, folder := range names {
		path := pycompat.Join(guides, folder)
		if !pycompat.IsDir(path) {
			continue
		}
		rows = append(rows, GuideFolderCoverage{Folder: folder,
			OnDisk:  countTree(path, GuidePageExtensions),
			Indexed: countTree(pycompat.Join(clean, folder), []string{".txt"})})
	}
	return rows, nil
}

// reportWriter collects the first write error of a report.
type reportWriter struct {
	w   io.Writer
	err error
}

func (r *reportWriter) line(s string) {
	if r.err == nil {
		_, r.err = io.WriteString(r.w, s+"\n")
	}
}

// pyFmt is format(v, spec) for the constant specs of the reports.
func pyFmt(v any, spec string) string {
	s, err := pycompat.Format(v, spec)
	if err != nil {
		panic(err) // a constant spec of this file is wrong
	}
	return s
}

// PrintGuideCoverage is cmd_guide_coverage: it prints the guide extraction
// coverage and returns the number of GAP folders (fewer pages indexed in
// guides-clean/ than HTML pages under guides/).
func PrintGuideCoverage(w io.Writer, baseDir string) (int, error) {
	rows, err := GuideCoverage(baseDir)
	if err != nil {
		return 0, err
	}
	var gaps []GuideFolderCoverage
	onDisk, indexed := 0, 0
	for _, r := range rows {
		if r.Indexed < r.OnDisk {
			gaps = append(gaps, r)
		}
		onDisk += r.OnDisk
		indexed += r.Indexed
	}
	out := &reportWriter{w: w}
	out.line("GUIDE COVERAGE (guides/ HTML vs guides-clean/ text)")
	out.line(strings.Repeat("=", 60))
	out.line(fmt.Sprintf("Overall: %d/%d pages indexed across %d folders, %d gap folder(s)",
		indexed, onDisk, len(rows), len(gaps)))
	for _, r := range gaps {
		out.line(fmt.Sprintf("  GAP  %s/%s %s", pyFmt(r.Indexed, ">5d"), pyFmt(r.OnDisk, "<5d"), r.Folder))
	}
	out.line("")
	return len(gaps), out.err
}

// CoverageResult is what cmd_coverage returns to the CLI: the overview's
// number of guide folders with unindexed pages (exit 1 when not zero), and
// Miss when the module matched no class (nothing was printed: the CLI
// prints scope.report_miss(baseDir, "coverage", module, "modules")).
type CoverageResult struct {
	GuideGaps int
	Miss      bool
}

// coverageIndexes are the indexes cmd_coverage loads, in its order (every
// one required, also those a mode does not read).
type coverageIndexes struct {
	class, source, method, slots *index.Object
}

func loadCoverageIndexes(baseDir string) (*coverageIndexes, error) {
	var x coverageIndexes
	for _, l := range []struct {
		name string
		dst  **index.Object
	}{{"class-index.json", &x.class}, {"source-index.json", &x.source},
		{"method-index.json", &x.method}, {"slots-index.json", &x.slots}} {
		o, err := loadReportIndex(baseDir, l.name, true)
		if err != nil {
			return nil, err
		}
		*l.dst = o
	}
	return &x, nil
}

// documentedClasses is the has_doc set: the names of bajadoc-clean/**.txt
// with every ".txt" removed (str.replace).
func documentedClasses(baseDir string) map[string]bool {
	has := map[string]bool{}
	dir := pycompat.Join(baseDir, "bajadoc-clean")
	if !pycompat.IsDir(dir) {
		return has
	}
	pyWalk(dir, func(_ string, dirs, files []string) []string {
		for _, f := range files {
			if strings.HasSuffix(f, ".txt") {
				has[strings.ReplaceAll(f, ".txt", "")] = true
			}
		}
		return dirs
	})
	return has
}

// moduleNorm is module_export._module_norm.
func moduleNorm(name string) string { return strings.ReplaceAll(textfold.Lower(name), "-", "") }

// sourceModule is data.get("module", def) of a source-index record.
func sourceModule(cls string, data any, def string) (string, error) {
	o, ok := data.(*index.Object)
	if !ok {
		return "", &IndexShapeError{"source-index.json", "the record of " + cls + " is not an object"}
	}
	v, has := o.Get("module")
	if !has {
		return def, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", &IndexShapeError{"source-index.json", "the module of " + cls + " is not a string"}
	}
	return s, nil
}

// ModuleClasses is module_export.module_classes: the sorted classes of
// source-index.json whose module, without case and dashes, starts with
// module (a trailing rt/ux dropped): "alarm-rt" matches alarm-rt, alarm-ux
// and alarm-wb.
func ModuleClasses(source *index.Object, module string) ([]string, error) {
	norm := moduleNorm(module)
	if strings.HasSuffix(norm, "rt") || strings.HasSuffix(norm, "ux") {
		norm = norm[:len(norm)-2]
	}
	var out []string
	for i, cls := range source.Keys() {
		mod, err := sourceModule(cls, source.Values()[i], "")
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(moduleNorm(mod), norm) {
			out = append(out, cls)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Coverage is cmd_coverage(baseDir, module): with a module, the coverage
// of its classes; without ("" is Python's None), the guide coverage then
// the overview by module. All four indexes are loaded first, so a missing
// one (*MissingIndexError) prints nothing; an index of an unexpected shape
// (*IndexShapeError: a top level that is not an object, in any of the
// four, or a record the mode reads) also prints nothing, where Python may
// print part of the report first or, reading no bad value, all of it.
func Coverage(w io.Writer, baseDir, module string) (CoverageResult, error) {
	x, err := loadCoverageIndexes(baseDir)
	if err != nil {
		return CoverageResult{}, err
	}
	hasDoc := documentedClasses(baseDir)
	if module != "" {
		return moduleCoverage(w, x, hasDoc, module)
	}
	groups, err := groupByModule(x.source)
	if err != nil {
		return CoverageResult{}, err
	}
	gaps, err := PrintGuideCoverage(w, baseDir)
	if err != nil {
		return CoverageResult{}, err
	}
	return CoverageResult{GuideGaps: gaps}, overviewCoverage(w, x.source, groups, hasDoc)
}
