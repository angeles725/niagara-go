package helpbuild

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The frozen expectations of the tests that compared this package with the
// niagara-help Python reference until D47 retired it: testdata/frozen/<name>.txt
// holds, one per line, what the Go side printed on the synthetic fixture when
// the Python-vs-Go parity test last passed, so each file is the Python output
// of that fixture. Paths are relabeled (<base>, <dir>, ...) and trees are
// listed by treeLines. A deliberate change regenerates them from Go:
//
//	go test ./internal/helpbuild -run <Test> -update-frozen
//
// then review the diff and name each intended change in the commit message.

var updateFrozen = flag.Bool("update-frozen", false, "rewrite testdata/frozen/ from the Go side (review the diff)")

// frozenOnly skips t off Linux: the frozen files are a POSIX run's output
// (separators, permission bits, CRLF-free text), as the parity tests were.
func frozenOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("the frozen expectations are a Linux run's output")
	}
}

// relabeled is lines with each path of pairs (path, label, ...) shown as its
// label; list the longer paths first.
func relabeled(lines []string, pairs ...string) []string {
	r := strings.NewReplacer(pairs...)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = r.Replace(l)
	}
	return out
}

// frozenTemp matches a t.TempDir() path ($TMPDIR/<test name><digits>/<seq>)
// that no relabel named: checkFrozen shows it as <tmp>.
var frozenTemp = regexp.MustCompile(regexp.QuoteMeta(os.TempDir()) + `/[^/"]+/[0-9]+`)

// checkFrozen compares got (an element may hold several lines; a temporary
// directory left in it shows as <tmp>) with testdata/frozen/<name>.txt, or
// rewrites the file under -update-frozen.
func checkFrozen(t *testing.T, name string, got []string) {
	t.Helper()
	path := filepath.Join("testdata", "frozen", filepath.FromSlash(name)+".txt")
	text := frozenTemp.ReplaceAllString(strings.Join(got, "\n"), "<tmp>")
	if *updateFrozen {
		writeFile(t, path, []byte(text+"\n"))
		return
	}
	got = strings.Split(text, "\n")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (a new case is written with -update-frozen)", name, err)
	}
	diffLines(t, name, strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"), got)
}
