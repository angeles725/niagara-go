package searchdoor

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/textfold"
)

// The query fan-out of searchdoor/door.py: three source jobs run concurrently,
// the CODE source's calls share one deadline.

const (
	maxWorkers   = 3  // one per source; never more processes than sources
	defaultLimit = 5  // ranked results shown per source
	rankRows     = 20 // module_nav rank rows per question (its default)
	grepRows     = 30 // degraded path only: module_nav grep stops after this many matching lines
	scopeLines   = 6
)

// Paths are the backend scripts, the course-code root and the answer-key checkout.
type Paths struct{ Blocks, Docs, Code, Course, Keys string }

// door runs the sources with a Backend and a monotonic clock in seconds.
type door struct {
	backend Backend
	now     func() float64
}

// bind tells a backend that answers in process which scripts are the sources
// and how to read the environment (the command's cli.Stdio.Env).
func (d *door) bind(p Paths, getenv envFunc) {
	if b, ok := d.backend.(pathBinder); ok {
		b.bindPaths(p, getenv)
	}
}

// job is one source job, named so a failure can be typed as that source.
type job struct {
	key, title, command string
	run                 func() *Outcome
}

func fromRaw(key, title, command string, raw Raw, parse parser) *Outcome {
	o := newOutcome(key, title, command, raw.State)
	o.Reason, o.MS = raw.Reason, raw.MS
	switch raw.State {
	case OK:
		o.Scope, o.Items = parse(raw.Out)
	case Empty:
		o.Scope = strippedNonblank(raw.Out, scopeLines)
	}
	return o
}

func modeArgs(mode string) []string {
	if mode == "" {
		return nil
	}
	return []string{"--" + mode}
}

func (d *door) blocksJob(path, query string, limit int, mode string, timeout int, title string) job {
	// corpus-nav --limit caps the printed LINES, not blocks: 400 lines per wanted block keeps the top N
	argv := append([]string{path, "find", query, "--limit", strconv.Itoa(max(2000, 400*limit))}, modeArgs(mode)...)
	return job{"blocks", title, "corpus-nav find", func() *Outcome {
		return fromRaw("blocks", title, "corpus-nav find", d.backend.Run(argv, float64(timeout)), parseBlocks)
	}}
}

// docsJob: strict makes a result niagara_help widened to 'any term' a zero
// (license join); version pins niagara-help's documentation (empty: its default).
func (d *door) docsJob(path, query string, limit int, mode string, timeout int, title string, strict bool, version string) job {
	argv := []string{path}
	command := "niagara_help find"
	if version != "" {
		argv = append(argv, "--niagara-version", version)
		command += " (docs " + version + ")"
	}
	argv = append(append(argv, "find", query, "-n", strconv.Itoa(max(limit, 5))), modeArgs(mode)...)
	return job{"docs", title, command, func() *Outcome {
		o := fromRaw("docs", title, command, d.backend.Run(argv, float64(timeout)), parseDocs)
		if strict && o.State == OK && relaxed(o.Scope) {
			o.State, o.Items = Empty, nil
			o.Notes = append(o.Notes, "no page names every term; the backend's any-term matches are not shown")
		}
		return o
	}}
}

var (
	identRx = re(`[A-Za-z_$][\w$]*`)
	humpRx  = re(`[a-z][A-Z]`)
	bTypeRx = re(`^B[A-Z][a-z]`)
)

// identifiers are the Java identifiers in the query (camelCase hump or a Baja
// 'BType' name), once each: exact token lookups.
func identifiers(query string) []string {
	var out []string
	for _, w := range textfold.Unique(identRx.FindAll(query)) {
		if humpRx.MatchString(w) || bTypeRx.MatchString(w) {
			out = append(out, w)
		}
	}
	return out
}

// codeTerms are the stop-word-free stemmed terms, or the raw words when the
// query is only stop-words.
func codeTerms(query string) []string {
	if t := textfold.QueryTerms(query); len(t) > 0 {
		return t
	}
	return textfold.ParseQuery(query)
}

// codeRegex is the degraded grep's regex: any term, case-insensitive.
func codeRegex(terms []string) string {
	esc := make([]string, len(terms))
	for i, t := range terms {
		esc[i] = reEscape(t)
	}
	return "(?i)" + strings.Join(esc, "|")
}

