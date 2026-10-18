package helpbuild

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The sweep report and gate (install_sweep.cmd_sweep / print_report) and
// the course row it carries (course.gate / print_gate). The library
// writes the report to the io.Writer it is given and returns the exit
// status; the caller owns stdout.

// sweepFirst is install_sweep.FIRST: how many UNCLASSIFIED rows are shown.
const sweepFirst = 25

// Exit statuses of RunSweep (cmd_sweep).
const (
	SweepPass        = 0 // nothing UNCLASSIFIED
	SweepFail        = 1 // at least one UNCLASSIFIED entry
	SweepUnavailable = 2 // the install is not available: gate not evaluated
)

// CourseGate is the dict course.gate returns: the course row of the sweep.
type CourseGate struct {
	CourseDir         string
	PDFs, Paired      int
	Text, TextIndexed int
	Code, CodeIndexed int
	Excluded          map[ReasonKey]int
	Unclassified      [][2]string // (course path, why)
}

// courseStem is course._stem: the file name without its extension.
func courseStem(rel string) string {
	root, _ := pycompat.PosixSplitext(pyBasename(rel))
	return root
}

// FreshFulltextPath is the read-only side of fulltext_index.open_index: the
// up-to-date full-text database of baseDir that already exists
// (indexes/fulltext.sqlite or its user-cache copy), else
// *fts.StaleIndexError. open_index builds a missing or stale index instead;
// a caller that wants that passes its own FulltextDocs.
func FreshFulltextPath(baseDir string) (string, error) {
	p, missing := fts.FreshPath(baseDir, fts.Fingerprint(fts.SourceDirs(baseDir)))
	if p == "" {
		return "", &fts.StaleIndexError{BaseDir: baseDir, Missing: missing}
	}
	return p, nil
}

// FreshFulltextDocs is the FulltextDocs of an up-to-date index that
// already exists (FreshFulltextPath, then FulltextDocsAt): the sweep's
// default.
func FreshFulltextDocs(baseDir string) (map[string]bool, error) {
	p, err := FreshFulltextPath(baseDir)
	if err != nil {
		return nil, err
	}
	return FulltextDocsAt(p)
}

func (o SweepOptions) fulltext() FulltextDocs {
	if o.Fulltext == nil {
		return FreshFulltextDocs
	}
	return o.Fulltext
}

// GateCourse is course.gate(baseDir, rules): nil when no course is bound
// to this version. rules must be non-nil; docs reads the full-text index
// (nil: FreshFulltextDocs).
func GateCourse(baseDir string, rules *Rules, docs FulltextDocs) (*CourseGate, error) {
	src, err := courseDir(baseDir)
	if err != nil || src == "" {
		return nil, err
	}
	rep := &CourseGate{CourseDir: src, Excluded: map[ReasonKey]int{}}
	if !pycompat.IsDir(src) {
		rep.Unclassified = [][2]string{{src, "course_dir not found"}}
		return rep, nil
	}
	indexedDocs, err := SweepOptions{Fulltext: docs}.fulltext()(baseDir)
	if err != nil {
		return nil, err
	}
	rows := CoursePlan(src, rules)
	texts := map[string]bool{}
	for _, r := range rows {
		if r.Kind == "text" && strings.HasPrefix(r.Rel, "text/") {
			texts[courseStem(r.Rel)] = true
		}
	}
	codeTop := "devguide-examples" + string(filepath.Separator)
	for _, r := range rows {
		switch r.Kind {
		case "text", "code":
			label, p := CourseTextDir, r.Dest[len(CourseTextDir)+1:]
			n, indexed := &rep.Text, &rep.TextIndexed
			if r.Kind == "code" {
				label, p = "devguide-examples", r.Dest[len(codeTop):]
				n, indexed = &rep.Code, &rep.CodeIndexed
			}
			*n++
			if indexedDocs[label+"/"+filepath.ToSlash(p)] {
				*indexed++
			} else {
				rep.Unclassified = append(rep.Unclassified, [2]string{r.Rel, "copied to " + r.Dest + " but not indexed"})
			}
			continue
		case "pdf":
			rep.PDFs++
			if stem := courseStem(r.Rel); !texts[stem] {
				rep.Unclassified = append(rep.Unclassified, [2]string{r.Rel, "course PDF without its text/" + stem + ".txt"})
				continue
			}
			rep.Paired++
		}
		if rule := rules.ExcludeRule(CoursePseudoJar, r.Rel); rule != nil {
			rep.Excluded[ReasonKey{rule.ID, rule.Reason}]++
		} else {
			rep.Unclassified = append(rep.Unclassified, [2]string{r.Rel, `no course rule matches (add one to install_docs.json, jar "(course)")`})
		}
	}
	return rep, nil
}

