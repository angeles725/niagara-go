package key

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Source paths, relative to the root and always with "/".
const (
	LabsFile   = "docs/n4-dev-course-solutions/labs.md"
	ReviewFile = "docs/n4-dev-course-solutions/review-assignments.md"
	ExamFile   = "docs/n4-dev-course-solutions/certification-exam.md"
	LabText    = "sources/n4-dev-course/text/Lab Instructions.txt"
)

// SourceFiles are parsed in this order; every one is required.
var SourceFiles = []string{LabsFile, ReviewFile, ExamFile}

// Statuses is the vocabulary in print order; "written" means none of the others.
var Statuses = []string{"predicted", "unverified", "corrected", "identical", "verified", "written"}

// Range is lines A..B (1-based, inclusive) of a text file.
type Range struct{ A, B int }

func (r Range) String() string {
	if r.A == r.B {
		return strconv.Itoa(r.A)
	}
	return fmt.Sprintf("%d-%d", r.A, r.B)
}

// Item is one answer of a key: a lab step, a requirement, a decision, a trap, a check or a
// problem.
type Item struct {
	ID         string   // as written in the source (a range keeps its dash)
	Covers     []string // the IDs a range item stands for; nil for one ID
	Kind       string
	Group      string
	Section    string
	Subsection string
	Step       string
	TextFile   string // relative path of the text the ranges point into, "" if none
	Ranges     []Range
	Status     []string // markers in vocabulary order; empty = written
	Verified   []string // the evidence of the verified markers, in EvidenceKinds order ("live", "offline"); nil = none or unstated
	LiveRun    bool
	File       string
	Line       int
	Answer     string
}

func (it *Item) HasStatus(s string) bool {
	if s == "written" {
		return len(it.Status) == 0
	}
	for _, x := range it.Status {
		if x == s {
			return true
		}
	}
	return false
}

func (it *Item) StatusList() []string {
	if len(it.Status) == 0 {
		return []string{"written"}
	}
	return it.Status
}

// StatusLabels is StatusList with the evidence of a verified item spelled out: "verified (live)",
// "verified (live, offline)"; a verified status of unstated evidence stays "verified".
func (it *Item) StatusLabels() []string {
	out := append([]string(nil), it.StatusList()...)
	for i, s := range out {
		if s == "verified" && len(it.Verified) > 0 {
			out[i] = "verified (" + strings.Join(it.Verified, ", ") + ")"
		}
	}
	return out
}

// HasEvidence reports whether one of the item's verified markers is of evidence kind e.
func (it *Item) HasEvidence(e string) bool {
	for _, x := range it.Verified {
		if x == e {
			return true
		}
	}
	return false
}

// Counts is one row of course status: the items, then the items per status. An item counts under
// every status it carries (the columns overlap); written = none of the others. Live and Offline
// split Verified by the evidence of the markers (an item with both counts in both; one whose
// evidence is unstated, in neither).
type Counts struct {
	Items      int `json:"items"`
	Predicted  int `json:"predicted"`
	Unverified int `json:"unverified"`
	Corrected  int `json:"corrected"`
	Identical  int `json:"identical"`
	Verified   int `json:"verified"`
	Live       int `json:"verified_live"`
	Offline    int `json:"verified_offline"`
	Written    int `json:"written"`
}

// Add counts one item.
func (r *Counts) Add(it *Item) {
	r.Items++
	for _, s := range it.StatusList() {
		switch s {
		case "predicted":
			r.Predicted++
		case "unverified":
			r.Unverified++
		case "corrected":
			r.Corrected++
		case "identical":
			r.Identical++
		case "verified":
			r.Verified++
		case "written":
			r.Written++
		}
	}
	if !it.HasStatus("verified") {
		return // evidence counts only under the verified status: live, offline <= verified
	}
	if it.HasEvidence(EvidenceLive) {
		r.Live++
	}
	if it.HasEvidence(EvidenceOffline) {
		r.Offline++
	}
}

// CountColumns are the column names of Values.
var CountColumns = []string{"items", "predicted", "unverified", "corrected", "identical", "verified", "live", "offline", "written"}

// Values are the counts in column order: items, then Statuses with verified split into live and
// offline right after it.
func (r *Counts) Values() []int {
	return []int{r.Items, r.Predicted, r.Unverified, r.Corrected, r.Identical, r.Verified, r.Live, r.Offline, r.Written}
}

// Group is a lab, a review assignment, the needs-a-run section or the exam.
type Group struct {
	Key      string
	Title    string
	TextFile string
	Ranges   []Range
}

// Corpus is every item of the three answer keys.
type Corpus struct {
	Items  []*Item
	Groups []*Group
	byKey  map[string]*Item
	errs   []parseError
}

type parseError struct {
	file string
	line int
	msg  string
}

func newCorpus() *Corpus { return &Corpus{byKey: map[string]*Item{}} }

func (c *Corpus) errorf(file string, line int, format string, args ...any) {
	c.errs = append(c.errs, parseError{file, line, fmt.Sprintf(format, args...)})
}

