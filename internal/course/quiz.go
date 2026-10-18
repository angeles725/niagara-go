package course

import (
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// The kinds of quiz question, in print order: a lab item that asks *Why ...?*, a design decision
// of a review assignment (why it was taken), a trap (what goes wrong, what the solution does).
const (
	quizWhy      = "why"
	quizDecision = "decision"
	quizTrap     = "trap"
)

var quizKinds = []string{quizWhy, quizDecision, quizTrap}

// quizContext is how many course text lines a question prints; the rest is one line naming the
// command that prints them all.
const quizContext = 8

// whyRe is a *Why ...?* (or *why ...?*) question of a lab item.
var whyRe = regexp.MustCompile(`(?i)^\*why [^*]*\?\*`)
var whyAnyRe = regexp.MustCompile(`(?i)\*why [^*]*\?\*`)

// question is one quiz entry: the item, its kind, what it asks and its answer.
type question struct {
	it     *key.Item
	kind   string
	prompt string
	answer string
}

// asQuestion returns the item as a question of kind, or false: a why question is a lab item whose
// step asks it, or whose answer starts with it (the question then moves into the prompt).
func asQuestion(it *key.Item) (question, bool) {
	q := question{it: it, prompt: it.Step, answer: it.Answer}
	switch {
	case it.Kind == "decision":
		q.kind, q.prompt = quizDecision, "Why this decision? "+it.Step
	case it.Kind == "trap":
		q.kind, q.prompt = quizTrap, "What goes wrong, and what does this solution do? "+it.Step
	case it.Kind != "lab item":
		return q, false
	case whyAnyRe.MatchString(it.Step):
		q.kind = quizWhy
	case whyRe.MatchString(it.Answer):
		m := whyRe.FindString(it.Answer)
		q.kind, q.prompt = quizWhy, it.Step+"\n"+m
		q.answer = strings.TrimSpace(strings.TrimPrefix(it.Answer, m))
	default:
		return q, false
	}
	return q, true
}

// shuffle is a Fisher-Yates shuffle driven by a PCG seeded with seed: the same seed draws the same
// order on every platform and Go release (PCG's output is fixed; the shuffle is ours).
func shuffle(qs []question, seed uint64) {
	r := rand.NewPCG(seed, 0)
	for i := len(qs) - 1; i > 0; i-- {
		j := int(r.Uint64() % uint64(i+1))
		qs[i], qs[j] = qs[j], qs[i]
	}
}

type quizOpts struct {
	lab, kind string
	hasLab    bool
	count     int
	seed      int
	hasSeed   bool
	answers   bool
}

// again is the command that draws the same quiz with its answers.
func (o quizOpts) again() string {
	parts := []string{"course quiz"}
	if o.hasLab {
		parts = append(parts, "--lab "+o.lab)
	}
	if o.kind != "" {
		parts = append(parts, "--kind "+o.kind)
	}
	if o.count != 5 {
		parts = append(parts, "-n "+strconv.Itoa(o.count))
	}
	return strings.Join(append(parts, "--seed "+strconv.Itoa(o.seed), "--answers"), " ")
}

// cmdQuiz draws count questions (why, decision, trap; one kind with --kind) from the selected
// groups in the order of the seed, prints each prompt with its course text, then, with --answers,
// every answer and its status. Exit: 0 drawn; 1 an unknown group or no question in the selection
// (the honest zero); 2 -n below 1, or a text file that cannot be read.
func cmdQuiz(c *key.Corpus, s cli.Stdio, root string, o quizOpts) int {
	if o.count < 1 {
		fmt.Fprintf(s.Err, "course: -n %d asks no question (want 1 or more)\n", o.count)
		return cli.ExitCannotSearch
	}
	if !o.hasSeed {
		o.seed = int(now().UnixNano() % 1000000)
	}
	groups, scopeLabel := []string(nil), "every group"
	if o.hasLab {
		if groups = c.GroupKeys(o.lab); len(groups) == 0 {
			fmt.Fprintf(s.Err, "course: no group %s (groups: %s)\n", o.lab, c.GroupList())
			return cli.ExitNoMatch
		}
		scopeLabel = strings.Join(groups, ", ")
	}
	kinds := quizKinds
	if o.kind != "" {
		kinds = []string{o.kind}
	}
	var pool []question
	searched := 0
	for _, it := range c.Items {
		if groups != nil && !slices.Contains(groups, it.Group) {
			continue
		}
		searched++
		if q, ok := asQuestion(it); ok && slices.Contains(kinds, q.kind) {
			pool = append(pool, q)
		}
	}
	if len(pool) == 0 {
		fmt.Fprintf(s.Err, "course: no %s question in %s (searched %s)\n", strings.Join(kinds, ", "), scopeLabel,
			key.Plural(searched, "item"))
		return cli.ExitNoMatch
	}
	shuffle(pool, uint64(o.seed))
	drawn := pool[:min(o.count, len(pool))]
	texts := map[string][]string{}
	for _, q := range drawn {
		if f := q.it.TextFile; f != "" && texts[f] == nil {
			lines, msg := readText(root, f)
			if msg != "" {
				fmt.Fprintf(s.Err, "course: %s\n", msg)
				return cli.ExitCannotSearch
			}
			texts[f] = lines
		}
	}
	fmt.Fprintf(s.Out, "quiz: %d of %s (%s; %s), seed %d\n", len(drawn), key.Plural(len(pool), "question"),
		strings.Join(kinds, ", "), scopeLabel, o.seed)
	for i, q := range drawn {
		fmt.Fprintln(s.Out)
		printQuestion(s.Out, i+1, q, texts[q.it.TextFile])
	}
	fmt.Fprintln(s.Out)
	if !o.answers {
		fmt.Fprintf(s.Out, "answers: %s\n", o.again())
		return cli.ExitFound
	}
	fmt.Fprintln(s.Out, "answers")
	for i, q := range drawn {
		fmt.Fprintf(s.Out, "\nA%d. %s [%s] %s:%d\n", i+1, q.it.ID, strings.Join(q.it.StatusLabels(), ", "), q.it.File, q.it.Line)
		printIndented(s.Out, q.answer)
	}
	return cli.ExitFound
}

func printIndented(w io.Writer, text string) {
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
}

// printQuestion prints "Q<n>. <ID> (<kind> · <section>)", the prompt, and up to quizContext lines
// of the item's course text (lines is that file, nil when it points at none).
func printQuestion(w io.Writer, n int, q question, lines []string) {
	fmt.Fprintf(w, "Q%d. %s (%s · %s)\n", n, q.it.ID, q.kind, q.it.Section)
	printIndented(w, q.prompt)
	shown := 0
	for _, r := range q.it.Ranges {
		if lines == nil || r.A < 1 || r.B > len(lines) || r.B < r.A {
			continue
		}
		if shown == 0 {
			fmt.Fprintf(w, "    %s:%s (%s)\n", q.it.TextFile, rangesText(q.it.Ranges), q.it.ID)
		}
		for l := r.A; l <= r.B; l++ {
			if shown == quizContext {
				fmt.Fprintf(w, "    … more: course text %s\n", q.it.ID)
				return
			}
			fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("    %5d  %s", l, lines[l-1]), " \t"))
			shown++
		}
	}
}

func rangesText(rs []key.Range) string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.String()
	}
	return strings.Join(out, ", ")
}
