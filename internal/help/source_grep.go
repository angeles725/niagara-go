package help

// Regex search of Java sources (source_search.py): source --grep (one file)
// and source-grep (every file of source-index.json). Patterns are Python
// regexes (internal/pyre), matched case-insensitively against each line with
// its "\n", as Python iterates a text file.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// compileRegex is scope.compile_regex(pattern, re.IGNORECASE): an invalid
// pattern is a usage error (exit 2) quoting re.error.
func compileRegex(pattern string) (*pyre.Regexp, error) {
	rx, err := pyre.Compile(pattern, pyre.IGNORECASE)
	if err != nil {
		return nil, usageError(fmt.Sprintf("invalid regex '%s': %v", pattern, err))
	}
	return rx, nil
}

// sourceLinesKeepEnds reads a source file the way `for line in open(path,
// "r", encoding="utf-8", errors="replace")` yields it: like sourceLines, but
// each line keeps its "\n" (the last one has none when the file does not end
// with a newline).
func sourceLinesKeepEnds(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	text := pycompat.DecodeReplace(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// grepSource is the --grep branch of cmd_source: every line of the class's
// source file the pattern matches, after the entry header.
func (e *Env) grepSource(data *index.Object, pattern string) error {
	rx, err := compileRegex(pattern)
	if err != nil {
		return err
	}
	fmt.Fprintln(e.Out)
	fmt.Fprintf(e.Out, "Grep '%s':\n", pattern)
	file := pycompat.PosixJoin(e.BaseDir, "source", getStr(data, "path", ""))
	lines, err := sourceLinesKeepEnds(file)
	if err != nil {
		fmt.Fprintf(e.Out, "  Error reading source: %s\n", index.OSError(err, file))
		return e.reportMiss("source --grep", pattern, "source")
	}
	hits := 0
	for i, line := range lines {
		if rx.MatchString(line) {
			fmt.Fprintf(e.Out, "  L%d: %s\n", i+1, rstrip(line))
			hits++
		}
	}
	if hits == 0 {
		return e.reportMiss("source --grep", pattern, "source")
	}
	return nil
}

// SourceGrepParams are the arguments of `source-grep`.
type SourceGrepParams struct {
	Pattern string
	Max     int
}

// grepHit is one matching line of source-grep.
type grepHit struct {
	line int
	text string // the line stripped of surrounding whitespace
}

// grepClass is the hits of one class with its index data.
type grepClass struct {
	module, pkg string
	hits        []grepHit
}

// SourceGrep is source_search.cmd_source_grep: the lines of every source file
// the pattern matches (imports and package declarations skipped), grouped by
// class in sorted order, at most Max lines shown. The walk stops after the
// first file that brings the hits to 3 * Max.
func (e *Env) SourceGrep(p SourceGrepParams) error {
	idx, err := e.loadIndex("source-index.json", false)
	if err != nil {
		return err
	}
	rx, err := compileRegex(p.Pattern)
	if err != nil {
		return err
	}
	byClass := map[string]*grepClass{}
	total := 0
	for _, cls := range idx.Keys() {
		data := idx.Object(cls)
		file := pycompat.PosixJoin(e.BaseDir, "source", getStr(data, "path", ""))
		if !pycompat.IsFile(file) {
			continue
		}
		lines, err := sourceLinesKeepEnds(file)
		if err != nil {
			continue
		}
		for i, line := range lines {
			stripped := pycompat.Strip(line)
			if strings.HasPrefix(stripped, "import ") || strings.HasPrefix(stripped, "package ") {
				continue
			}
			if !rx.MatchString(line) {
				continue
			}
			c := byClass[cls]
			if c == nil {
				c = &grepClass{module: pyStr(get(data, "module", nil)), pkg: pyStr(get(data, "package", nil))}
				byClass[cls] = c
			}
			c.hits = append(c.hits, grepHit{i + 1, stripped})
			total++
		}
		if total >= p.Max*3 {
			break
		}
	}
	if total == 0 {
		return e.reportMiss("source-grep", p.Pattern, "source")
	}
	classes := make([]string, 0, len(byClass))
	for cls := range byClass {
		classes = append(classes, cls)
	}
	sort.Strings(classes)
	fmt.Fprintf(e.Out, "%d matches for '%s' in %d classes:\n", total, p.Pattern, len(byClass))
	shown := 0
	for _, cls := range classes {
		if shown >= p.Max {
			if remaining := total - shown; remaining > 0 {
				fmt.Fprintf(e.Out, "\n  ... and %d more matches\n", remaining)
			}
			break
		}
		c := byClass[cls]
		fmt.Fprintf(e.Out, "\n  %s (%s, %s):\n", cls, c.module, c.pkg)
		for _, h := range c.hits {
			fmt.Fprintf(e.Out, "    L%d: %s\n", h.line, clipRunes(h.text, 130, 127))
			shown++
			if shown >= p.Max {
				break
			}
		}
	}
	return nil
}

// clipRunes is `text[:keep] + "..."` when len(text) > limit, in code points.
func clipRunes(text string, limit, keep int) string {
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	return string(r[:keep]) + "..."
}
