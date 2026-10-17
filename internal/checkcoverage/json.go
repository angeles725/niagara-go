package checkcoverage

import (
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// jsonReport is the --json document, json.dumps(..., indent=2, ensure_ascii=False).
// With unreadable block files it adds "unreadable_blocks" and an uncovered run is
// "incomplete" rather than "clear" (intended difference: Python skips them silently).
func jsonReport(results map[string]*coverage, order []string, nfiles, unreadable int, covered bool) string {
	hits := func(hs []hit) pyjson.Value {
		items := make([]pyjson.Value, len(hs))
		for i, h := range hs {
			item := pyjson.NewObject()
			item.Set("block", jsonStr(h.block))
			item.Set("h1", jsonStr(h.h1))
			items[i] = item
		}
		return pyjson.Value{Kind: pyjson.Array, A: items}
	}
	names := pyjson.NewObject()
	for _, n := range order {
		entry := pyjson.NewObject()
		entry.Set("subject", hits(results[n].subject))
		entry.Set("body", hits(results[n].body))
		names.Set(jsonStr(n).S, entry)
	}
	verdict := "clear"
	switch {
	case covered:
		verdict = "prior-coverage-found"
	case unreadable > 0:
		verdict = "incomplete"
	}
	doc := pyjson.NewObject()
	doc.Set("scanned_blocks", pyjson.IntValue(int64(nfiles)))
	doc.Set("names", names)
	if unreadable > 0 {
		doc.Set("unreadable_blocks", pyjson.IntValue(int64(unreadable)))
	}
	doc.Set("verdict", pyjson.String(verdict))
	return pyjson.Dumps(doc, pyjson.DumpOpts{Indent: 2, ItemSep: ",", KeySep: ": "})
}

// jsonStr is s as a pyjson str. It passes through []rune first: pyjson reads WTF-8, and
// this keeps an undecodable byte (a name from argv, a file name) one U+FFFD.
func jsonStr(s string) pyjson.Value { return pyjson.String(string([]rune(s))) }
