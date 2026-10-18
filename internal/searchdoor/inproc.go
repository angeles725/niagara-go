package searchdoor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
)

// The backends, all in process (decisions D29, D30, D38, D42): the blocks source
// (corpus-nav find), the docs source (niagara_help find) and the code source's
// token, grep and rank calls are answered by the libraries the Go ports share,
// and every other module_nav call by mnav's own command (nav.RunAt), producing
// the exact text the backend CLI prints, so parse.go and the goldens see the
// same rows without a process start. Nothing runs Python: the script paths only
// locate each backend's checkout.

var processStart = time.Now()

// testBackend, when set (tests only), answers every backend call of newDoor: the
// hermetic goldens' fake backends (fake_test.go).
var testBackend Backend

// newDoor is the front door of the binaries: the in-process backends (door.bind
// then gives them the script paths they answer for).
func newDoor() *door {
	b := testBackend
	if b == nil {
		b = &inprocBackend{clock: time.Now}
	}
	return &door{backend: b, now: func() float64 { return time.Since(processStart).Seconds() }}
}

// blocksCacheTag keeps nsearch's corpus-nav find cache apart from corpus-nav's
// own (its fingerprint holds the executable's stat).
const blocksCacheTag = ".nsearch"

// inprocBackend answers the backend calls in process. paths are the backend
// scripts it answers for (bindPaths); pick chooses the in-process answer of a
// call (route; tests replace it).
type inprocBackend struct {
	paths Paths
	clock func() time.Time
	pick  func(Paths, []string) func([]string) answer
}

// pathBinder is a Backend that must know the backend scripts of the question.
type pathBinder interface{ bindPaths(Paths) }

func (b *inprocBackend) bindPaths(p Paths) { b.paths = p }

// answer is one in-process run: what the backend would have printed, or ok
// false for a call this package has no answer for (noAnswer).
type answer struct {
	rc             int
	stdout, stderr string
	ok             bool
}

// Run answers argv in process within timeout seconds. A script path that is
// not a file is the Python door's "tool not found"; a call no source knows (an
// argv the door never sends) is typed UNAVAILABLE, never a silent zero.
func (b *inprocBackend) Run(argv []string, timeout float64) Raw {
	if !isFile(argv[0]) {
		return Raw{State: Unavailable, Reason: "unavailable: tool not found at " + pyPath(argv[0])}
	}
	if timeout <= 0 { // subprocess.run(timeout=0) never lets the backend answer
		return timedOut(timeout)
	}
	pick := b.pick
	if pick == nil {
		pick = route
	}
	run := pick(b.paths, argv)
	if run == nil {
		return unknownCall(argv)
	}
	t0 := b.clock()
	a, done := bounded(func() answer { return run(argv) }, timeout)
	switch {
	case !done:
		return timedOut(timeout)
	case !a.ok:
		return unknownCall(argv)
	}
	return Classify(a.rc, a.stdout, a.stderr, int(b.clock().Sub(t0)/time.Millisecond))
}

// unknownCall is the Raw of a backend call no in-process source answers.
func unknownCall(argv []string) Raw {
	return Raw{State: Unavailable, Reason: "unavailable: no in-process answer for `" +
		strings.Join(append([]string{filepath.Base(argv[0])}, argv[1:]...), " ") + "` (a call nsearch never makes)"}
}

// timedOut is the Raw of a call killed at its timeout.
func timedOut(timeout float64) Raw {
	return Raw{State: Unavailable, Reason: "unavailable: timed out after " + formatG3(timeout) + "s",
		MS: int(timeout * 1000)}
}

// route picks the in-process answer for a backend call, or nil when none does.
func route(p Paths, argv []string) func([]string) answer {
	switch {
	case len(argv) < 2:
	case argv[0] == p.Blocks:
		return blocksFind
	case argv[0] == p.Docs:
		return docsFind
	case argv[0] == p.Code && argv[1] == "token":
		return orNav(codeToken)
	case argv[0] == p.Code && argv[1] == "grep":
		return orNav(codeGrep)
	case argv[0] == p.Code && argv[1] == "rank":
		return orNav(codeRank)
	case argv[0] == p.Code:
		return navRun
	}
	return nil
}

