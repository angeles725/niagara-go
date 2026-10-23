package scope_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// scopeNames are every scope a miss block can name (scope.py SCOPES).
var scopeNames = []string{
	"guides", "devguide", "classes", "jdk", "methods", "slots", "source",
	"inheritance", "xref", "modules", "pdf", "fulltext",
}

var hint = regexp.MustCompile(`([a-z][a-z-]*(?: --[a-z-]+)*(?: \([^)]*\))?) 'BZzz'`)

// TestMissHintsAreLiveCommands: every "try:" hint of every scope is a command
// line nh accepts. A retired command (fuzzy, method-sig, inherited-slots,
// module) exits 2, so a hint naming one sends the reader into a dead end.
func TestMissHintsAreLiveCommands(t *testing.T) {
	// nh rebuilds a stale fulltext index in place: run on a copy so the tracked
	// testdata/indexes/fulltext.sqlite is never rewritten by a test run.
	data := filepath.Join(t.TempDir(), "testdata")
	if err := os.CopyFS(data, os.DirFS("../testdata")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(help.EnvDir, data)
	for _, name := range scopeNames {
		lines, err := scope.Miss(t.TempDir(), "x", "BZzz", name, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		try := lines[3]
		for _, m := range hint.FindAllStringSubmatch(try, -1) {
			words := strings.Fields(regexp.MustCompile(` \([^)]*\)`).ReplaceAllString(m[1], ""))
			args := append([]string{"--niagara-version", "4.14.0.162", words[0], "BZzz"}, words[1:]...)
			var out, errb bytes.Buffer
			code := help.Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			if code == 2 {
				t.Errorf("scope %s hint %q: nh %v exits 2: %s%s", name, m[1], args[2:], errb.String(), out.String())
			}
		}
	}
}

// TestSourceScopesClaimOnlyTheIndex: a source, xref or module miss says what
// nh's source index holds, never what the vendor shipped: the index holds
// docSource-doc.jar and the vendor .java of module jars (4.14
// clHVACHeating-rt.jar), not third-party packages such as org/mockito.
func TestSourceScopesClaimOnlyTheIndex(t *testing.T) {
	for _, name := range []string{"source", "xref", "modules"} {
		lines, err := scope.Miss(t.TempDir(), "source", "BZzz", name, nil)
		if err != nil {
			t.Fatal(err)
		}
		excluded := lines[2]
		if strings.Contains(excluded, "shipped") || !strings.Contains(excluded, "not in nh's source index") ||
			!strings.Contains(excluded, "docSource-doc.jar and the vendor .java of module jars are indexed, "+
				"third-party packages excluded") {
			t.Errorf("scope %s: %q", name, excluded)
		}
	}
}
