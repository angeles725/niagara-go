package hdbread

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func formatG(v float64, digits int) string { return strconv.FormatFloat(v, 'g', digits, 64) }

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// pyRepr renders a float64 exactly like Python repr(): shortest round-trip digits, positional
// notation for exponents -4..15 (with ".0" when integral), otherwise d.ddde+XX.
func pyRepr(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(v) {
		sign = "-"
	}
	mant, expPart, _ := strings.Cut(strconv.FormatFloat(math.Abs(v), 'e', -1, 64), "e")
	exp, _ := strconv.Atoi(expPart)
	digits := strings.Replace(mant, ".", "", 1)
	switch {
	case exp < -4 || exp >= 16:
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		return fmt.Sprintf("%s%se%+03d", sign, m, exp)
	case exp < 0:
		return sign + "0." + strings.Repeat("0", -exp-1) + digits
	case len(digits) <= exp+1:
		return sign + digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	default:
		return sign + digits[:exp+1] + "." + digits[exp+1:]
	}
}

// jsonFloat is Python json.dumps of a float: repr, with NaN/Infinity/-Infinity spelled out.
func jsonFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}
	return pyRepr(v)
}

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
		return pyRepr(v.f)
	case 'b':
		return strconv.FormatBool(v.b)
	}
	return strconv.FormatInt(v.i, 10)
}

func (v value) json() string {
	if v.kind == 'f' {
		return jsonFloat(v.f)
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

// decodeReplace is Python bytes.decode("utf-8", "replace"): each maximal invalid subpart of a
// sequence becomes one U+FFFD.
func decodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			sb.WriteByte(c)
			i++
			continue
		}
		need, lo, hi := 0, byte(0x80), byte(0xbf)
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need, lo = 2, 0xa0
		case c >= 0xe1 && c <= 0xef && c != 0xed:
			need = 2
		case c == 0xed:
			need, hi = 2, 0x9f
		case c == 0xf0:
			need, lo = 3, 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need, hi = 3, 0x8f
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lo || b[j] > hi {
				break
			}
			j++
			lo, hi = 0x80, 0xbf
		}
		if need > 0 && j == i+1+need {
			sb.Write(b[i:j])
		} else {
			sb.WriteRune('�')
		}
		i = j
	}
	return sb.String()
}