// reEscape is re.escape: a backslash before ()[]{}?*+-|^$\.&~# and whitespace controls.
func reEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("()[]{}?*+-|^$\\.&~# \t\n\r\v\f", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ranked is the ranked part of the code source: module_nav rank, degrading to
// module_nav grep only when rank cannot search (exit 2).
func (d *door) ranked(path, query string, deadline float64, timeout int, title string, notes *[]string) *Outcome {
	left := deadline - d.now()
	if left <= 0 {
		return unavailable("code", title, "module_nav rank",
			fmt.Sprintf("unavailable: the %ds budget was spent on token lookups before rank", timeout))
	}
	raw := d.backend.Run([]string{path, "rank", "-n", strconv.Itoa(rankRows), "--", query}, left)
	if !(raw.HasRC && raw.RC == 2) {
		return fromRaw("code", title, "module_nav rank", raw, parseRank)
	}
	terms := codeTerms(query)
	rx := codeRegex(terms)
	command := "module_nav grep '" + rx + "' (DEGRADED)"
	*notes = append(*notes, fmt.Sprintf("DEGRADED: module_nav rank cannot search (%s); fell back to module_nav grep, "+
		"which keeps only the first %d matching lines and does not rank the whole tree", raw.Reason, grepRows))
	left = deadline - d.now()
	if left <= 0 {
		return unavailable("code", title, command,
			fmt.Sprintf("unavailable: the %ds budget was spent before the degraded grep", timeout))
	}
	return fromRaw("code", title, command, d.backend.Run([]string{path, "grep", rx, "-n", strconv.Itoa(grepRows)}, left),
		func(out string) ([]string, []Item) { return parseGrep(out, terms) })
}

// sub runs one sub-scope builder; a panic is that sub-scope's failure.
func sub(title string, make func() *Outcome) (o *Outcome) {
	defer func() {
		if r := recover(); r != nil {
			o = unavailable("code", title, "?", fmt.Sprintf("unavailable: the front door failed on this sub-scope: panic: %v", r))
		}
	}()
	return make()
}

// codeJob: identifiers go to the token index first, module_nav rank fills the
// list, token hits that only mention an identifier follow. Every call shares
// ONE deadline of timeout seconds; a structural question adds the structure
// lead sub-scope, coursePath the course-code sub-scope.
func (d *door) codeJob(path, query string, timeout int, title, coursePath string) job {
	return job{"code", title, "module_nav token/rank", func() *Outcome {
		deadline := d.now() + float64(timeout)
		var structure *Outcome
		if s := sub(structureTitle, func() *Outcome { return d.structureJob(path, query, deadline) }); s != nil {
			structure = s
		}
		o := d.rankedCode(path, query, deadline, timeout, title)
		if coursePath != "" { // same deadline: past it the sub-scope is a named timeout
			o.Subs = append(o.Subs, sub(courseTitle, func() *Outcome { return d.courseSearch(coursePath, query, deadline) }))
		}
		if structure != nil { // the structure answer always leads the CODE source
			structure.Lead, structure.Enumeration, structure.Limit = true, true, structRows
			o.Subs = append([]*Outcome{structure}, o.Subs...)
		}
		return o
	}}
}

// rankedCode is the ranked CODE rows: token lookups, then rank (or the degraded grep).
func (d *door) rankedCode(path, query string, deadline float64, timeout int, title string) *Outcome {
	idents := identifiers(query)
	var notes, outs []string
	for _, word := range idents {
		left := deadline - d.now()
		if left <= 0 {
			notes = append(notes, fmt.Sprintf("token %s: skipped, the %ds budget of this source is spent", word, timeout))
			continue
		}
		raw := d.backend.Run([]string{path, "token", word, "-n", "100000"}, left)
		if raw.State == OK && strings.Contains(raw.Out, "TOKEN:") {
			outs = append(outs, raw.Out)
		} else {
			reason := raw.Reason
			if reason == "" {
				reason = "not in the token index"
			}
			notes = append(notes, "token "+word+": "+reason+"; ranking the question")
		}
	}
	rk := d.ranked(path, query, deadline, timeout, title, &notes)
	if len(outs) == 0 {
		rk.Notes = append(notes, rk.Notes...)
		return rk
	}
	scope, found := parseTokens(outs, idents)
	switch rk.State {
	case Empty:
		notes = append(notes, "rank: 0 matches for the question; showing the token hits only")
	case Unavailable:
		notes = append(notes, rk.Command+": "+rk.Reason+"; showing the token hits only")
	}
	o := newOutcome("code", title, "module_nav token "+strings.Join(idents, " ")+" + "+rk.Command, OK)
	o.Scope = scope
	if rk.State == OK {
		o.Scope = append(o.Scope, rk.Scope...)
	}
	var exact, mention []Item
	for _, it := range found {
		if slices.Contains(idents, it.Key) {
			exact = append(exact, it)
		} else {
			mention = append(mention, it)
		}
	}
	o.Items = merge(exact, rk.Items, mention)
	o.Notes = notes
	return o
}

// guarded runs one source job: a panic types THAT source unavailable; a source
// that answered OK without rows is a zero, never a found claim.
func guarded(j job) (o *Outcome) {
	defer func() {
		if r := recover(); r != nil {
			o = unavailable(j.key, j.title, j.command, fmt.Sprintf("unavailable: the front door failed on this source: panic: %v", r))
		}
	}()
	o = j.run()
	if o.State == OK && len(o.Items) == 0 {
		o.State = Empty
		o.Notes = append(o.Notes, "the backend answered, but no result rows could be read from it: counted as a zero")
	}
	return o
}

// runJobs runs the jobs concurrently (<= maxWorkers) and returns the outcomes in job order.
func runJobs(jobs []job) []*Outcome {
	out := make([]*Outcome, len(jobs))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			out[i] = guarded(j)
		}()
	}
	wg.Wait()
	return out
}

// search is door.search: the blocks, docs and code outcomes for a free-text query,
// led by the course items it names (courseItems, decision D41).
func (d *door) search(query string, p Paths, limit int, mode string, timeout int, docVersion string) []*Outcome {
	outcomes := runJobs([]job{
		d.blocksJob(p.Blocks, query, limit, mode, timeout, "SOURCE 1 · OUR BLOCKS"),
		d.docsJob(p.Docs, query, limit, mode, timeout, "SOURCE 2 · OFFICIAL DOCS", false, docVersion),
		d.codeJob(p.Code, query, timeout, "SOURCE 3 · CODE", p.Course),
	})
	if lead := d.courseItems(query, p.Keys); lead != nil {
		outcomes = append([]*Outcome{lead}, outcomes...)
	}
	return outcomes
}
