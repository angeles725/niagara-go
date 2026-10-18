package navbuild

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The two json.dump layouts the builders use, both with ensure_ascii=False.
var (
	// Indented is json.dump(indent=2, ensure_ascii=False): module-inventory, class-index.
	Indented = pyjson.DumpOpts{Indent: 2, ItemSep: ",", KeySep: ": "}
	// Compact is json.dump(separators=(",", ":"), ensure_ascii=False): inheritance, xref.
	Compact = pyjson.DumpOpts{Indent: -1, ItemSep: ",", KeySep: ":"}
)

// now is time.Now, replaceable by the tests.
var now = time.Now

// Field is one member of an object written in a fixed key order (a _meta
// block or an index entry).
type Field struct {
	Key   string
	Value pyjson.Value
}

// object is the dict literal {fields...} in their order.
func object(fields ...Field) pyjson.Value {
	o := pyjson.NewObject()
	for _, f := range fields {
		o.Set(f.Key, f.Value)
	}
	return o
}

func str(s string) pyjson.Value { return pyjson.String(s) }

func integer(n int) pyjson.Value { return pyjson.IntValue(int64(n)) }

func boolean(b bool) pyjson.Value { return pyjson.Value{Kind: pyjson.Bool, B: b} }

func float(f float64) pyjson.Value { return pyjson.Value{Kind: pyjson.Float, F: f} }

// optStr is a str, or None for "".
func optStr(s string) pyjson.Value {
	if s == "" {
		return pyjson.Value{}
	}
	return str(s)
}

func strList(ss []string) pyjson.Value {
	a := make([]pyjson.Value, len(ss))
	for i, s := range ss {
		a[i] = str(s)
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

// round1 is Python's round(x, 1): correctly rounded, ties to even, which is
// what strconv's fixed-precision formatting does.
func round1(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	return r
}

// stamped appends the two volatile _meta members every builder ends with:
// "build_time_sec": round(elapsed, 1) and "timestamp":
// time.strftime("%Y-%m-%dT%H:%M:%S") (local time).
func stamped(fields []Field, elapsed time.Duration, at time.Time) []Field {
	return append(fields,
		Field{"build_time_sec", float(round1(elapsed.Seconds()))},
		Field{"timestamp", str(at.Format("2006-01-02T15:04:05"))})
}

// jsonWriter streams a document in a json.dump layout, so an index of a few
// hundred thousand entries never exists as one pyjson tree: each member value
// is dumped on its own and re-indented to its nesting level (a dumped value
// holds no raw newline but the layout's own, since strings escape "\n").
type jsonWriter struct {
	b strings.Builder
	o pyjson.DumpOpts
}

// value writes v as json.dump writes it at nesting depth level.
func (w *jsonWriter) value(v pyjson.Value, level int) {
	s := pyjson.Dumps(v, w.o)
	if w.o.Indent > 0 && level > 0 {
		s = strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", w.o.Indent*level))
	}
	w.b.WriteString(s)
}

// object writes an object of n members at depth level: member i has key
// key(i) and its value is written by val(i, level+1).
func (w *jsonWriter) object(level, n int, key func(i int) string, val func(i, level int)) {
	if n == 0 {
		w.b.WriteString("{}")
		return
	}
	nl := ""
	if w.o.Indent >= 0 {
		nl = "\n" + strings.Repeat(" ", w.o.Indent*(level+1))
	}
	w.b.WriteString("{" + nl)
	for i := 0; i < n; i++ {
		if i > 0 {
			w.b.WriteString(w.o.ItemSep + nl)
		}
		w.b.WriteString(pyjson.EncodeString(key(i), w.o.EnsureASCII))
		w.b.WriteString(w.o.KeySep)
		val(i, level+1)
	}
	if w.o.Indent >= 0 {
		w.b.WriteString("\n" + strings.Repeat(" ", w.o.Indent*level))
	}
	w.b.WriteString("}")
}

// fields writes the members fs as an object at depth level.
func (w *jsonWriter) fields(level int, fs []Field) {
	w.object(level, len(fs), func(i int) string { return fs[i].Key },
		func(i, l int) { w.value(fs[i].Value, l) })
}

// IndexPath is <baseDir>/indexes/<name>, where every builder writes.
func IndexPath(baseDir, name string) string {
	return filepath.Join(baseDir, "indexes", name)
}

// writeIndex writes data to <baseDir>/indexes/<name> atomically (build.write_json:
// the previous index stays in place when the write fails). The temp file is
// helpbuild's ".tmp-*" sibling instead of Python's "<name>.tmp".
func writeIndex(baseDir, name, data string) error {
	return helpbuild.WriteText(IndexPath(baseDir, name), data)
}
