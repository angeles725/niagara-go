package indexes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The S8 index builders (class-index.json + jdk-class-index.json,
// inheritance.json, method-index.json) on the corners a checked-in fixture
// cannot hold, against their frozen Python output (frozen_test.go): the
// printed lines (stdout, then stderr; the root shown as <base>), the exit
// status and indexes/ after each builder, and the whole tree after the last.

// s8Modes are the S8 modes, in the order rebuild-all runs the builders.
var s8Modes = []string{"class", "inherit", "method"}

// goIndexBuild runs the Go builder of mode.
func goIndexBuild(mode, base string) (*IndexResult, error) {
	switch mode {
	case "class":
		return BuildClassIndex(base)
	case "inherit":
		return BuildInheritance(base)
	case "method":
		return BuildMethodIndex(base)
	}
	if build := s9Builders[mode]; build != nil {
		return build(base)
	}
	return nil, fmt.Errorf("unknown mode %s", mode)
}

// goIndexRun is a builder run as lines: ["out", line] per log line, ["err",
// line] per stderr line, then ["exit", status] or ["raise"].
func goIndexRun(mode, base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := goIndexBuild(mode, base)
	took := time.Since(start)
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(l)))
		}
		for _, l := range res.Stderr {
			out = append(out, testkit.DumpLine(testkit.Str("err"), testkit.Str(l)))
		}
	}
	if err != nil {
		return append(out, testkit.DumpLine(testkit.Str("raise"))), took
	}
	return append(out, testkit.DumpLine(testkit.Str("exit"), testkit.IntV(res.Status))), took
}

// relabelIndex shows base as <base>.
func relabelIndex(lines []string, base string) []string {
	return testkit.Relabeled(lines, base, "<base>")
}

// TestS8CornersFrozen: every builder on the checked-in fixture, then on the
// corners a fixture cannot hold (s8Corners, s8CleanCorners), and on roots
// without the folders each builder reads.
func TestS8CornersFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "s8", "root"), base)
	for _, mode := range s8Modes { // the fixture's own output is the golden of checkS8Golden
		goIndexRun(mode, base)
	}
	s8Corners(t, base)
	s8CleanCorners(t, base)
	for _, mode := range s8Modes {
		indexFrozen(t, "s8/corners", mode, base)
	}
	treeFrozen(t, "s8/corners", base)
	for _, c := range []struct{ name, keep string }{
		{"empty-root", ""},
		{"only-bajadoc-clean", "bajadoc-clean"},
		{"only-devguide", "devguide"},
		{"only-jdk", "jdk"},
	} {
		b := filepath.Join(t.TempDir(), "go")
		if c.keep == "" {
			testkit.WriteFile(t, filepath.Join(b, "readme.txt"), nil)
		} else {
			testkit.CopyTree(t, filepath.Join("testdata", "s8", "root", c.keep), filepath.Join(b, c.keep))
		}
		for _, mode := range s8Modes {
			indexFrozen(t, "s8/"+c.name, mode, b)
		}
		treeFrozen(t, "s8/"+c.name, b)
	}
}

// s8Corners adds to a copy of the fixture what a checked-in fixture cannot
// hold for the class index: a symlinked folder (listed, not entered) and a
// symlinked .bajadoc under devguide/, the javadoc list as a symlink.
func s8Corners(t *testing.T, base string) {
	t.Helper()
	dev := filepath.Join(base, "devguide", "zz")
	testkit.WriteFile(t, filepath.Join(dev, "real", "BLinked.bajadoc"), []byte(`<bajadoc><class qualifiedName="zz.BLinked" name="BLinked" packageName="zz"/></bajadoc>`))
	for link, target := range map[string]string{"alias": "real", "Alias.bajadoc": "real/BLinked.bajadoc"} {
		if err := os.Symlink(target, filepath.Join(dev, link)); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(base, "bajadoc", "allclasses-noframe.html")
	if err := os.Rename(list, list+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("allclasses-noframe.html.real", list); err != nil {
		t.Fatal(err)
	}
}

// s8CleanCorners adds bajadoc-clean/ pages that are not UTF-8 (in the
// header, after it in the same decode chunk, past it, a sequence cut by a
// chunk end, a bad continuation after the cut, the blank line's "\r" at a
// chunk end, a truncated end, a header line longer than a chunk) and a
// symlinked folder.
func s8CleanCorners(t *testing.T, base string) {
	t.Helper()
	clean := filepath.Join(base, "bajadoc-clean", "zz")
	head := "CLASS: BBad\nPACKAGE: zz\n\n"         // a whole header
	open := "CLASS: BOpen\nPACKAGE: zz\nNOTE: "    // a header still open
	fill := func(prefix string, room int) string { // prefix padded to room bytes
		return prefix + strings.Repeat("x", room-len(prefix))
	}
	for name, data := range map[string]string{
		"InHeader.txt":    "CLASS: BIn\xffHeader\nPACKAGE: zz\n",
		"AfterHeader.txt": head + "DESCRIPTION:\n\xff\n",              // same decode chunk: still an error
		"PastChunk.txt":   fill(head, textChunk) + "\xff\n",           // next chunk: never read
		"CutAtChunk.txt":  fill(open, textChunk-1) + "\u20ac\xff\n",   // the euro sign spans the chunks
		"CutBad.txt":      fill(open, textChunk-1) + "\xe2\x28\xac\n", // a bad continuation after the cut
		"CrAtChunk.txt":   fill(open, textChunk-2) + "\r\r\xff",       // the blank line's "\r" waits for the next chunk
		"Truncated.txt":   "CLASS: BTrunc\nPACKAGE: zz\nEXTENDS: BObject\n\xe2\x82",
		"LongLine.txt":    "CLASS: BLong\nPACKAGE: zz\nIMPLEMENTS: " + strings.Repeat("BIa, ", 4000) + "BIz\n\n\xff",
	} {
		testkit.WriteFile(t, filepath.Join(clean, name), []byte(data))
	}
	if err := os.Symlink("../javax", filepath.Join(clean, "linked")); err != nil {
		t.Fatal(err)
	}
}

// A .bajadoc or a bajadoc-clean/ page that cannot be read: class-index and
// method-index stop (Python raised), inheritance lists it under Errors.
func TestS8UnreadableFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "s8", "root"), base)
	lockFiles(t, filepath.Join(base, "devguide", "baja", "javax", "baja", "sys", "BIObject.bajadoc"),
		filepath.Join(base, "bajadoc-clean", "javax", "baja", "sys", "BMode.txt"))
	if err := os.Symlink("missing.bajadoc", filepath.Join(base, "jdk", "java", "Dangling.bajadoc")); err != nil {
		t.Fatal(err)
	}
	for _, mode := range s8Modes {
		lines, _ := goIndexRun(mode, base)
		testkit.CheckFrozen(t, "s8/unreadable/"+mode, relabelIndex(lines, base))
	}
}