// bounded runs run under the backend timeout (seconds). Past it the library keeps
// running and its answer is dropped, as a killed process's output would be. A
// panic in run is raised again in the caller's goroutine, where the source guard
// (guarded, sub) types it UNAVAILABLE; one past the timeout is dropped with the
// answer, so a library fault never ends the process.
func bounded(run func() answer, timeout float64) (answer, bool) {
	type result struct {
		a        answer
		panicked bool
		value    any
	}
	done := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			if v := recover(); v != nil {
				r = result{panicked: true, value: v}
			}
			done <- r
		}()
		r.a = run()
	}()
	timer := time.NewTimer(time.Duration(timeout * float64(time.Second)))
	defer timer.Stop()
	select {
	case r := <-done:
		if r.panicked {
			panic(r.value)
		}
		return r.a, true
	case <-timer.C:
		return answer{}, false
	}
}

func exact(rc int, stdout, stderr string) answer { return answer{rc, stdout, stderr, true} }

// noAnswer is the answer of a call the source does not know.
var noAnswer = answer{}

// shape matches argv[1:] against a fixed command line: a literal word, "*" for one
// positional (any text: a query that starts with "-" is searched as text, where
// the Python backends' argparse read it as an option and failed), "#" for a
// positive integer. opt lists the optional trailing flags.
func shape(args []string, want []string, opt ...string) (vals []string, flag string, ok bool) {
	if len(args) < len(want) || len(args) > len(want)+1 {
		return nil, "", false
	}
	for i, w := range want {
		a := args[i]
		switch w {
		case "*":
			vals = append(vals, a)
		case "#":
			if n, err := strconv.Atoi(a); err != nil || n < 1 || strconv.Itoa(n) != a {
				return nil, "", false
			}
			vals = append(vals, a)
		default:
			if a != w {
				return nil, "", false
			}
		}
	}
	if len(args) == len(want) {
		return vals, "", true
	}
	for _, o := range opt {
		if args[len(want)] == o {
			return vals, o, true
		}
	}
	return nil, "", false
}

// toolDir is the folder above the backend script's own folder (tools/..), as the
// Python tools find their checkout from __file__.
func toolDir(script string, resolve bool) string {
	dir, err := filepath.Abs(filepath.Dir(script))
	if err != nil {
		return ""
	}
	if resolve {
		if dir, err = filepath.EvalSymlinks(dir); err != nil {
			return ""
		}
	}
	return filepath.Dir(dir)
}

func isFile(p string) bool { st, err := os.Stat(p); return err == nil && st.Mode().IsRegular() }

func isDir(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

// blocksFind is `corpus-nav find <query> --limit N [--and|--or]` (blocksJob) through
// blocksearch, on the corpus corpus-nav.py would use: $CORPUS_NAV_ROOT, else the
// parent of its tools/ folder (a folder without CATALOG.md is searched like
// corpus-nav does: no block titles).
func blocksFind(argv []string) answer {
	v, flag, ok := shape(argv[1:], []string{"find", "*", "--limit", "#"}, "--and", "--or")
	if !ok {
		return noAnswer
	}
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		root = toolDir(argv[0], true)
	}
	if root == "" {
		return noAnswer
	}
	limit, _ := strconv.Atoi(v[1])
	c := blocksearch.New(root, os.Getenv("CORPUS_NAV_INDEX_DIR"))
	c.CacheTag = blocksCacheTag
	r := c.Find(blocksearch.Query{Text: v[0], Scope: "blocks", Limit: limit, RequireAll: flag == "--and"})
	return exact(r.Exit(), r.Stdout(), r.Stderr())
}
