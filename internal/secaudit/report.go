package secaudit

import (
	"fmt"
	"io"
	"sort"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// sevRank orders severities (SEV in the Python tool).
var sevRank = map[string]int{"crit": 3, "high": 2, "med": 1, "info": 0}

// ANSI colours; the Python tool prints them unconditionally.
var color = map[string]string{"crit": "\033[91m", "high": "\033[93m", "med": "\033[96m", "info": "\033[90m",
	"ok": "\033[92m", "off": "\033[0m"}

// row is one check result.
type row struct {
	id, sev, title, observed, secure, verdict, block, note string
}

// report collects the check rows in the order the checks ran.
type report struct{ rows []row }

func (r *report) add(id, sev, title, observed, secure, verdict, block, note string) {
	r.rows = append(r.rows, row{id, sev, title, observed, secure, verdict, block, note})
}

// dump prints the rows as JSON (in check order) or as the sorted text report.
func (r *report) dump(w io.Writer, asJSON bool) {
	if asJSON {
		keys := []string{"id", "sev", "title", "observed", "secure", "verdict", "block", "note"}
		objs := make([]pyjson.Value, len(r.rows))
		for i, x := range r.rows {
			objs[i] = jsonRecord(keys, jsonStr(x.id), jsonStr(x.sev), jsonStr(x.title), jsonStr(x.observed),
				jsonStr(x.secure), jsonStr(x.verdict), jsonStr(x.block), jsonStr(x.note))
		}
		fmt.Fprintln(w, jsonDumps(objs))
		return
	}
	sort.SliceStable(r.rows, func(i, j int) bool {
		a, b := r.rows[i], r.rows[j]
		if sevRank[a.sev] != sevRank[b.sev] {
			return sevRank[a.sev] > sevRank[b.sev]
		}
		return a.id < b.id
	})
	var fails []row
	for _, x := range r.rows {
		if x.verdict == "FAIL" {
			fails = append(fails, x)
		}
	}
	fmt.Fprintf(w, "\n  Niagara N4 security posture — %d finding(s) / %d checks\n\n", len(fails), len(r.rows))
	for _, x := range r.rows {
		mark := "MANUAL"
		if x.verdict == "FAIL" || x.verdict == "PASS" || x.verdict == "SKIP" {
			mark = x.verdict
		}
		col := color["info"]
		switch x.verdict {
		case "FAIL":
			col = color[x.sev]
		case "PASS":
			col = color["ok"]
		}
		fmt.Fprintf(w, "  %s[%-6s]%s %s (%s) %s  [%s]\n", col, mark, color["off"], x.id, x.sev, x.title, x.block)
		if x.verdict == "FAIL" || x.verdict == "MANUAL" || x.verdict == "SKIP" {
			fmt.Fprintf(w, "           observed: %s\n", x.observed)
			if x.verdict == "FAIL" {
				fmt.Fprintf(w, "           secure  : %s\n", x.secure)
			}
			if x.note != "" {
				fmt.Fprintf(w, "           note    : %s\n", x.note)
			}
		}
	}
	count := func(sev string) int {
		n := 0
		for _, x := range fails {
			if x.sev == sev {
				n++
			}
		}
		return n
	}
	fmt.Fprintf(w, "\n  summary: %d critical · %d high · %d med · %d findings\n\n",
		count("crit"), count("high"), count("med"), len(fails))
}

// jsonDumps is json.dumps(objs, indent=2) with the default ensure_ascii=True.
func jsonDumps(objs []pyjson.Value) string {
	return pyjson.Dumps(pyjson.Value{Kind: pyjson.Array, A: objs},
		pyjson.DumpOpts{EnsureASCII: true, Indent: 2, ItemSep: ",", KeySep: ": "})
}

// jsonRecord is a JSON object with keys[i] mapped to vals[i], in that order.
func jsonRecord(keys []string, vals ...pyjson.Value) pyjson.Value {
	o := pyjson.NewObject()
	for i, k := range keys {
		o.Set(k, vals[i])
	}
	return o
}

// jsonStr is a JSON string. Text that is not valid UTF-8 (a file name) reads as one U+FFFD per
// invalid byte, as the audit has always printed it (Python would show \udcXX surrogates).
func jsonStr(s string) pyjson.Value { return pyjson.String(string([]rune(s))) }
