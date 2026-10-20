package course

import (
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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
	ordWordRe  = regexp.MustCompile(`\bORDs?\b`) // a section about ORDs, not RECORDS
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
	noContext bool
	ids       string // --ids as given; hasIDs when it was
	hasIDs    bool
	// interactive asks each question, shows its answer and records a self-grade to history
	// (--history as given; hasHistory when it was).
	interactive bool
	history     string
	hasHistory  bool
	// missed and due draw from the history's missed or Leitner-due questions (due at nowFlag,
	// RFC 3339, when hasNow); the replay line then names the drawn IDs.
	missed, due bool
	nowFlag     string
	hasNow      bool
	picked      []string // the IDs --ids asked, as the keys spell them
	usage       func(message string) int
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
	switch {
	case o.hasIDs, o.missed, o.due:
		parts = append(parts, "--ids "+shellQuote(strings.Join(o.picked, ",")))
	case o.count != 5:
		parts = append(parts, "-n "+strconv.Itoa(o.count), "--seed "+strconv.Itoa(o.seed))
	default:
		parts = append(parts, "--seed "+strconv.Itoa(o.seed))
	}
	if o.noContext {
		parts = append(parts, "--no-context")
	}
	return strings.Join(append(parts, "--answers"), " ")
}

// safeArg is a word the shell neither splits nor expands.
var safeArg = regexp.MustCompile(`^[A-Za-z0-9._/+,-]+$`)

