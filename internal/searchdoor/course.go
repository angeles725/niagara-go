package searchdoor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
)

// The course code: the N4 developer course's Java as a labelled, BM25-ranked
// sub-scope of the CODE source (searchdoor/course.py). Every *.java under
// <root>/course (labs, slide code), <root>/solutions (the verified exam and
// review solutions) and <root>/niagara-4.15.6.20/dev (the dev kit examples) is
// ranked with bm25.Textsearch over the class name (camel case split, the
// title), the relative path and the content; a Spanish question also ranks its
// English translations (glossary.Default().WeightedTerms).

const courseTitle = "course code"

var (
	courseAreas = [][2]string{{"course", "course"}, {"solutions", "verified solution"},
		{"niagara-4.15.6.20/dev", "dev kit example"}}
	camelRx = re(`[A-Z]+(?![a-z])|[A-Z]?[a-z]+|[0-9]+`)
)

type courseFile struct{ label, path, rel string }

// courseFiles lists (area label, path) for every .java under the three areas,
// each area in pathlib's sorted(rglob("*.java")) order (pypath.Less: path parts
// compared in turn, case-folded on Windows; symlinked directories are not entered).
func courseFiles(root string) []courseFile {
	var out []courseFile
	for _, area := range courseAreas {
		base := filepath.Join(root, filepath.FromSlash(area[0]))
		if st, err := os.Stat(base); err != nil || !st.IsDir() {
			continue
		}
		var paths []string
		_ = filepath.WalkDir(base, func(p string, e fs.DirEntry, err error) error {
			if err == nil && p != base && strings.HasSuffix(e.Name(), ".java") {
				if st, serr := os.Stat(p); serr == nil && !st.IsDir() {
					paths = append(paths, p)
				}
			}
			return nil
		})
		sort.Slice(paths, func(i, j int) bool { return pypath.Less(paths[i], paths[j]) })
		for _, p := range paths {
			rel, _ := filepath.Rel(root, p)
			out = append(out, courseFile{area[1], p, filepath.ToSlash(rel)})
		}
	}
	return out
}

// stem is PurePath.stem: the last part without its last suffix.
func stem(rel string) string {
	name := rel[strings.LastIndex(rel, "/")+1:]
	if i := strings.LastIndex(name, "."); i > 0 {
		return name[:i]
	}
	return name
}

// courseIndex builds the BM25 index; a deadline passed while reading files
// is reported as (nil, "read N of M .java files").
func (d *door) courseIndex(root string, deadline float64) (*bm25.Index[int], []courseFile, map[string]int, string) {
	files := courseFiles(root)
	docs := make([]bm25.Doc[int], 0, len(files))
	counts := map[string]int{}
	for n, f := range files {
		if d.now() >= deadline {
			return nil, nil, nil, fmt.Sprintf("read %d of %d .java files", n, len(files))
		}
		text, _ := pycompat.ReadTextReplace(f.path)
		s := stem(f.rel)
		title := s + " " + strings.Join(camelRx.FindAll(s), " ")
		docs = append(docs, bm25.Doc[int]{Key: n, Title: title, Body: strings.ReplaceAll(f.rel, "/", " ") + "\n" + text})
		counts[f.label]++
	}
	return bm25.NewIndex(docs, bm25.Textsearch, 8), files, counts, ""
}

// courseSearch is course.search: the course-code sub-scope for query (all
// ranked rows; the renderer caps them). Past the CODE source's deadline the
// index cannot be built and the sub-scope is a named timeout.
func (d *door) courseSearch(root, query string, deadline float64) *Outcome {
	command := "BM25 over " + root + "/{course,solutions,niagara-4.15.6.20/dev}/**/*.java"
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return unavailable("code", courseTitle, command, "unavailable: course root not found: "+pypath.PurePosixPath(root))
	}
	ix, files, counts, spent := d.courseIndex(root, deadline)
	if spent != "" {
		when := "ran out building the course index"
		if strings.HasPrefix(spent, "read 0 ") {
			when = "was spent before the course code"
		}
		return unavailable("code", courseTitle, command, "unavailable: timeout, the CODE budget "+when+" ("+spent+")")
	}
	o := newOutcome("code", courseTitle, command, OK)
	parts := make([]string, len(courseAreas))
	for i, area := range courseAreas {
		parts[i] = fmt.Sprintf("%s %d", area[1], counts[area[1]])
	}
	o.Scope = []string{fmt.Sprintf("searched %d .java files of the N4 developer course (%s)", len(files),
		strings.Join(parts, ", "))}
	if len(files) == 0 {
		o.State = Unavailable
		o.Reason = "unavailable: no .java file under " + pypath.PurePosixPath(root) + " (course, solutions, niagara-4.15.6.20/dev)"
		return o
	}
	for _, hit := range ix.Rank(query, false, glossary.Default().WeightedTerms(query)) {
		f := files[hit.Key]
		s := stem(f.rel)
		o.Items = append(o.Items, keyed(s, fmt.Sprintf("%s  [%s]  score %.3f", s, f.label, hit.Score), clip(f.rel, snippetWidth)))
	}
	if len(o.Items) == 0 {
		o.State = Empty
		o.Scope[0] = "0 matches for " + pycompat.StrRepr(query) + "; " + o.Scope[0]
	}
	return o
}
