package helpbuild

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/pdftext"
	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pypath"
)

// `coverage --doc-jars` (niagara_help_lib/doc_coverage.py): the mechanical
// completeness gate over the doc jars. Every entry of every doc jar of a
// version is read from its install (zip listing; installs are read-only),
// mapped with DestPath, classified (Classify) and counted, per jar and
// content type, as extracted (the corpus file exists under the data root),
// indexed (the derived index or text holds it) or excluded (an explicit
// reason); anything else is a gap. Exit status: 0 when every entry is
// indexed or excluded and the guide-folder check is clean, 1 otherwise, 2
// when the install is unavailable.

// FirstGaps is FIRST_GAPS: the gaps the report lists.
const FirstGaps = 25

var errNoFulltext = errors.New("coverage --doc-jars: no full-text index reader (FulltextDocs) for an example entry")

// DocJarKey is a report row: a jar (base name) and a content type.
type DocJarKey struct{ Jar, ContentType string }

// DocJarRow are the counts of one row.
type DocJarRow struct{ InJar, Extracted, Indexed, Excluded, Gap int }

// DocJarGap is one unexplained entry: the jar, the corpus path (the entry
// name when it maps to none) and why.
type DocJarGap struct{ Jar, Rel, Why string }

// ReasonCount is an exclusion reason and its entry count.
type ReasonCount struct {
	Reason string
	Count  int
}

// DocJarScan is scan's (rows, reasons, gaps); Reasons keep the order in
// which each reason was first met.
type DocJarScan struct {
	Rows    map[DocJarKey]*DocJarRow
	Reasons []ReasonCount
	Gaps    []DocJarGap
}

func (s *DocJarScan) addReason(reason string) {
	for i := range s.Reasons {
		if s.Reasons[i].Reason == reason {
			s.Reasons[i].Count++
			return
		}
	}
	s.Reasons = append(s.Reasons, ReasonCount{reason, 1})
}

// jarEntries is _entries: the names of a jar's file entries (directories
// dropped, .class files kept), ok false when the jar cannot be read.
func jarEntries(jar string) (names []string, ok bool) {
	infos, err := zipfile.ReadCentralDir(jar)
	if err != nil {
		return nil, false
	}
	for _, i := range infos {
		if !strings.HasSuffix(i.Name, "/") {
			names = append(names, i.Name)
		}
	}
	return names, true
}

// entryVerdict is _verdict: the corpus path (mapped false when none), the
// verdict, the index kind or exclusion reason, and the content type.
func entryVerdict(jarName, entry string, rules *Rules) (rel string, mapped bool, verdict, detail, ctype string) {
	rel, mapped = DestPath(jarName, entry, rules)
	switch {
	case !mapped && IsModuleJar(jarName):
		return "", false, VerdictExclude, Reasons["module-content"], "(module, not doc/)"
	case !mapped:
		verdict, detail = ClassifyUnmapped(entry)
		_, ext := pypath.PosixSplitext(entry)
		if ext == "" {
			ext = entry
		}
		return "", false, verdict, detail, "(unmapped) " + ext
	}
	verdict, detail = Classify(rel)
	return rel, true, verdict, detail, ContentType(rel)
}