// shellQuote returns s as one POSIX shell word: as is when safe, else single-quoted.
func shellQuote(s string) string {
	if safeArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// cmdQuiz draws count questions (every kind; one with --kind) from the selected groups in the
// order of the seed, or asks the --ids items in their order, prints each prompt, then, with
// --answers, every answer, its status and its course text. Exit: 0 drawn; 1 an unknown group, no
// question in the selection (the honest zero) or an --ids item that is no question of it; 2 -n
// below 1 without --ids (where -n does not apply), an --ids ID the keys do not hold (a usage error), or a text file that cannot be read or
// does not hold a drawn item's range (the errors of course text).
func cmdQuiz(c *key.Corpus, s cli.Stdio, root string, o quizOpts) int {
	if !o.hasIDs && o.count < 1 { // -n does not apply under --ids
		fmt.Fprintf(s.Err, "course: -n %d asks no question (want 1 or more)\n", o.count)
		return cli.ExitCannotSearch
	}
	if !o.hasSeed && !o.hasIDs {
		o.seed = int(now().UnixNano() % 1000000)
	}
	if (o.missed && o.due) || ((o.missed || o.due) && o.hasIDs) {
		return o.usage("--missed, --due and --ids exclude each other")
	}
	at := now()
	if o.hasNow {
		var err error
		if at, err = time.Parse(time.RFC3339, o.nowFlag); err != nil {
			return o.usage("--now " + o.nowFlag + " is not an RFC 3339 time (2026-10-19T08:00:00Z)")
		}
	}
	var history string
	if o.interactive || o.missed || o.due {
		var msg string
		if history, msg = historyPath(s, o.history, o.hasHistory); history == "" {
			return o.usage(msg)
		}
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
	if o.missed || o.due {
		var code int
		if pool, scopeLabel, code = historyPool(c, s, o, pool, kinds, scopeLabel, history, at); pool == nil {
			return code
		}
	}
	var drawn []question
	if o.hasIDs {
		var code int
		if drawn, code = pickIDs(c, s, &o, pool, kinds, scopeLabel); drawn == nil {
			return code
		}
	} else if len(pool) == 0 {
		fmt.Fprintf(s.Err, "course: no %s question in %s (searched %s)\n", strings.Join(kinds, ", "), scopeLabel,
			key.Plural(searched, "item"))
		return cli.ExitNoMatch
	}
	if !o.hasIDs {
		shuffle(pool, uint64(o.seed))
		drawn = pool[:min(o.count, len(pool))]
		if o.missed || o.due { // the history moves on: replay these IDs
			for _, q := range drawn {
				o.picked = append(o.picked, q.it.ID)
			}
		}
	}
	texts := map[string][]string{}
	for _, q := range drawn {
		if o.noContext {
			break // no course text is printed, so none is read
		}
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
	order := "seed " + strconv.Itoa(o.seed)
	if o.hasIDs {
		order = "ids " + strings.Join(o.picked, ",")
	}
	fmt.Fprintf(s.Out, "quiz: %d of %s (%s; %s), %s\n", len(drawn), key.Plural(len(pool), "question"),
		strings.Join(kinds, ", "), scopeLabel, order)
	if o.interactive {
		return runSession(s, drawn, texts, history, time.Time{})
	}
	for i, q := range drawn {
		fmt.Fprintln(s.Out)
		printQuestion(s.Out, i+1, q)
	}
	fmt.Fprintln(s.Out)
	if !o.answers {
		fmt.Fprintf(s.Out, "answers: %s\n", o.again())
		return cli.ExitFound
	}
	fmt.Fprintln(s.Out, "answers")
	for i, q := range drawn {
		printAnswer(s.Out, i+1, q, texts[q.it.TextFile])
	}
	return cli.ExitFound
}

// pickIDs returns the pool questions of the --ids items, in their order and once each, and
// records their IDs in o.picked; nil and the exit when an ID is empty or not in the keys (a usage
// error) or an item is no question of the pool (the IDs on stderr, exit 1).
func pickIDs(c *key.Corpus, s cli.Stdio, o *quizOpts, pool []question, kinds []string, scopeLabel string) ([]question, int) {
	var picked []question
	var outside []string
	seen := map[*key.Item]bool{}
	for _, id := range strings.Split(o.ids, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, o.usage("--ids: an empty ID in " + o.ids)
		}
		it := c.Lookup(id)
		if it == nil {
			return nil, o.usage(fmt.Sprintf("--ids: no item %s (%s)", id, scope(c)))
		}
		if seen[it] {
			continue
		}
		seen[it] = true
		i := slices.IndexFunc(pool, func(q question) bool { return q.it == it })
		if i < 0 {
			outside = append(outside, it.ID)
			continue
		}
		picked = append(picked, pool[i])
		o.picked = append(o.picked, it.ID)
	}
	if len(outside) > 0 {
		fmt.Fprintf(s.Err, "course: %s: not a question of %s in %s (%s)\n", strings.Join(outside, ", "),
			strings.Join(kinds, ", "), scopeLabel, key.Plural(len(pool), "question"))
		return nil, cli.ExitNoMatch
	}
	return picked, cli.ExitFound
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

// printQuestion prints "Q<n>. <ID> (<kind> · <section>)", the unconfirmed tag of the answer
// ([predicted], [unverified]) when it has one, and the prompt.
func printQuestion(w io.Writer, n int, q question) {
	tag := ""
	if u := unconfirmed(q.it); len(u) > 0 {
		tag = " [" + strings.Join(u, ", ") + "]"
	}
	fmt.Fprintf(w, "Q%d. %s (%s · %s)%s\n", n, q.it.ID, q.kind, q.it.Section, tag)
	printIndented(w, q.prompt)
}

// unconfirmed is the statuses that leave the item's answer unconfirmed: predicted and
// unverified, unless a verified marker has confirmed it since; nil for a confirmed answer.
func unconfirmed(it *key.Item) []string {
	if it.HasStatus("verified") {
		return nil
	}
	var out []string
	for _, st := range []string{"predicted", "unverified"} {
		if it.HasStatus(st) {
			out = append(out, st)
		}
	}
	return out
}

// printContext prints up to quizContext lines of the item's course text after its answer (lines
// is that file, nil when it points at none or --no-context read none), then "… more: course text
// <ID>" when more remain. It follows the answer because the text may state it.
func printContext(w io.Writer, it *key.Item, lines []string) {
	if lines == nil {
		return
	}
	shown := 0
	for _, r := range it.Ranges { // cmdQuiz checked every range against lines
		if shown == 0 {
			fmt.Fprintf(w, "    %s:%s (%s)\n", it.TextFile, rangesText(it.Ranges), it.ID)
		}
		for l := r.A; l <= r.B; l++ {
			if shown == quizContext {
				fmt.Fprintf(w, "    … more: course text %s\n", it.ID)
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
