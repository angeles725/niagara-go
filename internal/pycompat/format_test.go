package pycompat

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// formatGolden is one testdata/golden/format_*.golden file written by testdata/gen.py: the
// Python version, the typed values and one [spec, results] row per spec, where a result is
// the str format(value, spec) returned or [exception class, message], and null marks a pair
// another golden holds.
type formatGolden struct {
	Python string              `json:"python"`
	Values []json.RawMessage   `json:"values"`
	Format [][]json.RawMessage `json:"format"`
}

func replayFormatGolden(t *testing.T, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	var g formatGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	values := make([]any, len(g.Values))
	for i, raw := range g.Values {
		values[i] = typed(t, raw)
	}
	checked := 0
	for _, c := range g.Format {
		var spec string
		var results []json.RawMessage
		decode(t, c[0], &spec)
		decode(t, c[1], &results)
		if len(results) != len(values) {
			t.Fatalf("%s: spec %q has %d results for %d values", name, spec, len(results), len(values))
		}
		for i, raw := range results {
			if string(raw) == "null" {
				continue
			}
			checked++
			got, err := Format(values[i], spec)
			var fe *FormatError
			if raw[0] == '[' {
				var want [2]string
				decode(t, raw, &want)
				if !errors.As(err, &fe) || fe.Type != want[0] || fe.Msg != want[1] {
					t.Errorf("format(%s, %q) = %q, %v; want %s: %s", g.Values[i], spec, got, err, want[0], want[1])
				}
				continue
			}
			var want string
			decode(t, raw, &want)
			if err != nil || got != want {
				t.Errorf("format(%s, %q) = %q, %v; want %q", g.Values[i], spec, got, err, want)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("%s: no format cases", name)
	}
	t.Logf("%s: %d format cases from Python %s", name, checked, g.Python)
}

func TestFormatStrInt(t *testing.T) { replayFormatGolden(t, "format_str_int.golden") }

func TestFormatFloat(t *testing.T) { replayFormatGolden(t, "format_float.golden") }

func TestFormatGoIntegerKinds(t *testing.T) {
	for _, c := range []struct {
		v    any
		want string
	}{
		{int8(-5), "-5"}, {int16(-5), "-5"}, {int32(-5), "-5"}, {int64(-5), "-5"}, {big.NewInt(-5), "-5"},
		{uint(5), "+5"}, {uint8(5), "+5"}, {uint16(5), "+5"}, {uint32(5), "+5"}, {uint64(1 << 63), "+9223372036854775808"},
		{float32(0.5), "+0.5"},
	} {
		if got, err := Format(c.v, "+"); err != nil || got != c.want {
			t.Errorf("Format(%T, \"+\") = %q, %v; want %q", c.v, got, err, c.want)
		}
	}
	if _, err := Format(struct{}{}, ""); err == nil {
		t.Error("Format of an unsupported Go type succeeded")
	}
}
