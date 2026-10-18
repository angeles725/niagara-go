package course

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// scope is "searched N items in labs.md, review-assignments.md, certification-exam.md".
func scope(c *key.Corpus) string {
	names := make([]string, len(key.SourceFiles))
	for i, f := range key.SourceFiles {
		names[i] = f[strings.LastIndex(f, "/")+1:]
	}
	return fmt.Sprintf("searched %s in %s", key.Plural(len(c.Items), "item"), strings.Join(names, ", "))
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func textLine(it *key.Item) string {
	if it.TextFile == "" || len(it.Ranges) == 0 {
		return "(none)"
	}
	rs := make([]string, len(it.Ranges))
	for i, r := range it.Ranges {
		rs[i] = r.String()
	}
	return it.TextFile + ":" + strings.Join(rs, ", ")
}

func printItem(w io.Writer, it *key.Item) {
	field := func(label, value string) { fmt.Fprintf(w, "  %-10s%s\n", label+":", value) }
	fmt.Fprintf(w, "%s (%s)\n", it.ID, it.Kind)
	section := it.Section
	if it.Subsection != "" {
		section += " / " + it.Subsection
	}
	field("section", section)
	if len(it.Covers) > 0 {
		field("covers", strings.Join(it.Covers, ", "))
	}
	field("step", it.Step)
	field("text", textLine(it))
	field("status", strings.Join(it.StatusList(), ", "))
	if it.LiveRun {
		field("live run", "needed")
	}
	field("source", fmt.Sprintf("%s:%d", it.File, it.Line))
	fmt.Fprintln(w, "  answer:")
	for _, l := range strings.Split(it.Answer, "\n") {
		if strings.TrimSpace(l) == "" {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
}

type itemJSON struct {
	ID         string   `json:"id"`
	Covers     []string `json:"covers"`
	Kind       string   `json:"kind"`
	Group      string   `json:"group"`
	Section    string   `json:"section"`
	Subsection string   `json:"subsection"`
	Step       string   `json:"step"`
	TextFile   string   `json:"text_file"`
	Ranges     []string `json:"ranges"`
	Status     []string `json:"status"`
	LiveRun    bool     `json:"live_run"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Answer     string   `json:"answer"`
}

func toJSON(it *key.Item) itemJSON {
	j := itemJSON{ID: it.ID, Covers: append([]string{}, it.Covers...), Kind: it.Kind, Group: it.Group,
		Section: it.Section, Subsection: it.Subsection, Step: it.Step, TextFile: it.TextFile,
		Ranges: []string{}, Status: it.StatusList(), LiveRun: it.LiveRun, File: it.File, Line: it.Line, Answer: it.Answer}
	for _, r := range it.Ranges {
		j.Ranges = append(j.Ranges, r.String())
	}
	return j
}

// cmdShow prints every found item; any unknown ID makes the exit 1.
func cmdShow(c *key.Corpus, s cli.Stdio, ids []string, asJSON bool) int {
	var found []*key.Item
	code := cli.ExitFound
	for _, id := range ids {
		it := c.Lookup(id)
		if it == nil {
			fmt.Fprintf(s.Err, "course: no item %s (%s)\n", id, scope(c))
			code = cli.ExitNoMatch
			continue
		}
		found = append(found, it)
	}
	if asJSON {
		out := []itemJSON{}
		for _, it := range found {
			out = append(out, toJSON(it))
		}
		writeJSON(s.Out, out)
		return code
	}
	for i, it := range found {
		if i > 0 {
			fmt.Fprintln(s.Out)
		}
		printItem(s.Out, it)
	}
	return code
}

// cmdText prints the numbered text lines of an item's or a group's ranges.
func cmdText(c *key.Corpus, s cli.Stdio, root, target string) int {
	var file, label string
	var ranges []key.Range
	if it := c.Lookup(target); it != nil {
		file, ranges, label = it.TextFile, it.Ranges, it.ID
	} else if keys := c.GroupKeys(target); len(keys) == 1 {
		g := c.Group(keys[0])
		file, ranges, label = g.TextFile, g.Ranges, g.Key
	} else {
		fmt.Fprintf(s.Err, "course: no item or group %s (searched %s, %s)\n", target,
			key.Plural(len(c.Items), "item"), key.Plural(len(c.Groups), "group"))
		return cli.ExitNoMatch
	}
	if file == "" || len(ranges) == 0 {
		fmt.Fprintf(s.Err, "course: %s has no text range\n", label)
		return cli.ExitNoMatch
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if os.IsNotExist(err) {
		fmt.Fprintf(s.Err, "course: missing text file %s\n", file)
		return cli.ExitCannotSearch
	}
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot read %s: %v\n", file, err)
		return cli.ExitCannotSearch
	}
	lines := key.SplitLines(raw)
	for _, r := range ranges {
		if r.A < 1 || r.B < r.A || r.B > len(lines) {
			fmt.Fprintf(s.Err, "course: %s: range %s is outside %s (%s)\n", label, r, file, key.Plural(len(lines), "line"))
			return cli.ExitCannotSearch
		}
	}
	for i, r := range ranges {
		if i > 0 {
			fmt.Fprintln(s.Out)
		}
		fmt.Fprintf(s.Out, "%s:%s (%s)\n", file, r, label)
		for n := r.A; n <= r.B; n++ {
			// A blank text line prints only its number: no trailing whitespace.
			fmt.Fprintln(s.Out, strings.TrimRight(fmt.Sprintf("%5d  %s", n, lines[n-1]), " \t"))
		}
	}
	return cli.ExitFound
}
