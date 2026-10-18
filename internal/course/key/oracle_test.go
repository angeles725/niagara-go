package key

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The recall oracle rebuilds the ID set from the raw files with plain regexes, independent
// of the parser's line state machine, and expands ranges with its own rule; the parsed items
// must cover exactly that set.
var (
	oracleLab    = regexp.MustCompile(`\*\*((?:L\d+|LB|LA)\.\d+(?:[–-](?:L\d+|LB|LA)\.\d+)?)\*\*`)
	oracleTable  = regexp.MustCompile(`(?m)^\| (RA\d+\.[A-Za-z0-9]+(?:[–-][A-Za-z0-9]+)?|D\d+\.\d+|T\d+\.\d+|X\d+)[ |]`)
	oracleExam   = regexp.MustCompile(`(?m)^## \d+\. Problem (\d+) `)
	oracleSpread = regexp.MustCompile(`^(.*?)(\d+)[–-](?:[^.]*\.)?\D*(\d+)$`)
)

func oracleExpand(id string) []string {
	m := oracleSpread.FindStringSubmatch(id)
	if m == nil {
		return []string{id}
	}
	lo, _ := strconv.Atoi(m[2])
	hi, _ := strconv.Atoi(m[3])
	var out []string
	for n := lo; n <= hi; n++ {
		out = append(out, m[1]+strconv.Itoa(n))
	}
	return out
}

