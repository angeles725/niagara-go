package pycsv

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// The fixtures are frozen output of the retired testdata/gen.py (Python 3.15's csv module).

func value(t *testing.T, pair []json.RawMessage) any {
	t.Helper()
	var kind, s string
	var b bool
	must(t, json.Unmarshal(pair[0], &kind))
	switch kind {
	case "none":
		return nil
	case "bool":
		must(t, json.Unmarshal(pair[1], &b))
		return b
	}
	must(t, json.Unmarshal(pair[1], &s))
	switch kind {
	case "int":
		n, _ := new(big.Int).SetString(s, 10)
		if n.IsInt64() {
			return int(n.Int64())
		}
		return n
	case "float":
		switch s {
		case "nan":
			return math.NaN()
		case "inf":
			return math.Inf(1)
		case "-inf":
			return math.Inf(-1)
		}
		var f float64
		must(t, json.Unmarshal([]byte(s), &f))
		if s == "-0.0" {
			f = math.Copysign(0, -1)
		}
		return f
	}
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriterMatchesPython(t *testing.T) {
	data, err := os.ReadFile("testdata/fixtures.json")
	must(t, err)
	var fx struct {
		Cases [][]json.RawMessage `json:"cases"`
	}
	must(t, json.Unmarshal(data, &fx))
	if len(fx.Cases) == 0 {
		t.Fatal("no fixtures")
	}
	for _, c := range fx.Cases {
		var delim, term, want string
		var raw [][][]json.RawMessage
		must(t, json.Unmarshal(c[0], &delim))
		must(t, json.Unmarshal(c[1], &term))
		must(t, json.Unmarshal(c[2], &raw))
		must(t, json.Unmarshal(c[3], &want))
		rows := make([][]any, len(raw))
		for i, r := range raw {
			rows[i] = make([]any, len(r))
			for j, v := range r {
				rows[i][j] = value(t, v)
			}
		}
		var b strings.Builder
		w := NewWriter(&b)
		w.Delimiter, _ = utf8.DecodeRuneInString(delim)
		w.LineTerminator = term
		for _, row := range rows {
			if err := w.WriteRow(row); err != nil {
				t.Fatal(err)
			}
		}
		if b.String() != want {
			t.Errorf("delimiter %q terminator %q rows %v:\n got %q\nwant %q", delim, term, rows, b.String(), want)
		}
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteErrorIsReturned(t *testing.T) {
	if err := NewWriter(failWriter{}).WriteRow([]any{"a"}); err == nil || err.Error() != "disk full" {
		t.Errorf("WriteRow error = %v, want disk full", err)
	}
}
