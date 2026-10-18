package corpusnav

import (
	"sort"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// jsonDumps is json.dumps(v, ensure_ascii=False) without indent: ", " and ": ".
// v is a string, int, []any or pyjson.Obj (an ordered dict).
func jsonDumps(v any) string {
	return pyjson.DumpsAny(v, pyjson.DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "})
}

func sortStable(xs []string, less func(a, b string) bool) {
	sort.SliceStable(xs, func(i, j int) bool { return less(xs[i], xs[j]) })
}
