package honest

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// fixtures is testdata/fixtures.json (python3 -I internal/honest/testdata/gen.py).
type fixtures struct {
	ExitInput int                 `json:"exit_input"`
	Scope     [][]json.RawMessage `json:"scope"`
	Miss      [][]json.RawMessage `json:"miss"`
	Fail      [][]json.RawMessage `json:"fail"`
}

func load(t *testing.T) fixtures {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func dec(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

func counts(t *testing.T, raw json.RawMessage) []Count {
	var pairs [][]json.RawMessage
	dec(t, raw, &pairs)
	out := make([]Count, len(pairs))
	for i, p := range pairs {
		dec(t, p[0], &out[i].Label)
		dec(t, p[1], &out[i].N)
	}
	return out
}

func TestExitInputIsCannotSearch(t *testing.T) {
	if ExitInput != load(t).ExitInput || ExitInput != cli.ExitCannotSearch {
		t.Fatalf("ExitInput %d", ExitInput)
	}
}

func TestScopeAndMiss(t *testing.T) {
	f := load(t)
	for _, c := range f.Scope {
		var where, want string
		dec(t, c[0], &where)
		dec(t, c[2], &want)
		if got := Scope(where, counts(t, c[1])...); got != want {
			t.Errorf("Scope = %q, Python %q", got, want)
		}
	}
	for _, c := range f.Miss {
		var what, where, want string
		dec(t, c[0], &what)
		dec(t, c[1], &where)
		dec(t, c[3], &want)
		if got := Miss(what, where, counts(t, c[2])...); got != want {
			t.Errorf("Miss = %q, Python %q", got, want)
		}
	}
}

func TestFail(t *testing.T) {
	for _, c := range load(t).Fail {
		var tool, msg, stderr string
		var code, exit int
		dec(t, c[0], &tool)
		dec(t, c[1], &msg)
		dec(t, c[2], &code)
		dec(t, c[3], &stderr)
		dec(t, c[4], &exit)
		var buf bytes.Buffer
		if got := Fail(&buf, tool, msg, code); got != exit || buf.String() != stderr {
			t.Errorf("Fail(%q, %q, %d) = %d %q, Python %d %q", tool, msg, code, got, buf.String(), exit, stderr)
		}
	}
}
