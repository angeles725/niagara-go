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

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// quizContext is how many course text lines a question prints; the rest is one line naming the
// command that prints them all.
const quizContext = 8

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
// below 1 without --ids (where -n does not apply), an --ids ID the keys do not hold, --history or
// --now where they do not apply (usage errors), or a text file that cannot be read or does not
// hold a drawn item's range (the errors of course text).
func cmdQuiz(c *key.Corpus, s cli.Stdio, root string, o quizOpts) int {
	at, history, code, ok := o.resolve(s)
	if !ok {
		return code
	}
	d, code, ok := drawQuiz(c, s, &o, at, history)
	if !ok {
		return code
	}
	texts, code, ok := quizTexts(s, root, d.drawn, o.noContext)
	if !ok {
		return code
	}
	order := "seed " + strconv.Itoa(o.seed)
	if o.hasIDs {
		order = "ids " + strings.Join(o.picked, ",")
	}
	fmt.Fprintf(s.Out, "quiz: %d of %s (%s; %s), %s\n", len(d.drawn), key.Plural(len(d.pool), "question"),
		strings.Join(d.kinds, ", "), d.scopeLabel, order)
	if o.interactive {
		return runSession(s, d.drawn, texts, history, time.Time{})
	}
	printQuiz(s.Out, o, d.drawn, texts)
	return cli.ExitFound
}

// resolve checks the flag combinations (the usage errors and -n below 1 of cmdQuiz), draws the
// seed from the clock when none was given, and returns the time --due measures against and the
// history path (when --interactive, --missed or --due use one); ok false with the exit otherwise.
func (o *quizOpts) resolve(s cli.Stdio) (at time.Time, history string, code int, ok bool) {
	if !o.hasIDs && o.count < 1 { // -n does not apply under --ids
		fmt.Fprintf(s.Err, "course: -n %d asks no question (want 1 or more)\n", o.count)
		return at, "", cli.ExitCannotSearch, false
	}
	if !o.hasSeed && !o.hasIDs {
		o.seed = int(now().UnixNano() % 1000000)
	}
	if (o.missed && o.due) || ((o.missed || o.due) && o.hasIDs) {
		return at, "", o.usage("--missed, --due and --ids exclude each other"), false
	}
	switch {
	case o.hasHistory && o.history == "":
		return at, "", o.usage("--history: an empty path"), false
	case o.hasHistory && !o.interactive && !o.missed && !o.due:
		return at, "", o.usage("--history applies to --interactive, --missed and --due"), false
	case o.hasNow && !o.due:
		return at, "", o.usage("--now applies to --due"), false
	}
	at = now()
	if o.hasNow {
		var err error
		if at, err = time.Parse(time.RFC3339, o.nowFlag); err != nil {
			return at, "", o.usage("--now " + o.nowFlag + " is not an RFC 3339 time (2026-10-19T08:00:00Z)"), false
		}
	}
	if o.interactive || o.missed || o.due {
		var msg string
		if history, msg = historyPath(s, o.history, o.hasHistory); history == "" {
			return at, "", o.usage(msg), false
		}
	}
	return at, history, cli.ExitFound, true
}

// quizDraw is what a quiz asks: the drawn questions, the pool they came from (its size is in the
// header), the kinds and the scope label the header and the refusals name.
type quizDraw struct {
	drawn, pool []question
	kinds       []string
	scopeLabel  string
}

// drawQuiz builds the pool of the selected groups and kinds (narrowed to the history's missed or
// due questions with --missed or --due), then picks the --ids items or draws count questions in
// the order of the seed; ok false with the exit for an unknown group, an empty pool or a failed
// --ids pick.
func drawQuiz(c *key.Corpus, s cli.Stdio, o *quizOpts, at time.Time, history string) (d quizDraw, code int, ok bool) {
	var groups []string
	d.scopeLabel = "every group"
	if o.hasLab {
		if groups = c.GroupKeys(o.lab); len(groups) == 0 {
			fmt.Fprintf(s.Err, "course: no group %s (groups: %s)\n", o.lab, c.GroupList())
			return d, cli.ExitNoMatch, false
		}
		d.scopeLabel = strings.Join(groups, ", ")
	}
	d.kinds = quizKinds
	if o.kind != "" {
		d.kinds = []string{o.kind}
	}
	pool, searched := buildPool(c, groups, d.kinds)
	if o.missed || o.due {
		if pool, d.scopeLabel, code = historyPool(c, s, *o, pool, d.kinds, d.scopeLabel, history, at); pool == nil {
			return d, code, false
		}
	}
	d.pool = pool
	if o.hasIDs {
		if d.drawn, code = pickIDs(c, s, o, pool, d.kinds, d.scopeLabel); d.drawn == nil {
			return d, code, false
		}
		return d, cli.ExitFound, true
	}
	if len(pool) == 0 {
		fmt.Fprintf(s.Err, "course: no %s question in %s (searched %s)\n", strings.Join(d.kinds, ", "), d.scopeLabel,
			key.Plural(searched, "item"))
		return d, cli.ExitNoMatch, false
	}
	shuffle(pool, uint64(o.seed))
	d.drawn = pool[:min(o.count, len(pool))]
	if o.missed || o.due { // the history moves on: replay these IDs
		for _, q := range d.drawn {
			o.picked = append(o.picked, q.it.ID)
		}
	}
	return d, cli.ExitFound, true
}

// quizTexts reads the course text file of each drawn question once and checks its ranges against
// it, in draw order; with noContext none is read (no course text is printed). ok false with exit 2
// for a file that cannot be read or does not hold a range.
func quizTexts(s cli.Stdio, root string, drawn []question, noContext bool) (map[string][]string, int, bool) {
	texts := map[string][]string{}
	for _, q := range drawn {
		if noContext {
			break // no course text is printed, so none is read
		}
		if f := q.it.TextFile; f != "" && texts[f] == nil {
			lines, msg := readText(root, f)
			if msg != "" {
				fmt.Fprintf(s.Err, "course: %s\n", msg)
				return nil, cli.ExitCannotSearch, false
			}
			texts[f] = lines
		}
		if msg := rangeOutside(q.it, texts[q.it.TextFile]); msg != "" {
			fmt.Fprintf(s.Err, "course: %s\n", msg)
			return nil, cli.ExitCannotSearch, false
		}
	}
	return texts, cli.ExitFound, true
}

// printQuiz prints every drawn prompt, then, with --answers, every answer with its status and
// course text, else the command that draws the same quiz with them.
func printQuiz(w io.Writer, o quizOpts, drawn []question, texts map[string][]string) {
	for i, q := range drawn {
		fmt.Fprintln(w)
		printQuestion(w, i+1, q)
	}
	fmt.Fprintln(w)
	if !o.answers {
		fmt.Fprintf(w, "answers: %s\n", o.again())
		return
	}
	fmt.Fprintln(w, "answers")
	for i, q := range drawn {
		printAnswer(w, i+1, q, texts[q.it.TextFile])
	}
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
