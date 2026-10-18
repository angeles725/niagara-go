package searchdoor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
)

// In-process backends (decisions D29, D30): the blocks source (corpus-nav find),
// the docs source (niagara_help find) and the code source's token, grep and rank
// calls are answered by the libraries the Go ports share, producing the exact text the
// backend CLI prints, so parse.go and the goldens see the same rows without a
// process start. Every case the library cannot answer exactly (no data, a stale
// full-text index (D21), a miss whose text belongs to a printing package, an
// argv shape this file does not know) runs the backend through exec as before:
// the exec path stays the parity oracle and NSEARCH_EXEC=1 forces it.

// ExecEnv names the variable that forces every backend through exec.
const ExecEnv = "NSEARCH_EXEC"

// blocksCacheTag keeps nsearch's corpus-nav find cache apart from corpus-nav's
// own (its fingerprint holds the executable's stat).
const blocksCacheTag = ".nsearch"

// inprocBackend answers the calls it knows in process and hands the rest to exec.
// paths are the backend scripts it answers for (bindPaths); pick chooses the
// in-process answer of a call (route; tests replace it).
type inprocBackend struct {
	exec  Backend
	paths Paths
	clock func() time.Time
	pick  func(Paths, []string) func([]string) answer
}

// pathBinder is a Backend that must know the backend scripts of the question.
type pathBinder interface{ bindPaths(Paths) }

func (b *inprocBackend) bindPaths(p Paths) { b.paths = p }

// answer is one in-process run: what the backend would have printed, or ok
// false when only exec can answer exactly.
type answer struct {
	rc             int
	stdout, stderr string
	ok             bool
}

// Run answers argv in process when it can, else through exec. A call that falls
// back after trying in process gives exec only the budget it has left, so one
// call never spends more than timeout seconds.
func (b *inprocBackend) Run(argv []string, timeout float64) Raw {
	pick := b.pick
	if pick == nil {
		pick = route
	}
	run := pick(b.paths, argv)
	if run == nil || !isFile(argv[0]) { // a missing script is exec's "tool not found"
		return b.exec.Run(argv, timeout)
	}
	t0 := b.clock()
	a, done := bounded(func() answer { return run(argv) }, timeout)
	left := timeout - b.clock().Sub(t0).Seconds()
	switch {
	case !done || (!a.ok && left <= 0):
		return timedOut(timeout)
	case !a.ok:
		raw := b.exec.Run(argv, left)
		if raw.State == Unavailable && raw.Reason == timedOut(left).Reason {
			return timedOut(timeout) // name the call's budget, as the Python door does
		}
		return raw
	}
	return Classify(a.rc, a.stdout, a.stderr, int(b.clock().Sub(t0)/time.Millisecond))
}

// timedOut is the Raw of a call killed at its timeout.
func timedOut(timeout float64) Raw {
	return Raw{State: Unavailable, Reason: "unavailable: timed out after " + formatG3(timeout) + "s",
		MS: int(timeout * 1000)}
}

// route picks the in-process answer for a backend call, or nil for exec.
func route(p Paths, argv []string) func([]string) answer {
	switch {
	case len(argv) < 2:
	case argv[0] == p.Blocks:
		return blocksFind
	case argv[0] == p.Docs:
		return docsFind
	case argv[0] == p.Code && argv[1] == "token":
		return codeToken
	case argv[0] == p.Code && argv[1] == "grep":
		return codeGrep
	case argv[0] == p.Code && argv[1] == "rank":
		return codeRank
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

var viaExec = answer{}

// shape matches argv[1:] against a fixed command line: a literal word, "*" for one
// positional that must not look like an option (argparse would read it as one),
// "#" for a positive integer. opt lists the optional trailing flags.
func shape(args []string, want []string, opt ...string) (vals []string, flag string, ok bool) {
	if len(args) < len(want) || len(args) > len(want)+1 {
		return nil, "", false
	}
	for i, w := range want {
		a := args[i]
		switch w {
		case "*":
			if a == "" || strings.HasPrefix(a, "-") {
				return nil, "", false
			}
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
// parent of its tools/ folder; a folder without CATALOG.md is left to exec.
func blocksFind(argv []string) answer {
	v, flag, ok := shape(argv[1:], []string{"find", "*", "--limit", "#"}, "--and", "--or")
	if !ok {
		return viaExec
	}
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		root = toolDir(argv[0], true)
	}
	if root == "" || !isFile(filepath.Join(root, "CATALOG.md")) {
		return viaExec
	}
	limit, _ := strconv.Atoi(v[1])
	c := blocksearch.New(root, os.Getenv("CORPUS_NAV_INDEX_DIR"))
	c.CacheTag = blocksCacheTag
	r := c.Find(blocksearch.Query{Text: v[0], Scope: "blocks", Limit: limit, RequireAll: flag == "--and"})
	return exact(r.Exit(), r.Stdout(), r.Stderr())
}
