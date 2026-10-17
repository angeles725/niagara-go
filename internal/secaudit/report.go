package secaudit

import (
	"fmt"
	"io"
	"sort"
	"strings"
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
		var objs []string
		for _, x := range r.rows {
			objs = append(objs, jsonObject([][2]string{{"id", pyJSONString(x.id)}, {"sev", pyJSONString(x.sev)},
				{"title", pyJSONString(x.title)}, {"observed", pyJSONString(x.observed)},
				{"secure", pyJSONString(x.secure)}, {"verdict", pyJSONString(x.verdict)},
				{"block", pyJSONString(x.block)}, {"note", pyJSONString(x.note)}}))
		}
		fmt.Fprintln(w, jsonList(objs))
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

// jsonObject renders ordered key/value pairs (values already JSON) like json.dumps(indent=2)
// at nesting level 1.
func jsonObject(kv [][2]string) string {
	parts := make([]string, len(kv))
	for i, p := range kv {
		parts[i] = "    " + pyJSONString(p[0]) + ": " + p[1]
	}
	return "  {\n" + strings.Join(parts, ",\n") + "\n  }"
}

// jsonList renders already-indented objects as a json.dumps(indent=2) list.
func jsonList(objs []string) string {
	if len(objs) == 0 {
		return "[]"
	}
	return "[\n" + strings.Join(objs, ",\n") + "\n]"
}
