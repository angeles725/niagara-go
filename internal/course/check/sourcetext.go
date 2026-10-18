package check

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The source-text kind: a Go regexp over one file of a Niagara source tree (--source,
// COURSE_NIAGARA_SOURCE: the 4.15.6.20 sources a niagara-research checkout extracted to
// niagara-help/versions/4.15.6.20/source, one directory per module part). It re-derives an answer
// predicted from the 4.14 decompiled code from the 4.15 source itself.

// NeedsSource reports whether a check reads the source tree, so a summary names it.
func NeedsSource(checks []*Check) bool {
	for _, ck := range checks {
		if ck.Kind == kindSourceText {
			return true
		}
	}
	return false
}

// readText reads a whole text file within the file-text bound (a larger file is an error, never cut).
func readText(path string) (string, error) {
	lines, err := fileLines(path)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// excerpt is a match as evidence: its first line, cut at 80 characters.
func excerpt(m string) string {
	first, _, more := strings.Cut(m, "\n")
	first = strings.TrimSpace(first)
	if r := []rune(first); len(r) > 80 {
		first = string(r[:79]) + "…"
	} else if more {
		first += " …"
	}
	return first
}

// checkSourceText counts the matches of Match in the file Glob of the source tree (Glob is one
// path, "/"-separated, relative to the tree): at least / exactly N. Without a tree, or with a tree
// that is not a readable directory (a configuration error), the check is SKIP; a missing file in
// the tree is FAIL, as a missing jar is.
func (s *Station) checkSourceText(ck *Check) (string, string) {
	if s.source == "" {
		return Skip, "no --source DIR: " + ck.Glob + " not checked"
	}
	if fi, err := os.Stat(s.source); err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", s.source, err)
	} else if !fi.IsDir() {
		return Skip, fmt.Sprintf("cannot read %s: not a directory", s.source)
	}
	re, err := regexp.Compile(ck.Match)
	if err != nil {
		return Skip, fmt.Sprintf("bad pattern %s: %v", ck.Match, err)
	}
	text, err := readText(filepath.Join(s.source, filepath.FromSlash(ck.Glob)))
	if os.IsNotExist(err) {
		return Fail, "missing " + ck.Glob + " under " + s.source
	}
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", ck.Glob, err)
	}
	found := re.FindAllStringIndex(text, -1)
	ok := countOK(ck.Op, len(found), ck.N)
	noun := "matches"
	if len(found) == 1 {
		noun = "match"
	}
	ev := fmt.Sprintf("%s: %d %s", ck.Glob, len(found), noun)
	if !ok {
		ev += fmt.Sprintf(", want %s of %s", want(ck.Op, ck.N), ck.Match)
	}
	if len(found) > 0 {
		at := found[0][0]
		ev += fmt.Sprintf(" (first at line %d: %s)", strings.Count(text[:at], "\n")+1, excerpt(text[at:found[0][1]]))
	}
	return verdict(ok), ev
}