func readSource(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func oracleIDs(t *testing.T, root string) []string {
	t.Helper()
	set := map[string]bool{}
	for _, m := range oracleLab.FindAllStringSubmatch(readSource(t, root, LabsFile), -1) {
		for _, id := range oracleExpand(m[1]) {
			set[normID(id)] = true
		}
	}
	for _, m := range oracleTable.FindAllStringSubmatch(readSource(t, root, ReviewFile), -1) {
		for _, id := range oracleExpand(m[1]) {
			set[normID(id)] = true
		}
	}
	for _, m := range oracleExam.FindAllStringSubmatch(readSource(t, root, ExamFile), -1) {
		set["P"+m[1]] = true
	}
	return sortedKeys(set)
}

func parsedIDs(c *Corpus) []string {
	set := map[string]bool{}
	for _, it := range c.Items {
		if len(it.Covers) == 0 {
			set[normID(it.ID)] = true
		}
		for _, id := range it.Covers {
			set[normID(id)] = true
		}
	}
	return sortedKeys(set)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func diff(a, b []string) (onlyA, onlyB []string) {
	in := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	ma, mb := in(a), in(b)
	for _, x := range a {
		if !mb[x] {
			onlyA = append(onlyA, x)
		}
	}
	for _, x := range b {
		if !ma[x] {
			onlyB = append(onlyB, x)
		}
	}
	return
}

// checkRecall loads root and compares the parsed IDs with the oracle; it also checks that
// every item's source line names its ID.
func checkRecall(t *testing.T, root string) *Corpus {
	t.Helper()
	c, problems := Load(root)
	if c == nil {
		t.Fatalf("load %s: %s", root, strings.Join(problems, "\n"))
	}
	missed, extra := diff(oracleIDs(t, root), parsedIDs(c))
	if len(missed) > 0 || len(extra) > 0 {
		t.Errorf("recall: oracle IDs not parsed %v; parsed IDs not in the oracle %v", missed, extra)
	}
	files := map[string][]string{}
	for _, f := range SourceFiles {
		files[f] = strings.Split(strings.ReplaceAll(readSource(t, root, f), "\r\n", "\n"), "\n")
	}
	for _, it := range c.Items {
		id := it.ID
		if it.Kind == "exam problem" {
			id = "Problem " + strings.TrimPrefix(it.ID, "P")
		}
		lines := files[it.File]
		if it.Line < 1 || it.Line > len(lines) {
			t.Errorf("%s: line %d is outside %s (%d lines)", it.ID, it.Line, it.File, len(lines))
			continue
		}
		if line := lines[it.Line-1]; !strings.Contains(line, id) {
			t.Errorf("%s: %s:%d does not name it: %q", it.ID, it.File, it.Line, line)
		}
	}
	return c
}

func TestRecallFixture(t *testing.T) {
	c := checkRecall(t, "../testdata/root")
	if got := len(c.Items); got != 24 {
		t.Errorf("fixture items = %d, want 24", got)
	}
}

func TestParseErrorsFixture(t *testing.T) {
	c, problems := Load("../testdata/dup-root")
	want := []string{
		LabsFile + ":6: malformed item ID L1.x",
		LabsFile + ":7: item L2.1 is outside its group L1",
		LabsFile + ":8: duplicate ID L1.1 (first at " + LabsFile + ":5)",
		LabsFile + ":12: live-run ID L1.9 is not an item",
		LabsFile + ":12: live-run range L1.4–L1.2 is reversed or crosses groups",
		ReviewFile + ":10: duplicate ID D1.1 (first at " + ReviewFile + ":9)",
		ReviewFile + ":11: table row has 2 cells, want 3",
		ReviewFile + ":17: traceability ID RA1.R9 is not a requirement",
		ReviewFile + ":19: traceability row FOO is not a requirement ID",
		"9 parse errors; nothing searched",
	}
	if c != nil || !reflect.DeepEqual(problems, want) {
		t.Errorf("dup-root: corpus %v, problems:\n%s\nwant:\n%s", c != nil, strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
}

// countsByGroup counts the items per group and in total through Counts.Add, the counter of
// course status, so the pinned numbers are the ones the command prints.
func countsByGroup(c *Corpus) map[string]*Counts {
	rows := map[string]*Counts{"total": {}}
	for _, it := range c.Items {
		if rows[it.Group] == nil {
			rows[it.Group] = &Counts{}
		}
		rows[it.Group].Add(it)
		rows["total"].Add(it)
	}
	return rows
}

// TestRecallReal runs the oracle on the real answer keys and pins their counts, so a doc
// edit that the parser no longer reads fails here instead of shrinking an answer.
func TestRecallReal(t *testing.T) {
	root := os.Getenv(RootEnv)
	if root == "" {
		t.Skipf("%s is not set: the real niagara-research answer keys are not checked", RootEnv)
	}
	if !HasMarker(root) {
		t.Skipf("%s=%s has no %s: the real answer keys are not checked", RootEnv, root, LabsFile)
	}
	c := checkRecall(t, root)
	rows := countsByGroup(c)
	want := map[string]int{"L1": 26, "L2": 12, "L3": 11, "L4": 16, "L5": 25, "L6": 35, "L7": 21, "L8": 17,
		"L9": 15, "L10": 25, "L11": 43, "LB": 21, "LA": 4, "RA1": 39, "RA2": 47, "RAX": 10, "EXAM": 3}
	for g, n := range want {
		if rows[g] == nil || rows[g].Items != n {
			t.Errorf("group %s items = %v, want %d", g, rows[g], n)
		}
	}
	got := *rows["total"]
	// 2026-10-17: L10.24 is verified live (from the exported CSV), no longer predicted.
	// 2026-10-18: every verified marker is live (L10.23, L10.24 and the exam's file status); none offline yet.
	pinned := Counts{Items: 370, Predicted: 39, Unverified: 20, Corrected: 10, Identical: 50, Verified: 5, Live: 5, Written: 258}
	if got != pinned {
		t.Errorf("totals = %+v, want %+v", got, pinned)
	}
	live, ids := 0, len(parsedIDs(c))
	for _, it := range c.Items {
		if it.LiveRun {
			live++
		}
	}
	if live != 10 || ids != 382 {
		t.Errorf("live-run items = %d (want 10), distinct IDs with ranges expanded = %d (want 382)", live, ids)
	}
	text := strings.Split(strings.TrimSuffix(readSource(t, root, LabText), "\n"), "\n")
	for _, it := range c.Items {
		for _, r := range it.Ranges {
			if it.TextFile == LabText && (r.A < 1 || r.B < r.A || r.B > len(text)) {
				t.Errorf("%s: LI range %s outside Lab Instructions.txt (%d lines)", it.ID, r, len(text))
			}
		}
	}
}
