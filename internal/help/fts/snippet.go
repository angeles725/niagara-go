package fts

// Snippets (fulltext.py snippet, clip): the body line of a hit's best
// document that matches the most query groups, shortened around its first
// match.

import (
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// snippetWidth is the default width of snippet and clip.
const snippetWidth = 160

// isHeaderLine is fulltext_index._HEADER (`^([A-Z][A-Z ]*):\s?(.*)$`)
// matching a line split at "\n": an upper-case key, then a colon.
func isHeaderLine(line string) bool {
	if line == "" || !isUpper(line[0]) {
		return false
	}
	j := 1
	for j < len(line) && (isUpper(line[j]) || line[j] == ' ') {
		j++
	}
	return j < len(line) && line[j] == ':'
}

// bodyStart is the index part of fulltext_index._split_header: the first
// body line after the "KEY: value" lines and one blank line (0 without a header).
func bodyStart(lines []string) int {
	i := 0
	for i < len(lines) && isHeaderLine(lines[i]) {
		i++
	}
	if i > 0 && i < len(lines) && pycompat.Strip(lines[i]) == "" {
		i++
	}
	return i
}

type numbered struct {
	n    int
	text string
}

// candidateLines is fulltext._candidate_lines: the numbered lines of the
// hit's best document, its page for paged text (pages split at form feeds,
// the file header skipped), else the body after the header.
func candidateLines(dirs []SourceDir, h *Hit) []numbered {
	dir := DirOf(dirs, h.Source)
	if dir == "" {
		return nil
	}
	text, err := pycompat.ReadTextReplace(filepath.Join(dir, filepath.FromSlash(h.Path)))
	if err != nil {
		return nil
	}
	lines := strings.Split(text, "\n")
	start := bodyStart(lines)
	var out []numbered
	if h.Page != 0 {
		pages := strings.Split(text, "\f")
		page := ""
		if h.Page <= len(pages) {
			page = pages[h.Page-1]
		}
		for i, t := range strings.Split(page, "\n") {
			if n := h.Line + i; n > start {
				out = append(out, numbered{n, t})
			}
		}
		return out
	}
	for i, t := range lines[start:] {
		out = append(out, numbered{start + 1 + i, t})
	}
	return out
}

// skipSpaces skips re's \s (str.isspace) characters.
func skipSpaces(s string) string {
	return strings.TrimLeftFunc(s, pycompat.IsSpace)
}

// prevNext matches `Prev\s*\|\s*Next` against the whole of s.
func prevNext(s string) bool {
	rest, ok := strings.CutPrefix(s, "Prev")
	if !ok {
		return false
	}
	rest, ok = strings.CutPrefix(skipSpaces(rest), "|")
	return ok && skipSpaces(rest) == "Next"
}

// isNavLine is fulltext._NAV (`^(Index\s*\|\s*)?Prev\s*\|\s*Next$`) on a
// stripped line.
func isNavLine(line string) bool {
	if rest, ok := strings.CutPrefix(line, "Index"); ok {
		if after, ok := strings.CutPrefix(skipSpaces(rest), "|"); ok && prevNext(skipSpaces(after)) {
			return true
		}
	}
	return prevNext(line)
}

// Snippet is fulltext.snippet: (line number, clipped text) of the body line
// of the hit's best document matching the most groups, the earliest on a
// tie; a table-of-contents line (a run of five dots) counts half a group
// less. Header, navigation and title-only lines never qualify. Line 0 means none.
func Snippet(dirs []SourceDir, h *Hit, groups []Group) (int, string) {
	bestN, bestLine, best := 0.0, 0, ""
	for _, c := range candidateLines(dirs, h) {
		line := pycompat.Strip(c.text)
		if line == "" || isNavLine(line) || line == h.Title {
			continue
		}
		terms := TermSet(line)
		n := 0.0
		for _, g := range groups {
			if g.Matches(terms) {
				n++
			}
		}
		if strings.Contains(line, ".....") {
			n -= 0.5
		}
		if n > bestN {
			bestN, bestLine, best = n, c.n, line
			if n == float64(len(groups)) {
				break
			}
		}
	}
	if bestLine == 0 {
		return 0, ""
	}
	return bestLine, Clip(best, groups, snippetWidth)
}

// Clip is fulltext.clip: a line longer than width code points is cut to
// width around its first matching word, at a space when one precedes the
// match, with "..." marking each cut. As in Python, the match offset is
// taken in the folded line and applied to the original one.
func Clip(line string, groups []Group, width int) string {
	r := []rune(line)
	if len(r) <= width {
		return line
	}
	pos := 0
	for _, w := range scanWords(Fold(line)) {
		match := false
		for _, g := range groups {
			match = match || g.MatchesWord(w.text)
		}
		if match {
			pos = w.start
			break
		}
	}
	start := max(0, min(pos-width/3, len(r)-width))
	if start > 0 {
		for i := start; i < pos && i < len(r); i++ { // line.find(" ", start, pos)
			if r[i] == ' ' {
				start = i + 1
				break
			}
		}
	}
	end := min(start+width, len(r))
	text := string(r[start:end])
	if start > 0 {
		text = "..." + text
	}
	if start+width < len(r) {
		text += "..."
	}
	return text
}
