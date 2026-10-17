package course

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
)

func (c *Corpus) scope() string {
	names := make([]string, len(sourceFiles))
	for i, f := range sourceFiles {
		names[i] = f[strings.LastIndex(f, "/")+1:]
	}
	return fmt.Sprintf("searched %s in %s", plural(len(c.Items), "item"), strings.Join(names, ", "))
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func textLine(it *Item) string {
	if it.TextFile == "" || len(it.Ranges) == 0 {
		return "(none)"
	}
	rs := make([]string, len(it.Ranges))
	for i, r := range it.Ranges {
		rs[i] = r.String()
	}
	return it.TextFile + ":" + strings.Join(rs, ", ")
}

func printItem(w io.Writer, it *Item) {
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
	field("status", strings.Join(it.statusList(), ", "))
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

func toJSON(it *Item) itemJSON {
	j := itemJSON{ID: it.ID, Covers: append([]string{}, it.Covers...), Kind: it.Kind, Group: it.Group,
		Section: it.Section, Subsection: it.Subsection, Step: it.Step, TextFile: it.TextFile,
		Ranges: []string{}, Status: it.statusList(), LiveRun: it.LiveRun, File: it.File, Line: it.Line, Answer: it.Answer}
	for _, r := range it.Ranges {
		j.Ranges = append(j.Ranges, r.String())
	}
	return j
}

// cmdShow prints every found item; any unknown ID makes the exit 1.
func cmdShow(c *Corpus, s cli.Stdio, ids []string, asJSON bool) int {
	var found []*Item
	code := cli.ExitFound
	for _, id := range ids {
		it := c.lookup(id)
		if it == nil {
			fmt.Fprintf(s.Err, "course: no item %s (%s)\n", id, c.scope())
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

// groupKeys resolves a --lab / text spelling to group keys: 3 and L3, B and LB, A and LA,
// RA (RA1, RA2 and RAX together) or a key in any case.
func (c *Corpus) groupKeys(name string) []string {
	n := strings.ToUpper(strings.TrimSpace(name))
	switch {
	case n == "B" || n == "A":
		n = "L" + n
	case n != "" && strings.Trim(n, "0123456789") == "":
		n = "L" + n
	case n == "RA":
		var keys []string
		for _, g := range c.Groups {
			if strings.HasPrefix(g.Key, "RA") {
				keys = append(keys, g.Key)
			}
		}
		return keys
	}
	if c.group(n) == nil {
		return nil
	}
	return []string{n}
}

func (c *Corpus) groupList() string {
	keys := make([]string, len(c.Groups))
	for i, g := range c.Groups {
		keys[i] = g.Key
	}
	return strings.Join(keys, ", ")
}

// cmdText prints the numbered text lines of an item's or a group's ranges.
func cmdText(c *Corpus, s cli.Stdio, root, target string) int {
	var file, label string
	var ranges []Range
	if it := c.lookup(target); it != nil {
		file, ranges, label = it.TextFile, it.Ranges, it.ID
	} else if keys := c.groupKeys(target); len(keys) == 1 {
		g := c.group(keys[0])
		file, ranges, label = g.TextFile, g.Ranges, g.Key
	} else {
		fmt.Fprintf(s.Err, "course: no item or group %s (searched %s, %s)\n", target,
			plural(len(c.Items), "item"), plural(len(c.Groups), "group"))
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
	lines := splitLines(raw)
	for _, r := range ranges {
		if r.A < 1 || r.B < r.A || r.B > len(lines) {
			fmt.Fprintf(s.Err, "course: %s: range %s is outside %s (%s)\n", label, r, file, plural(len(lines), "line"))
			return cli.ExitCannotSearch
		}
	}
	for i, r := range ranges {
		if i > 0 {
			fmt.Fprintln(s.Out)
		}
		fmt.Fprintf(s.Out, "%s:%s (%s)\n", file, r, label)
		for n := r.A; n <= r.B; n++ {
			fmt.Fprintf(s.Out, "%5d  %s\n", n, lines[n-1])
		}
	}
	return cli.ExitFound
}
