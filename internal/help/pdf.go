package help

// pdf (pdf_search.cmd_pdf): the ranked full-text search over the PDF manual
// text (docs-text/), or a name match with --no-fulltext. A version without
// manuals searches the shared ones and says so.

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// PDFParams are the arguments of `pdf`.
type PDFParams struct {
	Query      string
	NoFulltext bool
	Category   *string // --category: software or hardware
}

// pdfMaxResults is cmd_pdf's max_results.
const pdfMaxResults = 10

// pdfMetadata is pdf_search._metadata: pdf-index.json of the data root that
// owns the searched docs-text/ (empty when absent).
func (e *Env) pdfMetadata() (*index.Object, error) {
	root, _ := fts.DocsTextRoot(e.BaseDir)
	if root == e.BaseDir {
		return e.loadIndex("pdf-index.json", false)
	}
	path := index.Path(root, "pdf-index.json")
	if _, err := os.Stat(path); err != nil {
		return index.NewObject(), nil
	}
	return index.LoadObject(path)
}

// pdfNameMatches is pdf_search._name_matches: the manuals (sorted) whose
// name and source PDF hold every query word.
func pdfNameMatches(meta *index.Object, query string) []string {
	groups := fts.ParseQuery(query, false)
	names := append([]string(nil), meta.Keys()...)
	sort.Strings(names)
	var out []string
	for _, name := range names {
		terms := fts.TermSet(strings.ReplaceAll(name, "_", " ") + " " + getStr(meta.Object(name), "source_pdf", ""))
		all := true
		for _, g := range groups {
			all = all && g.Matches(terms)
		}
		if all {
			out = append(out, name)
		}
	}
	return out
}

// PDF is pdf_search.cmd_pdf.
func (e *Env) PDF(p PDFParams) error {
	meta, err := e.pdfMetadata()
	if err != nil {
		return err
	}
	_, sharedFrom := fts.DocsTextRoot(e.BaseDir)
	var r *fts.Result
	var names []string
	hits := map[string]*fts.Hit{}
	if p.NoFulltext {
		names = pdfNameMatches(meta, p.Query)
	} else {
		if r, err = e.search(p.Query, []string{"docs-text"}, "and"); err != nil {
			return err
		}
		for _, h := range r.Hits {
			name, _ := pycompat.PosixSplitext(h.Path)
			names = append(names, name)
			hits[name] = h
		}
	}
	if p.Category != nil {
		var kept []string
		for _, n := range names {
			if c, ok := get(meta.Object(n), "category", nil).(string); ok && c == *p.Category {
				kept = append(kept, n)
			}
		}
		names = kept
	}
	if len(names) == 0 {
		return e.reportMiss("pdf", p.Query, "pdf")
	}
	how := ""
	if r != nil && r.Fallback {
		how = fmt.Sprintf(" (fewer than %d manuals have every word; ranked by how many match)", fts.MinAndHits)
	}
	fmt.Fprintf(e.Out, "%d PDF manuals matching '%s'%s:\n", len(names), p.Query, how)
	if sharedFrom != "" {
		fmt.Fprintf(e.Out, "  (this version ships no PDF manuals; searching the shared manuals of Niagara %s)\n",
			sharedFrom)
	}
	fmt.Fprintln(e.Out)
	dirs := fts.SourceDirs(e.BaseDir)
	for _, name := range names[:min(len(names), pdfMaxResults)] {
		data := meta.Object(name)
		fmt.Fprintf(e.Out, "  %s (%s pages, %s KB, %s)\n", strings.ReplaceAll(name, "_", " "),
			getStr(data, "pages", "?"), formatF0(get(data, "size_kb", 0)), getStr(data, "category", "?"))
		fmt.Fprintf(e.Out, "    source: %s\n", getStr(data, "source_pdf", "?"))
		if h := hits[name]; h != nil {
			if n, text := fts.Snippet(dirs, h, r.Groups); n != 0 {
				page := ""
				if h.Page != 0 {
					page = fmt.Sprintf(" p.%d", h.Page)
				}
				fmt.Fprintf(e.Out, "    L%d%s: %s\n", n, page, text)
			}
		}
		fmt.Fprintf(e.Out, "    path: docs-text/%s.txt\n\n", name)
	}
	if len(names) > pdfMaxResults {
		fmt.Fprintf(e.Out, "  ... and %d more manuals\n", len(names)-pdfMaxResults)
	}
	return nil
}

// formatF0 is "{:.0f}".format(v) for a JSON number (round half to even on
// the binary value, as strconv does).
func formatF0(v any) string {
	switch x := v.(type) {
	case int:
		return strconv.Itoa(x)
	case float64:
		return strconv.FormatFloat(x, 'f', 0, 64)
	}
	f, ok := index.Float(v)
	if !ok {
		return pyStr(v)
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}
