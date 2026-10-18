// Package key parses the N4 developer-course answer keys of a niagara-research checkout
// (labs.md, review-assignments.md, certification-exam.md) into items: lab steps, review
// requirements, decisions, traps, needs-a-run checks and exam problems, each with its
// verification status. It prints nothing; the parse contract is docs/course.md.
package key

import (
	"fmt"
	"os"
	"path/filepath"
)

// RootEnv names the variable that points at a niagara-research checkout.
const RootEnv = "NIAGARA_RESEARCH_DIR"

// HasMarker reports whether dir holds labs.md, the file that marks a checkout.
func HasMarker(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(LabsFile)))
	return err == nil && st.Mode().IsRegular()
}

// Load parses the three answer keys under root. Missing files and parse errors are returned
// as lines to print; a corpus is returned only when both lists are empty.
func Load(root string) (*Corpus, []string) {
	var problems []string
	data := map[string][]string{}
	for _, f := range SourceFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		switch {
		case os.IsNotExist(err):
			problems = append(problems, "missing source "+f)
		case err != nil:
			problems = append(problems, fmt.Sprintf("cannot read %s: %v", f, err))
		default:
			data[f] = SplitLines(raw)
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	c := newCorpus()
	parseLabs(c, data[LabsFile])
	parseReview(c, data[ReviewFile])
	parseExam(c, data[ExamFile])
	for _, e := range c.sortedErrors() {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", e.file, e.line, e.msg))
	}
	if len(problems) > 0 {
		return nil, append(problems, fmt.Sprintf("%s; nothing searched", Plural(len(c.errs), "parse error")))
	}
	return c, nil
}

// Plural is "1 noun" or "N nouns".
func Plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
