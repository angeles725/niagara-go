package scope_test

import (
	"bytes"
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
	data, err := filepath.Abs("../testdata")
	if err != nil {
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
