package modulefind

import (
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/pycsv"
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
			keys, header := make([]string, len(rows[0])), make([]any, len(rows[0]))
			for i, f := range rows[0] {
				keys[i], header[i] = f.key, f.key
			}
			w := pycsv.NewWriter(o.out)
			_ = w.WriteRow(header)
			for _, r := range rows {
				vals := make([]any, len(keys))
				for i, k := range keys {
					vals[i] = csvValue(r.get(k))
				}
				_ = w.WriteRow(vals)
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

// csvValue is a row value as csv.writer receives it: a *float64 is the float, or None
// when nil.
func csvValue(v any) any {
	if x, ok := v.(*float64); ok {
		if x == nil {
			return nil
		}
		return *x
	}
	return v
}

// jsonDumps is json.dumps(v, indent=2) (ensure_ascii) of rows (ordered dicts), lists,
// strings, ints, bools, *float64 and None.
func jsonDumps(v any) string {
	return pyjson.DumpsAny(v, pyjson.Indent2)
}

// Keys and Values make a row a pyjson.Dict.
func (r row) Keys() []string {
	keys := make([]string, len(r))
	for i, f := range r {
		keys[i] = f.key
	}
	return keys
}

// Values is the row's values in key order.
func (r row) Values() []any {
	vals := make([]any, len(r))
	for i, f := range r {
		vals[i] = f.val
	}
	return vals
}
