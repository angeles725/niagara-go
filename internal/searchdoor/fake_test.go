package searchdoor

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// scriptFakes is the Go port of the fake backend scripts the frozen nsearch goldens were
// recorded with (niagara-research tests/searchdoor fakes, copied once to testdata/fake): the
// goldens test the front door (fan-out, parsing, rendering, budgets) against canned backend
// answers, so they need no Python and no corpus. Each call is answered by the fake of its
// source (the scripts corpus-nav.py, niagara_help.py, module_nav.py). A script path that
// does not exist is the backends' "tool not found"; a word that made a fake sleep 10 s sleeps
// until the call's budget and is then timed out, like the killed process.
type scriptFakes struct{}

// fakeSleep is the fakes' time.sleep(10).
const fakeSleep = 10.0

func (scriptFakes) Run(src source, argv []string, timeout float64) Raw {
	if _, err := os.Stat(argv[0]); err != nil {
		return Raw{State: Unavailable, Reason: "unavailable: tool not found at " + pyPath(argv[0])}
	}
	if timeout <= 0 {
		return timedOut(timeout)
	}
	f := &fakeRun{timeout: timeout}
	switch src {
	case blocksSource:
		f.corpusNav(argv[1:])
	case docsSource:
		f.niagaraHelp(argv[1:])
	case codeSource:
		f.moduleNav(argv[1:])
	default:
		panic(fmt.Sprintf("scriptFakes: unknown source %d (%s)", src, argv[0]))
	}
	if f.timedOut {
		return timedOut(timeout)
	}
	return Classify(f.rc, f.out.String(), f.err.String(), 0)
}

// fakeRun is one fake process: its stdout, stderr and exit status.
type fakeRun struct {
	timeout  float64
	out, err strings.Builder
	rc       int
	timedOut bool
	done     bool // sys.exit was called: nothing more is printed
}

func (f *fakeRun) print(format string, a ...any) {
	if !f.done {
		fmt.Fprintf(&f.out, format+"\n", a...)
	}
}

func (f *fakeRun) exit(rc int) {
	if !f.done {
		f.rc, f.done = rc, true
	}
}

// sleep is time.sleep(10) under the call's timeout: past it the process is killed.
func (f *fakeRun) sleep() {
	if f.timeout < fakeSleep {
		time.Sleep(time.Duration(f.timeout * float64(time.Second)))
		f.timedOut, f.done = true, true
		return
	}
	time.Sleep(fakeSleep * time.Second)
}

// opt is the fakes' opt(): the value after name, or def.
func opt(args []string, name, def string) string {
	if i := slices.Index(args, name); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return def
}

func mode(args []string) string {
	switch {
	case slices.Contains(args, "--and"):
		return "and"
	case slices.Contains(args, "--or"):
		return "or"
	}
	return "default"
}

func anyZZ(words []string) bool {
	return slices.ContainsFunc(words, func(w string) bool { return strings.HasPrefix(w, "zz") })
}

func (f *fakeRun) corpusNav(args []string) {
	query := args[1]
	words := pycompat.Fields(pycompat.Lower(query))
	limit, m := opt(args, "--limit", "50"), mode(args)
	if slices.Contains(words, "blocksslow") {
		f.sleep()
	}
	switch {
	case slices.Contains(words, "blocksdown"):
		if !f.done {
			f.err.WriteString("Traceback (most recent call last):\nRuntimeError: block index is corrupt\n")
		}
		f.exit(2)
	case slices.Contains(words, "blockscrash"):
		if !f.done {
			f.err.WriteString("ValueError: unexpected block header\n")
		}
		f.exit(1)
	case slices.Contains(words, "blocksambig"):
		f.exit(0)
	case slices.Contains(words, "blocksmiss0"):
		f.print("(no matches; searched 9 blocks for %s)", pycompat.StrRepr(query))
		f.exit(0)
	case anyZZ(words):
		f.print("(no matches; searched 9 blocks for %s)", pycompat.StrRepr(query))
		f.exit(1)
	case slices.Contains(words, "blocksgarbage"):
		f.print("block index v0 dump")
		f.print("  row one without a header")
		f.exit(0)
	}
	if f.done {
		return
	}
	n := map[string]int{"and": 2, "or": 7}[m]
	if n == 0 {
		n = 4
	}
	for i := range n {
		word := words[i%len(words)]
		f.print("B%d · Block %d — %s research (%s mode, --limit %s)", 101+i, 101+i, word, m, limit)
		f.print("    %d: > a line about %s in block %d", 12+i, word, 101+i)
		f.print("    %d: a second line", 40+i)
	}
	f.exit(0)
}

func (f *fakeRun) niagaraHelp(args []string) {
	version := ""
	if len(args) > 0 && args[0] == "--niagara-version" {
		version, args = args[1], args[2:]
	}
	query := args[1]
	words := pycompat.Fields(pycompat.Lower(query))
	n, _ := strconv.Atoi(opt(args, "-n", "10"))
	m := mode(args)
	if slices.Contains(words, "docsslow") {
		f.sleep()
	}
	switch {
	case slices.Contains(words, "docsdown"):
		if !f.done {
			f.err.WriteString("niagara_help: no search index for this version\n")
		}
		f.exit(2)
	case anyZZ(words):
		f.print("No match for %s in this scope.", pycompat.StrRepr(query))
		f.print("  (searched 31 files of docs 4.14.0.162)")
		f.exit(1)
	}
	if f.done {
		return
	}
	total := 3 + len(words)
	shown := min(n, total)
	if version == "" {
		version = "default"
	}
	f.print("%d files matching %s (docs %s, %s mode):", total, pycompat.StrRepr(query), version, m)
	if slices.Contains(words, "relaxed") {
		f.print("  (no file has every term; showing files with any term)")
	}
	f.print("by source: guides-clean 3, devguide-clean 2")
	for i := range shown {
		word := fakeTitle(words[i%len(words)])
		f.print("  [guides-clean] %s guide, page %d", word, i+1)
		f.print("    L%d: how the %s setting works", 10+i, words[i%len(words)])
		if i < shown-1 {
			f.print("    path: guides-clean/%s/page%d.html", word, i+1)
		}
	}
	if total > shown {
		f.print("  ... and %d more files", total-shown)
	}
	f.exit(0)
}

// fakeTitle is Python's str.title() for the fakes' word inputs: a letter
// after a non-letter is upper-cased, every other letter lower-cased.
func fakeTitle(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range s {
		cased := unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r)
		switch {
		case cased && !prev:
			b.WriteRune(unicode.ToTitle(r))
		case cased:
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
		prev = cased
	}
	return b.String()
}
