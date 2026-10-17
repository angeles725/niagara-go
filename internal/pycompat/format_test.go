package pycompat

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// TestFormatRefusesHugeResults covers the one intended difference from Python: a result
// longer than maxFormatLen code points is a MemoryError instead of an allocation attempt,
// while the specs whose output stays short still format and earlier errors still win.
func TestFormatRefusesHugeResults(t *testing.T) {
	huge := strconv.Itoa(maxFormatLen + 1)
	for _, c := range []struct {
		v          any
		spec       string
		want, errT string
	}{
		{"abc", huge, "", "MemoryError"},
		{"abc", "*^" + huge, "", "MemoryError"},
		{"abc", "9223372036854775807", "", "MemoryError"},
		{"abc", "." + huge, "abc", ""},
		{"abc", huge + "d", "", "ValueError"},
		{42, huge, "", "MemoryError"},
		{42, "0" + huge, "", "MemoryError"},
		{42, "0=" + huge + ",", "", "MemoryError"},
		{-1.5, "<" + huge + "e", "", "MemoryError"},
		{1.5, ".2147482623f", "", "MemoryError"},
		{1.5, ".2147482623%", "", "MemoryError"},
		{1, ".2147482623E", "", "MemoryError"},
		{1.5, "#.2147482623g", "", "MemoryError"},
		{1.5, ".2147482623g", "1.5", ""},
		{math.Inf(-1), ".2147482623f", "-inf", ""},
		{math.NaN(), "#.2147482623G", "NAN", ""},
	} {
		got, err := Format(c.v, c.spec)
		var fe *FormatError
		switch {
		case c.errT == "":
			if err != nil || got != c.want {
				t.Errorf("Format(%v, %q) = %q, %v; want %q", c.v, c.spec, got, err, c.want)
			}
		case !errors.As(err, &fe) || fe.Type != c.errT:
			t.Errorf("Format(%v, %q) = %q, %v; want a %s", c.v, c.spec, got, err, c.errT)
		case c.errT == "MemoryError" && fe.Msg != "":
			t.Errorf("Format(%v, %q) MemoryError message %q, want Python's empty str()", c.v, c.spec, fe.Msg)
		}
	}
}

// TestDigitCapsAreExact proves the strconv precision caps of sigDigits and fixedDigits drop
// nothing: past them a float64 only has zeros, so the trimmed digits equal uncapped ones.
func TestDigitCapsAreExact(t *testing.T) {
	trim := func(s string) (string, int) {
		mant, exp, _ := strings.Cut(s, "e")
		digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
		e, _ := strconv.Atoi(exp)
		return digits, e + 1
	}
	for _, v := range []float64{5e-324, 2.2250738585072009e-308, 2.2250738585072014e-308, 1 / 3.0, 0.1, 1.7976931348623157e308, 123456.789} {
		d, p := sigDigits(v, 3000)
		wd, wp := trim(strconv.FormatFloat(v, 'e', 2999, 64))
		if d != wd || p != wp {
			t.Errorf("sigDigits(%g, 3000) = %d digits at %d, uncapped %d at %d", v, len(d), p, len(wd), wp)
		}
		fd, fp := fixedDigits(v, 3000)
		all := strconv.FormatFloat(v, 'f', 3000, 64)
		intLen := strings.IndexByte(all, '.')
		joined := strings.Replace(all, ".", "", 1)
		lead := len(joined) - len(strings.TrimLeft(joined, "0"))
		if want := strings.Trim(joined, "0"); fd != want || fp != intLen-lead {
			t.Errorf("fixedDigits(%g, 3000) = %d digits at %d, uncapped %d at %d", v, len(fd), fp, len(want), intLen-lead)
		}
	}
}
