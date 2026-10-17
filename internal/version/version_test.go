package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func TestLineFormat(t *testing.T) {
	f := load(t)
	if ToolsVersion != f.ToolsVersion {
		t.Fatalf("ToolsVersion %q, fixture %q", ToolsVersion, f.ToolsVersion)
	}
	for _, c := range f.Lines {
		if got := lineWith(c[0], c[1]); got != c[2] {
			t.Errorf("line(%q, %q) = %q, Python %q", c[0], c[1], got, c[2])
		}
	}
}

// TestToolsVersionMatchesPython keeps ToolsVersion equal to tools/lib/version.py.
func TestToolsVersionMatchesPython(t *testing.T) {
	root := os.Getenv("NIAGARA_RESEARCH")
	if root == "" {
		root = "/home/cristian/niagara-research"
	}
	src, err := os.ReadFile(filepath.Join(root, "tools", "lib", "version.py"))
	if err != nil {
		t.Skipf("tools/lib/version.py not available: %v", err)
	}
	m := regexp.MustCompile(`(?m)^TOOLS_VERSION = "([^"]+)"`).FindSubmatch(src)
	if m == nil || string(m[1]) != ToolsVersion {
		t.Fatalf("tools/lib/version.py TOOLS_VERSION %q, Go %q", m, ToolsVersion)
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
