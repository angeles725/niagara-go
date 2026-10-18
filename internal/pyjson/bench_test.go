package pyjson

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// methodIndexLike is a value shaped like niagara-help's method-index.json: an
// object of many entries, each a list of method records whose strings are short
// identifiers and Java signatures. About 1 string in 20 holds non-ASCII text and
// about 1 in 50 a character json must escape, so both the plain-run fast path and
// the escape paths are exercised.
func methodIndexLike(entries int) Value {
	rng := rand.New(rand.NewPCG(1, 2))
	words := []string{"get", "set", "Value", "Component", "Status", "Ord", "Slot", "Property",
		"Action", "Topic", "Facets", "Point", "Boolean", "Numeric", "Enum", "String", "Writable"}
	spice := []string{"é", "ñ", "中文", "😀", "\u00a0", "\t", "\"", "\\", "\n", "\x7f"}
	ident := func() string {
		s := ""
		for n := 1 + rng.IntN(3); n > 0; n-- {
			s += words[rng.IntN(len(words))]
		}
		switch k := rng.IntN(100); {
		case k < 5:
			s += spice[rng.IntN(4)]
		case k < 7:
			s += spice[4+rng.IntN(len(spice)-4)]
		}
		return s
	}
	o := NewObject()
	for e := 0; e < entries; e++ {
		var recs []Value
		for n := 1 + rng.IntN(3); n > 0; n-- {
			cls := "javax.baja." + ident()
			ret, name, param := ident(), ident(), ident()
			recs = append(recs, ObjectOf(
				Field{"class", String(cls)},
				Field{"module", String("baja")},
				Field{"signature", String(fmt.Sprintf("public %s %s(%s arg0)", ret, name, param))},
				Field{"line", IntValue(int64(rng.IntN(5000)))},
				Field{"modifiers", StringList([]string{"public", "final"})},
				Field{"return_type", String(ret)},
				Field{"params", StringList([]string{param})},
			))
		}
		o.Set(fmt.Sprintf("%s#%d", ident(), e), Value{Kind: Array, A: recs})
	}
	return o
}

func BenchmarkDumps(b *testing.B) {
	v := methodIndexLike(2000)
	for _, bc := range []struct {
		name string
		o    DumpOpts
	}{
		{"compact/ensure_ascii=false", DumpOpts{Indent: -1, ItemSep: ",", KeySep: ":"}},
		{"compact/ensure_ascii=true", Canonical},
		{"indent2/ensure_ascii=false", DumpOpts{Indent: 2, ItemSep: ",", KeySep: ": "}},
		{"indent2/ensure_ascii=true", Indent2},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(len(Dumps(v, bc.o))))
			b.ReportAllocs()
			for b.Loop() {
				Dumps(v, bc.o)
			}
		})
	}
}

func BenchmarkEncodeString(b *testing.B) {
	for _, bc := range []struct{ name, s string }{
		{"ascii", "public BStatusNumeric getOutStatusValue(BComponent arg0)"},
		{"nonascii", "public String getDescripción(Lugar año) // 中文 😀"},
	} {
		for _, ensure := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/ensure_ascii=%v", bc.name, ensure), func(b *testing.B) {
				b.SetBytes(int64(len(bc.s)))
				b.ReportAllocs()
				for b.Loop() {
					EncodeString(bc.s, ensure)
				}
			})
		}
	}
}
