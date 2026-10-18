package niagaraannot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/slotomatic"
)

// FlagMask evaluates a flags expression: Flags.X (also javax.baja.sys.Flags.X) and int literals
// joined with | or +, in parentheses or not; "" is 0. Any other term or operator is an error.
// The names, bits and letters are slotomatic.Flags, the one table of javax.baja.sys.Flags; its
// DecodeFlags is not used because it ignores a term it cannot read, where an annotation check
// must refuse to decide.
func FlagMask(expr string) (uint32, error) {
	toks, err := lex(expr)
	if err != nil {
		return 0, err
	}
	if len(toks) == 0 {
		return 0, nil
	}
	e := flagEval{toks: toks, expr: expr}
	m, err := e.or()
	if err == nil && e.i < len(toks) {
		err = e.fail()
	}
	return m, err
}

// FlagLetters is the Slot Sheet text of a flags expression: Flags.encodeToString of its mask
// ("Flags.SUMMARY | Flags.READONLY" → "rs", "" → "").
func FlagLetters(expr string) (string, error) {
	m, err := FlagMask(expr)
	if err != nil {
		return "", err
	}
	return EncodeFlags(m), nil
}

// EncodeFlags is Flags.encodeToString: the letter of each set flag, in the order of
// slotomatic.Flags (the FLAGS array encodeToString walks).
func EncodeFlags(m uint32) string {
	var b strings.Builder
	for _, f := range slotomatic.Flags {
		if m&uint32(f.Bit) != 0 {
			b.WriteString(f.Letter)
		}
	}
	return b.String()
}

type flagEval struct {
	toks []token
	i    int
	expr string
}

func (e *flagEval) fail() error {
	return fmt.Errorf("cannot evaluate flags %q", e.expr)
}

func (e *flagEval) punct(text string) bool {
	if e.i < len(e.toks) && e.toks[e.i].kind == tPunct && e.toks[e.i].text == text {
		e.i++
		return true
	}
	return false
}

// or is sum ('|' sum)*.
func (e *flagEval) or() (uint32, error) {
	m, err := e.sum()
	for err == nil && e.punct("|") {
		var n uint32
		n, err = e.sum()
		m |= n
	}
	return m, err
}

// sum is term ('+' term)*: Java int addition, which binds tighter than | (BRelation writes
// Flags.HIDDEN + Flags.READONLY).
func (e *flagEval) sum() (uint32, error) {
	m, err := e.term()
	for err == nil && e.punct("+") {
		var n uint32
		n, err = e.term()
		m += n
	}
	return m, err
}

// term is (expr), an int literal, or a qualified Flags.NAME.
func (e *flagEval) term() (uint32, error) {
	if e.punct("(") {
		m, err := e.or()
		if err != nil || !e.punct(")") {
			return 0, e.fail()
		}
		return m, nil
	}
	if e.i >= len(e.toks) {
		return 0, e.fail()
	}
	tk := e.toks[e.i]
	if tk.kind == tNumber {
		e.i++
		n, err := strconv.ParseUint(strings.TrimRight(tk.text, "lL"), 0, 32)
		if err != nil {
			return 0, e.fail()
		}
		return uint32(n), nil
	}
	var path []string
	for e.i < len(e.toks) && e.toks[e.i].kind == tIdent {
		path = append(path, e.toks[e.i].text)
		e.i++
		if !e.punct(".") {
			break
		}
	}
	q := strings.Join(path, ".")
	if len(path) < 2 || (q != "Flags."+path[len(path)-1] && q != "javax.baja.sys.Flags."+path[len(path)-1]) {
		return 0, e.fail()
	}
	for _, f := range slotomatic.Flags {
		if f.Name == path[len(path)-1] {
			return uint32(f.Bit), nil
		}
	}
	return 0, fmt.Errorf("unknown flag %s in %q", q, e.expr)
}
