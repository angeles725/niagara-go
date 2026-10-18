// Package reindex is `mnav reindex`, the Go port of module-navigator's
// reindex.sh: it rebuilds the module-navigator indexes that kept commands
// read (D32/D33) with the internal/navbuild builders, in dependency order,
// backing up the current indexes, validating every output (no error, at least
// 10 KiB, not shrunk below half of a previous index over 100 KiB), rolling a
// bad one back from the backup, and aborting when the inventory or the class
// index fails (every later builder reads them).
//
// Differences from reindex.sh: the builders run in-process, so no python3 or
// tools/ preflight; only the seven kept indexes are built (string, field,
// exceptions, swing and annotations are retired: `stats` shows them as
// missing); a builder's log holds its result summary or error, not the Python
// builders' progress output; the base directory is --base-dir, else the
// module-navigator checkout found from the working directory or the
// executable (D24).
package reindex

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/navbuild"
)

// Builder is one step of the rebuild: its reindex.sh name (the Python
// builder it replaces, which --only accepts as well as the index name), the
// index it writes and the build, which returns its log summary.
type Builder struct {
	Name, Index string
	Build       func(base, organized string) (string, error)
}

// Builders are the steps in dependency order: the inventory (the submodule
// list), the class index (read by every later builder), inheritance (read by
// the call graph), then the rest.
var Builders = []Builder{
	{"build_module_inventory.py", navbuild.InventoryFile, func(base, org string) (string, error) {
		inv, err := navbuild.BuildInventory(base, org)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d submodules", len(inv.Submodules)), nil
	}},
	{"build_class_index.py", navbuild.ClassIndexFile, func(base, org string) (string, error) {
		ci, err := navbuild.BuildClassIndex(base, org)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d files, %d unique names", ci.Stats.FilesProcessed, len(ci.Names)), nil
	}},
	{"build_inheritance.py", navbuild.InheritanceFile, func(base, _ string) (string, error) {
		in, err := navbuild.BuildInheritance(base)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d top-level types, %d with chains", in.TopLevel, len(in.ChainNames)), nil
	}},
	{"build_xref.py", navbuild.XrefFile, func(base, org string) (string, error) {
		_, err := navbuild.BuildXref(base, org)
		return "built", err
	}},
	{"build_token_index.py", navbuild.TokenFile, func(base, org string) (string, error) {
		ti, err := navbuild.BuildTokenIndex(base, org, navbuild.DefaultMinTokenLen)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d files, %d postings, %d unique tokens", ti.FilesScanned, ti.TotalPostings, ti.UniqueTokens), nil
	}},
	{"build_method_index.py", navbuild.MethodFile, func(base, org string) (string, error) {
		mi, err := navbuild.BuildMethodIndex(base, org)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d files parsed, %d failed, %d method names", mi.FilesParsed, mi.FilesFailed, len(mi.Names)), nil
	}},
	{"build_callgraph_index.py", navbuild.CallgraphFile, func(base, org string) (string, error) {
		cg, err := navbuild.BuildCallgraph(base, org)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d files parsed, %d failed, %d callers", cg.FilesParsed, cg.FilesFailed, len(cg.Calls)), nil
	}},
}

// Validation thresholds of run_one, replaceable by the tests.
var (
	minSize       int64 = 10240  // smaller is "tiny"
	shrinkMinPrev int64 = 102400 // the 50% shrink rule applies above this previous size
)

const usage = `usage: mnav reindex [--check] [--no-backup] [--only NAME] [--organized DIR] [--base-dir DIR]

Rebuild the module-navigator indexes in dependency order, with a backup,
validation and rollback (reindex.sh).

  --check           preflight only: validate the setup and print the plan
  --no-backup       skip the index backup (faster, no rollback)
  --only NAME       rebuild one builder (its reindex.sh name or its index)
  --organized DIR   index the corpus at DIR instead of NAV_CORPUS_BASE or ../organized
  --base-dir DIR    the module-navigator checkout (default: found from the
                    working directory or the executable)
`

type options struct {
	check, noBackup          bool
	only, organized, baseDir string
}