// ScanDocJars is scan(installDir, root, jars): it classifies every entry of
// every jar (nil jars: ListDocJars(installDir, true, nil)) against the data
// root. An unreadable jar is one gap row, never an error; an error is what
// Python raises (an index of an unexpected shape, an unreadable .bajadoc,
// a failing full-text reader).
func ScanDocJars(installDir, root string, jars []string, docs FulltextDocs) (*DocJarScan, error) {
	rules, err := Default()
	if err != nil {
		return nil, err
	}
	if jars == nil {
		if jars, err = ListDocJars(installDir, true, nil); err != nil {
			return nil, err
		}
	}
	indexed := newIndexedViews(root, docs)
	s := &DocJarScan{Rows: map[DocJarKey]*DocJarRow{}}
	for _, jar := range jars {
		name := pypath.PosixBasename(jar)
		entries, ok := jarEntries(jar)
		if !ok {
			s.Rows[DocJarKey{name, "(unreadable jar)"}] = &DocJarRow{Gap: 1}
			s.Gaps = append(s.Gaps, DocJarGap{name, name, "jar unreadable (corrupt zip or I/O error)"})
			continue
		}
		for _, entry := range entries {
			if err := s.scanEntry(indexed, name, entry, rules); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *DocJarScan) scanEntry(indexed *indexedViews, name, entry string, rules *Rules) error {
	rel, mapped, verdict, detail, ctype := entryVerdict(name, entry, rules)
	key := DocJarKey{name, ctype}
	row := s.Rows[key]
	if row == nil {
		row = &DocJarRow{}
		s.Rows[key] = row
	}
	row.InJar++
	path := ""
	if mapped {
		path = pycompat.Join(indexed.root, rel)
	}
	if path != "" && pycompat.IsFile(path) {
		row.Extracted++
	}
	if verdict == VerdictExclude {
		row.Excluded++
		s.addReason(detail)
		return nil
	}
	var why string
	switch {
	case verdict == VerdictIndex && !mapped:
		why = "index rule for an unmapped entry (" + detail + ")"
	case verdict == VerdictIndex:
		if pycompat.IsFile(path) {
			ok, err := indexed.check(detail, rel)
			if err != nil {
				return err
			}
			if ok {
				row.Indexed++
				return nil
			}
		}
		why = "not indexed (" + detail + ")"
		if !pycompat.IsFile(path) {
			why = "not extracted"
		}
	default:
		why = "unknown content type (no index rule, no exclusion)"
	}
	row.Gap++
	shown := rel
	if shown == "" {
		shown = entry
	}
	s.Gaps = append(s.Gaps, DocJarGap{name, shown, why})
	return nil
}

// registeredInstall is the install_dir cmd_doc_jar_coverage and
// install_sweep.cmd_sweep read from the registry of repoRoot:
// reg["versions"].get(vid, {}).get("install_dir", ""); known is false for
// a vid of None.
func registeredInstall(repoRoot, vid string, known bool) (string, error) {
	reg, ok, err := registry.Load(repoRoot)
	if err != nil {
		return "", err
	}
	if !ok || !reg.Truthy() {
		return "", nil
	}
	vers, has := reg.Get("versions")
	if reg.Kind != pyjson.Object || !has {
		return "", registry.NewShapeError(repoRoot, `no "versions" in the registry`)
	}
	if vers.Kind != pyjson.Object {
		return "", registry.NewShapeError(repoRoot, `"versions" is not an object`)
	}
	if !known {
		return "", nil
	}
	entry, has := vers.Get(vid)
	if !has {
		return "", nil
	}
	if entry.Kind != pyjson.Object {
		return "", registry.NewShapeError(repoRoot, "the entry of "+vid+" is not an object")
	}
	inst, has := entry.Get("install_dir")
	if !has || !inst.Truthy() {
		return "", nil
	}
	if inst.Kind != pyjson.Str {
		return "", registry.NewShapeError(repoRoot, "the install_dir of "+vid+" is not a string")
	}
	return inst.S, nil
}

// DocJarCoverage is cmd_doc_jar_coverage(repoRoot, baseDir, versionID): it
// prints the gate report to w and returns the exit status (0, 1, or 2 when
// the install is unavailable). versionID "" is identified from baseDir.
// The jar listings go through the repository's listing cache, which is
// saved (as Python does) before the scan.
func DocJarCoverage(w io.Writer, repoRoot, baseDir, versionID string, docs FulltextDocs) (int, error) {
	vid, known := versionID, versionID != ""
	if !known {
		target, err := registry.AbsTarget(baseDir)
		if err != nil {
			return 0, err
		}
		if vid, err = registry.Identify(target); err != nil {
			return 0, err
		}
		known = vid != ""
	}
	shown := vid
	if !known {
		shown = "None"
	}
	install, err := registeredInstall(repoRoot, vid, known)
	if err != nil {
		return 0, err
	}
	out := &reportWriter{w: w}
	if install == "" || !pycompat.IsDir(install) {
		if install == "" {
			install = "?"
		}
		out.line(fmt.Sprintf("DOC-JAR COVERAGE %s: install not available (%s); cannot read the jars, gate not evaluated.",
			shown, install))
		return 2, out.err
	}
	cache, err := CachePath(repoRoot, install)
	if err != nil {
		return 0, err
	}
	ls := NewListings(cache)
	jars, err := ListDocJars(install, true, ls)
	if err != nil {
		return 0, err
	}
	if err := ls.Save(); err != nil {
		return 0, err
	}
	if jars == nil {
		jars = []string{} // no jar: ScanDocJars must not list them again
	}
	scan, err := ScanDocJars(install, baseDir, jars, docs)
	if err != nil {
		return 0, err
	}
	printDocJarReport(out, shown, install, len(jars), scan)
	guideGaps, err := PrintGuideCoverage(io.Discard, baseDir)
	if err != nil {
		return 0, err
	}
	out.line("")
	if guideGaps == 0 {
		out.line("GUIDE FOLDERS: every folder fully extracted")
	} else {
		out.line(fmt.Sprintf("GUIDE FOLDERS: %d folder(s) with unindexed pages (run `coverage`)", guideGaps))
	}
	if len(scan.Gaps) == 0 && guideGaps == 0 {
		out.line("RESULT: PASS - every jar entry is indexed or excluded with a reason")
		return 0, out.err
	}
	plural := "ies"
	if len(scan.Gaps) == 1 {
		plural = "y"
	}
	out.line(fmt.Sprintf("RESULT: FAIL - %d unexplained entr%s", len(scan.Gaps), plural))
	return 1, out.err
}

// docJarLine is one table line of _print_report.
func docJarLine(jar, ctype string, r DocJarRow) string {
	return "  " + pyFmt(jar, "34s") + " " + pyFmt(ctype, "20s") + " " + pyFmt(r.InJar, ">7,") + " " +
		pyFmt(r.Extracted, ">9,") + " " + pyFmt(r.Indexed, ">7,") + " " + pyFmt(r.Excluded, ">8,") + " " +
		pyFmt(r.Gap, ">5,")
}

// printDocJarReport is _print_report.
func printDocJarReport(out *reportWriter, vid, install string, nJars int, s *DocJarScan) {
	out.line(fmt.Sprintf("DOC-JAR COVERAGE %s (%d jars from %s)", vid, nJars, install))
	out.line("  " + pyFmt("jar", "34s") + " " + pyFmt("content type", "20s") + " " + pyFmt("in-jar", ">7s") + " " +
		pyFmt("extracted", ">9s") + " " + pyFmt("indexed", ">7s") + " " + pyFmt("excluded", ">8s") + " " +
		pyFmt("gaps", ">5s"))
	keys := make([]DocJarKey, 0, len(s.Rows))
	for k := range s.Rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Jar != keys[j].Jar {
			return keys[i].Jar < keys[j].Jar
		}
		return keys[i].ContentType < keys[j].ContentType
	})
	var total DocJarRow
	for _, k := range keys {
		r := *s.Rows[k]
		total.InJar += r.InJar
		total.Extracted += r.Extracted
		total.Indexed += r.Indexed
		total.Excluded += r.Excluded
		total.Gap += r.Gap
		mark := ""
		if r.Gap != 0 {
			mark = "  <- GAP"
		}
		out.line(docJarLine(pdftext.FirstRunes(k.Jar, 34), pdftext.FirstRunes(k.ContentType, 20), r) + mark)
	}
	out.line(docJarLine("TOTAL", "", total))
	out.line("")
	out.line("EXCLUDED WITH REASON:")
	reasons := append([]ReasonCount(nil), s.Reasons...)
	sort.SliceStable(reasons, func(i, j int) bool { return reasons[i].Count > reasons[j].Count })
	for _, r := range reasons {
		out.line("  " + pyFmt(r.Count, ">7,") + "  " + r.Reason)
	}
	if len(s.Gaps) > 0 {
		out.line("")
		n := min(FirstGaps, len(s.Gaps))
		out.line(fmt.Sprintf("GAPS (first %d of %d):", n, len(s.Gaps)))
		for _, g := range s.Gaps[:n] {
			out.line(fmt.Sprintf("  %s  %s  -- %s", g.Jar, g.Rel, g.Why))
		}
	}
}
