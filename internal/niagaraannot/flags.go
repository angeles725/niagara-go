package niagaraannot

import (
	"fmt"
	"strconv"
	"strings"
)

// Flag is one javax.baja.sys.Flags constant: its name, mask and Slot Sheet letter.
type Flag struct {
	Name   string
	Mask   uint32
	Letter byte
}

// Flags is the FLAGS table of javax.baja.sys.Flags in its own order, the order
// encodeToString writes the letters in (docSource baja/javax/baja/sys/Flags.java:183-228,
// encodeToString :638-644). NON_CRITICAL comes before LINK_TARGET there although its bit is
// higher.
var Flags = []Flag{
	{"READONLY", 0x00000001, 'r'},
	{"TRANSIENT", 0x00000002, 't'},
	{"HIDDEN", 0x00000004, 'h'},
	{"SUMMARY", 0x00000008, 's'},
	{"ASYNC", 0x00000010, 'a'},
	{"NO_RUN", 0x00000020, 'n'},
	{"DEFAULT_ON_CLONE", 0x00000040, 'd'},
	{"CONFIRM_REQUIRED", 0x00000080, 'c'},
	{"OPERATOR", 0x00000100, 'o'},
	{"EXECUTE_ON_CHANGE", 0x00000200, 'x'},
	{"FAN_IN", 0x00000400, 'f'},
	{"NO_AUDIT", 0x00000800, 'A'},
	{"COMPOSITE", 0x00001000, 'p'},
	{"REMOVE_ON_CLONE", 0x00002000, 'R'},
	{"METADATA", 0x00004000, 'm'},
	{"NON_CRITICAL", 0x00010000, 'N'},
	{"LINK_TARGET", 0x00008000, 'L'},
	{"USER_DEFINED_1", 0x10000000, '1'},
	{"USER_DEFINED_2", 0x20000000, '2'},
	{"USER_DEFINED_3", 0x40000000, '3'},
	{"USER_DEFINED_4", 0x80000000, '4'},
}

// FlagMask evaluates a flags expression: Flags.X (also javax.baja.sys.Flags.X) and int literals
// joined with | or +, in parentheses or not; "" is 0. Any other term or operator is an error.
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

// EncodeFlags is Flags.encodeToString: the letter of each set flag, in table order.
func EncodeFlags(m uint32) string {
	var b strings.Builder
	for _, f := range Flags {
		if m&f.Mask != 0 {
			b.WriteByte(f.Letter)
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
	for _, f := range Flags {
		if f.Name == path[len(path)-1] {
			return f.Mask, nil
		}
	}
	return 0, fmt.Errorf("unknown flag %s in %q", q, e.expr)
}
