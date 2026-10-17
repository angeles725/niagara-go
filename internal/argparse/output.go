package argparse

import (
	"io"
)

// FormatUsage is parser.format_usage(): "usage: ...\n".
func (p *Parser) FormatUsage() string {
	f := newFormatter(p)
	f.addUsage(p.cfg.Usage, p.actions, p.mutexGroups, "usage: ")
	return f.formatHelp()
}

// FormatHelp is parser.format_help(): usage, description, the argument sections, epilog.
func (p *Parser) FormatHelp() string {
	f := newFormatter(p)
	f.addUsage(p.cfg.Usage, p.actions, p.mutexGroups, "usage: ")
	f.addText(p.cfg.Description)
	for _, g := range p.groups {
		f.startSection(g.title, g.hasTitle)
		f.addText(g.description)
		for _, a := range g.actions {
			f.addArgument(a)
		}
		f.endSection()
	}
	f.addText(p.cfg.Epilog)
	return f.formatHelp()
}

// PrintHelp is parser.print_help(file=w).
func (p *Parser) PrintHelp(w io.Writer) { writeMsg(w, p.FormatHelp()) }

// PrintUsage is parser.print_usage(file=w).
func (p *Parser) PrintUsage(w io.Writer) { writeMsg(w, p.FormatUsage()) }

// Error is parser.error(message) without the exit: it writes the usage and
// "<prog>: error: <message>" to stderr and returns the exit status (2 unless
// Config.ErrorExit says otherwise).
func (p *Parser) Error(stderr io.Writer, message string) int {
	p.PrintUsage(stderr)
	writeMsg(stderr, p.prog+": error: "+message+"\n")
	if p.cfg.ErrorExit != 0 {
		return p.cfg.ErrorExit
	}
	return 2
}

// writeMsg is _print_message: nothing is written for an empty message.
func writeMsg(w io.Writer, msg string) {
	if msg != "" && w != nil {
		_, _ = io.WriteString(w, msg)
	}
}
