package course

import (
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// The kinds of quiz question, in print order: a lab item that asks *Why ...?*, a design decision
// of a review assignment (why it was taken), a trap (what goes wrong, what the solution does), a
// lab item that names a value (Name = `value`: recall the value), a lab item that asks you to
// *Observe* something (predict what you will see, and why).
const (
	quizWhy      = "why"
	quizDecision = "decision"
	quizTrap     = "trap"
	quizRecall   = "recall"
	quizPredict  = "predict"
)

var quizKinds = []string{quizWhy, quizDecision, quizTrap, quizRecall, quizPredict}

// quizContext is how many course text lines a question prints; the rest is one line naming the
// command that prints them all.
const quizContext = 8

// quizMinAnswer is the shortest predict answer the pool keeps, in runes of substance (the answer
// without its status markers and evidence lines). On the real keys it leaves out "as stated.",
// "The wizard wrote it." and "standard JDWP banner." (10-21 runes); the shortest kept is 40.
const quizMinAnswer = 30

var (
	// whyAnyRe is a *Why ...?* (or *why ...?*) question of a lab item, anywhere in its text.
	whyAnyRe = regexp.MustCompile(`(?i)\*why [^*]*\?\*`)
	// recallRe is a step that names a value: a plain name, " = ", then the backticked value.
	recallRe = regexp.MustCompile("^([A-Z][A-Za-z ]*?) = (`.+)$")
	// observeRe is a step that asks to observe.
	observeRe = regexp.MustCompile(`\*[Oo]bserve\*`)
	// liCiteRe is a (`LI:n-m`) citation of the lab text, with the full stop that may close the step.
	liCiteRe = regexp.MustCompile("\\s*\\(`LI:[^`]*`\\)\\.?")
	// markRe and evidenceRe are the status markers and evidence lines that substance drops.
	markRe     = regexp.MustCompile(`\*\([^)]*\)\*`)
	evidenceRe = regexp.MustCompile(`(?m)evidence:.*$`)
	spaceRe    = regexp.MustCompile(`\s+`)
)

// placeholders are the answers that answer nothing: a step with no answer (key/labs.go writes
// "(step only)") and the setup steps.
var placeholders = []string{"(step only)", "Setup.", "Setup only."}

// question is one quiz entry: the item, its kind, what it asks and its answer.
type question struct {
	it     *key.Item
	kind   string
	prompt string
	answer string
}

// asQuestion returns the item as a question, or false. A lab item is, in this order: a why
// question when its step asks *Why ...?*, or its answer does (the question joins the prompt, the
// answer is what follows it); a recall question when its step is Name = `value` (the value, then
// the item's own answer, is the answer); a predict question when its step asks to *Observe*. A
// question must also be solid (see solid).
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
	case whyAnyRe.MatchString(it.Answer):
		m := whyAnyRe.FindStringIndex(it.Answer)
		q.kind, q.prompt = quizWhy, it.Step+"\n"+spaceRe.ReplaceAllString(it.Answer[m[0]:m[1]], " ")
		q.answer = strings.TrimSpace(it.Answer[m[1]:])
	case recallRe.MatchString(it.Step):
		m := recallRe.FindStringSubmatch(it.Step)
		q.kind, q.prompt = quizRecall, "What is "+m[1]+"?"
		if strings.Contains(it.Section, "ORD") {
			q.prompt = "What is the ORD of " + m[1] + "?"
		}
		q.answer = strings.TrimSpace(liCiteRe.ReplaceAllString(m[2], ""))
		if !isPlaceholder(it.Answer) {
			q.answer += "\n" + it.Answer
		}
	case observeRe.MatchString(it.Step):
		q.kind, q.prompt = quizPredict, "Predict what you observe, and why: "+it.Step
	default:
		return q, false
	}
	return q, q.solid()
}

// solid reports whether the question can be studied: its answer is neither empty nor a
// placeholder, a lab question points at a course text range (the source its answer is traced
// to), and a predict answer holds at least quizMinAnswer runes of substance. Decisions and traps
// are answered by the review-assignment key itself, which has no course text range.
func (q question) solid() bool {
	if isPlaceholder(q.answer) {
		return false
	}
	if q.it.Kind == "lab item" && (q.it.TextFile == "" || len(q.it.Ranges) == 0) {
		return false
	}
	return q.kind != quizPredict || utf8.RuneCountInString(substance(q.answer)) >= quizMinAnswer
}

func isPlaceholder(answer string) bool {
	a := strings.TrimSpace(answer)
	return a == "" || slices.Contains(placeholders, a)
}

// substance is an answer without its status markers (*(predicted)*, *(verified live ...)*) and
// evidence lines, trimmed: what it says.
func substance(answer string) string {
	return strings.TrimSpace(evidenceRe.ReplaceAllString(markRe.ReplaceAllString(answer, ""), ""))
}

// buildPool returns the questions of kinds among the items of groups (nil = every group), in
// corpus order, and how many items it searched.
func buildPool(c *key.Corpus, groups, kinds []string) (pool []question, searched int) {
	for _, it := range c.Items {
		if groups != nil && !slices.Contains(groups, it.Group) {
			continue
		}
		searched++
		if q, ok := asQuestion(it); ok && slices.Contains(kinds, q.kind) {
			pool = append(pool, q)
		}
	}
	return pool, searched
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
		parts = append(parts, "--lab "+shellQuote(o.lab))
	}
	if o.kind != "" {
		parts = append(parts, "--kind "+o.kind)
	}
	if o.count != 5 {
		parts = append(parts, "-n "+strconv.Itoa(o.count))
	}
	return strings.Join(append(parts, "--seed "+strconv.Itoa(o.seed), "--answers"), " ")
}

// safeArg is a word the shell neither splits nor expands.
var safeArg = regexp.MustCompile(`^[A-Za-z0-9._/+-]+$`)

// shellQuote returns s as one POSIX shell word: as is when safe, else single-quoted.
func shellQuote(s string) string {
	if safeArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// cmdQuiz draws count questions (why, decision, trap; one kind with --kind) from the selected
// groups in the order of the seed, prints each prompt with its course text, then, with --answers,
// every answer and its status. Exit: 0 drawn; 1 an unknown group or no question in the selection
// (the honest zero); 2 -n below 1, or a text file that cannot be read or does not hold a drawn
// item's range (the errors of course text).
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
	pool, searched := buildPool(c, groups, kinds)
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
		if msg := rangeOutside(q.it, texts[q.it.TextFile]); msg != "" {
			fmt.Fprintf(s.Err, "course: %s\n", msg)
			return cli.ExitCannotSearch
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

// rangeOutside is course text's refusal for the first range of it that its
// text file (lines) does not hold, "" when every range fits. An item with no
// text file has no context to print, so its ranges are not checked.
func rangeOutside(it *key.Item, lines []string) string {
	if it.TextFile == "" {
		return ""
	}
	for _, r := range it.Ranges {
		if r.A < 1 || r.B < r.A || r.B > len(lines) {
			return fmt.Sprintf("%s: range %s is outside %s (%s)", it.ID, r, it.TextFile, key.Plural(len(lines), "line"))
		}
	}
	return ""
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
	for _, r := range q.it.Ranges { // cmdQuiz checked every range against lines
		if lines == nil {
			return
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
