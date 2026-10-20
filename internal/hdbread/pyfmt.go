package hdbread

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pycsv"
)

func formatG(v float64, digits int) string { return strconv.FormatFloat(v, 'g', digits, 64) }

// newCSVWriter is csv.writer(sys.stdout, lineterminator="\n"), the writer tools/hdbread.py used.
func newCSVWriter(w io.Writer) *pycsv.Writer {
	cw := pycsv.NewWriter(w)
	cw.LineTerminator = "\n"
	return cw
}

// stringRow is fields as the []any row pycsv writes.
func stringRow(fields ...string) []any {
	row := make([]any, len(fields))
	for i, f := range fields {
		row[i] = f
	}
	return row
}

func (v value) csv() string {
	switch v.kind {
	case 'f':
		return pycompat.FloatRepr(v.f)
	case 'b':
		return strconv.FormatBool(v.b)
	}
	return strconv.FormatInt(v.i, 10)
}

func (v value) json() string {
	if v.kind == 'f' {
		return pycompat.JSONFloat(v.f)
	}
	return v.csv()
}

func printCSV(w io.Writer, recs []record) {
	cw := newCSVWriter(w)
	cw.WriteRow(stringRow("timestamp", "millis", "status", "trendFlags", "value"))
	for _, r := range recs {
		cw.WriteRow(stringRow(r.timestamp, strconv.FormatInt(r.millis, 10), r.status, r.trendFlags, r.value.csv()))
	}
}

// printJSON mirrors print(json.dumps(records, indent=1)); every string here is plain ASCII.
func printJSON(w io.Writer, recs []record) {
	if len(recs) == 0 {
		fmt.Fprintln(w, "[]")
		return
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, r := range recs {
		fmt.Fprintf(&b, " {\n  \"timestamp\": %s,\n  \"millis\": %d,\n  \"status\": %s,\n  \"trendFlags\": %s,\n"+
			"  \"value\": %s\n }", strconv.Quote(r.timestamp), r.millis, strconv.Quote(r.status),
			strconv.Quote(r.trendFlags), r.value.json())
		if i < len(recs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	io.WriteString(w, b.String())
}
