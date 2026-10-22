// Package greenmaxtablero is the Go port of niagara-research tools/greenmax-tablero-gen.py: it turns
// an original HARBOR GreenMAX tablero Px into the redesigned one (HOA buttons and calendar popups
// removed, a name selector under every pill) as a Markdown file with the Px in a fenced block.
package greenmaxtablero

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// prog is the installed command name used in usage and error lines (I4-2).
const prog = "greenmax-tablero-gen"

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: prog, Description: "Generate a redesigned GreenMAX tablero Px."})
	p.Add(argparse.Arg{Help: "original tablero .px from the station backup"}, "src")
	p.Add(argparse.Arg{Required: true, Type: gmType, Help: "control folder under Iluminacion: GM and two digits, e.g. GM02 or GM10"}, "--gm")
	p.Add(argparse.Arg{Help: "output .md path (fenced Px). Omit with --stdout."}, "--out")
	p.Add(argparse.Arg{Default: "", Help: "PxView/tablero display name for the doc header"}, "--tab")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "also add the read-only 'Horarios' legend"}, "--legend")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "print the .md to stdout instead of a file"}, "--stdout")
	return p
}

// gmRE is a GreenMAX control-folder name (Config/Iluminacion/GM02 ... GM16).
var gmRE = regexp.MustCompile(`^GM[0-9]{2}$`)

// gmType rejects a --gm that names no control folder (7, GM7, gm08): the ords it would write
// (Iluminacion/7/...) resolve to nothing on the station.
var gmType = &argparse.Type{Name: "gm", Convert: func(v string) (any, error) {
	if !gmRE.MatchString(v) {
		return nil, argparse.NewTypeError("%s is not a control folder: use GM and two digits, e.g. GM02", pycompat.StrRepr(v))
	}
	return v, nil
}}

// fileGMRE finds the panel a tablero Px file is named after: "GreenMAX07 TabAL7.px" or "...GM07...".
var fileGMRE = regexp.MustCompile(`(?i)(?:GreenMAX|GM)([0-9]{2})(?:[^0-9]|$)`)

// warnFileMismatch warns when the source file name names another panel than --gm.
func warnFileMismatch(src, gm string, s cli.Stdio) {
	m := fileGMRE.FindStringSubmatch(src[strings.LastIndexAny(src, `/\`)+1:])
	if m != nil && "GM"+m[1] != gm {
		fmt.Fprintf(s.Err, "%s: warning: --gm %s does not match GM%s in the Px file name %s\n", prog, gm, m[1], src)
	}
}

// Main is the greenmax-tablero-gen command line. Exit 0 ok, 1 a Px the transform refuses, 2 a
// usage error (a --gm that is not GM and two digits), an unreadable source or unwritable output.
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	return run(ns.String("src"), ns.String("gm"), ns.String("out"), ns.String("tab"),
		ns.Bool("legend"), ns.Bool("stdout"), s)
}

func run(src, gm, out, tab string, legend, toStdout bool, s cli.Stdio) int {
	warnFileMismatch(src, gm, s)
	text, err := readSource(src)
	if err != nil {
		// Python ends in a traceback (FileNotFoundError, UnicodeDecodeError).
		fmt.Fprintf(s.Err, "greenmax-tablero-gen: %s\n", err)
		return cli.ExitCannotSearch
	}
	px, n, st, err := transform(text, gm, legend)
	if err != nil {
		fmt.Fprintln(s.Err, err)
		if code, ok := cli.ExitCode(err); ok {
			return code
		}
		return cli.ExitCannotSearch
	}
	if tab == "" {
		tab = src[strings.LastIndex(src, "/")+1:]
	}
	legendNote := ""
	if legend {
		legendNote = "; + leyenda Horarios"
	}
	doc := fmt.Sprintf("# %s — Tablero (`%s`) con selector de nombre\n\n"+
		"> %d circuitos. Original de %s: HOA+popups quitados, pills+LEDs conservados, "+
		"%d selectores NameMux/SelR (ords absolutos Iluminacion/%s/)%s.\n\n```xml\n", gm, tab, n, gm, n, gm, legendNote) +
		strings.TrimRightFunc(px, textfold.IsSpace) + "\n```\n"
	if toStdout || out == "" {
		fmt.Fprint(s.Out, doc)
	} else {
		if err := os.WriteFile(out, []byte(doc), 0o666); err != nil {
			// Python ends in a traceback when --out cannot be opened.
			fmt.Fprintf(s.Err, "greenmax-tablero-gen: %s\n", pycompat.OSErrorText(err))
			return cli.ExitCannotSearch
		}
		fmt.Fprintf(s.Err, "written %s\n", out)
	}
	fmt.Fprintf(s.Err, "%s: %s\n", gm, st)
	return 0
}

// readSource is open(src).read() under UTF-8 mode: strict UTF-8, universal newlines.
func readSource(src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", errors.New(pycompat.OSErrorText(err))
	}
	if !utf8.Valid(data) {
		i := 0
		for i < len(data) {
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size <= 1 {
				break
			}
			i += size
		}
		return "", fmt.Errorf("'utf-8' codec can't decode byte 0x%02x in position %d of %s", data[i], i, src)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n"), nil
}
