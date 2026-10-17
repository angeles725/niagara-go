package corpusnav

import (
	"fmt"
	"sort"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// kv / obj are an ordered JSON object (a Python dict keeps insertion order).
type kv struct {
	key string
	val any
}

type obj []kv

// jsonDumps is json.dumps(v, ensure_ascii=False) without indent: ", " and ": ".
func jsonDumps(v any) string {
	return pyjson.Dumps(jsonValue(v), pyjson.DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "})
}

// jsonValue is v (a string, int, []any or obj) as a pyjson value. A string passes through
// []rune first: pyjson reads WTF-8, and this keeps an undecodable byte one U+FFFD.
func jsonValue(v any) pyjson.Value {
	switch x := v.(type) {
	case string:
		return pyjson.String(string([]rune(x)))
	case int:
		return pyjson.IntValue(int64(x))
	case []any:
		items := make([]pyjson.Value, len(x))
		for i, it := range x {
			items[i] = jsonValue(it)
		}
		return pyjson.Value{Kind: pyjson.Array, A: items}
	case obj:
		o := pyjson.NewObject()
		for _, f := range x {
			o.Set(string([]rune(f.key)), jsonValue(f.val))
		}
		return o
	}
	panic(fmt.Sprintf("jsonValue: unsupported %T", v))
}

func sortStable(xs []string, less func(a, b string) bool) {
	sort.SliceStable(xs, func(i, j int) bool { return less(xs[i], xs[j]) })
}
