package hdbread

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

func formatG(v float64, digits int) string { return strconv.FormatFloat(v, 'g', digits, 64) }

// csvField quotes a field the way Python csv.writer (QUOTE_MINIMAL) does.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
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
	fmt.Fprintln(w, "timestamp,millis,status,trendFlags,value")
	for _, r := range recs {
		fields := []string{r.timestamp, strconv.FormatInt(r.millis, 10), r.status, r.trendFlags, r.value.csv()}
		for i := range fields {
			fields[i] = csvField(fields[i])
		}
		fmt.Fprintln(w, strings.Join(fields, ","))
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
