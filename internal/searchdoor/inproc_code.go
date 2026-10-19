package searchdoor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/codesearch"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The code source's token and grep calls through codesearch, on the checkout
// module_nav.py would use: the backend's checkout (toolDir) when that holds
// indexes/, and $NAV_CORPUS_BASE or the organized/ folder beside it.

// navDir is module_nav's detect_base_dir for the script: "" without indexes/.
func navDir(script string) string {
	if dir := toolDir(script, false); dir != "" && isDir(filepath.Join(dir, "indexes")) {
		return dir
	}
	return ""
}

// codeToken is `module_nav token <word> -n N` (rankedCode): the compact table and
// the module summary, or the miss (tokenMiss). navRun (mnav token) answers when the token index
// is missing or unreadable (module_nav's exit-2 errors).
func codeToken(argv []string) answer {
	v, _, ok := shape(argv[1:], []string{"token", "*", "-n", "#"})
	base := navDir(argv[0])
	if !ok || base == "" {
		return noAnswer
	}
	path := core.IndexPath(base, "token-index.db")
	if !isFile(path) {
		return noAnswer
	}
	db, err := index.OpenSQLite(path)
	if err != nil {
		return noAnswer
	}
	defer db.Close()
	word := v[0]
	limit, _ := strconv.Atoi(v[1])
	hits, err := codesearch.Tokens(db, word, "")
	if err != nil {
		return noAnswer
	}
	var b strings.Builder
	if len(hits) == 0 { // source.tokenMiss without a module filter, minus its similar tokens
		return exact(1, tokenMiss(word), "")
	}
	fmt.Fprintf(&b, "\n%s\n\n", codesearch.FormatTokenHeader(word, "", hits))
	head := codesearch.TokenTableHeader()
	fmt.Fprintf(&b, "%s\n%s\n", head[0], head[1])
	mods := codesearch.ByModule(hits)
	tokenRows(&b, hits, mods, limit)
	b.WriteString("  Top modules:\n")
	for _, m := range codesearch.TopModules(mods, 10) {
		b.WriteString(codesearch.FormatModuleHits(m) + "\n")
	}
	if len(mods) > 10 {
		fmt.Fprintf(&b, "    ... and %d more modules\n", len(mods)-10)
	}
	b.WriteString("\n")
	return exact(0, b.String(), "")
}

// tokenMiss is module_nav's token miss WITHOUT its "Similar tokens:" line
// (intended difference): that line is a LIKE scan of every posting, seconds on
// the real 1 GB index (slower in pure-Go SQLite than the Python run it would
// replace), and rankedCode reads only the state of a token miss (a miss line,
// exit 1: EMPTY with no reason), never its text, so nsearch prints the same bytes.
func tokenMiss(word string) string {
	return "\n  0 matches for token '" + word + "' in the postings of token-index.db.\n\n"
}

// tokenRows is source.tokenCompact's table: up to limit rows by module, then a
// blank line (or the "... and N more" line).
func tokenRows(b *strings.Builder, hits []codesearch.TokenHit, mods []codesearch.ModuleHits, limit int) {
	shown := 0
	for _, m := range mods {
		for _, h := range m.Hits {
			if shown >= limit {
				fmt.Fprintf(b, "  ... and %s more (use -n %s to see all)\n\n",
					core.Commas(int64(len(hits)-shown)), core.Commas(int64(len(hits))))
				return
			}
			b.WriteString(codesearch.FormatTokenRow(h) + "\n")
			shown++
		}
	}
	b.WriteString("\n")
}

// codeGrep is `module_nav grep <regex> -n N` (the degraded path of ranked): the
// '[module]' groups of 'Class:line text' rows, or the counted miss. navRun (mnav grep) answers
// an invalid regex and a missing corpus (module_nav's exit-2 errors).
func codeGrep(getenv envFunc, argv []string) answer {
	v, _, ok := shape(argv[1:], []string{"grep", "*", "-n", "#"})
	base := navDir(argv[0])
	if !ok || base == "" {
		return noAnswer
	}
	pattern := v[0]
	limit, _ := strconv.Atoi(v[1])
	re, err := pyre.Compile(pattern, 0)
	if err != nil {
		return noAnswer
	}
	organized := getenv("NAV_CORPUS_BASE")
	if organized == "" {
		organized = filepath.Join(filepath.Dir(base), "organized")
	}
	if !isDir(organized) {
		return noAnswer
	}
	res := codesearch.Grep(organized, re, limit, codesearch.GrepFilter{})
	if len(res.Hits) == 0 {
		return exact(1, fmt.Sprintf("  0 matches for /%s/ in %s.\n", pattern,
			codesearch.GrepScope(organized, res, codesearch.GrepFilter{})), "")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n\n", codesearch.FormatGrepHeader(pattern, res, limit))
	for i, h := range res.Hits {
		if i == 0 || h.Sub != res.Hits[i-1].Sub {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(codesearch.FormatGrepModule(h.Sub) + "\n")
		}
		b.WriteString(codesearch.FormatGrepRow(h) + "\n")
	}
	if res.Full(limit) {
		fmt.Fprintf(&b, "\n  ... limit reached (%d) — use -n to increase\n", limit)
	}
	b.WriteString("\n")
	return exact(0, b.String(), "")
}
