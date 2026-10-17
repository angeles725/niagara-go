package bognav

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
)

// kv is one member of a row or JSON object, in output order (a Python dict entry).
type kv struct {
	K string
	V any
}

// row is a Python dict of output fields. Values: nil (None), string, *string (str or None),
// bool, int, row, []row, []any, []string.
type row []kv

func (r row) get(k string) any {
	for _, m := range r {
		if m.K == k {
			return m.V
		}
	}
	return nil
}

// s is f"{r[k]}".
func (r row) s(k string) string { return pyStr(r.get(k)) }

func (r *row) set(k string, v any) {
	for i, m := range *r {
		if m.K == k {
			(*r)[i].V = v
			return
		}
	}
	*r = append(*r, kv{k, v})
}

// env is one command run: the parsed arguments, the output streams and the searched bog.
type env struct {
	ns       *argparse.Namespace
	out, err io.Writer
	bog      *bog.Bog
	scope    string // honest scope of the bog ("" in selftest, like args without _scope)
}

func (e *env) flag(k string) bool  { return e.ns.Bool(k) }
func (e *env) str(k string) string { return e.ns.String(k) }
func (e *env) isText() bool        { return !(e.flag("json") || e.flag("csv")) }

func (e *env) println(a ...string) { io.WriteString(e.out, strings.Join(a, "")+"\n") }

// render is output._render: --json (a list of objects), --csv (header + rows) or one line per
// row, with --limit applied and an honest miss line. It returns the number of rows before --limit.
func (e *env) render(rows []row, line func(row) string, empty string, cols []string) int {
	total := len(rows)
	if !e.isText() && len(rows) == 0 && e.scope != "" {
		fmt.Fprintf(e.err, "%s -- %s\n", empty, e.scope)
	}
	rows, note := e.cap(rows)
	switch {
	case e.flag("json"):
		items := make([]any, len(rows))
		for i, r := range rows {
			items[i] = r
		}
		e.println(dumps(items, 2))
	case e.flag("csv"):
		if len(rows) > 0 {
			keys := cols
			if keys == nil {
				for _, m := range rows[0] {
					keys = append(keys, m.K)
				}
			}
			csvRow(e.out, keys)
			for _, r := range rows {
				vals := make([]string, len(keys))
				for i, k := range keys {
					vals[i] = csvStr(r.get(k))
				}
				csvRow(e.out, vals)
			}
		}
	case len(rows) == 0:
		if e.scope != "" {
			e.println(empty, " -- ", e.scope)
		} else {
			e.println(empty)
		}
	default:
		for _, r := range rows {
			e.println(line(r))
		}
		if note != "" {
			e.println(note)
		}
	}
	return total
}

// cap is cliflags.cap: the rows --limit keeps and the cut-off note, which goes to stdout in text
// mode (returned) and to stderr for --json/--csv.
func (e *env) cap(rows []row) ([]row, string) {
	limit := e.ns.Int("limit")
	if limit <= 0 || len(rows) <= limit {
		return rows, ""
	}
	note := fmt.Sprintf("... %d more rows (raise --limit)", len(rows)-limit)
	if !e.isText() {
		fmt.Fprintln(e.err, note)
		note = ""
	}
	return rows[:limit], note
}

// csvRow is csv.writer(sys.stdout).writerow (excel dialect: QUOTE_MINIMAL, "\r\n").
func csvRow(w io.Writer, fields []string) {
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if strings.ContainsAny(f, ",\"\r\n") || (len(fields) == 1 && f == "") {
			b.WriteString(`"` + strings.ReplaceAll(f, `"`, `""`) + `"`)
		} else {
			b.WriteString(f)
		}
	}
	b.WriteString("\r\n")
	io.WriteString(w, b.String())
}

// csvStr is how csv.writer prints a value: None is empty, anything else str().
func csvStr(v any) string {
	if v == nil {
		return ""
	}
	if p, ok := v.(*string); ok && p == nil {
		return ""
	}
	return pyStr(v)
}

// pyStr is Python str() of a row value.
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case *string:
		if x == nil {
			return "None"
		}
		return *x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	}
	return fmt.Sprint(v)
}

// ljust and rjust pad to n code points, like format specs ":<n" / ":n" on str and int.
func ljust(s string, n int) string {
	if k := utf8.RuneCountInString(s); k < n {
		return s + strings.Repeat(" ", n-k)
	}
	return s
}

func rjust(s string, n int) string {
	if k := utf8.RuneCountInString(s); k < n {
		return strings.Repeat(" ", n-k) + s
	}
	return s
}

// head is s[:n] in code points.
func head(s string, n int) string {
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

// pyRepr is Python repr() of a str.
func pyRepr(s string) string {
	q := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == rune(q) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < ' ' || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r >= 0x80 && !isPrintable(r):
			switch {
			case r <= 0xff:
				fmt.Fprintf(&b, `\x%02x`, r)
			case r <= 0xffff:
				fmt.Fprintf(&b, `\u%04x`, r)
			default:
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func isPrintable(r rune) bool { return unicode.IsPrint(r) }
