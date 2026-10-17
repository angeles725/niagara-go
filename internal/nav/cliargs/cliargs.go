// Package cliargs holds argument types shared by several module-nav commands
// (module_nav_lib/cliargs.py), so a rule such as "a count must be at least 1" is
// written once.
package cliargs

import (
	"math/big"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// PositiveInt is positive_int: a count that must be at least 1, else a usage error
// (`rank -n`, `ask -n`, `selftest --sample`). Like argparse's int, a value beyond
// int stays a *big.Int.
var PositiveInt = &argparse.Type{Name: "positive_int", Convert: func(text string) (any, error) {
	n, err := pycompat.ParseBigInt(text)
	if err != nil || n.Sign() < 1 {
		return nil, argparse.NewTypeError("must be a positive integer, got %s", pycompat.StrRepr(text))
	}
	if n.IsInt64() && int64(int(n.Int64())) == n.Int64() {
		return int(n.Int64()), nil
	}
	return new(big.Int).Set(n), nil
}}
