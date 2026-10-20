package course

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

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

var quizKinds = []string{quizWhy, quizDecision, quizTrap, quizRecall, quizPredict, quizRequirement, quizExam}

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
	ordWordRe  = regexp.MustCompile(`\bORDs?\b`) // a section about ORDs, not RECORDS
)

// placeholders are the answers that answer nothing: a step with no answer (key/labs.go writes
// "(step only)"), the setup steps and a requirement no traceability row satisfies.
var placeholders = []string{"(step only)", "Setup.", "Setup only.", "(no traceability row)"}

// question is one quiz entry: the item, its kind, what it asks and its answer.
type question struct {
	it     *key.Item
	kind   string
	prompt string
	answer string
	before string // the answer text before a mid-answer *Why ...?*, printed with the answer
}

// asQuestion returns the item as a question, or false. A lab item is, in this order: a why
// question when its step asks *Why ...?*, or its answer does (the question joins the prompt, the
// answer is what follows it, the text before it prints with the answer); a recall question when
// its step is Name = `value` (the value, then the item's own answer, is the answer; the prompt asks
// for an ORD in a section naming the word ORD or ORDs); a predict question when its step asks to *Observe*. A
// question must also be solid (see solid).
func asQuestion(it *key.Item) (question, bool) {
	q := question{it: it, prompt: it.Step, answer: it.Answer}
	switch {
	case it.Kind == "decision":
		q.kind, q.prompt = quizDecision, "Why this decision? "+it.Step
	case it.Kind == "trap":
		q.kind, q.prompt = quizTrap, "What goes wrong, and what does this solution do? "+it.Step
	case it.Kind == "requirement":
		q.kind, q.prompt = quizRequirement, requirementPrompt+it.Step
	case it.Kind == "exam problem":
		q.kind = quizExam
		q.prompt, q.answer = examQuestion(it)
	case it.Kind != "lab item":
		return q, false
	case whyAnyRe.MatchString(it.Step):
		q.kind = quizWhy
	case whyAnyRe.MatchString(it.Answer):
		m := whyAnyRe.FindStringIndex(it.Answer)
		q.kind, q.prompt = quizWhy, it.Step+"\n"+spaceRe.ReplaceAllString(it.Answer[m[0]:m[1]], " ")
		q.answer = strings.TrimSpace(it.Answer[m[1]:])
		q.before = strings.TrimSpace(spaceRe.ReplaceAllString(it.Answer[:m[0]], " "))
	case recallRe.MatchString(it.Step):
		m := recallRe.FindStringSubmatch(it.Step)
		q.kind, q.prompt = quizRecall, "What is "+m[1]+"?"
		if ordWordRe.MatchString(it.Section) {
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
