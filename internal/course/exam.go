package course

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// The question kinds of the review requirements and the exam problems, after the study kinds.
const (
	quizRequirement = "requirement"
	quizExam        = "exam"
)

// examGroup is the group of the certification exam.
const examGroup = "EXAM"

// requirementPrompt is what a requirement question asks; its answer is the traceability row.
const requirementPrompt = "How is this requirement satisfied and verified? "

// requirementLead opens the requirement paragraph of an exam problem.
const requirementLead = "**Requirement.**"

// examQuestion splits an exam problem: its step, then its **Requirement.** paragraph (the first
// paragraph, when it opens with requirementLead, without the lead), is the prompt; the rest of the
// section is the answer. A section with no such paragraph is answered by all of its body.
func examQuestion(it *key.Item) (prompt, answer string) {
	body := strings.TrimSpace(it.Answer)
	if !strings.HasPrefix(body, requirementLead) {
		return it.Step, body
	}
	para, rest, _ := strings.Cut(body, "\n\n")
	return it.Step + "\n" + strings.TrimSpace(strings.TrimPrefix(para, requirementLead)), strings.TrimSpace(rest)
}

type examOpts struct {
	count, minutes       int
	hasCount, hasMinutes bool
	seed                 int
	hasSeed              bool
	history              string
	hasHistory           bool
	usage                func(message string) int
}

// cmdExam simulates the exam: it draws --count exam problems (every one by default) in the order
// of the seed and asks them as quiz --interactive does, until the input ends or --minutes have
// passed, then prints the score; each graded answer is recorded with kind exam. Exam problems
// point at no course text, so none is read. Exit: 0 asked; 1
// no exam question (the honest zero); 2 --count or --minutes below 1, no history path, or an
// answer that cannot be recorded.
func cmdExam(c *key.Corpus, s cli.Stdio, o examOpts) int {
	switch {
	case o.hasCount && o.count < 1:
		return o.usage(fmt.Sprintf("--count %d asks no question (want 1 or more)", o.count))
	case o.hasMinutes && o.minutes < 1:
		return o.usage(fmt.Sprintf("--minutes %d leaves no time (want 1 or more)", o.minutes))
	}
	history, msg := historyPath(s, o.history, o.hasHistory)
	if history == "" {
		return o.usage(msg)
	}
	if !o.hasSeed {
		o.seed = int(now().UnixNano() % 1000000)
	}
	pool, searched := buildPool(c, []string{examGroup}, []string{quizExam})
	if len(pool) == 0 {
		fmt.Fprintf(s.Err, "course: no exam question in %s (searched %s)\n", examGroup, key.Plural(searched, "item"))
		return cli.ExitNoMatch
	}
	shuffle(pool, uint64(o.seed))
	drawn := pool
	if o.hasCount {
		drawn = pool[:min(o.count, len(pool))]
	}
	limit, deadline := "no time limit", time.Time{}
	if o.hasMinutes {
		limit = key.Plural(o.minutes, "minute")
		deadline = now().Add(time.Duration(o.minutes) * time.Minute)
	}
	fmt.Fprintf(s.Out, "exam: %d of %s (%s), seed %s, %s\n", len(drawn), key.Plural(len(pool), "problem"), examGroup,
		strconv.Itoa(o.seed), limit)
	return runSession(s, drawn, nil, history, deadline)
}
