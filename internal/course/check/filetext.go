package check

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The file-text kind: lines of files the station wrote into its directory (shared/ is the
// station's Files space, the file:^ root).

// maxFileText bounds a file-text read; a larger file is an error, never read in part. A variable so
// that a test can shrink it.
var maxFileText = 16 << 20

// fileLines reads a text file as lines: a UTF-8 BOM and the CR of CRLF line ends are dropped.
func fileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(maxFileText)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileText {
		// A cut file would be checked on its first lines only: the check cannot run (SKIP).
		return nil, fmt.Errorf("larger than %d bytes", maxFileText)
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// stationFiles lists the regular files matching a Glob under the station directory, sorted, as
// paths relative to it with "/".
func (s *Station) stationFiles(glob string) (pattern string, files []string, err error) {
	pattern = strings.ReplaceAll(glob, "{station}", globEscape(s.Name))
	matches, err := filepath.Glob(filepath.Join(globEscape(s.dir), filepath.FromSlash(pattern)))
	if err != nil {
		return pattern, nil, err
	}
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && st.Mode().IsRegular() {
			rel, _ := filepath.Rel(s.dir, m)
			files = append(files, filepath.ToSlash(rel))
		}
	}
	sort.Strings(files)
	return pattern, files, nil
}

// fileText checks one file: Line must match re, or (Line 0) the matching lines must reach N.
// It returns whether the file passes and what was found.
func (s *Station) fileText(ck *Check, re *regexp.Regexp, rel string) (bool, string, error) {
	lines, err := fileLines(filepath.Join(s.dir, filepath.FromSlash(rel)))
	if err != nil {
		return false, "", err
	}
	if ck.Line > 0 {
		if ck.Line > len(lines) {
			return false, fmt.Sprintf("%s has %s", rel, lineCount(len(lines))), nil
		}
		got := lines[ck.Line-1]
		return re.MatchString(got), fmt.Sprintf("%s:%d = %s", rel, ck.Line, got), nil
	}
	n := 0
	for _, l := range lines {
		if re.MatchString(l) {
			n++
		}
	}
	verb := "lines match"
	if n == 1 {
		verb = "line matches"
	}
	return countOK(ck.Op, n, ck.N), fmt.Sprintf("%s: %d %s", rel, n, verb), nil
}

func lineCount(n int) string {
	if n == 1 {
		return "1 line"
	}
	return fmt.Sprintf("%d lines", n)
}

func (s *Station) checkFileText(ck *Check) (string, string) {
	re, err := regexp.Compile(strings.ReplaceAll(ck.Match, "{station}", regexp.QuoteMeta(s.Name)))
	if err != nil {
		return Skip, fmt.Sprintf("bad pattern %s: %v", ck.Match, err)
	}
	pattern, files, err := s.stationFiles(ck.Glob)
	if err != nil {
		return Skip, fmt.Sprintf("bad pattern %s: %v", pattern, err)
	}
	if len(files) == 0 {
		return Fail, "no file " + pattern
	}
	first, bad := "", ""
	pass := 0
	for _, rel := range files {
		ok, ev, err := s.fileText(ck, re, rel)
		if err != nil {
			return Skip, fmt.Sprintf("cannot read %s: %v", rel, err)
		}
		if first == "" {
			first = ev
		}
		if ok {
			pass++
		} else if bad == "" {
			bad = ev
		}
	}
	if bad == "" {
		if len(files) == 1 {
			return Pass, first
		}
		return Pass, fmt.Sprintf("%d of %d files: %s", pass, len(files), first)
	}
	target := fmt.Sprintf("line %d =~ %s", ck.Line, ck.Match)
	if ck.Line == 0 {
		noun := "lines"
		if ck.N == 1 {
			noun = "line"
		}
		target = fmt.Sprintf("%s %s =~ %s", want(ck.Op, ck.N), noun, ck.Match)
	}
	return Fail, fmt.Sprintf("%d of %d files pass, want %s: %s", pass, len(files), target, bad)
}
