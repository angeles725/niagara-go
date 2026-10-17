package dashpreview

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// AUTO-MOCK: parse a *Reader.java and generate plausible {v,st} equipment data.

// Python's \w and \s are Unicode-aware; these classes approximate them.
var (
	stringArrayRe = regexp.MustCompile(`String\[\][\s\v\p{Z}]+([\p{L}\p{N}_]+)[\s\v\p{Z}]*=[\s\v\p{Z}]*\{([^}]+)\}`)
	quotedRe      = regexp.MustCompile(`"([^"]*)"`)
	digitRe       = regexp.MustCompile(`\p{Nd}`)
	nameDisplayRe = regexp.MustCompile(`(?i)name|display`)
)

type namedArray struct {
	name  string
	items []string
}

// parseStringArrays mirrors _parse_string_arrays (dict order: first
// definition position, last definition value).
func parseStringArrays(src string) []namedArray {
	var out []namedArray
	index := map[string]int{}
	for _, m := range stringArrayRe.FindAllStringSubmatch(src, -1) {
		var items []string
		for _, q := range quotedRe.FindAllStringSubmatch(m[2], -1) {
			items = append(items, q[1])
		}
		if len(items) == 0 {
			continue
		}
		if i, ok := index[m[1]]; ok {
			out[i].items = items
			continue
		}
		index[m[1]] = len(out)
		out = append(out, namedArray{m[1], items})
	}
	return out
}

// classifyReaderArrays mirrors _classify_reader_arrays.
func classifyReaderArrays(arrays []namedArray) (entities, numeric, labels []string, err error) {
	var leftover []namedArray
	for _, a := range arrays {
		nu := strings.ToUpper(a.name)
		switch {
		case strings.Contains(nu, "LABEL"):
			labels = a.items
		case strings.Contains(nu, "NUM") || strings.Contains(nu, "SLOT") && strings.Contains(nu, "DISPLAY"):
			numeric = a.items
		default:
			leftover = append(leftover, a)
		}
	}
	if numeric == nil {
		for i, a := range leftover {
			if strings.Contains(strings.ToUpper(a.name), "SLOT") {
				numeric = a.items
				leftover = append(leftover[:i:i], leftover[i+1:]...)
				break
			}
		}
	}
	for _, a := range leftover {
		if a.items[0] == "" {
			return nil, nil, nil, errors.New("IndexError: string index out of range")
		}
		first := []rune(a.items[0])[0]
		if unicode.IsUpper(first) || digitRe.MatchString(a.items[0]) {
			entities = a.items
			break
		}
	}
	return entities, numeric, labels, nil
}

// pyRound mirrors round(x, n) for a float (correctly rounded, half-even on
// the exact binary value).
func pyRound(x float64, n int) pyjson.Value {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', n, 64), 64)
	return pyjson.Value{Kind: pyjson.Float, F: f}
}

type slotRule struct {
	re *regexp.Regexp
	fn func(ei, si int) pyjson.Value
}

var slotRules = []slotRule{
	{regexp.MustCompile(`temp|temperature`), func(ei, si int) pyjson.Value { return pyRound(5.0+float64(ei)*1.3+float64(si)*0.7, 1) }},
	{regexp.MustCompile(`current|amp`), func(ei, si int) pyjson.Value { return pyRound(8.0+float64(ei)*0.5+float64(si)*0.4, 1) }},
	{regexp.MustCompile(`pressure|psig|suction`), func(ei, si int) pyjson.Value { return pyRound(150.0+float64(ei*5)+float64(si)*2.0, 1) }},
	{regexp.MustCompile(`voltage|volt`), func(ei, _ int) pyjson.Value { return pyRound(24.0+float64(ei)*0.3, 1) }},
	{regexp.MustCompile(`state|status|mode`), func(ei, _ int) pyjson.Value { return pyjson.Value{Kind: pyjson.Int, I: big.NewInt(int64(ei % 4))} }},
	{regexp.MustCompile(`humid|rh`), func(ei, _ int) pyjson.Value { return pyRound(50.0+float64(ei*3), 1) }},
	{regexp.MustCompile(`flow`), func(ei, _ int) pyjson.Value { return pyRound(200.0+float64(ei*10), 1) }},
	{regexp.MustCompile(`power|kw`), func(ei, _ int) pyjson.Value { return pyRound(1.5+float64(ei)*0.3, 2) }},
}

func plausibleValue(slot string, ei, si int) pyjson.Value {
	low := strings.ToLower(slot)
	for _, r := range slotRules {
		if r.re.MatchString(low) {
			return r.fn(ei, si)
		}
	}
	return pyRound(10.0+float64(ei)*1.1+float64(si)*0.5, 1)
}

// generateAutoMock mirrors generate_auto_mock: {"Entity/slot": {v, st}} with
// fault/stale/overridden injected at (1,0), (2,1), (3,2).
func generateAutoMock(entities, numeric, labels []string) pyjson.Value {
	inject := map[[2]int]string{{1, 0}: "fault", {2, 1}: "stale", {3, 2}: "overridden"}
	sections := []string{"Zona Norte", "Zona Sur", "Zona Este", "Zona Oeste"}
	result := pyjson.NewObject()
	entry := func(v pyjson.Value, st string) pyjson.Value {
		o := pyjson.NewObject()
		o.Set("v", v)
		o.Set("st", pyjson.String(st))
		return o
	}
	for ei, entity := range entities {
		for si, slot := range numeric {
			st := inject[[2]int{ei, si}]
			if st == "" {
				st = "ok"
			}
			v := pyjson.Value{}
			if st != "fault" {
				v = plausibleValue(slot, ei, si)
			}
			result.Set(entity+"/"+slot, entry(v, st))
		}
		for _, slot := range labels {
			v := sections[ei%len(sections)]
			if nameDisplayRe.MatchString(slot) {
				v = entity
			}
			result.Set(entity+"/"+slot, entry(pyjson.String(v), "ok"))
		}
	}
	return result
}

// findReaderJava mirrors find_reader_java: the first *Reader.java under the
// parent of rcDir, walking top-down (files of a directory before its
// subdirectories; names sorted, where os.walk uses directory order).
func findReaderJava(rcDir string) string {
	var walk func(dir string) string
	walk = func(dir string) string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "Reader.java") {
				return filepath.Join(dir, e.Name())
			}
		}
		for _, e := range entries {
			if e.IsDir() {
				if p := walk(filepath.Join(dir, e.Name())); p != "" {
					return p
				}
			}
		}
		return ""
	}
	return walk(filepath.Dir(rcDir))
}

// strList renders a Python list repr of strings.
func strList(xs []string) string {
	v := pyjson.Value{Kind: pyjson.Array}
	for _, x := range xs {
		v.A = append(v.A, pyjson.String(x))
	}
	return v.Repr()
}
