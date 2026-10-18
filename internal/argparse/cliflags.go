package argparse

import (
	"math"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The niagara-research cliflags helpers the tools share: positive_int and output_parents.

// PositiveInt is cliflags.positive_int: a whole number >= 1, parsed like Python int().
// Python's int has no bound, so a huge value saturates to math.MaxInt (a huge --limit caps
// nothing) and a huge negative one is still named in full in the error.
var PositiveInt = &Type{Name: "positive_int", Convert: func(s string) (any, error) {
	n, err := pycompat.ParseBigInt(s)
	if err != nil {
		return nil, NewTypeError("invalid int value: %s", pycompat.StrRepr(s))
	}
	if n.Sign() < 1 {
		return nil, NewTypeError("must be at least 1, not %s", n)
	}
	if !n.IsInt64() || n.Int64() > math.MaxInt {
		return math.MaxInt, nil
	}
	return int(n.Int64()), nil
}}

// OutputParent is one of cliflags.output_parents: --json, --csv and --limit/-n. The main
// parser's copy defaults to False/None; a subcommand's copy (suppress) defaults to SUPPRESS so
// it never resets what the main parser already parsed.
func OutputParent(suppress bool) *Parser {
	var flag, limit any = false, nil
	if suppress {
		flag, limit = Suppress, Suppress
	}
	p := New(Config{NoHelp: true})
	p.Add(Arg{Action: StoreTrue, Default: flag, Help: "machine JSON output"}, "--json")
	p.Add(Arg{Action: StoreTrue, Default: flag, Help: "CSV output (header + rows)"}, "--csv")
	p.Add(Arg{Type: PositiveInt, Default: limit, Metavar: "N",
		Help: "print at most N rows (default: all); the cut is reported"}, "--limit", "-n")
	return p
}