// sortedErrors orders the errors by source file, then line.
func (c *Corpus) sortedErrors() []parseError {
	rank := map[string]int{}
	for i, f := range SourceFiles {
		rank[f] = i
	}
	errs := append([]parseError(nil), c.errs...)
	sort.SliceStable(errs, func(i, j int) bool {
		if rank[errs[i].file] != rank[errs[j].file] {
			return rank[errs[i].file] < rank[errs[j].file]
		}
		return errs[i].line < errs[j].line
	})
	return errs
}

func (c *Corpus) Group(key string) *Group {
	for _, g := range c.Groups {
		if g.Key == key {
			return g
		}
	}
	return nil
}

func (c *Corpus) addGroup(g *Group) *Group {
	if old := c.Group(g.Key); old != nil {
		return old
	}
	c.Groups = append(c.Groups, g)
	return g
}

// add registers the item under its ID and every covered ID; a key already taken is a
// duplicate error and the item is still listed, so counts stay complete.
func (c *Corpus) add(it *Item) {
	c.Items = append(c.Items, it)
	keys := append([]string{it.ID}, it.Covers...)
	for _, k := range keys {
		nk := normID(k)
		if old, ok := c.byKey[nk]; ok && old != it {
			c.errorf(it.File, it.Line, "duplicate ID %s (first at %s:%d)", k, old.File, old.Line)
			continue
		}
		c.byKey[nk] = it
	}
}

// lookup finds an item by ID, range ID or covered ID, in any case.
func (c *Corpus) Lookup(id string) *Item { return c.byKey[normID(id)] }

func normID(s string) string { return strings.ToUpper(strings.ReplaceAll(s, "–", "-")) }

var (
	rangeLeft  = regexp.MustCompile(`^(.*?)(\d+)$`)
	rangeRight = regexp.MustCompile(`^(.*?)(\d+)$`)
)

// expandRange turns "L2.2–L2.4" or "RA1.V9–V14" into its IDs. The right side repeats the
// whole prefix, the part after the last dot, or nothing; ok is false for anything else.
func expandRange(id string) (ids []string, ok bool) {
	parts := strings.Split(strings.ReplaceAll(id, "–", "-"), "-")
	if len(parts) != 2 {
		return nil, false
	}
	l, r := rangeLeft.FindStringSubmatch(parts[0]), rangeRight.FindStringSubmatch(parts[1])
	if l == nil || r == nil || l[1] == "" {
		return nil, false
	}
	prefix := l[1]
	short := prefix[strings.LastIndex(prefix, ".")+1:]
	if r[1] != "" && r[1] != prefix && r[1] != short {
		return nil, false
	}
	lo, _ := strconv.Atoi(l[2])
	hi, _ := strconv.Atoi(r[2])
	if hi <= lo || hi-lo > 1000 {
		return nil, false
	}
	for n := lo; n <= hi; n++ {
		ids = append(ids, prefix+strconv.Itoa(n))
	}
	return ids, true
}

var (
	predictedRe  = regexp.MustCompile(`\*\(predicted\)\*`)
	unverifiedRe = regexp.MustCompile(`\*\(unverified — not compiled\)\*`)
	correctedRe  = regexp.MustCompile(`(?i)\*\*corrected\*\*`)
	identicalRe  = regexp.MustCompile(`(?i)\*\*identical\*\*`)
	// verifiedRe is an item confirmed after the key was written: live (from what a station
	// stores: its bog, its files, its histories) or offline (from a jar, the source tree or the lab
	// sources); the group is the evidence kind.
	verifiedRe = regexp.MustCompile(`\*\(verified (live|offline) \d{4}-\d{2}-\d{2}\)\*`)
)

// The evidence kinds of a verified marker, in print order.
const (
	EvidenceLive    = "live"
	EvidenceOffline = "offline"
)

// EvidenceKinds is the print order of Item.Verified.
var EvidenceKinds = []string{EvidenceLive, EvidenceOffline}

// evidence returns the evidence kinds of the verified markers in text, plus the inherited ones,
// in EvidenceKinds order.
func evidence(text string, inherited ...string) []string {
	found := map[string]bool{}
	for _, e := range inherited {
		found[e] = true
	}
	for _, m := range verifiedRe.FindAllStringSubmatch(text, -1) {
		found[m[1]] = true
	}
	var out []string
	for _, e := range EvidenceKinds {
		if found[e] {
			out = append(out, e)
		}
	}
	return out
}

// markers returns the Statuses whose marker occurs in text, plus the inherited ones.
func markers(text string, inherited ...string) []string {
	found := map[string]bool{}
	for _, s := range inherited {
		found[s] = true
	}
	found["predicted"] = found["predicted"] || predictedRe.MatchString(text)
	found["unverified"] = found["unverified"] || unverifiedRe.MatchString(text)
	found["corrected"] = found["corrected"] || correctedRe.MatchString(text)
	found["identical"] = found["identical"] || identicalRe.MatchString(text)
	found["verified"] = found["verified"] || verifiedRe.MatchString(text)
	var out []string
	for _, s := range Statuses {
		if found[s] {
			out = append(out, s)
		}
	}
	return out
}

// headingStatus is the status a section heading gives the items under it.
func headingStatus(heading string) []string {
	if strings.Contains(strings.ToLower(heading), "unverified") || strings.Contains(heading, "Needs a run") {
		return []string{"unverified"}
	}
	return nil
}

// SplitLines splits a file into lines, accepting CRLF.
func SplitLines(data []byte) []string {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func isFence(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "```") }
