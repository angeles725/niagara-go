package argparse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

type fixtureCase struct {
	Parser  string   `json:"parser"`
	Columns int      `json:"columns"`
	Argv    []string `json:"argv"`
	Exit    *int     `json:"exit"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
	NS      *string  `json:"ns"`
}

type fixtureDoc struct {
	Python  string          `json:"python"`
	Parents map[string]spec `json:"parents"`
	Parsers map[string]spec `json:"parsers"`
	Cases   []fixtureCase   `json:"cases"`
}

func loadFixtures(t *testing.T) fixtureDoc {
	t.Helper()
	data, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc fixtureDoc
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// firstDiff describes where got and want first differ, line by line.
func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl || i >= len(g) || i >= len(w) {
			return fmt.Sprintf("line %d:\n   got %q\n  want %q", i+1, gl, wl)
		}
	}
	return "equal"
}

// TestFormatHelpMatchesFixtures checks FormatHelp against every recorded top-level
// `-h`/`--help` case.
func TestFormatHelpMatchesFixtures(t *testing.T) {
	doc := loadFixtures(t)
	n := 0
	for i, c := range doc.Cases {
		if len(c.Argv) != 1 || (c.Argv[0] != "-h" && c.Argv[0] != "--help") || c.Exit == nil || *c.Exit != 0 {
			continue
		}
		n++
		t.Run(fmt.Sprintf("%03d_%s_%d", i, c.Parser, c.Columns), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(c.Columns))
			parents := map[string]*Parser{}
			for pname, ps := range doc.Parents {
				parents[pname] = specParser(t, ps, parents)
			}
			if got := specParser(t, doc.Parsers[c.Parser], parents).FormatHelp(); got != c.Stdout {
				t.Errorf("help differs at %s", firstDiff(got, c.Stdout))
			}
		})
	}
	if n == 0 {
		t.Fatal("no help cases")
	}
}