// sortedReasons is sorted(reasons.items(), key=(-count, (rule, reason))).
func sortedReasons(m map[ReasonKey]int) []ReasonKey {
	keys := make([]ReasonKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if m[a] != m[b] {
			return m[a] > m[b]
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Reason < b.Reason
	})
	return keys
}

func reasonLines(m map[ReasonKey]int) []string {
	var out []string
	for _, k := range sortedReasons(m) {
		out = append(out, "  "+pyFmt(m[k], ">7,")+"  "+k.Rule+": "+k.Reason)
	}
	return out
}

// sweepRowLine is print_report's
// "  {:28s} {:>6s} {:>9s} {:>9s} {:>8s} {:>8s} {:>12s}".
func sweepRowLine(cells ...string) string {
	specs := []string{">6s", ">9s", ">9s", ">8s", ">8s", ">12s"}
	line := "  " + pyFmt(cells[0], "28s")
	for i, c := range cells[1:] {
		line += " " + pyFmt(c, specs[i])
	}
	return line
}

func (r *SweepRow) cells(loc string) []string {
	out := []string{loc}
	for _, n := range []int{r.Jars, r.WithDocs, r.DocLike, r.Indexed, r.Excluded, r.Unclassified} {
		out = append(out, pyFmt(n, ","))
	}
	return out
}

// SweepReport is install_sweep.print_report: the report lines (without
// line ends) of vid's sweep; rules gives the documentation-like line
// (cmd_sweep prints install_rules.default()). On an error the lines
// printed before it are returned with it.
func SweepReport(vid, install string, res *SweepResult, baseDir string, course *CourseGate, rules *Rules) ([]string, error) {
	out := []string{
		fmt.Sprintf("INSTALL SWEEP %s (%d jars under %s; %d listed anew, the rest from the jar-listing cache)",
			pyNone(vid), res.Jars, install, res.Listed),
		"  documentation-like: " + strings.Join(rules.DocExt, " ") + " + " + strings.Join(rules.DocNames, " "),
		sweepRowLine("location", "jars", "with-docs", "doc-like", "indexed", "excluded", "UNCLASSIFIED"),
	}
	locs := make([]string, 0, len(res.Rows))
	for loc := range res.Rows {
		locs = append(locs, loc)
	}
	sort.Strings(locs)
	var total SweepRow
	for _, loc := range locs {
		r := res.Rows[loc]
		total.Jars += r.Jars
		total.WithDocs += r.WithDocs
		total.DocLike += r.DocLike
		total.Indexed += r.Indexed
		total.Excluded += r.Excluded
		total.Unclassified += r.Unclassified
		out = append(out, sweepRowLine(r.cells(firstRunes(loc, 28))...))
	}
	out = append(out, sweepRowLine(total.cells("TOTAL")...))
	if len(res.Skipped) > 0 {
		out = append(out, "", "SKIPPED DIRECTORIES (install_docs.json skip_dirs):")
		for _, d := range res.Skipped {
			out = append(out, "  "+d.Dir+"/  "+d.Reason)
		}
	}
	out = append(out, "", "EXCLUDED (rule: reason):")
	out = append(out, reasonLines(res.Reasons)...)
	pdf, err := pdfManualsLine(baseDir)
	if err != nil {
		return append(out, ""), err // print_report stops after the blank line
	}
	out = append(out, "", pdf)
	out = append(out, courseGateLines(baseDir, course)...)
	gaps := res.Unclassified
	if len(gaps) > 0 {
		out = append(out, "", fmt.Sprintf("UNCLASSIFIED (first %d of %d):", min(sweepFirst, len(gaps)), len(gaps)))
		for _, g := range gaps[:min(sweepFirst, len(gaps))] {
			out = append(out, "  "+g.Jar+"  "+g.Entry+"  -- "+g.Why)
		}
	}
	switch len(gaps) {
	case 0:
		out = append(out, "RESULT: PASS - every documentation-like entry is indexed or excluded with a reason")
	case 1:
		out = append(out, "RESULT: FAIL - 1 unclassified entry")
	default:
		out = append(out, fmt.Sprintf("RESULT: FAIL - %d unclassified entries", len(gaps)))
	}
	return out, nil
}