// parseArgs is reindex.sh's argument loop (plus --base-dir and --help).
func parseArgs(args []string, errw io.Writer) (opts options, code int, done bool) {
	value := func(i int, what string) (string, bool) {
		if i+1 >= len(args) || args[i+1] == "" {
			fmt.Fprintln(errw, what)
			return "", false
		}
		return args[i+1], true
	}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help":
			fmt.Fprint(errw, usage)
			return opts, cli.ExitFound, true
		case "--check":
			opts.check = true
		case "--no-backup":
			opts.noBackup = true
		case "--only":
			v, ok := value(i, "--only needs a builder name (e.g. build_token_index.py)")
			if !ok {
				return opts, cli.ExitCannotSearch, true
			}
			opts.only, i = v, i+1
		case "--organized":
			v, ok := value(i, "--organized needs a directory")
			if !ok {
				return opts, cli.ExitCannotSearch, true
			}
			if st, err := os.Stat(v); err != nil || !st.IsDir() {
				fmt.Fprintln(errw, "--organized: not a directory: "+v)
				return opts, cli.ExitCannotSearch, true
			}
			abs, err := filepath.Abs(v)
			if err != nil {
				fmt.Fprintln(errw, "--organized: "+err.Error())
				return opts, cli.ExitCannotSearch, true
			}
			opts.organized, i = abs, i+1
		case "--base-dir", "-d":
			v, ok := value(i, a+" needs a directory")
			if !ok {
				return opts, cli.ExitCannotSearch, true
			}
			opts.baseDir, i = v, i+1
		default:
			fmt.Fprintln(errw, "Unknown arg: "+a)
			return opts, cli.ExitCannotSearch, true
		}
	}
	return opts, 0, false
}

func red(w io.Writer, s string)    { fmt.Fprintf(w, "\033[31m%s\033[0m\n", s) }
func green(w io.Writer, s string)  { fmt.Fprintf(w, "\033[32m%s\033[0m\n", s) }
func yellow(w io.Writer, s string) { fmt.Fprintf(w, "\033[33m%s\033[0m\n", s) }

