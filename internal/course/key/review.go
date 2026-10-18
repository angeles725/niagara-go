package key

import (
	"regexp"
	"strings"
)

var (
	raHeadRe   = regexp.MustCompile(`^## \d+\. Review Assignment (\d+) — `)
	runHeadRe  = regexp.MustCompile(`^## \d+\. Needs a run`)
	fromRe     = regexp.MustCompile("\\(from `([^`]+)`\\)")
	reqIDRe    = regexp.MustCompile(`^RA(\d+)\.(?:[A-Z]\d+[a-z]?|S)$`)
	reqRangeRe = regexp.MustCompile(`^RA(\d+)\.[A-Z]\d+[–-][A-Z]?\d+$`)
	decIDRe    = regexp.MustCompile(`^D(\d+)\.\d+$`)
	trapIDRe   = regexp.MustCompile(`^T(\d+)\.\d+$`)
	checkIDRe  = regexp.MustCompile(`^X\d+$`)
	srcLineRe  = regexp.MustCompile(`:(\d+)(?:-(\d+))?`)
	problemRe  = regexp.MustCompile(`^## \d+\. (Problem (\d+) — .*)$`)
	fileStatRe = regexp.MustCompile(`\*\*Status: ([^*]+)\*\*`)
	// labelCellRe is a traceability row that names no requirement: "(robustness)".
	labelCellRe = regexp.MustCompile(`^\([^()]+\)$`)
)

type tableKind int

const (
	noTable tableKind = iota
	reqTable
	decisionTable
	trapTable
	checkTable
	traceTable
)

func classify(cells []string) tableKind {
	if len(cells) < 3 {
		return noTable
	}
	switch {
	case cells[0] == "ID" && cells[1] == "Requirement":
		return reqTable
	case cells[0] == "#" && cells[1] == "Decision":
		return decisionTable
	case cells[0] == "Trap" && cells[1] == "What goes wrong":
		return trapTable
	case cells[0] == "ID" && cells[1] == "What is unconfirmed":
		return checkTable
	case cells[0] == "Requirement" && cells[1] == "Satisfied by":
		return traceTable
	}
	return noTable
}

