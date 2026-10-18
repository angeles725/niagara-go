// Package course looks up the N4 developer-course answer keys of a niagara-research
// checkout (labs.md, review-assignments.md, certification-exam.md) by item ID, counts the
// items by verification status, and prints the course text an item points at. The parse
// contract is docs/course.md.
//
// Exit: 0 found, 1 searched with no match (scope and counts on stderr), 2 cannot search
// (no root, a missing or ambiguous answer key, a missing text file, a usage error).
package course

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/version"
)

const description = `Look up the N4 developer-course answer keys of a niagara-research checkout
(labs, review assignments, certification exam) by item ID, count them by
verification status, print the course text an item points at, and check what
the labs leave in a station.

root: --root DIR, else NIAGARA_RESEARCH_DIR, else the first directory above the
working directory (then above this executable) that holds
docs/n4-dev-course-solutions/labs.md.

exit: 0 found; 1 searched, no match (show: any unknown ID, even when the
others were printed); 2 cannot search (no root, a missing or ambiguous answer
key, a missing text file). check: 0 no FAIL (SKIPs are counted, never passed);
1 at least one FAIL; 2 the station cannot be read, or every check was skipped.
verify-predictions: 0 no predicted item contradicted; 1 at least one
CONTRADICTED; 2 cannot search, the station cannot be read, or every linked
check was skipped.

examples:
  course show L1.9 RA1.R6 P2
  course status --lab 6 --status predicted
  course text L5.16
  course check --station ~/Niagara4.15/tridium/stations/TrainingStation \
    --install /opt/Niagara/Niagara-4.15.6.20 --lab 4 --lab 10
  course verify-predictions --station ~/Niagara4.15/tridium/stations/TrainingStation`

// rootParent holds --root: None on the main parser, SUPPRESS on the subcommands, so the
// flag works before or after the subcommand.
func rootParent(suppress bool) *argparse.Parser {
	var def any
	if suppress {
		def = argparse.Suppress
	}
	p := argparse.New(argparse.Config{NoHelp: true})
	p.Add(argparse.Arg{Default: def, Metavar: "DIR",
		Help: "niagara-research checkout (default: $NIAGARA_RESEARCH_DIR, then found from cwd)"}, "--root")
	return p
}

func buildParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "course", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter, Parents: []*argparse.Parser{rootParent(false)}})
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Dest: argparse.Suppress, Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line("course"))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	add := func(name, help string) *argparse.Parser {
		return sub.AddParser(name, argparse.SubparserConfig{
			Config: argparse.Config{Parents: []*argparse.Parser{rootParent(true)}}, Help: help})
	}
	jsonHelp := argparse.Arg{Action: argparse.StoreTrue, Help: "machine JSON output"}
	s := add("show", "print items by ID (any case; L2.3 also finds the range item L2.2–L2.4)")
	s.Add(argparse.Arg{Nargs: argparse.OneOrMore, Metavar: "ID", Help: "item ID: L1.9, LB.3, RA1.R6, D2.4, T1.1, X3, P2"}, "ids")
	s.Add(jsonHelp, "--json")
	st := add("status", "items per group and status; --status lists the matching items")
	st.Add(argparse.Arg{Metavar: "GROUP", Help: "one group: a lab number (3 or L3), B, A, RA1, RA2, RAX, RA (all three), EXAM"}, "--lab")
	choices := make([]any, len(statuses))
	for i, s := range statuses {
		choices[i] = s
	}
	st.Add(argparse.Arg{Choices: choices, Help: "list the items carrying this status (written = no marker)"}, "--status")
	st.Add(jsonHelp, "--json")
	t := add("text", "print the course text lines an item (or a lab) points at")
	t.Add(argparse.Arg{Metavar: "ID|GROUP", Help: "item ID or group (L3, 3, B, A)"}, "target")
	ck := add("check", "check what each lab leaves in a station (and the built jars): PASS/FAIL/SKIP per check")
	ck.Add(argparse.Arg{Required: true, Metavar: "PATH", Help: "station directory, or its config.bog (zipped or plain XML)"}, "--station")
	ck.Add(argparse.Arg{Metavar: "DIR", Help: "Niagara install whose modules/ holds the built jars (jar checks SKIP without it)"}, "--install")
	ck.Add(argparse.Arg{Action: argparse.Append, Metavar: "N", Help: "only this lab (1-11, B); repeatable"}, "--lab")
	ck.Add(jsonHelp, "--json")
	vp := add("verify-predictions", "run the checks linked to every *(predicted)* item: CONFIRMED, CONTRADICTED or UNDECIDED")
	vp.Add(argparse.Arg{Required: true, Metavar: "PATH", Help: "station directory, or its config.bog (zipped or plain XML)"}, "--station")
	vp.Add(argparse.Arg{Metavar: "DIR", Help: "Niagara install whose modules/ holds the built jars (jar checks SKIP without it)"}, "--install")
	vp.Add(argparse.Arg{Metavar: "YYYY-MM-DD", Help: "date written in the suggested *(verified live ...)* lines (default: today)"}, "--date")
	vp.Add(jsonHelp, "--json")
	return p
}

// Main runs course with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	p := buildParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	if ns.String("cmd") == "check" { // reads a station, not the answer keys
		return cmdCheck(s, ns.String("station"), ns.String("install"), ns.Strings("lab"), ns.Bool("json"))
	}
	root, err := findRoot(ns.String("root"), ns.Get("root") != nil)
	if err != nil {
		fmt.Fprintf(s.Err, "course: %v\n", err)
		return cli.ExitCannotSearch
	}
	c, problems := load(root)
	if c == nil {
		for _, pr := range problems {
			fmt.Fprintf(s.Err, "course: %s\n", pr)
		}
		return cli.ExitCannotSearch
	}
	switch ns.String("cmd") {
	case "show":
		return cmdShow(c, s, ns.Strings("ids"), ns.Bool("json"))
	case "verify-predictions":
		return cmdVerifyPredictions(c, s, ns.String("station"), ns.String("install"), ns.String("date"), ns.Bool("json"))
	case "status":
		return cmdStatus(c, s, ns.Get("lab") != nil, ns.String("lab"), ns.String("status"), ns.Bool("json"))
	default:
		return cmdText(c, s, root, ns.String("target"))
	}
}
