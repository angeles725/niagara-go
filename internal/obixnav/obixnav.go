// Package obixnav is a read-only facade<->rt auditor for the PANCCADIA JACE over oBIX (port of
// niagara-research tools/obix-nav.py): one oBIX Batch read pulls hundreds of slot values, so a
// whole-station audit takes about a second. It never writes.
package obixnav

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Options is the parsed command line.
type Options struct {
	Base     string // oBIX root, e.g. https://127.0.0.1:18443/obix
	PassFile string // file holding the API user's password
	Command  string // map, cond, wiring, config, read or dump
	Room     int    // map/wiring/config --room; 0 means every room
	JSON     bool   // map --json (accepted, no effect, as in Python)
	Ord      string // read <ord>
	DumpRoom string // dump <room>, parsed like Python int()
}

type runner struct {
	o Options
	c *client
	s cli.Stdio
}

// Run executes one obix-nav invocation: 0 done, 1 no pass file, 2 the station failed (auth, TLS,
// refused, HTTP error, wrong --base) or a bad room argument.
func Run(o Options, s cli.Stdio) int {
	if _, err := os.Stat(o.PassFile); err != nil {
		fmt.Fprintf(s.Err, "No pass file: %s\n", o.PassFile)
		return cli.ExitNoMatch
	}
	raw, err := os.ReadFile(o.PassFile)
	if err != nil {
		fmt.Fprintf(s.Err, "obix-nav: cannot read %s: %v\n", o.PassFile, err)
		return cli.ExitCannotSearch
	}
	base := strings.TrimRight(o.Base, "/")
	r := &runner{o: o, c: newClient(base, pyStrip(string(raw))), s: s}
	cmds := map[string]func() error{"map": r.cmdMap, "cond": r.cmdCond, "wiring": r.cmdWiring,
		"config": r.cmdConfig, "read": r.cmdRead, "dump": r.cmdDump}
	cmd, ok := cmds[o.Command]
	if !ok {
		fmt.Fprintf(s.Err, "obix-nav: unknown command %q\n", o.Command)
		return cli.ExitCannotSearch
	}
	err = cmd()
	var ve valueError
	switch {
	case err == nil:
		return cli.ExitFound
	case errors.As(err, &ve):
		fmt.Fprintf(s.Err, "obix-nav: %s: %s\n", o.Command, err)
	default:
		fmt.Fprintf(s.Err, "obix-nav: %s failed against %s: %s\n", o.Command, base, err)
	}
	return cli.ExitCannotSearch
}

func (r *runner) cmdRead() error {
	o := r.o.Ord
	if !strings.HasPrefix(o, "/") {
		o = "/" + o
	}
	vals, err := r.c.batch([]string{o})
	if err != nil {
		return err
	}
	items := make([]string, len(vals.keys))
	for i, k := range vals.keys {
		items[i] = pyStrRepr(k) + ": " + tupleRepr(vals.m[k])
	}
	fmt.Fprintf(r.s.Out, "{%s}\n", strings.Join(items, ", "))
	return nil
}

func tupleRepr(res result) string {
	return "(" + pyStrRepr(res.tag) + ", " + optRepr(res.val) + ")"
}

var pyIntRE = regexp.MustCompile(`^[+-]?[0-9](?:_?[0-9])*$`)

// pyInt parses like Python int(str) for decimal text.
func pyInt(s string) (int, error) {
	t := pyStrip(s)
	if !pyIntRE.MatchString(t) {
		return 0, valueError(fmt.Sprintf("invalid literal for int() with base 10: %s", pyStrRepr(s)))
	}
	var n int
	if _, err := fmt.Sscan(strings.ReplaceAll(strings.TrimPrefix(t, "+"), "_", ""), &n); err != nil {
		return 0, valueError(fmt.Sprintf("invalid literal for int() with base 10: %s", pyStrRepr(s)))
	}
	return n, nil
}

func (r *runner) cmdDump() error {
	ri, err := pyInt(r.o.DumpRoom)
	if err != nil {
		return err
	}
	cr, err := r.coldRoom(ri)
	if err != nil {
		return err
	}
	units, err := r.listUnits(cr)
	if err != nil {
		return err
	}
	fac := fmt.Sprintf("/Services/DashboardService/Cuarto%d/", ri)
	xml, err := r.c.getConfig(strings.TrimSuffix(fac, "/"))
	if err != nil {
		return err
	}
	slotRE := regexp.MustCompile(fmt.Sprintf(`href="[^"]*?/Cuarto%d/([A-Za-z0-9_]+)/?"`, ri))
	seen := map[string]bool{}
	var ords []string
	for _, m := range slotRE.FindAllStringSubmatch(xml, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ords = append(ords, fac+m[1])
		}
	}
	vals, err := r.c.batch(ords)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.s.Out, "=== Cuarto%d facade (%d slots) ===\n", ri, len(ords))
	for _, o := range ords {
		fmt.Fprintf(r.s.Out, "  %-26s %s\n", o[strings.LastIndex(o, "/")+1:], tupleRepr(vals.get(o)))
	}
	fmt.Fprintf(r.s.Out, "  units rt: %s\n", pyListRepr(units))
	return nil
}
