package helpbuild

import (
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The whole-install sweep (niagara_help_lib/install_sweep.py, the
// `coverage --sweep` gate and a required gate of `versions add`): every
// documentation-like entry of every jar of an install, and every file of
// its docs/ folder, is indexed, excluded with a reason, or UNCLASSIFIED.
// A directory the walk cannot list is UNCLASSIFIED too. sweepreport.go
// prints the report and runs the gate (cmd_sweep).

// Entry statuses of the sweep.
const (
	statusIndexed      = "indexed"
	statusExcluded     = "excluded"
	statusUnclassified = "UNCLASSIFIED"
)

// docsPseudoJar is the jar name the exclude rules see for a loose file
// under the install's docs/.
const docsPseudoJar = "(docs)"

// jarPath is one jar the sweep found: its install-relative path
// ("/"-separated) and its path on disk.
type jarPath struct{ Rel, Path string }

// dirError is a directory the walk could not list and the strerror.
type dirError struct{ Dir, Err string }

// pyWalkErr is os.walk(top, onerror=...) as pycompat.Walk walks it; onerror
// receives the path of each directory that cannot be listed, relative to
// top ("." for top itself), and the strerror (str(err) without one).
func pyWalkErr(top string, onerror func(rel, why string), visit func(dir, rel string, dirs, files []string) []string) {
	rels := map[string]string{top: ""}
	pycompat.WalkWith(top, pycompat.WalkOptions{OnError: func(dir string, err error) {
		if onerror != nil {
			rel := strings.TrimSuffix(rels[dir], "/")
			if rel == "" {
				rel = "."
			}
			onerror(rel, pycompat.StrerrorOf(err))
		}
	}}, func(dir string, dirs, files []string) []string {
		descend := visit(dir, rels[dir], dirs, files)
		for _, d := range descend {
			rels[pycompat.Join(dir, d)] = rels[dir] + d + "/"
		}
		return descend
	})
}

// findJars is install_sweep.find_jars: every .jar (any letter case) of the
// install in sorted walk order, the top-level directories the rules skip
// (with their reason) and the directories that could not be listed.
func findJars(installDir string, rules *Rules) (jars []jarPath, skipped []SkipDir, unreadable []dirError) {
	pyWalkErr(installDir, func(rel, why string) {
		unreadable = append(unreadable, dirError{rel, why})
	}, func(dir, rel string, dirs, files []string) []string {
		if dir == installDir {
			sort.Strings(dirs)
			kept := dirs[:0:0]
			for _, d := range dirs {
				if why, ok := rules.SkipReason(d); ok {
					skipped = append(skipped, SkipDir{d, why})
				} else {
					kept = append(kept, d)
				}
			}
			dirs = kept
		}
		sort.Strings(dirs)
		sort.Strings(files)
		for _, name := range files {
			if strings.HasSuffix(textfold.Lower(name), ".jar") {
				jars = append(jars, jarPath{rel + name, pycompat.Join(dir, name)})
			}
		}
		return dirs
	})
	return jars, skipped, unreadable
}

// sweepLocation is install_sweep.location: the report row of a jar.
func sweepLocation(rel string) string {
	top, _, nested := strings.Cut(rel, "/")
	if !nested {
		return "(root)"
	}
	return top + "/"
}

// isInstalledJar is install_sweep.is_installed: modules/<jar> or
// javadoc/<jar>, the jars the extractor reads.
func isInstalledJar(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 2 && (parts[0] == "modules" || parts[0] == "javadoc")
}

// SweepOptions tune scan and RunSweep; the zero value is Python's
// defaults (install_rules.default(), an in-memory listing cache, the
// full-text index read only).
type SweepOptions struct {
	Rules    *Rules       // nil: Default()
	Listings *Listings    // scan only; nil: in memory (RunSweep uses the repo's cache)
	Fulltext FulltextDocs // nil: FreshFulltextDocs
}

// sweeper is install_sweep.Sweep.
type sweeper struct {
	root      string
	rules     *Rules
	ls        *Listings
	indexed   *indexedViews
	installed []jarPath
	identical map[Entry]string
}

// installedEntry is Sweep.installed_entry: the install path of an
// installed jar holding this exact entry (path, size, CRC-32), "" if none.
func (s *sweeper) installedEntry(e Entry) string {
	if s.identical == nil {
		s.identical = map[Entry]string{}
		for _, j := range s.installed {
			if l := s.ls.Listing(j.Path); l != nil {
				for _, x := range l.Entries {
					if _, seen := s.identical[x]; !seen && s.rules.IsDocLike(x.Name) {
						s.identical[x] = j.Rel
					}
				}
			}
		}
	}
	return s.identical[e]
}

// verdict is one classification: status, detail (reason or message) and
// the rule id ("" for an UNCLASSIFIED entry).
type verdict struct{ status, detail, rule string }

// mapped is Sweep._mapped.
func (s *sweeper) mapped(rel string) (verdict, error) {
	v, why := classify(rel)
	switch v {
	case verdictExclude:
		return verdict{statusExcluded, why, "doc_types"}, nil
	case verdictIndex:
		isFile := pycompat.IsFile(pycompat.Join(s.root, rel))
		if isFile {
			ok, err := s.indexed.check(why, rel)
			if err != nil {
				return verdict{}, err
			}
			if ok {
				return verdict{statusIndexed, rel, why}, nil
			}
		}
		state := "extracted"
		if isFile {
			state = "indexed"
		}
		return verdict{statusUnclassified, "maps to " + rel + " but it is not " + state, ""}, nil
	}
	return verdict{statusUnclassified, "maps to " + rel + " but no content rule covers it (doc_types)", ""}, nil
}

// classify is Sweep.classify: one documentation-like entry of the jar at
// install path relJar; shadowedBy is the earlier entry whose corpus file
// this one collides with (nil when none).
func (s *sweeper) classify(relJar string, e Entry, shadowedBy *Entry) (verdict, error) {
	name := pyBasename(relJar)
	installed := isInstalledJar(relJar)
	if !installed && s.installedEntry(e) != "" {
		return verdict{statusExcluded, "byte-identical (path, size, CRC-32) to the entry of an " +
			"installed jar, classified there", "installed-copy"}, nil
	}
	dest, mapped := destPath(name, e.Name, s.rules)
	if installed && mapped {
		if isSourceArchive(dest) {
			return verdict{statusExcluded, reasons["source-archive"], "doc_types"}, nil
		}
		if shadowedBy != nil {
			return shadowed(e, *shadowedBy, dest), nil
		}
		return s.mapped(dest)
	}
	if rule := s.rules.ExcludeRule(name, e.Name); rule != nil {
		return verdict{statusExcluded, rule.Reason, rule.ID}, nil
	}
	if mapped {
		return verdict{statusUnclassified, "documentation in a jar outside modules/ that the installed " +
			"copy does not have (not extracted)", ""}, nil
	}
	return verdict{statusUnclassified, "no extract or exclude rule matches (add one to install_docs.json)", ""}, nil
}

// shadowed is Sweep._shadowed: an entry the extractor skips because an
// earlier entry of the same jar fills its corpus path.
func shadowed(e, first Entry, dest string) verdict {
	if e.Size == first.Size && e.CRC == first.CRC {
		return verdict{statusExcluded, "same corpus file as an earlier entry of this jar, byte-identical " +
			"(size, CRC-32): that entry is extracted", "in-jar-duplicate"}
	}
	return verdict{statusUnclassified, "maps to " + dest + ", which the earlier entry " + first.Name +
		" of this jar fills: not extracted, and its content differs", ""}
}

// shadowedEntries is Sweep.shadowed_entries: position in the listing ->
// the earlier entry of an installed jar the extractor writes instead.
func (s *sweeper) shadowedEntries(relJar string, l *Listing) map[int]Entry {
	if !isInstalledJar(relJar) {
		return nil
	}
	names := make([]string, len(l.Entries))
	for i, e := range l.Entries {
		names[i] = e.Name
	}
	extracted := map[string]Entry{}
	out := map[int]Entry{}
	for _, m := range mappedEntries(pyBasename(relJar), names, s.rules) {
		if !m.Collides {
			extracted[m.Rel] = l.Entries[m.Pos]
		} else {
			out[m.Pos] = extracted[m.Rel]
		}
	}
	return out
}

// classifyLooseDoc is Sweep.classify_loose_doc: a file under docs/.
func (s *sweeper) classifyLooseDoc(rel string, pdfSources map[string]bool) verdict {
	low := textfold.Lower(rel)
	if strings.HasSuffix(low, ".pdf") {
		if pdfSources[low] {
			return verdict{statusIndexed, "docs-text (pdf-index.json)", "pdf"}
		}
		return verdict{statusUnclassified, "PDF manual not in pdf-index.json (run rebuild-all)", ""}
	}
	if rule := s.rules.ExcludeRule(docsPseudoJar, rel); rule != nil {
		return verdict{statusExcluded, rule.Reason, rule.ID}
	}
	return verdict{statusUnclassified, `no exclude rule matches (add one to install_docs.json, jar "(docs)")`, ""}
}

// pdfSources are the lower-cased source_pdf values of pdf-index.json
// (none when the file does not exist).
func pdfSources(root string) (map[string]bool, error) {
	out := map[string]bool{}
	if !pycompat.IsFile(pycompat.Join(root, "indexes", "pdf-index.json")) {
		return out, nil
	}
	const name = "pdf-index.json"
	idx, err := loadReportIndex(root, name, false)
	if err != nil {
		return nil, err
	}
	for i, key := range idx.Keys() {
		v, err := recordOf(name, key, idx.Values()[i])
		if err != nil {
			return nil, err
		}
		src, _ := v.Get("source_pdf")
		switch str, isStr := src.(string); {
		case !pyTruthy(src):
			out[""] = true
		case isStr:
			out[textfold.Lower(str)] = true
		default:
			return nil, &indexShapeError{name, "the source_pdf of " + key + " is not a string"}
		}
	}
	return out, nil
}

// sweepRow is one location row of the report.
type sweepRow struct{ Jars, WithDocs, DocLike, Indexed, Excluded, Unclassified int }

func (r *sweepRow) add(status string) {
	switch status {
	case statusIndexed:
		r.Indexed++
	case statusExcluded:
		r.Excluded++
	default:
		r.Unclassified++
	}
}

// reasonKey is one exclusion reason of the report: the rule id and the
// reason.
type reasonKey struct{ Rule, Reason string }

// gap is one UNCLASSIFIED row: the jar (or "docs/", "(course)", a
// directory), the entry and why.
type gap struct{ Jar, Entry, Why string }

// sweepResult is the dict install_sweep.scan returns.
type sweepResult struct {
	Rows         map[string]*sweepRow // per location
	Reasons      map[reasonKey]int
	Unclassified []gap
	Skipped      []SkipDir
	Jars         int // jars found
	Listed       int // jars listed anew (not from the cache)
	Unreadable   []string
}

func (r *sweepResult) row(loc string) *sweepRow {
	if r.Rows[loc] == nil {
		r.Rows[loc] = &sweepRow{}
	}
	return r.Rows[loc]
}

// count is scan's count(): one documentation-like entry.
func (r *sweepResult) count(row *sweepRow, v verdict, jar, entry string) {
	row.DocLike++
	row.add(v.status)
	switch v.status {
	case statusExcluded:
		r.Reasons[reasonKey{v.rule, v.detail}]++
	case statusUnclassified:
		r.Unclassified = append(r.Unclassified, gap{jar, entry, v.detail})
	}
}

// isEmptyFile is install_sweep._is_empty: a size of 0 (a stat error is not
// empty).
func isEmptyFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() == 0
}

// scan is install_sweep.scan: classify the whole install against the data
// root. The listing cache is saved at the end (Listings.Save). An error is
// what Python raises (an index that cannot be read, a cache that cannot be
// written).
func scan(installDir, root string, opts SweepOptions) (*sweepResult, error) {
	rules := opts.Rules
	if rules == nil {
		var err error
		if rules, err = Default(); err != nil {
			return nil, err
		}
	}
	ls := opts.Listings
	if ls == nil {
		ls = newListings("")
	}
	s := &sweeper{root: root, rules: rules, ls: ls, indexed: newIndexedViews(root, opts.fulltext())}
	jars, skipped, badDirs := findJars(installDir, rules)
	for _, j := range jars {
		if isInstalledJar(j.Rel) {
			s.installed = append(s.installed, j)
		}
	}
	res := &sweepResult{Rows: map[string]*sweepRow{}, Reasons: map[reasonKey]int{}, Skipped: skipped, Jars: len(jars)}
	for _, j := range jars {
		row := res.row(sweepLocation(j.Rel))
		row.Jars++
		l := ls.Listing(j.Path)
		if l == nil && isEmptyFile(j.Path) {
			row.DocLike++
			row.Excluded++
			res.Reasons[reasonKey{"empty-file", "empty file named .jar (0 bytes, nothing to list)"}]++
			continue
		}
		if l == nil {
			res.Unreadable = append(res.Unreadable, j.Rel)
			row.Unclassified++
			res.Unclassified = append(res.Unclassified, gap{j.Rel, "(whole jar)", "unreadable jar (corrupt zip or I/O error)"})
			continue
		}
		var docs []int
		for pos, e := range l.Entries {
			if rules.IsDocLike(e.Name) {
				docs = append(docs, pos)
			}
		}
		if len(docs) == 0 {
			continue
		}
		row.WithDocs++
		shadow := s.shadowedEntries(j.Rel, l)
		for _, pos := range docs {
			var by *Entry
			if first, ok := shadow[pos]; ok {
				by = &first
			}
			v, err := s.classify(j.Rel, l.Entries[pos], by)
			if err != nil {
				return nil, err
			}
			res.count(row, v, j.Rel, l.Entries[pos].Name)
		}
	}
	docsBad, err := s.scanLooseDocs(installDir, res)
	if err != nil {
		return nil, err
	}
	// A directory the walk could not list may hide documentation: the gate
	// cannot pass over it; one met by both walks is reported once.
	reported := map[string]bool{}
	for _, d := range append(badDirs, docsBad...) {
		if reported[d.Dir] {
			continue
		}
		reported[d.Dir] = true
		res.row(sweepLocation(d.Dir+"/x")).Unclassified++
		res.Unclassified = append(res.Unclassified, gap{d.Dir, "(directory)", "unreadable directory, not swept: " + d.Err})
	}
	if err := ls.Save(); err != nil {
		return nil, err
	}
	res.Listed = ls.Reads
	return res, nil
}

// scanLooseDocs classifies the documentation-like files under the
// install's docs/ and returns the directories it could not list (as
// install paths).
func (s *sweeper) scanLooseDocs(installDir string, res *sweepResult) ([]dirError, error) {
	docsDir := pycompat.Join(installDir, "docs")
	if !pycompat.IsDir(docsDir) {
		return nil, nil
	}
	sources, err := pdfSources(s.root)
	if err != nil {
		return nil, err
	}
	row := res.row("docs/ (loose files)")
	var bad []dirError
	pyWalkErr(docsDir, func(rel, why string) {
		if rel == "." {
			bad = append(bad, dirError{"docs", why})
		} else {
			bad = append(bad, dirError{"docs/" + rel, why})
		}
	}, func(_, rel string, dirs, files []string) []string {
		sort.Strings(dirs)
		sort.Strings(files)
		for _, name := range files {
			if r := rel + name; s.rules.IsDocLike(r) {
				res.count(row, s.classifyLooseDoc(r, sources), "docs/", r)
			}
		}
		return dirs
	})
	return bad, nil
}
