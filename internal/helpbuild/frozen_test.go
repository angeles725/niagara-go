package helpbuild

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/difflib"
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

// frozenName is s as a portable file name: each run of characters other
// than ASCII letters, digits, "." and "_" becomes one "-".
func frozenName(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		if r < 0x80 && (r == '.' || r == '_' || '0' <= r && r <= '9' || 'a' <= r|0x20 && r|0x20 <= 'z') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
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

// frozenRoot is testdata/frozen, absolute: read at init, while the working
// directory is the package's, so a test may change directory.
var frozenRoot = func() string {
	p, err := filepath.Abs(filepath.Join("testdata", "frozen"))
	if err != nil {
		panic(err)
	}
	return p
}()

// frozenTemp matches a t.TempDir() path ($TMPDIR/<test name><digits>/<seq>)
// that no relabel named: checkFrozen shows it as <tmp>.
var frozenTemp = regexp.MustCompile(regexp.QuoteMeta(os.TempDir()) + `/[^/"]+/[0-9]+`)

// checkFrozen compares got (an element may hold several lines; a temporary
// directory left in it shows as <tmp>) with testdata/frozen/<name>.txt, or
// rewrites the file under -update-frozen.
func checkFrozen(t *testing.T, name string, got []string) {
	t.Helper()
	path := filepath.Join(frozenRoot, filepath.FromSlash(name)+".txt")
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

// treeDelta lists how the tree listing cur differs from prev (treeLines
// output): "+ " before each entry that is new or changed, "- " before each
// entry that is gone. A frozen step records the delta of what it wrote, so
// the listing of the fixture it starts from (built by the test) is not
// stored again for every step.
func treeDelta(prev, cur []string) []string {
	had := map[string]bool{}
	for _, l := range prev {
		had[l] = true
	}
	has := map[string]bool{}
	var out []string
	for _, l := range cur {
		has[l] = true
		if !had[l] {
			out = append(out, "+ "+l)
		}
	}
	for _, l := range prev {
		if !has[l] {
			out = append(out, "- "+l)
		}
	}
	return out
}

// around is a, cut to 160 bytes around its first difference from b (jar
// lines of a real install are megabytes long).
func around(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo, hi := i-80, i+80
	if lo < 0 {
		lo = 0
	}
	if hi > len(a) {
		hi = len(a)
	}
	return fmt.Sprintf("...[%d]%q...", lo, a[lo:hi])
}

// diffLines fails t when got differs from want, showing the first ten
// differing lines around their first difference.
func diffLines(t *testing.T, what string, want, got []string) {
	t.Helper()
	n := max(len(want), len(got))
	bad := 0
	for i := 0; i < n; i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			bad++
			if bad <= 10 {
				t.Errorf("%s line %d:\n want %s\n  got %s", what, i+1, around(w, g), around(g, w))
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%s: %d of %d lines differ (want %d, got %d)", what, bad, n, len(want), len(got))
	}
	t.Logf("%s: %d lines identical", what, n)
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// lineDelta is how the line sequence cur differs from prev, as hunks with
// no context: "@@ i1,i2 j1,j2" (prev[i1:i2] becomes cur[j1:j2]), then "- "
// before each line of prev[i1:i2] and "+ " before each of cur[j1:j2]
// (difflib's matching blocks, without autojunk). Unlike treeDelta it keeps
// the order and repeated lines, so it fits observations that are not tree
// listings.
func lineDelta(prev, cur []string) []string {
	var out []string
	i, j := 0, 0
	for _, m := range difflib.New(nil, prev, cur, false).GetMatchingBlocks() {
		if i < m.A || j < m.B {
			out = append(out, fmt.Sprintf("@@ %d,%d %d,%d", i, m.A, j, m.B))
			for _, l := range prev[i:m.A] {
				out = append(out, "- "+l)
			}
			for _, l := range cur[j:m.B] {
				out = append(out, "+ "+l)
			}
		}
		i, j = m.A+m.Size, m.B+m.Size
	}
	return out
}
