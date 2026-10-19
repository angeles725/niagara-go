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
	"github.com/angeles725/niagara-go/internal/course/key"
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
others were printed; find: no item matches); 2 cannot search (no root, a missing or ambiguous answer
key, a missing text file). check: 0 no FAIL (SKIPs are counted, never passed);
1 at least one FAIL; 2 the station cannot be read, or every check was skipped.
quiz: 0 drawn; 1 an unknown group or no question in it; 2 cannot search.
verify, verify-predictions (= verify --status predicted), progress: 0 no item
contradicted; 1 at least one CONTRADICTED; 2 cannot search, the station
cannot be read, or every linked check was skipped.

stations: --station, else $COURSE_TRAINING_STATION (labs),
$COURSE_REVIEW_STATION (RA1, RA2), $COURSE_CERT_STATION (EXAM);
--install, else $COURSE_NIAGARA_INSTALL; --source, else
$COURSE_NIAGARA_SOURCE (the 4.15 source tree the source-text checks read);
--labsrc, else $COURSE_LAB_SOURCES (the lab module sources the annotation
and lab-source checks read); --solutions, else $COURSE_SOLUTIONS (the corpus
solutions the review rows read).

examples:
  course show L1.9 RA1.R6 P2
  course status --lab 6 --status predicted
  course text L5.16
  course check --station ~/Niagara4.15/tridium/stations/TrainingStation \
    --install /opt/Niagara/Niagara-4.15.6.20 --lab 4 --lab 10
  course verify-predictions --station ~/Niagara4.15/tridium/stations/TrainingStation
  course verify --status written --lab RA1
  course progress --lab 3
  course quiz --lab 3 --seed 1 --answers
  course find -i 'history:'`

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

// buildParser returns the main parser and the subcommand parsers by name (their usage errors).
func buildParser() (*argparse.Parser, map[string]*argparse.Parser) {
	p := argparse.New(argparse.Config{Prog: "course", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter, Parents: []*argparse.Parser{rootParent(false)}})
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Dest: argparse.Suppress, Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line("course"))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	subs := map[string]*argparse.Parser{}
	add := func(name, help string) *argparse.Parser {
		subs[name] = sub.AddParser(name, argparse.SubparserConfig{
			Config: argparse.Config{Parents: []*argparse.Parser{rootParent(true)}}, Help: help})
		return subs[name]
	}
	jsonHelp := argparse.Arg{Action: argparse.StoreTrue, Help: "machine JSON output"}
	stationArg := argparse.Arg{Metavar: "PATH", Help: "station directory, or its config.bog (zipped or plain XML); default: " +
		"$COURSE_TRAINING_STATION for the labs, $COURSE_REVIEW_STATION for RA1/RA2, $COURSE_CERT_STATION for EXAM"}
	installArg := argparse.Arg{Metavar: "DIR", Help: "Niagara install whose modules/ holds the built jars " +
		"(default: $COURSE_NIAGARA_INSTALL; jar checks SKIP without either)"}
	sourceArg := argparse.Arg{Metavar: "DIR", Help: "Niagara source tree, one directory per module part, e.g. " +
		"niagara-help/versions/4.15.6.20/source (default: $COURSE_NIAGARA_SOURCE; source checks SKIP without either)"}
	labsrcArg := argparse.Arg{Metavar: "DIR", Help: "lab module sources: the Niagara dev home holding training/ and " +
		"trafficLightDriver/ (default: $COURSE_LAB_SOURCES; lab-source checks SKIP without either)"}
	solutionsArg := argparse.Arg{Metavar: "DIR", Help: "the participant's module sources as niagara-research keeps them, " +
		"sources/n4-dev-course/solutions (default: $COURSE_SOLUTIONS; solutions checks SKIP without either)"}
	s := add("show", "print items by ID (any case; L2.3 also finds the range item L2.2–L2.4)")
	s.Add(argparse.Arg{Nargs: argparse.OneOrMore, Metavar: "ID", Help: "item ID: L1.9, LB.3, RA1.R6, D2.4, T1.1, X3, P2"}, "ids")
	s.Add(jsonHelp, "--json")
	st := add("status", "items per group and status; --status lists the matching items")
	st.Add(argparse.Arg{Metavar: "GROUP", Help: "one group: a lab number (3 or L3), B, A, RA1, RA2, RAX, RA (all three), EXAM"}, "--lab")
	choices := make([]any, len(key.Statuses))
	for i, s := range key.Statuses {
		choices[i] = s
	}
	st.Add(argparse.Arg{Choices: choices, Help: "list the items carrying this status (written = no marker)"}, "--status")
	st.Add(jsonHelp, "--json")
	t := add("text", "print the course text lines an item (or a lab) points at")
	t.Add(argparse.Arg{Metavar: "ID|GROUP", Help: "item ID or group (L3, 3, B, A)"}, "target")
	ck := add("check", "check what each lab leaves in a station (and the built jars): PASS/FAIL/SKIP per check")
	ck.Add(stationArg, "--station")
	ck.Add(installArg, "--install")
	ck.Add(sourceArg, "--source")
	ck.Add(labsrcArg, "--labsrc")
	ck.Add(solutionsArg, "--solutions")
	ck.Add(argparse.Arg{Action: argparse.Append, Metavar: "N", Help: "only this lab (1-11, B; RA1, RA2, EXAM run only when named); repeatable"}, "--lab")
	ck.Add(jsonHelp, "--json")
	vp := add("verify-predictions", "run the checks linked to every *(predicted)* item: CONFIRMED, PARTIAL, CONTRADICTED or UNDECIDED")
	vp.Add(stationArg, "--station")
	vp.Add(installArg, "--install")
	vp.Add(sourceArg, "--source")
	vp.Add(labsrcArg, "--labsrc")
	vp.Add(solutionsArg, "--solutions")
	vp.Add(argparse.Arg{Metavar: "YYYY-MM-DD", Help: "date written in the suggested *(verified live|offline ...)* lines (default: today)"}, "--date")
	vp.Add(jsonHelp, "--json")
	fd := add("find", "items whose step or answer matches a regexp: group, ID, status and the first matching line")
	fd.Add(argparse.Arg{Metavar: "REGEXP", Help: "a Go regexp (RE2), matched line by line"}, "query")
	fd.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "ignore case"}, "-i", "--ignore-case")
	vf := add("verify", "decide every item of a status from its linked checks: CONFIRMED, PARTIAL, CONTRADICTED or UNDECIDED")
	vf.Add(argparse.Arg{Required: true, Choices: choices, Help: "the status of the items to decide (written = no marker)"}, "--status")
	vf.Add(argparse.Arg{Metavar: "GROUP", Help: "one group: a lab number (3 or L3), B, A, RA1, RA2, RAX, RA (all three), EXAM"}, "--lab")
	vf.Add(stationArg, "--station")
	vf.Add(installArg, "--install")
	vf.Add(sourceArg, "--source")
	vf.Add(labsrcArg, "--labsrc")
	vf.Add(solutionsArg, "--solutions")
	vf.Add(argparse.Arg{Metavar: "YYYY-MM-DD", Help: "date written in the suggested *(verified live|offline ...)* lines (default: today)"}, "--date")
	vf.Add(jsonHelp, "--json")
	pg := add("progress", "where you are: verify's verdicts per group over every item, then the steps to redo and the markers to add")
	pg.Add(argparse.Arg{Metavar: "GROUP", Help: "one group: a lab number (3 or L3), B, A, RA1, RA2, RAX, RA (all three), EXAM"}, "--lab")
	pg.Add(stationArg, "--station")
	pg.Add(installArg, "--install")
	pg.Add(sourceArg, "--source")
	pg.Add(labsrcArg, "--labsrc")
	pg.Add(solutionsArg, "--solutions")
	pg.Add(argparse.Arg{Metavar: "YYYY-MM-DD", Help: "date written in the suggested *(verified live|offline ...)* lines (default: today)"}, "--date")
	pg.Add(jsonHelp, "--json")
	qz := add("quiz", "draw why, decision, trap, recall and predict questions from the answer keys; --answers prints the answers after them")
	qz.Add(argparse.Arg{Metavar: "GROUP", Help: "one group: a lab number (3 or L3), B, A, RA1, RA2, RAX, RA (all three), EXAM"}, "--lab")
	qz.Add(argparse.Arg{Choices: []any{quizWhy, quizDecision, quizTrap, quizRecall, quizPredict}, Help: "one kind: why (a lab *Why ...?* question), " +
		"decision (a review-assignment design decision), trap (a review-assignment trap), " +
		"recall (a lab Name = `value` step), predict (a lab *Observe* step)"}, "--kind")
	qz.Add(argparse.Arg{Type: argparse.Int, Default: 5, Metavar: "N", Help: "questions to draw (default: 5; fewer when the selection holds fewer)"}, "-n")
	qz.Add(argparse.Arg{Type: argparse.Int, Metavar: "S", Help: "the draw's seed: the same seed draws the same quiz (default: from the clock, printed)"}, "--seed")
	qz.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "print every answer and its status after the questions"}, "--answers")
	return p, subs
}

// Main runs course with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	p, subs := buildParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	cmd := ns.String("cmd")
	o := stationOpts{station: ns.String("station"), hasStation: ns.Get("station") != nil,
		install: ns.String("install"), hasInstall: ns.Get("install") != nil,
		source: ns.String("source"), hasSource: ns.Get("source") != nil,
		labsrc: ns.String("labsrc"), hasLabsrc: ns.Get("labsrc") != nil,
		solutions: ns.String("solutions"), hasSolutions: ns.Get("solutions") != nil,
		usage: func(message string) int { return subs[cmd].Error(s.Err, message) }}
	if cmd == "check" { // reads a station, not the answer keys
		return cmdCheck(s, o, ns.Strings("lab"), ns.Bool("json"))
	}
	if (cmd == "verify" || cmd == "verify-predictions" || cmd == "progress") && !o.anyStation() {
		return o.missingStation("", nil) // before the root and --date errors
	}
	root, err := findRoot(ns.String("root"), ns.Get("root") != nil)
	if err != nil {
		fmt.Fprintf(s.Err, "course: %v\n", err)
		return cli.ExitCannotSearch
	}
	c, problems := key.Load(root)
	if c == nil {
		for _, pr := range problems {
			fmt.Fprintf(s.Err, "course: %s\n", pr)
		}
		return cli.ExitCannotSearch
	}
	switch cmd {
	case "show":
		return cmdShow(c, s, ns.Strings("ids"), ns.Bool("json"))
	case "verify-predictions":
		return cmdVerify(c, s, o, "predicted", nil, ns.String("date"), ns.Bool("json"))
	case "find":
		return cmdFind(c, s, ns.String("query"), ns.Bool("ignore_case"))
	case "verify", "progress":
		var groups []string
		if ns.Get("lab") != nil {
			if groups = c.GroupKeys(ns.String("lab")); len(groups) == 0 {
				fmt.Fprintf(s.Err, "course: no group %s (groups: %s)\n", ns.String("lab"), c.GroupList())
				return cli.ExitNoMatch
			}
		}
		if cmd == "progress" {
			return cmdProgress(c, s, o, groups, ns.String("date"), ns.Bool("json"))
		}
		return cmdVerify(c, s, o, ns.String("status"), groups, ns.String("date"), ns.Bool("json"))
	case "quiz":
		return cmdQuiz(c, s, root, quizOpts{lab: ns.String("lab"), hasLab: ns.Get("lab") != nil, kind: ns.String("kind"),
			count: ns.Int("n"), seed: ns.Int("seed"), hasSeed: ns.Get("seed") != nil, answers: ns.Bool("answers")})
	case "status":
		return cmdStatus(c, s, ns.Get("lab") != nil, ns.String("lab"), ns.String("status"), ns.Bool("json"))
	default:
		return cmdText(c, s, root, ns.String("target"))
	}
}
