package course

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// findHit is the first line of an item (its step, then its answer lines) that a query matches.
type findHit struct {
	item *key.Item
	line string
}

// cmdFind prints one row per item whose step or answer matches the Go regexp q (case folded
// with -i): group, ID, statuses and the first matching line; with slides it searches the course
// text files instead (cmdFindSlides). Exit: 0 at least one match; 1 none (the scope on stderr,
// as show prints it); 2 a regexp that does not compile.
func cmdFind(c *key.Corpus, s cli.Stdio, root, q string, fold, slides bool) int {
	expr := q
	if fold {
		expr = "(?i)" + q
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		fmt.Fprintf(s.Err, "course: bad regexp %s: %v\n", q, err)
		return cli.ExitCannotSearch
	}
	if slides {
		return cmdFindSlides(s, root, q, re)
	}
	var hits []findHit
	for _, it := range c.Items {
		for _, l := range append([]string{it.Step}, strings.Split(it.Answer, "\n")...) {
			if re.MatchString(l) {
				hits = append(hits, findHit{it, strings.TrimSpace(l)})
				break
			}
		}
	}
	if len(hits) == 0 {
		fmt.Fprintf(s.Err, "course: no item matches %s (%s)\n", q, scope(c))
		return cli.ExitNoMatch
	}
	gw, iw, sw := 0, 0, 0
	for _, h := range hits {
		gw = max(gw, len(h.item.Group))
		iw = max(iw, len([]rune(h.item.ID)))
		sw = max(sw, len(strings.Join(h.item.StatusList(), ", ")))
	}
	for _, h := range hits {
		st := strings.Join(h.item.StatusList(), ", ")
		fmt.Fprintf(s.Out, "%-*s  %s%s  %-*s  %s\n", gw, h.item.Group, h.item.ID,
			strings.Repeat(" ", iw-len([]rune(h.item.ID))), sw, st, truncate(h.line, 120))
	}
	return cli.ExitFound
}
