package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// TestSelftestChecksEveryIndexKind: besides the full-text assertions, selftest checks one
// stable fact of each lookup kind (lexicon, course, package, class, slots, method, module, palette, tag,
// lexicon-index); a fact
// whose index or text source this version lacks (or holds empty) is a SKIP, never a PASS.
func TestSelftestChecksEveryIndexKind(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	for _, name := range []string{"module-index.json", "palette-index.json", "tag-index.json", "lexicon-index.json"} {
		data, err := os.ReadFile(filepath.Join(testdata, "indexes", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(checkout, "indexes", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, errs, code := run(t, checkout, "--niagara-version", "1.0", "selftest")
	for _, want := range []string{
		"  PASS  method: method 'ackAlarm' lists javax.baja.alarm.BAlarmRecord.ackAlarm()\n",
		"  SKIP  lexicon: lexicon-text/ is absent in this version\n",
		"  SKIP  course: course-text/ is absent in this version\n",
		"  SKIP  package: package-text/ is absent in this version\n",
		"  SKIP  class: indexes/class-index.json is empty in this version\n",
		"  SKIP  slots: indexes/slots-index.json is absent in this version\n",
		"  PASS  module: module 'kitControl-rt' lists its dependency on baja\n",
		"  PASS  palette: palette 'BooleanLatch' finds it in kitControl-rt's module.palette\n",
		"  PASS  tag: tag 'hs:ahu' is a Marker in haystack-rt's tags.csv\n",
		"  PASS  lexicon-index: lexicon 'linkcheck.level6' has kitControl's base value and its fr translation\n",
		"selftest: all assertions passed (5 skipped: their index or text source is absent or empty).\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if code != 0 || errs != "" {
		t.Errorf("code %d stderr %q stdout:\n%s", code, errs, out)
	}
}

// TestSelftestFactFails: a fact whose index holds the wrong answer is a FAIL on stderr
// and exit 1, not a SKIP: only an absent or empty index skips.
func TestSelftestFactFails(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	wrong := `{"ackAlarm": [{"class": "Other", "package": "p", "section": "methods", "signature": "void ackAlarm()"}]}`
	if err := os.WriteFile(filepath.Join(checkout, "indexes", "method-index.json"), []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, checkout, "--niagara-version", "1.0", "selftest")
	want := "  FAIL  method: expected: method 'ackAlarm' lists javax.baja.alarm.BAlarmRecord.ackAlarm()\n"
	if code != 1 || errs != want || strings.Contains(out, "all assertions passed") {
		t.Errorf("code %d stderr %q stdout:\n%s", code, errs, out)
	}
}

// TestSelftestInstallFactsFail: an install-built index that lost its fact
// fails that fact: kitControl-rt without its baja dependency, a palette
// whose BooleanLatch is another module's, an hs:ahu outside haystack-rt's
// tags.csv, a linkcheck.level6 without its French translation.
func TestSelftestInstallFactsFail(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, tt := range []struct{ index, wrong, want string }{
		{"module-index.json", `{"modules": {"kitControl-rt": {"name": "kitControl-rt", "dependencies": [{"name": "control-rt"}]}}}`,
			"  FAIL  module: expected: module 'kitControl-rt' lists its dependency on baja\n"},
		{"palette-index.json", `{"palettes": [{"module": "other-rt", "path": "module.palette", "tree": [{"name": "BooleanLatch", "type": "other:BooleanLatch"}]}]}`,
			"  FAIL  palette: expected: palette 'BooleanLatch' finds it in kitControl-rt's module.palette\n"},
		{"tag-index.json", `{"files": [], "tags": [{"dict": "hs", "name": "ahu", "file": "other-rt.jar!data/tags.csv", "kind": "Marker"}]}`,
			"  FAIL  tag: expected: tag 'hs:ahu' is a Marker in haystack-rt's tags.csv\n"},
		{"lexicon-index.json", `{"files": [{"jar": "kitControl-rt.jar", "path": "kitControl-rt.lexicon", "module": "kitControl",
			"language": "", "overrides": "", "entries": [["linkcheck.level6", "Level 6"]]}]}`,
			"  FAIL  lexicon-index: expected: lexicon 'linkcheck.level6' has kitControl's base value and its fr translation\n"},
	} {
		checkout := ftsfixture.Checkout(t, testdata, false)
		if err := os.WriteFile(filepath.Join(checkout, "indexes", tt.index), []byte(tt.wrong), 0o644); err != nil {
			t.Fatal(err)
		}
		out, errs, code := run(t, checkout, "--niagara-version", "1.0", "selftest")
		if code != 1 || errs != tt.want {
			t.Errorf("%s: code %d stderr %q stdout:\n%s", tt.index, code, errs, out)
		}
	}
}
