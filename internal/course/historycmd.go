package course

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// accuracyRow is one line of course history: the graded answers of a group or a kind.
type accuracyRow struct {
	Name     string `json:"-"`
	Group    string `json:"group,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Answers  int    `json:"answers"`
	Right    int    `json:"right"`
	Wrong    int    `json:"wrong"`
	Accuracy int    `json:"accuracy"` // percent right, rounded half up
}

func (r *accuracyRow) add(result string) {
	r.Answers++
	if result == resultRight {
		r.Right++
	} else {
		r.Wrong++
	}
	r.Accuracy = (200*r.Right + r.Answers) / (2 * r.Answers)
}

type historyJSON struct {
	History string        `json:"history"`
	Groups  []accuracyRow `json:"groups"`
	Kinds   []accuracyRow `json:"kinds"`
	Total   accuracyRow   `json:"total"`
}

// naturalLess orders names with their digit runs read as numbers: L2 before L10.
func naturalLess(a, b string) int {
	for a != "" && b != "" {
		da, db := unicode.IsDigit(rune(a[0])), unicode.IsDigit(rune(b[0]))
		if da && db {
			na, ra := splitDigits(a)
			nb, rb := splitDigits(b)
			if na != nb {
				return na - nb
			}
			a, b = ra, rb
			continue
		}
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		a, b = a[1:], b[1:]
	}
	return len(a) - len(b)
}

func splitDigits(s string) (int, string) {
	i := 0
	for i < len(s) && unicode.IsDigit(rune(s[i])) {
		i++
	}
	n, _ := strconv.Atoi(s[:min(i, 9)])
	return n, s[i:]
}

// rows returns the rows of m in natural name order.
func rows(m map[string]*accuracyRow) []accuracyRow {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int { // equal in natural order (L01, L1): byte order decides
		if c := naturalLess(a, b); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	out := make([]accuracyRow, len(names))
	for i, n := range names {
		out[i] = *m[n]
	}
	return out
}

// cmdHistory prints the accuracy of the graded answers of the history, per group and per kind.
// Exit: 0 printed; 1 no answer in the history (the honest zero; a missing file is empty); 2 no
// history path, or the history cannot be read.
func cmdHistory(s cli.Stdio, flag string, hasFlag, asJSON bool, usage func(string) int) int {
	path, msg := historyPath(s, flag, hasFlag)
	if path == "" {
		return usage(msg)
	}
	recs, err := loadHistory(s, path)
	if err != nil {
		fmt.Fprintf(s.Err, "course: history %s: %v\n", path, err)
		return cli.ExitCannotSearch
	}
	if len(recs) == 0 {
		fmt.Fprintf(s.Err, "course: no answer in the history %s (course quiz --interactive records them)\n", path)
		return cli.ExitNoMatch
	}
	groups, kinds := map[string]*accuracyRow{}, map[string]*accuracyRow{}
	total := accuracyRow{Name: "total", Group: "total"}
	for _, r := range recs {
		if groups[r.Group] == nil {
			groups[r.Group] = &accuracyRow{Name: r.Group, Group: r.Group}
		}
		if kinds[r.Kind] == nil {
			kinds[r.Kind] = &accuracyRow{Name: r.Kind, Kind: r.Kind}
		}
		groups[r.Group].add(r.Result)
		kinds[r.Kind].add(r.Result)
		total.add(r.Result)
	}
	if asJSON {
		if !writeJSON(s, historyJSON{History: path, Groups: rows(groups), Kinds: rows(kinds), Total: total}) {
			return cli.ExitCannotSearch
		}
		return cli.ExitFound
	}
	fmt.Fprintf(s.Out, "history: %s in %s\n\n", key.Plural(len(recs), "answer"), path)
	printAccuracy(s.Out, "group", append(rows(groups), total))
	fmt.Fprintln(s.Out)
	printAccuracy(s.Out, "kind", rows(kinds))
	return cli.ExitFound
}

func printAccuracy(w io.Writer, first string, rs []accuracyRow) {
	headers := []string{"answers", "right", "wrong", "accuracy"}
	w0 := len(first)
	for _, r := range rs {
		w0 = max(w0, len(r.Name))
	}
	fmt.Fprintf(w, "%-*s  %s\n", w0, first, strings.Join(headers, "  "))
	for _, r := range rs {
		fmt.Fprintf(w, "%-*s  %*d  %*d  %*d  %*s\n", w0, r.Name, len(headers[0]), r.Answers, len(headers[1]), r.Right,
			len(headers[2]), r.Wrong, len(headers[3]), strconv.Itoa(r.Accuracy)+"%")
	}
}
