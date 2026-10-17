package argparse

import (
	"fmt"
	"io"
)

// Suppress is argparse.SUPPRESS: as a Help it hides the argument, as a Dest or Default it
// keeps the argument out of the namespace until it is given.
const Suppress = "==SUPPRESS=="

// Formatter selects the argparse help formatter class.
type Formatter int

// The supported formatter classes.
const (
	HelpFormatter                 Formatter = iota // argparse.HelpFormatter
	RawDescriptionHelpFormatter                    // keeps description and epilog verbatim
	RawTextHelpFormatter                           // also keeps argument help verbatim
	ArgumentDefaultsHelpFormatter                  // appends " (default: %(default)s)"
)

// Config holds the ArgumentParser keyword arguments. An empty string means Python None.
type Config struct {
	Prog        string // default: basename of os.Args[0]
	Usage       string // %(prog)s is expanded; empty means generated
	Description string
	Epilog      string
	Formatter   Formatter
	Parents     []*Parser // add_help=False parsers whose arguments are copied
	NoHelp      bool      // add_help=False
	NoAbbrev    bool      // allow_abbrev=False
	NoSuggest   bool      // suggest_on_error=False
	// ErrorExit is the exit status of a usage error (0 means 2), for tools whose
	// ArgumentParser subclass overrides error() with another status.
	ErrorExit int
}

// ActionKind is the add_argument action= value.
type ActionKind int

// The supported actions.
const (
	Store       ActionKind = iota // "store" (the default)
	StoreConst                    // "store_const"
	StoreTrue                     // "store_true"
	StoreFalse                    // "store_false"
	Append                        // "append"
	AppendConst                   // "append_const"
	Count                         // "count"
	Extend                        // "extend"
	Help                          // "help"
	Version                       // "version"
	Custom                        // an argparse.Action subclass: Arg.Call runs it
	parsers                       // the add_subparsers action
)

// Nargs values besides a positive count N (Arg.Nargs zero is Python's nargs=None).
const (
	Optional   = -1 // "?"
	ZeroOrMore = -2 // "*"
	OneOrMore  = -3 // "+"
	NoArgs     = -4 // nargs=0, for Custom actions that take no value
	nargsPar   = -5 // argparse.PARSER, "A..."
)

// Arg holds the add_argument keyword arguments. Zero values mean "not given".
type Arg struct {
	Action       ActionKind
	Nargs        int
	Const        any
	Default      any // nil: not given (Python None); Suppress: argparse.SUPPRESS
	Type         *Type
	Choices      []any
	Required     bool
	Help         string
	Metavar      string
	MetavarTuple []string // a tuple metavar (nargs N, "+" or "*")
	Dest         string
	Version      string            // for Version
	Call         func(*Call) error // for Custom
}

// Type is an add_argument type= converter. Name is its __name__ in error messages.
type Type struct {
	Name    string
	Convert func(string) (any, error)
}

// The builtin type= converters.
var (
	// Int is type=int: Python int(text), base 10.
	Int = &Type{Name: "int", Convert: func(s string) (any, error) { return ParseInt(s) }}
	// Float is type=float: Python float(text).
	Float = &Type{Name: "float", Convert: func(s string) (any, error) { return ParseFloat(s) }}
	// String is type=str: the text unchanged.
	String = &Type{Name: "str", Convert: func(s string) (any, error) { return s, nil }}
)

// TypeError is argparse.ArgumentTypeError: its message replaces "invalid <type> value".
type TypeError struct{ Msg string }

func (e *TypeError) Error() string { return e.Msg }

// NewTypeError returns an ArgumentTypeError with the given message.
func NewTypeError(format string, a ...any) error {
	return &TypeError{Msg: fmt.Sprintf(format, a...)}
}

// Call is what a Custom action receives (argparse.Action.__call__ arguments).
type Call struct {
	Parser       *Parser
	Namespace    *Namespace
	Values       any
	OptionString string
	Out, Err     io.Writer
}

// ExitError, returned by a Custom action, stops parsing like parser.exit(Code, Message).
type ExitError struct {
	Code    int
	Message string // written to stderr when not empty
}

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }

// SubparsersConfig holds the add_subparsers keyword arguments.
type SubparsersConfig struct {
	Title       string // Title or Description: list the commands in their own group
	Description string
	Dest        string // empty: argparse.SUPPRESS
	Required    bool
	Help        string
	Metavar     string
	Prog        string // empty: derived from the parser's usage
}

// SubparserConfig holds the add_parser keyword arguments.
type SubparserConfig struct {
	Config
	Help    string // the one-line help in the parent's command list
	Listed  bool   // list the command even when Help is empty (Python help="")
	Aliases []string
}