// splitCells splits a Markdown table row on "|", keeping escaped "\|" inside a cell.
func splitCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && i+1 < len(row) && row[i+1] == '|':
			cur.WriteByte('|')
			i++
		case row[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(row[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

type traceRow struct {
	id, satisfied, verified string
	line                    int
}

// parseReview reads the requirement, decision, trap, check and traceability tables.
func parseReview(c *Corpus, lines []string) {
	var group *Group
	var section, sub, textFile string
	var inherited []string
	kind, fence, inTable := noTable, false, false
	var traces []traceRow
	for i, line := range lines {
		n := i + 1
		if isFence(line) {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			inTable = false
		}
		switch {
		case strings.HasPrefix(line, "## "):
			section, sub, textFile = strings.TrimPrefix(line, "## "), "", ""
			inherited = headingStatus(section)
			group = nil
			if m := raHeadRe.FindStringSubmatch(line); m != nil {
				group = c.addGroup(&Group{Key: "RA" + m[1], Title: section})
			} else if runHeadRe.MatchString(line) {
				group = c.addGroup(&Group{Key: "RAX", Title: section})
			}
		case strings.HasPrefix(line, "### "):
			sub = strings.TrimPrefix(line, "### ")
			inherited = append(headingStatus(section), headingStatus(sub)...)
			textFile = ""
			if m := fromRe.FindStringSubmatch(sub); m != nil {
				textFile = m[1]
			}
		case strings.HasPrefix(line, "|"):
			cells := splitCells(line)
			if !inTable {
				inTable, kind = true, classify(cells)
				continue
			}
			if strings.HasPrefix(line, "|---") || kind == noTable {
				continue
			}
			if len(cells) < 3 {
				c.errorf(ReviewFile, n, "table row has %d cells, want 3", len(cells))
				continue
			}
			if kind == traceTable {
				switch {
				case strings.HasPrefix(cells[0], "RA"):
					traces = append(traces, traceRow{cells[0], cells[1], cells[2], n})
				case labelCellRe.MatchString(cells[0]):
					// A parenthesised label row such as "(robustness)" traces no requirement.
				default:
					c.errorf(ReviewFile, n, "traceability row %s is not a requirement ID", cells[0])
				}
				continue
			}
			it := tableItem(c, kind, cells, n)
			if it == nil {
				continue
			}
			if group == nil {
				c.errorf(ReviewFile, n, "item %s is outside any group", it.ID)
				continue
			}
			if want := groupOfReviewID(it.ID); want != group.Key {
				c.errorf(ReviewFile, n, "item %s is outside its group %s", it.ID, group.Key)
			}
			it.Group, it.Section, it.Subsection = group.Key, section, sub
			if kind == reqTable && textFile != "" {
				it.TextFile = textFile
			} else {
				it.Ranges = nil
			}
			it.Status = markers(strings.Join(cells, " "), inherited...)
			it.Verified = evidence(strings.Join(cells, " "))
			c.add(it)
		}
	}
	joinTraces(c, traces)
}

// tableItem builds the item of one data row, or reports a malformed ID.
func tableItem(c *Corpus, kind tableKind, cells []string, n int) *Item {
	it := &Item{File: ReviewFile, Line: n}
	id := cells[0]
	switch kind {
	case reqTable:
		it.Kind, it.Step, it.Answer = "requirement", cells[1], "(no traceability row)"
		if reqRangeRe.MatchString(id) {
			covers, ok := expandRange(id)
			if !ok {
				c.errorf(ReviewFile, n, "malformed item ID %s", id)
				return nil
			}
			it.Covers = covers
		} else if !reqIDRe.MatchString(id) {
			c.errorf(ReviewFile, n, "malformed item ID %s", id)
			return nil
		}
		for _, m := range srcLineRe.FindAllStringSubmatch(cells[2], -1) {
			it.Ranges = append(it.Ranges, rangeOf(m[1], m[2]))
		}
	case decisionTable:
		it.Kind, it.Step, it.Answer = "decision", cells[1], cells[2]
		if !decIDRe.MatchString(id) {
			c.errorf(ReviewFile, n, "malformed item ID %s", id)
			return nil
		}
	case trapTable:
		id, step, _ := strings.Cut(cells[0], " ")
		it.Kind, it.Step = "trap", strings.TrimSpace(step)
		it.Answer = "What goes wrong: " + cells[1] + "\nThis solution: " + cells[2]
		if !trapIDRe.MatchString(id) {
			c.errorf(ReviewFile, n, "malformed item ID %s", id)
			return nil
		}
		it.ID = id
		return it
	case checkTable:
		it.Kind, it.Step, it.Answer = "check", cells[1], "Cheapest check: "+cells[2]
		if !checkIDRe.MatchString(id) {
			c.errorf(ReviewFile, n, "malformed item ID %s", id)
			return nil
		}
	}
	it.ID = id
	return it
}

// groupOfReviewID is the group an ID belongs to by its prefix.
func groupOfReviewID(id string) string {
	for _, re := range []*regexp.Regexp{reqIDRe, reqRangeRe, decIDRe, trapIDRe} {
		if m := re.FindStringSubmatch(id); m != nil {
			return "RA" + m[1]
		}
	}
	return "RAX"
}

// joinTraces sets the answer of every requirement a traceability row names; a row ID that is
// not a requirement (alone or as a range) is an error.
func joinTraces(c *Corpus, traces []traceRow) {
	for _, t := range traces {
		var targets []*Item
		if it := c.Lookup(t.id); it != nil && it.Kind == "requirement" && normID(it.ID) == normID(t.id) {
			targets = []*Item{it}
		} else if ids, ok := expandRange(t.id); ok && strings.ContainsAny(t.id, "–-") {
			for _, id := range ids {
				it := c.Lookup(id)
				if it == nil || it.Kind != "requirement" {
					c.errorf(ReviewFile, t.line, "traceability ID %s is not a requirement", id)
					continue
				}
				targets = append(targets, it)
			}
		} else if it := c.Lookup(t.id); it != nil && it.Kind == "requirement" {
			targets = []*Item{it}
		} else {
			c.errorf(ReviewFile, t.line, "traceability ID %s is not a requirement", t.id)
		}
		for _, it := range targets {
			text := "Satisfied by: " + t.satisfied + "\nVerified by: " + t.verified
			if it.Answer == "(no traceability row)" {
				it.Answer = text
			} else {
				it.Answer += "\n" + text
			}
		}
	}
}

// parseExam reads "## n. Problem m — title" sections; a file-level "**Status: verified ...**"
// line marks every problem verified.
func parseExam(c *Corpus, lines []string) {
	g := c.addGroup(&Group{Key: "EXAM", Title: "Certification exam"})
	var fileStatus, fileEvidence []string
	var cur *Item
	var body []string
	fence := false
	flush := func() {
		if cur == nil {
			return
		}
		for len(body) > 0 && strings.TrimSpace(body[0]) == "" {
			body = body[1:]
		}
		for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
		cur.Answer = strings.Join(body, "\n")
		cur.Status = markers(cur.Answer, fileStatus...)
		cur.Verified = evidence(cur.Answer, fileEvidence...)
		c.add(cur)
		cur, body = nil, nil
	}
	for i, line := range lines {
		if isFence(line) {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "## ") {
			flush()
			if m := problemRe.FindStringSubmatch(line); m != nil {
				cur = &Item{ID: "P" + m[2], Kind: "exam problem", Group: g.Key, Section: strings.TrimPrefix(line, "## "),
					Step: m[1], File: ExamFile, Line: i + 1}
			}
			continue
		}
		if cur != nil {
			body = append(body, line)
		} else if m := fileStatRe.FindStringSubmatch(line); m != nil && fileStatus == nil {
			s := strings.ToLower(m[1])
			fileEvidence = nil // each status line states its own evidence, or none
			if strings.HasPrefix(s, "verified") {
				fileStatus = []string{"verified"}
				if e := strings.Fields(strings.TrimRight(s, ".")); len(e) > 1 && (e[1] == EvidenceLive || e[1] == EvidenceOffline) {
					fileEvidence = []string{e[1]} // "verified live." / "verified offline"; plain "verified" states none
				}
			} else if strings.Contains(s, "unverified") {
				fileStatus = []string{"unverified"}
			}
		}
	}
	flush()
}
