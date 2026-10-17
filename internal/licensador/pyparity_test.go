package licensador

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// testdata/pyparity.json was produced by Python 3.15 on curated and seeded
// random inputs: each row is [input, expected] (expected "ERR:<str(exc)>" on
// an exception). It pins the Python semantics the CLI output depends on;
// the float and str repr tables live with internal/pyjson.

func loadParity(t *testing.T) map[string][][]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/pyparity.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string][][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func errOr(s string, err error) string {
	if err != nil {
		return "ERR:" + err.Error()
	}
	return s
}

func TestParityBase64(t *testing.T) {
	for _, row := range loadParity(t)["b64"] {
		b, err := b64uDecode(row[0])
		if got := errOr(hex.EncodeToString(b), err); got != row[1] {
			t.Errorf("b64 %q: got %q want %q", row[0], got, row[1])
		}
	}
}

func TestParityJSON(t *testing.T) {
	for _, row := range loadParity(t)["json"] {
		in, _ := hex.DecodeString(row[0])
		indent, canon := "", ""
		text, err := decodeJSONBytes(in)
		if err == nil {
			var v pyjson.Value
			v, err = pyjson.Loads(text)
			indent = pyjson.Dumps(v, pyjson.DumpOpts{EnsureASCII: true, Indent: 2, ItemSep: ",", KeySep: ": "})
			canon = pyjson.Dumps(v, pyjson.Canonical)
		}
		if err != nil {
			indent, canon = "ERR:"+err.Error(), ""
		}
		if indent != row[1] || canon != row[2] {
			t.Errorf("json %q:\n got %q %q\nwant %q %q", in, indent, canon, row[1], row[2])
		}
	}
}

func TestParityISO(t *testing.T) {
	for _, row := range loadParity(t)["iso"] {
		// Python's error text is never printed (a bad date is E_PARSE), so
		// only acceptance and the parsed value are compared.
		v, err := parseISO(row[0])
		got := errOr(v.isoformat(), err)
		if strings.HasPrefix(got, "ERR:") && strings.HasPrefix(row[1], "ERR:") {
			continue
		}
		if got != row[1] {
			t.Errorf("fromisoformat(%q): got %q want %q", row[0], got, row[1])
		}
	}
}

func TestParityInt(t *testing.T) {
	for _, row := range loadParity(t)["int"] {
		n, err := pyInt(row[0])
		got := ""
		if err == nil {
			got = n.String()
		}
		if got = errOr(got, err); got != row[1] {
			t.Errorf("int(%q): got %q want %q", row[0], got, row[1])
		}
	}
}
