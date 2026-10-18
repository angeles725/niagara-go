package searchdoor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/nav/classrank"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// codeRank is `module_nav rank -n N -- <question>` (ranked) through classrank:
// the header, the tab-separated rows and the "... and N more" line, or the
// counted miss. Exec answers when the class index cannot be opened (module_nav
// exits 2 there, which sends the code source to its DEGRADED grep) and when the
// xref index is absent (its NOTE line is module_nav's own printing).
func codeRank(argv []string) answer {
	args := argv[1:]
	if len(args) != 5 || args[0] != "rank" || args[1] != "-n" || args[3] != "--" {
		return viaExec
	}
	limit, err := strconv.Atoi(args[2])
	base := navDir(argv[0])
	if err != nil || limit < 1 || strconv.Itoa(limit) != args[2] || base == "" {
		return viaExec
	}
	src := classrank.SourceOf(base)
	if !isFile(src.XrefIndex) {
		return viaExec
	}
	r, err := classrank.Open(src)
	if err != nil {
		return viaExec
	}
	question := args[4]
	results := r.Rank(question)
	terms := strings.Join(r.QueryTerms(question), ", ")
	var b strings.Builder
	if len(results) == 0 { // classrank.Cmd's miss: env.Miss with its hint
		if terms == "" {
			terms = "none"
		}
		fmt.Fprintf(&b, "  0 matches for question '%s' (terms: %s) in %s.\n", question, terms, r.Scope())
		b.WriteString("  Try: grep '<regex>' for literal text, or search '*Name*' for class names\n")
		return exact(1, b.String(), "")
	}
	fmt.Fprintf(&b, "\n  Rank '%s': %s matching classes (terms: %s)\n", question, core.Commas(int64(len(results))), terms)
	b.WriteString("  Classes named in the question first, then BM25 x importer prior; " +
		"columns: rank, score, class, module, path\n\n")
	for k, x := range results[:min(limit, len(results))] {
		b.WriteString(classrank.FormatRow(k+1, x) + "\n")
	}
	if len(results) > limit {
		fmt.Fprintf(&b, "\n  ... and %s more (use -n to show more)\n", core.Commas(int64(len(results)-limit)))
	}
	b.WriteString("\n")
	return exact(0, b.String(), "")
}
