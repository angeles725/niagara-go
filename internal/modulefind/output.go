package modulefind

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// opts holds the parsed command line a command reads (module-find's argparse namespace).
type opts struct {
	json, csv bool
	limit     int    // 0: not given
	scope     string // args._scope: the honest scope line, "" in the selftest
	out, err  io.Writer
}

// field is one key of a row (a Python dict literal keeps its key order).
type field struct {
	key string
	val any // string, int, bool, *float64 (nil = None)
}

type row []field

func (r row) get(key string) any {
	for _, f := range r {
		if f.key == key {
			return f.val
		}
	}
	return ""
}

func (r row) str(key string) string { s, _ := r.get(key).(string); return s }

// render is module-find._render: --json (list of dicts), --csv (header + rows), else one
// line per row; --limit caps the rows with a note. It returns the rows before the cap.
func render(rows []row, o *opts, line func(row) string, empty string) int {
	total, text := len(rows), !o.json && !o.csv
	if len(rows) == 0 && !text && o.scope != "" {
		fmt.Fprintf(o.err, "%s -- %s\n", empty, o.scope)
	}
	rows, note := capRows(rows, o, text)
	switch {
	case o.json:
		items := make([]any, len(rows))
		for i, r := range rows {
			items[i] = r
		}
		fmt.Fprintln(o.out, jsonDumps(items))
	case o.csv:
		if len(rows) > 0 {
			keys := make([]string, len(rows[0]))
			for i, f := range rows[0] {
				keys[i] = f.key
			}
			writeCSV(o.out, keys)
			for _, r := range rows {
				vals := make([]string, len(keys))
				for i, k := range keys {
					vals[i] = csvStr(r.get(k))
				}
				writeCSV(o.out, vals)
			}
		}
	case len(rows) == 0:
		miss(o, empty)
	default:
		for _, r := range rows {
			fmt.Fprintln(o.out, line(r))
		}
		if note != "" {
			fmt.Fprintln(o.out, note)
		}
	}
	return total
}

// miss is module-find._miss: the honest zero names what was searched.
func miss(o *opts, text string) {
	if o.scope != "" {
		fmt.Fprintf(o.out, "%s -- %s\n", text, o.scope)
		return
	}
	fmt.Fprintln(o.out, text)
}

// capRows is cliflags.cap: the note goes to stdout in text mode, to stderr otherwise.
// A limit of 0 or less caps nothing (the command line only passes 0, "not given").
func capRows(rows []row, o *opts, text bool) ([]row, string) {
	if o.limit <= 0 || len(rows) <= o.limit {
		return rows, ""
	}
	note := fmt.Sprintf("... %d more rows (raise --limit)", len(rows)-o.limit)
	if !text {
		fmt.Fprintln(o.err, note)
		note = ""
	}
	return rows[:o.limit], note
}

// csvStr is how csv.writer writes a value: str(), None as "".
func csvStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case *float64:
		if x == nil {
			return ""
		}
		return pycompat.FloatRepr(*x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return fmt.Sprint(x)
	}
	return v.(string)
}

// writeCSV is csv.writer(sys.stdout).writerow (excel dialect): fields quoted only when
// they hold a comma, a quote, CR or LF (quotes doubled), a lone empty field quoted, CRLF.
func writeCSV(w io.Writer, fields []string) {
	out := make([]string, len(fields))
	for i, f := range fields {
		if strings.ContainsAny(f, ",\"\r\n") || (f == "" && len(fields) == 1) {
			f = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		}
		out[i] = f
	}
	fmt.Fprint(w, strings.Join(out, ",")+"\r\n")
}

// jsonDumps is json.dumps(v, indent=2) (ensure_ascii); jsonValue lists the types.
func jsonDumps(v any) string {
	return pyjson.Dumps(jsonValue(v), pyjson.DumpOpts{EnsureASCII: true, Indent: 2, ItemSep: ",", KeySep: ": "})
}

// jsonValue is v as a pyjson value: rows (ordered dicts), lists, strings, ints, bools,
// *float64 and None. A string passes through []rune first: pyjson reads WTF-8, and this
// keeps an undecodable byte one U+FFFD.
func jsonValue(v any) pyjson.Value {
	switch x := v.(type) {
	case nil:
		return pyjson.Value{Kind: pyjson.Null}
	case string:
		return pyjson.String(string([]rune(x)))
	case bool:
		return pyjson.Value{Kind: pyjson.Bool, B: x}
	case int:
		return pyjson.IntValue(int64(x))
	case *float64:
		if x == nil {
			return pyjson.Value{Kind: pyjson.Null}
		}
		return pyjson.Value{Kind: pyjson.Float, F: *x}
	case []string:
		items := make([]pyjson.Value, len(x))
		for i, it := range x {
			items[i] = jsonValue(it)
		}
		return pyjson.Value{Kind: pyjson.Array, A: items}
	case []any:
		items := make([]pyjson.Value, len(x))
		for i, it := range x {
			items[i] = jsonValue(it)
		}
		return pyjson.Value{Kind: pyjson.Array, A: items}
	case row:
		o := pyjson.NewObject()
		for _, f := range x {
			o.Set(string([]rune(f.key)), jsonValue(f.val))
		}
		return o
	}
	panic(fmt.Sprintf("jsonValue: unsupported %T", v))
}