// pyNone is str() of an optional string: "" is None.
func pyNone(s string) string {
	if s == "" {
		return "None"
	}
	return s
}

// pdfManualsLine is install_sweep._print_pdf_line: which PDF manuals this
// version searches (its own or another version's).
func pdfManualsLine(baseDir string) (string, error) {
	root, sharedFrom := fts.DocsTextRoot(baseDir)
	texts := filepath.Join(root, "docs-text")
	n := 0
	if pycompat.IsDir(texts) {
		ents, err := os.ReadDir(texts)
		if err != nil {
			return "", err
		}
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".txt") {
				n++
			}
		}
	}
	switch {
	case sharedFrom != "":
		return "PDF MANUALS: none shipped in this install; fallback: searches the " + pyFmt(n, ",") +
			" manuals of Niagara " + sharedFrom, nil
	case n > 0:
		return "PDF MANUALS: " + pyFmt(n, ",") + " of this version (docs-text/)", nil
	}
	return "PDF MANUALS: none (no registered version has docs-text/)", nil
}

// courseGateLines is course.print_gate: the COURSE section.
func courseGateLines(baseDir string, rep *CourseGate) []string {
	if rep == nil {
		line := "COURSE: none bound to this version"
		if _, owner := fts.SharedRoot(baseDir, CourseTextDir); owner != "" {
			line += "; find shows the course bound to Niagara " + owner + " with a note"
		}
		return []string{"", line}
	}
	out := []string{"",
		"COURSE (" + rep.CourseDir + "):",
		fmt.Sprintf("  course PDFs paired with their text file: %d of %d", rep.Paired, rep.PDFs),
		fmt.Sprintf("  page text files (course-text/): %d indexed of %d", rep.TextIndexed, rep.Text),
		fmt.Sprintf("  example code (devguide-examples/course/): %d indexed of %d", rep.CodeIndexed, rep.Code),
	}
	return append(out, reasonLines(rep.Excluded)...)
}

// RunSweep is install_sweep.cmd_sweep(repo, baseDir, versionID): it writes
// the report to w and returns the exit status (SweepPass, SweepFail or
// SweepUnavailable). versionID "" is identified from baseDir. The jar
// listings go through the repository's cache (.cache/jar-listings/).
// opts.Rules replaces install_rules.default() everywhere cmd_sweep reads
// it; opts.Listings is ignored. err is what Python raises (status 1).
func RunSweep(repo, baseDir, versionID string, w io.Writer, opts SweepOptions) (int, error) {
	vid := versionID
	if vid == "" {
		target, err := absTarget(baseDir)
		if err != nil {
			return 1, err
		}
		if vid, err = identify(target); err != nil {
			return 1, err
		}
	}
	install, err := registeredInstall(repo, vid, vid != "")
	if err != nil {
		return 1, err
	}
	if install == "" || !pycompat.IsDir(install) {
		shown := install
		if shown == "" {
			shown = "?"
		}
		_, err := fmt.Fprintf(w, "INSTALL SWEEP %s: install not available (%s); cannot read the jars, gate not evaluated.\n",
			pyNone(vid), shown)
		return SweepUnavailable, err
	}
	if opts.Rules == nil {
		if opts.Rules, err = Default(); err != nil {
			return 1, err
		}
	}
	cache, err := CachePath(repo, install)
	if err != nil {
		return 1, err
	}
	opts.Listings = NewListings(cache)
	res, err := Scan(install, baseDir, opts)
	if err != nil {
		return 1, err
	}
	course, err := GateCourse(baseDir, opts.Rules, opts.Fulltext)
	if err != nil {
		return 1, err
	}
	if course != nil {
		for _, u := range course.Unclassified {
			res.Unclassified = append(res.Unclassified, Gap{CoursePseudoJar, u[0], u[1]})
		}
	}
	lines, err := SweepReport(vid, install, res, baseDir, course, opts.Rules)
	if _, werr := io.WriteString(w, strings.Join(lines, "\n")+"\n"); err == nil && werr != nil {
		err = werr
	}
	if err != nil {
		return 1, err
	}
	if len(res.Unclassified) > 0 {
		return SweepFail, nil
	}
	return SweepPass, nil
}
