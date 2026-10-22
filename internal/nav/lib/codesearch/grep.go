package codesearch

import (
	"fmt"
	"strconv"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// GrepHit is one matching source line of a grep.
type GrepHit struct {
	Class string // the file name without .java
	Sub   string // the submodule (organized/<module>/<sub>/)
	Rel   string // the file path relative to organized/, forward slashes
	Line  int    // 1-based line number
	Text  string // the line, right-stripped
}

// GrepFilter narrows a grep to one submodule name (Sub) or one submodule
// kind (Type, a corpus.submodule_kind value); empty fields do not filter.
type GrepFilter struct{ Sub, Type string }

// GrepResult is what a grep found and what it searched before it stopped.
type GrepResult struct {
	Hits             []GrepHit
	FilesSearched    int // files read, up to the one that reached the limit
	FilesWithMatches int
	Submodules       int // distinct submodule names searched
}

// Full reports that the grep holds limit hits or more (Python's
// len(results) >= limit), the condition that stops it.
func (r GrepResult) Full(limit int) bool { return len(r.Hits) >= limit }

// Grep is the search of grep_search.cmd_grep: re.search over every line
// (newline included) of every vineflower .java file under organized/ in
// corpus order, stopping after the hit that reaches limit (an unreadable file
// is counted as searched and skipped).
func Grep(organized string, re *pyre.Regexp, limit int, f GrepFilter) GrepResult {
	var res GrepResult
	subs := map[string]bool{}
	var buf []rune
	keep := func(s core.Submodule) bool {
		return (f.Sub == "" || s.Name == f.Sub) && (f.Type == "" || core.SubmoduleKind(s.Module, s.Name) == f.Type)
	}
	core.WalkJava(organized, keep, func(jf core.JavaFile) bool {
		res.FilesSearched++
		subs[jf.Sub] = true
		text, err := pycompat.ReadTextReplace(jf.Path)
		if err != nil {
			return true
		}
		// Every line is a substring of the file: a file the pattern cannot
		// match (the literal fast path of pyre) has no hit to look for.
		if !re.MayMatch(text) {
			return !res.Full(limit)
		}
		lines := pycompat.Lines(text)
		cname, _ := pypath.PosixSplitext(pypath.PosixBasename(jf.Rel))
		matched := false
		for i, line := range lines {
			if !re.MayMatch(line) {
				continue
			}
			buf = buf[:0] // one rune buffer for every line: no allocation per line
			for _, r := range line {
				buf = append(buf, r)
			}
			if re.SearchRunes(buf, 0) == nil {
				continue
			}
			if !matched {
				matched = true
				res.FilesWithMatches++
			}
			res.Hits = append(res.Hits, GrepHit{cname, jf.Sub, jf.Rel, i + 1, RStrip(line)})
			if res.Full(limit) {
				break
			}
		}
		// Checked after every file read: a limit of 0 or less ends the walk
		// after the first readable file, with or without a hit.
		return !res.Full(limit)
	})
	res.Submodules = len(subs)
	return res
}

// FormatGrepHeader is the "  grep /<pattern>/ — N matches in F files (S files
// searched)" line; N is "<limit>+" when the grep stopped at the limit.
func FormatGrepHeader(pattern string, r GrepResult, limit int) string {
	n := strconv.Itoa(len(r.Hits))
	if r.Full(limit) {
		n = strconv.Itoa(limit) + "+"
	}
	return fmt.Sprintf("  grep /%s/ — %s matches in %d files (%s files searched)",
		pattern, n, r.FilesWithMatches, core.Commas(int64(r.FilesSearched)))
}

// FormatGrepModule is the "  [<submodule>]" line that opens a submodule's hits.
func FormatGrepModule(sub string) string { return "  [" + sub + "]" }

// FormatGrepRow is the "    Class:line text" line of a hit, the text cut to
// 120 code points.
func FormatGrepRow(h GrepHit) string {
	return fmt.Sprintf("    %s:%d %s", h.Class, h.Line, Ellipsis(h.Text, 120))
}

// GrepScope is the counted description of what a grep without hits searched.
func GrepScope(organized string, r GrepResult, f GrepFilter) string {
	scope := fmt.Sprintf("%s .java files in %s submodules under %s",
		core.Commas(int64(r.FilesSearched)), core.Commas(int64(r.Submodules)), organized)
	if f.Sub != "" {
		scope += " (--module " + f.Sub + ")"
	} else if f.Type != "" {
		scope += " (--type " + f.Type + ")"
	}
	return scope
}
