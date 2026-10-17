package argparse

import (
	"os"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// A port of argparse.HelpFormatter (colour disabled): items are added in order and
// rendered lazily, so the help column depends on every argument added before rendering.

type section struct {
	parent     *section
	heading    string
	hasHeading bool
	items      []func() string
}

type formatter struct {
	kind            Formatter
	prog            string
	indentIncrement int
	maxHelpPosition int
	width           int
	currentIndent   int
	actionMaxLength int
	root, current   *section
}

// terminalColumns is shutil.get_terminal_size().columns: $COLUMNS when it is a positive
// integer, else the width of the terminal on stdout, else 80.
func terminalColumns() int {
	cols := 0
	if v, ok := os.LookupEnv("COLUMNS"); ok {
		if n, err := pycompat.ParseInt(v); err == nil {
			cols = int(n)
		}
	}
	if cols <= 0 {
		cols = stdoutColumns()
	}
	if cols <= 0 {
		cols = 80
	}
	return cols
}

func newFormatter(p *Parser) *formatter { return newFormatterFor(p.cfg.Formatter, p.prog) }

func newFormatterFor(kind Formatter, prog string) *formatter {
	width := terminalColumns() - 2
	f := &formatter{kind: kind, prog: prog, indentIncrement: 2, width: width,
		maxHelpPosition: min(24, max(width-20, 4))}
	f.root = &section{}
	f.current = f.root
	return f
}

func (f *formatter) indent() { f.currentIndent += f.indentIncrement }
func (f *formatter) dedent() { f.currentIndent -= f.indentIncrement }

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// joinParts is _join_parts: the non-empty, non-SUPPRESS parts concatenated.
func joinParts(parts ...string) string {
	var b strings.Builder
	for _, s := range parts {
		if s != "" && s != Suppress {
			b.WriteString(s)
		}
	}
	return b.String()
}

func (s *section) format(f *formatter) string {
	if s.parent != nil {
		f.indent()
	}
	parts := make([]string, len(s.items))
	for i, item := range s.items {
		parts[i] = item()
	}
	help := joinParts(parts...)
	if s.parent != nil {
		f.dedent()
	}
	if help == "" {
		return ""
	}
	heading := ""
	if s.hasHeading && s.heading != Suppress {
		heading = spaces(f.currentIndent) + s.heading + ":\n"
	}
	return joinParts("\n", heading, help, "\n")
}

func (f *formatter) addItem(item func() string) {
	f.current.items = append(f.current.items, item)
}

func (f *formatter) startSection(heading string, hasHeading bool) {
	f.indent()
	s := &section{parent: f.current, heading: heading, hasHeading: hasHeading}
	f.addItem(func() string { return s.format(f) })
	f.current = s
}

func (f *formatter) endSection() {
	f.current = f.current.parent
	f.dedent()
}

func (f *formatter) addText(text string) {
	if text != "" && text != Suppress {
		f.addItem(func() string { return f.formatText(text) })
	}
}

func (f *formatter) addUsage(usage string, actions []*Action, groups []*MutexGroup, prefix string) {
	if usage != Suppress {
		f.addItem(func() string { return f.formatUsage(usage, actions, groups, prefix) })
	}
}

// subactions runs fn over the command help lines of a subparsers action, one level deeper.
func (f *formatter) subactions(a *Action, fn func(*Action)) {
	if a.sub == nil {
		return
	}
	f.indent()
	for _, s := range a.sub.choices {
		fn(s)
	}
	f.dedent()
}

func (f *formatter) addArgument(a *Action) {
	if a.Help == Suppress {
		return
	}
	longest := runeLen(f.invocation(a)) + f.currentIndent
	f.subactions(a, func(s *Action) {
		longest = max(longest, runeLen(f.invocation(s))+f.currentIndent)
	})
	f.actionMaxLength = max(f.actionMaxLength, longest)
	f.addItem(func() string { return f.formatAction(a) })
}

var longBreak = regexp.MustCompile(`\n\n\n+`)

func (f *formatter) formatHelp() string {
	help := f.root.format(f)
	if help != "" {
		help = longBreak.ReplaceAllString(help, "\n\n")
		help = strings.Trim(help, "\n") + "\n"
	}
	return help
}

func (f *formatter) formatText(text string) string {
	if strings.Contains(text, "%(prog)") {
		expanded, err := pycompat.Percent(text, map[string]any{"prog": f.prog})
		if err != nil {
			panic("argparse: badly formed text: " + err.Error())
		}
		text = expanded
	}
	width := max(f.width-f.currentIndent, 11)
	return f.fillText(text, width, spaces(f.currentIndent)) + "\n\n"
}

func (f *formatter) fillText(text string, width int, indent string) string {
	if f.kind == RawDescriptionHelpFormatter || f.kind == RawTextHelpFormatter {
		var b strings.Builder
		for _, line := range pycompat.SplitLines(text, true) {
			b.WriteString(indent + line)
		}
		return b.String()
	}
	return fillText(normalizeSpace(text), width, indent)
}

func (f *formatter) splitLines(text string, width int) []string {
	if f.kind == RawTextHelpFormatter {
		return pycompat.SplitLines(text, false)
	}
	return wrapText(normalizeSpace(text), width, "")
}

func (f *formatter) formatAction(a *Action) string {
	helpPosition := min(f.actionMaxLength+2, f.maxHelpPosition)
	helpWidth := max(f.width-helpPosition, 11)
	actionWidth := helpPosition - f.currentIndent - 2
	header := f.invocation(a)
	indentFirst := 0
	switch {
	case a.Help == "":
		header = spaces(f.currentIndent) + header + "\n"
	case runeLen(header) <= actionWidth:
		header = spaces(f.currentIndent) + padRight(header, actionWidth) + "  "
	default:
		header = spaces(f.currentIndent) + header + "\n"
		indentFirst = helpPosition
	}
	parts := []string{header}
	if a.Help != "" && pycompat.Strip(a.Help) != "" {
		text, err := f.expandHelp(a)
		if err != nil {
			panic("argparse: badly formed help string: " + err.Error())
		}
		if text != "" {
			lines := f.splitLines(text, helpWidth)
			for i, line := range lines {
				pad := helpPosition
				if i == 0 {
					pad = indentFirst
				}
				parts = append(parts, spaces(pad)+line+"\n")
			}
		}
	} else if !strings.HasSuffix(header, "\n") {
		parts = append(parts, "\n")
	}
	f.subactions(a, func(s *Action) { parts = append(parts, f.formatAction(s)) })
	return joinParts(parts...)
}

// invocation is _format_action_invocation: "-s, --long ARGS" or the positional metavar.
func (f *formatter) invocation(a *Action) string {
	if !a.isOptional() {
		return strings.Join(f.metavar(a, a.Dest, 1), " ")
	}
	opts := strings.Join(a.OptionStrings, ", ")
	if a.nargsZero {
		return opts
	}
	return opts + " " + f.formatArgs(a, strings.ToUpper(a.Dest))
}

// metavar is _metavar_formatter(action, default)(n).
func (f *formatter) metavar(a *Action, def string, n int) []string {
	if a.metavarTuple != nil {
		return a.metavarTuple
	}
	m := def
	switch {
	case a.Metavar != "":
		m = a.Metavar
	case a.sub != nil || a.choices != nil:
		m = "{" + joinStr(a.choiceValues(), ",") + "}"
	}
	out := make([]string, n)
	for i := range out {
		out[i] = m
	}
	return out
}

// formatArgs is _format_args.
func (f *formatter) formatArgs(a *Action, def string) string {
	switch {
	case a.nargsZero:
		return ""
	case a.nargs == 0:
		return f.metavar(a, def, 1)[0]
	case a.nargs == Optional:
		return "[" + f.metavar(a, def, 1)[0] + "]"
	case a.nargs == ZeroOrMore:
		m := f.metavar(a, def, 1)
		if len(m) == 2 {
			return "[" + m[0] + " [" + m[1] + " ...]]"
		}
		return "[" + m[0] + " ...]"
	case a.nargs == OneOrMore:
		m := f.metavar(a, def, 2)
		return m[0] + " [" + m[1] + " ...]"
	case a.nargs == nargsPar:
		return f.metavar(a, def, 1)[0] + " ..."
	}
	return strings.Join(f.metavar(a, def, a.nargs), " ")
}

// helpString is _get_help_string (ArgumentDefaultsHelpFormatter appends the default).
func (f *formatter) helpString(a *Action) string {
	help := a.Help
	if f.kind == ArgumentDefaultsHelpFormatter && !strings.Contains(help, "%(default)") &&
		!isSuppress(a.Default) && !a.Required &&
		(a.isOptional() || a.nargs == Optional || a.nargs == ZeroOrMore) {
		help += " (default: %(default)s)"
	}
	return help
}

// expandHelp is _expand_help: %-format the help with the action's attributes.
func (f *formatter) expandHelp(a *Action) (string, error) {
	help := f.helpString(a)
	if !strings.Contains(help, "%") {
		return help, nil
	}
	var nargs any
	switch {
	case a.nargsZero:
		nargs = 0
	case a.nargs == 0:
		nargs = nil
	case a.nargs == Optional:
		nargs = "?"
	case a.nargs == ZeroOrMore:
		nargs = "*"
	case a.nargs == OneOrMore:
		nargs = "+"
	case a.nargs == nargsPar:
		nargs = "A..."
	default:
		nargs = a.nargs
	}
	var typ, choices, metavar any
	if a.typ != nil {
		typ = a.typ.Name
	}
	if c := a.choiceValues(); c != nil || a.sub != nil {
		choices = joinStr(c, ", ")
	}
	if a.metavarTuple != nil {
		tuple := make(pycompat.Tuple, len(a.metavarTuple))
		for i, m := range a.metavarTuple {
			tuple[i] = m
		}
		metavar = tuple
	} else if a.Metavar != "" {
		metavar = a.Metavar
	}
	params := map[string]any{"option_strings": append([]string{}, a.OptionStrings...),
		"dest": a.Dest, "nargs": nargs, "const": a.constant, "default": a.Default, "type": typ,
		"choices": choices, "required": a.Required, "help": a.Help, "metavar": metavar,
		"deprecated": false, "prog": f.prog}
	if a.kind == Version {
		params["version"] = a.version
	}
	for k, v := range params {
		if isSuppress(v) {
			delete(params, k)
		}
	}
	return pycompat.Percent(help, params)
}