// sizeOf is size_of: a file's size, 0 when it does not exist.
func sizeOf(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

// IEC is `numfmt --to=iec n`: powers of 1024 with a K/M/G... suffix, one
// decimal below 10, rounded away from zero.
func IEC(n int64) string {
	val, power := float64(n), 0
	for val >= 1024 && power < 8 {
		val /= 1024
		power++
	}
	if power == 0 {
		return fmt.Sprint(n)
	}
	scale := 1.0
	if val < 10 {
		scale = 10
	}
	val = math.Ceil(val*scale) / scale
	if val >= 1024 {
		val /= 1024
		power++
	}
	suffix := string("KMGTPEZY"[power-1])
	if val < 10 {
		return fmt.Sprintf("%.1f%s", val, suffix)
	}
	return fmt.Sprintf("%.0f%s", val, suffix)
}

// Main runs `mnav reindex args...` and returns its exit code.
func Main(args []string, s cli.Stdio) int {
	opts, code, done := parseArgs(args, s.Err)
	if done {
		return code
	}
	nav := opts.baseDir
	if nav == "" {
		nav = core.FindNavDir()
	}
	if nav == "" {
		red(s.Out, "module-navigator checkout not found (no indexes/module-inventory.json above the working directory or the executable); use --base-dir DIR")
		return cli.ExitNoMatch
	}
	idx := filepath.Join(nav, "indexes")
	if st, err := os.Stat(idx); err != nil || !st.IsDir() {
		red(s.Out, "indexes/ not found at "+idx)
		return cli.ExitNoMatch
	}
	organized := opts.organized
	if organized == "" {
		organized = os.Getenv("NAV_CORPUS_BASE")
	}
	if organized == "" {
		organized = filepath.Join(filepath.Dir(nav), "organized")
	}
	if st, err := os.Stat(organized); err != nil || !st.IsDir() {
		red(s.Out, fmt.Sprintf("Corpus dir not found: '%s' (use --organized DIR or NAV_CORPUS_BASE)", organized))
		return cli.ExitNoMatch
	}
	green(s.Out, "Corpus: "+organized)
	if opts.only != "" && !known(opts.only) {
		red(s.Out, "--only: unknown builder '"+opts.only+"' (see --check for the list)")
		return cli.ExitCannotSearch
	}
	if opts.check {
		fmt.Fprintln(s.Out, "Plan (dependency order):")
		for i, b := range Builders {
			fmt.Fprintf(s.Out, "  %2d. %-28s -> %-22s (current: %s)\n", i+1, b.Name, b.Index, IEC(sizeOf(filepath.Join(idx, b.Index))))
		}
		green(s.Out, "Preflight OK. Run without --check to rebuild.")
		return cli.ExitFound
	}
	r := &run{out: s.Out, idx: idx, bak: filepath.Join(idx, ".reindex-bak"), logs: filepath.Join(idx, "logs"),
		backup: !opts.noBackup, nav: nav, organized: organized}
	if r.backup {
		if err := r.backupAll(); err != nil {
			red(s.Out, "backup failed: "+err.Error())
			return cli.ExitNoMatch
		}
	}
	fmt.Fprintln(s.Out, "Rebuilding indexes...")
	for _, b := range Builders {
		if opts.only != "" && b.Name != opts.only && b.Index != opts.only {
			continue
		}
		r.runOne(b)
		if r.fail && (b.Index == navbuild.InventoryFile || b.Index == navbuild.ClassIndexFile) {
			red(s.Out, "ABORT: "+b.Name+" failed; dependent builders not run.")
			return cli.ExitNoMatch
		}
	}
	fmt.Fprintln(s.Out)
	if r.fail {
		red(s.Out, "Re-index finished WITH ERRORS — see logs above. Broken indexes rolled back where possible.")
		return cli.ExitNoMatch
	}
	green(s.Out, "Re-index complete — all indexes rebuilt and validated.")
	return cli.ExitFound
}

func known(name string) bool {
	for _, b := range Builders {
		if b.Name == name || b.Index == name {
			return true
		}
	}
	return false
}

// run is one rebuild's state.
type run struct {
	out                      io.Writer
	nav, organized, idx, bak string
	logs                     string
	backup, fail             bool
}

// backupAll is the Backup section: a fresh .reindex-bak holding a copy (mode
// and mtime kept, as cp -p) of every index that exists.
func (r *run) backupAll() error {
	if err := os.RemoveAll(r.bak); err != nil {
		return err
	}
	if err := os.MkdirAll(r.bak, 0o777); err != nil {
		return err
	}
	yellow(r.out, "Backing up current indexes to "+r.bak+" ...")
	for _, b := range Builders {
		src := filepath.Join(r.idx, b.Index)
		if st, err := os.Stat(src); err == nil && st.Mode().IsRegular() {
			if err := copyFile(src, filepath.Join(r.bak, b.Index)); err != nil {
				return err
			}
		}
	}
	return nil
}

// runOne is run_one: build, validate, roll back a bad index.
func (r *run) runOne(b Builder) {
	out := filepath.Join(r.idx, b.Index)
	// The previous size: this run's backup, else the live index. With
	// --no-backup a .reindex-bak left by an earlier run is stale, so only the
	// live index counts (reindex.sh reads the stale backup).
	var prev int64
	if r.backup {
		prev = sizeOf(filepath.Join(r.bak, b.Index))
	}
	if prev == 0 {
		prev = sizeOf(out)
	}
	logPath := filepath.Join(r.logs, b.Name+".log")
	start := time.Now()
	summary, err := b.Build(r.nav, r.organized)
	secs := int(time.Since(start).Seconds())
	logText := b.Index + ": " + summary + "\n"
	if err != nil {
		logText = "ERROR: " + err.Error() + "\n"
	}
	if mkErr := os.MkdirAll(r.logs, 0o777); mkErr == nil {
		os.WriteFile(logPath, []byte(logText), 0o644) //nolint:errcheck // the log is best effort, as the shell redirect
	}
	now := sizeOf(out)
	var bad []string
	if err != nil {
		bad = append(bad, "exit=1")
	}
	if now < minSize {
		bad = append(bad, "tiny ("+IEC(now)+")")
	}
	if prev > shrinkMinPrev && now < prev/2 {
		bad = append(bad, fmt.Sprintf("shrank %dB->%dB", prev, now))
	}
	if len(bad) == 0 {
		green(r.out, fmt.Sprintf("  ✓ %s (%ds) -> %s %s", b.Name, secs, b.Index, IEC(now)))
		return
	}
	red(r.out, fmt.Sprintf("  ✗ %s (%ds) — %s [log: %s]", b.Name, secs, strings.Join(bad, ", "), logPath))
	if bak := filepath.Join(r.bak, b.Index); r.backup && isRegular(bak) {
		if err := copyFile(bak, out); err != nil {
			red(r.out, "    rollback of "+b.Index+" failed: "+err.Error())
		} else {
			yellow(r.out, "    rolled back "+b.Index+" from backup")
		}
	}
	r.fail = true
}

func isRegular(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// copyFile is cp -p src dst: the content, mode and modification time.
func copyFile(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(dst, st.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, st.ModTime(), st.ModTime())
}
