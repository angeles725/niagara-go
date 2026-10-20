package testkit

import (
	"github.com/angeles725/niagara-go/internal/pyjson"
)

func IntV(n int) pyjson.Value { return pyjson.IntValue(int64(n)) }

func DumpLine(vs ...pyjson.Value) string { return pyjson.Dumps(Array(vs...), pyjson.Defaults(-1)) }

func Str(s string) pyjson.Value { return pyjson.String(s) }

func Boolean(b bool) pyjson.Value {
	return pyjson.Value{Kind: pyjson.Bool, B: b}
}

func OptStr(s string) pyjson.Value {
	if s == "" {
		return pyjson.Value{}
	}
	return Str(s)
}

func Array(vs ...pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: vs} }
