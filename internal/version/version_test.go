package version

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// fixtures is testdata/fixtures.json (python3 -I internal/version/testdata/gen.py).
type fixtures struct {
	ToolsVersion string      `json:"tools_version"`
	Lines        [][3]string `json:"lines"`
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

// TestLineFormat checks the Python line format the fixture recorded, with the
// canonical ToolsVersion in place of the recorded TOOLS_VERSION.
func TestLineFormat(t *testing.T) {
	f := load(t)
	for _, c := range f.Lines {
		prefix := c[0] + " " + f.ToolsVersion
		rest, ok := strings.CutPrefix(c[2], prefix)
		if !ok {
			t.Fatalf("fixture line %q does not start with %q", c[2], prefix)
		}
		want := c[0] + " " + ToolsVersion + rest
		if got := lineWith(c[0], c[1]); got != want {
			t.Errorf("line(%q, %q) = %q, Python %q", c[0], c[1], got, want)
		}
	}
}

func TestDescribeFromBuildSettings(t *testing.T) {
	cases := []struct {
		rev, modified, want string
	}{
		{"d62aeb56a0123456789", "true", "d62aeb56a-dirty"},
		{"d62aeb56a0123456789", "false", "d62aeb56a"},
		{"abc", "", "abc"},
		{"", "true", ""},
	}
	for _, c := range cases {
		if got := describeVCS(c.rev, c.modified); got != c.want {
			t.Errorf("describeVCS(%q, %q) = %q, want %q", c.rev, c.modified, got, c.want)
		}
	}
}

// withDescribe sets the link-time description for one test.
func withDescribe(t *testing.T, d string) {
	t.Helper()
	old := gitDescribe
	gitDescribe = d
	t.Cleanup(func() { gitDescribe = old })
}

// stamp is what Describe returns from a toolchain VCS stamp: the git revision
// (always longer than 9) cut to 9 characters, with an optional "-dirty".
const stamp = `[0-9a-f]{9}(-dirty)?`

// stampRe also accepts empty: no VCS stamp, e.g. under go test.
var stampRe = regexp.MustCompile(`^(` + stamp + `)?$`)

func TestDescribe(t *testing.T) {
	t.Run("link-time value is printed as is", func(t *testing.T) {
		withDescribe(t, "v3.0.0-4-gd62aeb56a-dirty")
		if got := Describe(); got != "v3.0.0-4-gd62aeb56a-dirty" {
			t.Fatalf("Describe() = %q, want the -X value", got)
		}
	})
	t.Run("without link-time value it falls back to the build stamp", func(t *testing.T) {
		withDescribe(t, "")
		if got := Describe(); !stampRe.MatchString(got) {
			t.Fatalf("Describe() = %q, want empty or <rev[:9]>[-dirty]", got)
		}
	})
}

func TestLine(t *testing.T) {
	t.Run("with a description", func(t *testing.T) {
		withDescribe(t, "d62aeb56a-dirty")
		if got, want := Line("bog-nav"), "bog-nav "+ToolsVersion+" (git d62aeb56a-dirty)"; got != want {
			t.Fatalf("Line = %q, want %q", got, want)
		}
	})
	t.Run("matches Describe when only the build stamp is known", func(t *testing.T) {
		withDescribe(t, "")
		re := regexp.MustCompile(`^bog-nav ` + regexp.QuoteMeta(ToolsVersion) + `( \(git ` + stamp + `\))?$`)
		got := Line("bog-nav")
		if !re.MatchString(got) {
			t.Fatalf("Line = %q, want %s", got, re)
		}
		if d := Describe(); (d == "") != (got == "bog-nav "+ToolsVersion) {
			t.Fatalf("Line = %q disagrees with Describe() = %q", got, d)
		}
	})
}
