package course

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	labHeadRe  = regexp.MustCompile(`^## (?:Lab (\d+)|(Bonus lab)|(Appendix A)) — `)
	headLIRe   = regexp.MustCompile(`\(` + "`" + `LI:(\d+)(?:-(\d+))?` + "`" + `\)\s*$`)
	bulletRe   = regexp.MustCompile(`^- \*\*([^*]+)\*\* · (.*)$`)
	labIDRe    = regexp.MustCompile(`^(L\d+|LB|LA)\.\d+$`)
	labRangeRe = regexp.MustCompile(`^(L\d+|LB|LA)\.\d+[–-](L\d+|LB|LA)\.\d+$`)
	liRe       = regexp.MustCompile(`LI:(\d+)(?:-(\d+))?`)
	subRe      = regexp.MustCompile(`^\*\*([^*]+)\*\*$`)
	questionRe = regexp.MustCompile(`^\*[^*]+\?\*`)
	liveIDRe   = regexp.MustCompile(`(?:L\d+|LB|LA)\.\d+(?:[–-](?:L\d+|LB|LA)\.\d+)?`)
)

const liveHeading = "## Answers that need a live run"

type liveRef struct {
	id   string
	line int
}

// labsParser walks labs.md once; an open item collects its indented continuation lines.
type labsParser struct {
	c       *Corpus
	group   *Group
	section string
	sub     string
	cur     *Item
	raw     []string
	live    bool
	fence   bool
	refs    []liveRef
}

func parseLabs(c *Corpus, lines []string) {
	p := &labsParser{c: c}
	for i, line := range lines {
		p.line(i+1, line)
	}
	p.finish()
	for _, r := range p.refs {
		ids := []string{r.id}
		if strings.ContainsAny(r.id, "–-") {
			ids, _ = expandRange(r.id)
		}
		for _, id := range ids {
			it := c.lookup(id)
			if it == nil {
				c.errorf(labsFile, r.line, "live-run ID %s is not an item", id)
				continue
			}
			it.LiveRun = true
		}
	}
}

func (p *labsParser) line(n int, line string) {
	if p.cur != nil {
		if line == "" || strings.HasPrefix(line, "  ") {
			p.raw = append(p.raw, line)
			return
		}
		p.finish()
	}
	if isFence(line) {
		p.fence = !p.fence
		return
	}
	if p.fence {
		return
	}
	if strings.HasPrefix(line, "## ") {
		p.heading(line)
		return
	}
	if p.live {
		for _, id := range liveIDRe.FindAllString(line, -1) {
			p.refs = append(p.refs, liveRef{id, n})
		}
		return
	}
	if p.group == nil {
		return
	}
	if m := bulletRe.FindStringSubmatch(line); m != nil {
		p.start(n, m[1], m[2])
		return
	}
	if m := subRe.FindStringSubmatch(line); m != nil {
		p.sub = m[1]
	}
}

func (p *labsParser) heading(line string) {
	p.group, p.sub, p.live = nil, "", line == liveHeading
	m := labHeadRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	key := "L" + m[1]
	switch {
	case m[2] != "":
		key = "LB"
	case m[3] != "":
		key = "LA"
	}
	p.section = strings.TrimPrefix(line, "## ")
	g := &Group{Key: key, Title: p.section}
	if r := headLIRe.FindStringSubmatch(line); r != nil {
		g.TextFile, g.Ranges = labText, []Range{rangeOf(r[1], r[2])}
	}
	p.group = p.c.addGroup(g)
}

func (p *labsParser) start(n int, id, rest string) {
	var covers []string
	switch {
	case labIDRe.MatchString(id):
	case labRangeRe.MatchString(id):
		var ok bool
		if covers, ok = expandRange(id); !ok {
			p.c.errorf(labsFile, n, "malformed item ID %s", id)
			return
		}
	default:
		p.c.errorf(labsFile, n, "malformed item ID %s", id)
		return
	}
	if g := id[:strings.Index(id, ".")]; g != p.group.Key {
		p.c.errorf(labsFile, n, "item %s is outside its group %s", id, p.group.Key)
	}
	p.cur = &Item{ID: id, Covers: covers, Kind: "lab item", Group: p.group.Key, Section: p.section,
		Subsection: p.sub, File: labsFile, Line: n}
	p.raw = []string{rest}
}

// finish closes the open item: head paragraph, step/answer split, ranges and markers.
func (p *labsParser) finish() {
	it := p.cur
	if it == nil {
		return
	}
	p.cur = nil
	body := []string{p.raw[0]}
	for _, l := range p.raw[1:] {
		body = append(body, strings.TrimPrefix(l, "  "))
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	nhead := 0
	var head []string
	for _, l := range body {
		if strings.TrimSpace(l) == "" || isFence(l) {
			break
		}
		head = append(head, strings.TrimSpace(l))
		nhead++
	}
	headText := strings.Join(head, " ")
	step, ans := splitStep(headText)
	it.Step = step
	lines := append([]string{ans}, body[nhead:]...)
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	it.Answer = strings.Join(lines, "\n")
	if it.Answer == "" {
		it.Answer = "(step only)"
	}
	it.Ranges = liRanges(step)
	if len(it.Ranges) == 0 {
		it.Ranges = liRanges(headText)
	}
	if len(it.Ranges) > 0 {
		it.TextFile = labText
	}
	it.Status = markers(strings.Join(body, "\n"), headingStatus(it.Section)...)
	p.c.add(it)
}

// splitStep splits a head at its last " · " outside backticks; without one, a leading
// "*...?*" question is the step, otherwise the head is a step with no answer.
func splitStep(head string) (step, answer string) {
	cut, tick := -1, false
	for i := 0; i < len(head); i++ {
		switch {
		case head[i] == '`':
			tick = !tick
		case !tick && strings.HasPrefix(head[i:], " · "):
			cut = i
		}
	}
	if cut >= 0 {
		return head[:cut], strings.TrimSpace(head[cut+len(" · "):])
	}
	if q := questionRe.FindString(head); q != "" && strings.TrimSpace(head[len(q):]) != "" {
		return q, strings.TrimSpace(head[len(q):])
	}
	return head, ""
}

func liRanges(text string) []Range {
	var out []Range
	for _, m := range liRe.FindAllStringSubmatch(text, -1) {
		out = append(out, rangeOf(m[1], m[2]))
	}
	return out
}

func rangeOf(a, b string) Range {
	lo, _ := strconv.Atoi(a)
	hi := lo
	if b != "" {
		hi, _ = strconv.Atoi(b)
	}
	return Range{lo, hi}
}
