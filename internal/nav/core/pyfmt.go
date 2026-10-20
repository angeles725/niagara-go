package core

import (
	"strconv"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Python formatting the commands share. Widths and cuts count code points,
// like str.format and slicing.

// PyStr is str(v) of a decoded JSON value (pycompat.Str: repr for anything but a string).
func PyStr(v any) string { return pycompat.Str(v) }

// Commas is "{:,}".format(n) (pycompat.Commas).
func Commas(n int64) string { return pycompat.Commas(n) }

// Fixed is "{:.Nf}".format(f) (round half to even on exact ties, like Python).
func Fixed(f float64, prec int) string { return strconv.FormatFloat(f, 'f', prec, 64) }

// LJust is "{:<w}"/"{:w}" of a string: padded on the right to w code points.
func LJust(s string, w int) string { return pycompat.LJust(s, w, ' ') }

// RJust is "{:>w}": padded on the left to w code points.
func RJust(s string, w int) string { return pycompat.RJust(s, w, ' ') }

// Cut is s[:n] in code points.
func Cut(s string, n int) string { return pycompat.Cut(s, n) }
