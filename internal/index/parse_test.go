package index

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// TestPythonErrorTexts compares every error with the text Python printed for
// the same input (testdata/pyerrors.json, from gen_pyerrors.py).
func TestPythonErrorTexts(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "pyerrors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Loads [][2]*string `json:"loads"`
		File  [][2]*string `json:"file"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, err error, want *string) {
		t.Helper()
		switch {
		case want == nil && err != nil:
			t.Errorf("error %q, Python decoded it", err)
		case want != nil && (err == nil || err.Error() != *want):
			t.Errorf("error %v, want %q", err, *want)
		}
	}
	for _, c := range fx.Loads {
		t.Run("loads "+*c[0], func(t *testing.T) {
			_, err := Decode([]byte(*c[0]))
			check(t, err, c[1])
		})
	}
	dir := t.TempDir()
	for _, c := range fx.File {
		t.Run("file "+*c[0], func(t *testing.T) {
			data, err := hex.DecodeString(*c[0])
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "index.json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = LoadPython(path)
			check(t, err, c[1])
		})
	}
}

func TestDecodeValues(t *testing.T) {
	v, err := Decode([]byte(`{"s": "a\"\\\/\b\f\n\r\tz é😀 \ud800",
		"n": [0, -0, 12, 1.5e3, 1E+2, -2.25, NaN, Infinity, -Infinity],
		"e": [], "o": {}, "big": {"k0":0,"k1":1,"k2":2,"k3":3,"k4":4,"k5":5,"k6":6,"k7":7,
		"k8":8,"k9":9,"k10":10,"k11":11,"k12":12,"k13":13,"k1":99}}`))
	if err != nil {
		t.Fatal(err)
	}
	o := v.(*Object)
	if s, _ := o.String("s"); s != "a\"\\/\b\f\n\r\tz é😀 �" {
		t.Errorf("escapes: %q", s)
	}
	nums, _ := o.Get("n")
	want := []any{json.Number("0"), json.Number("-0"), json.Number("12"), json.Number("1.5e3"),
		json.Number("1E+2"), json.Number("-2.25"), json.Number("NaN"), json.Number("Infinity"),
		json.Number("-Infinity")}
	if !reflect.DeepEqual(nums, want) {
		t.Errorf("numbers %#v", nums)
	}
	ints := []bool{true, true, true, false, false, false, false, false, false}
	for i, n := range want {
		if IsInt(n.(json.Number)) != ints[i] {
			t.Errorf("IsInt(%s) = %v", n, !ints[i])
		}
	}
	if i, ok := Int(json.Number("-0")); !ok || i != 0 {
		t.Error("Int(-0)")
	}
	if _, ok := Int(json.Number("1.5e3")); ok {
		t.Error("Int of a float literal")
	}
	if f, ok := Float(json.Number("1E+2")); !ok || f != 100 {
		t.Error("Float(1E+2)")
	}
	if f, ok := Float(json.Number("1e400")); !ok || f < 1e308 {
		t.Error("Float(1e400) should be +Inf like Python")
	}
	if e, _ := o.Get("e"); !reflect.DeepEqual(e, []any{}) {
		t.Errorf("empty array %#v", e)
	}
	if o.Object("o").Len() != 0 {
		t.Error("empty object")
	}
	big := o.Object("big")
	if big.Len() != 14 || big.Keys()[1] != "k1" {
		t.Errorf("big object keys %q", big.Keys())
	}
	if k1, _ := big.Get("k1"); k1 != json.Number("99") {
		t.Errorf("repeated key in an indexed object: %v", k1)
	}
	if v13, _ := big.Get("k13"); v13 != json.Number("13") || big.Values()[13] != json.Number("13") {
		t.Error("indexed lookup")
	}
}

func TestOSErrorText(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPython(filepath.Join(dir, "missing.json"))
	want := "[Errno 2] No such file or directory: " + pycompat.FSRepr(filepath.Join(dir, "missing.json"))
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
	if runtime.GOOS != "windows" {
		_, err = LoadPython(dir)
		if want := "[Errno 21] Is a directory: '" + dir + "'"; err == nil || err.Error() != want {
			t.Errorf("directory: got %v, want %q", err, want)
		}
	}
	if got := OSError(errors.New("plain"), "p"); got != "plain" {
		t.Errorf("an error without errno: got %q", got)
	}
}

// TestDecodeNestingLimit: a document nested deeper than MaxDepth stops with a
// RecursionError naming the container, without exhausting the stack; MaxDepth
// itself still decodes.
func TestDecodeNestingLimit(t *testing.T) {
	for _, c := range []struct{ open, close, container string }{{"[", "]", "array"}, {`{"a":`, "}", "object"}} {
		ok := strings.Repeat(c.open, MaxDepth) + "1" + strings.Repeat(c.close, MaxDepth)
		if _, err := Decode([]byte(ok)); err != nil {
			t.Errorf("depth %d %s: %v", MaxDepth, c.container, err)
		}
		deep := strings.Repeat(c.open, MaxDepth+1) + "1" + strings.Repeat(c.close, MaxDepth+1)
		_, err := Decode([]byte(deep))
		want := "maximum nesting depth (10000) exceeded while decoding a JSON " + c.container +
			" from a unicode string"
		if err == nil || err.Error() != want {
			t.Errorf("depth %d %s: err %v, want %q", MaxDepth+1, c.container, err, want)
		}
	}
	if _, err := Decode([]byte(strings.Repeat("[", 1_000_000))); err == nil {
		t.Error("a million open brackets decoded")
	}
}
