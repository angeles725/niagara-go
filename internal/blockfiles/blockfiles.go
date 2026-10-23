// Package blockfiles is tools/lib/blockfiles.py, the one block-discovery rule of the
// niagara-research corpus tools (gen-catalog, check-coverage, corpus-nav): a block on
// disk is seen by every tool or by none.
//
// Main corpus, the repository root only: niagara-mental-model.md ("1-3"),
// niagara-mental-model-bloque<N>[-<suffix>].md ("<N>", leading zeros dropped) and
// niagara-mental-model-bloque-test-infrastructure.md ("TI"); dated snapshots
// niagara-mental-model.<YYYY-MM-DD>.md are not blocks. Sub-corpora, anywhere below:
// <stem>-block<N>.md ("R<N>" for reflow, else "<stem>-B<N>"), os413-bloque<N>.md
// ("os413-B<N>") and any other *bloque*.md (its repo-relative path without ".md").
// Skipped everywhere: organized/, _backups/, tools/, node_modules/, niagara-help/,
// module-navigator/ and dot-dirs. Block paths are joined like pathlib (Path(root) / name).
package blockfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Block is blockfiles.BlockFile.
type Block struct {
	Label  string // "720", "1-3", "TI" (main) or a sub-corpus label ("R2", "os413-B3", "notes/...")
	Path   string
	Corpus string // "main" or the sub-corpus name ("reflow", "os413", "notes", "loose", ...)
}

var skipDirs = map[string]bool{"organized": true, "_backups": true, "tools": true, "node_modules": true,
	"niagara-help": true, "module-navigator": true}

const (
	consolidated = "niagara-mental-model.md"
	testInfra    = "niagara-mental-model-bloque-test-infrastructure.md"
)

var (
	mainRe     = pyre.MustCompile(`^niagara-mental-model-bloque(\d+)(?:-[\w-]+)?\.md$`, 0)
	snapshotRe = pyre.MustCompile(`^niagara-mental-model\.\d{4}-\d{2}-\d{2}\.md$`, 0)
	subBlockRe = pyre.MustCompile(`^(?P<stem>.+)-block(?P<n>\d+)\.md$`, 0)
	subBloque  = pyre.MustCompile(`^(?P<stem>.+)-bloque(?P<n>\d+)\.md$`, 0)
	trailingRe = pyre.MustCompile(`^(.*?)(\d+)$`, 0)
)

// subPrefix maps a sub-corpus file prefix to (corpus, label prefix).
var subPrefix = map[string][2]string{"niagara-reflow": {"reflow", "R"}, "os413": {"os413", "os413-B"}}

// MainLabel is main_label: "1-3", "TI", "<N>" or "" when name is not a main block.
func MainLabel(name string) string {
	switch name {
	case consolidated:
		return "1-3"
	case testInfra:
		return "TI"
	}
	if m := mainRe.Search(name); m != nil {
		return PyInt(m.Group(1))
	}
	return ""
}

// IsMainLabel is is_main_label: a number, "1-3" or "TI".
func IsMainLabel(label string) bool { return IsDigit(label) || label == "1-3" || label == "TI" }

// isSnapshot reports a dated niagara-mental-model.<YYYY-MM-DD>.md snapshot.
func isSnapshot(name string) bool { return snapshotRe.MatchString(name) }

// PyInt is str(int(digits)) for the Unicode decimal digits re's \d matched ("0" if none parse).
func PyInt(digits string) string {
	if ascii := strings.TrimLeft(digits, "0"); len(digits) > 0 && len(ascii) <= 18 && isASCIIDigits(ascii) {
		if ascii == "" {
			return "0"
		}
		return ascii // the common case, without a big.Int
	}
	n, err := pycompat.ParseBigInt(digits)
	if err != nil {
		return "0"
	}
	return n.String()
}

func isASCIIDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// IsDigit is str.isdigit.
func IsDigit(s string) bool {
	for _, r := range s {
		if !pycompat.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// key is label_sort_key: (0,0,"") for "1-3", (1, int, "") for numbers, (2,0,"") for
// "TI", (3, prefix.lower(), trailing int) otherwise; numbers compare by value.
type key struct {
	group     int
	num, text string
}

func sortKey(label string) key {
	switch {
	case label == "1-3":
		return key{0, "0", ""}
	case IsDigit(label):
		return key{1, PyInt(label), ""}
	case label == "TI":
		return key{2, "0", ""}
	}
	if m := trailingRe.Search(label); m != nil {
		return key{3, PyInt(m.Group(2)), textfold.Lower(m.Group(1))}
	}
	return key{3, "0", textfold.Lower(label)}
}

func (k key) less(o key) bool {
	if k.group != o.group {
		return k.group < o.group
	}
	if k.group == 3 && k.text != o.text {
		return k.text < o.text
	}
	if len(k.num) != len(o.num) {
		return len(k.num) < len(o.num)
	}
	return k.num < o.num
}

// less orders labels by label_sort_key: consolidated first, numbers by value, TI after
// them, sub-corpus labels last.
func less(a, b string) bool { return sortKey(a).less(sortKey(b)) }

// SortLabels sorts labels in place, stably, by label_sort_key.
func SortLabels(labels []string) {
	sort.SliceStable(labels, func(i, j int) bool { return less(labels[i], labels[j]) })
}

func sortBlocks(bs []Block, tie func(a, b Block) bool) {
	sort.SliceStable(bs, func(i, j int) bool {
		ki, kj := sortKey(bs[i].Label), sortKey(bs[j].Label)
		if ki != kj {
			return ki.less(kj)
		}
		return tie(bs[i], bs[j])
	})
}

// join is str(Path(dir) / name) for the host (pypath.Join): on Windows pathlib spells
// every separator "\" (the tie-break sort on str(path) then compares that spelling, as
// Python does).
func join(dir, name string) string { return pypath.Join(dir, name) }

// MainBlocks is main_blocks: the main blocks directly under root, sorted by label, then
// file name. A root that does not exist (or is not a directory) has no blocks, as
// Path.glob says; a root that exists but cannot be listed (permission, I/O) is an error,
// never an empty corpus: Path.glob would hide it, so this is an intended fail-closed
// difference from Python.
func MainBlocks(root string) ([]Block, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, listErr(err)
	}
	var out []Block
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "niagara-mental-model") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if label := MainLabel(name); label != "" {
			out = append(out, Block{label, join(root, name), "main"})
		}
	}
	sortBlocks(out, func(a, b Block) bool { return filepath.Base(a.Path) < filepath.Base(b.Path) })
	return out, nil
}

// listErr is the error of a root that cannot be listed: nil when it does not exist or is
// not a directory (no blocks, as Path.glob and os.walk say), err otherwise.
func listErr(err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}

// subLabel is _sub_label: (corpus, label) of a sub-corpus block, ok false otherwise.
func subLabel(root, path string) (corpus, label string, ok bool) {
	name := filepath.Base(path)
	if MainLabel(name) != "" || isSnapshot(name) {
		return "", "", false // main-corpus names count only at the root (elsewhere they are copies)
	}
	for _, re := range []*pyre.Regexp{subBlockRe, subBloque} {
		if m := re.Search(name); m != nil {
			stem, _ := m.Named("stem")
			if p, known := subPrefix[stem]; known {
				n, _ := m.Named("n")
				return p[0], p[1] + PyInt(n), true
			}
		}
	}
	if m := subBlockRe.Search(name); m != nil {
		stem, _ := m.Named("stem")
		n, _ := m.Named("n")
		return stem, stem + "-B" + PyInt(n), true
	}
	if strings.Contains(name, "bloque") {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", "", false
		}
		rel = filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))
		if first, _, nested := strings.Cut(rel, "/"); nested {
			return first, rel, true
		}
		return "loose", rel, true
	}
	return "", "", false
}

// mayBeSubBlock is the exact cheap test before subLabel: every name subLabel accepts
// contains "-block" (subBlockRe) or "bloque" (subBloque and the *bloque* fallback), and
// those patterns are case-sensitive, so the regexes never run on any other name.
func mayBeSubBlock(name string) bool {
	return strings.Contains(name, "-block") || strings.Contains(name, "bloque")
}

// walkWorkers bounds the directories read at once by SubBlocks.
func walkWorkers() int { return min(runtime.GOMAXPROCS(0), 8) }

// SubBlocks is sub_blocks: every sub-corpus block below root (os.walk: skipped and dot
// directories pruned, symlinked directories not entered), sorted by label, then path. Directories are read in parallel; the full
// sort makes the result independent of walk order. A warm call is answered from the
// walk cache (subcache.go) when every walked directory is unchanged. A root that does
// not exist (or is not a directory) has no blocks; a root that exists but cannot be
// listed is an error, never an empty sub-corpus (os.walk would hide it: an intended
// fail-closed difference from Python, as in MainBlocks). So is a directory below the root
// that exists but cannot be listed: skipping it would answer without its blocks (a false
// "clear" in check-coverage); UnlistedDir names it. A directory that vanishes mid-walk is
// still skipped, as os.walk does.
func SubBlocks(root string) ([]Block, error) {
	if path := cachePath(root); path != "" {
		return cachedSubBlocks(root, path)
	}
	bs, _, err := walkSubBlocks(root, false)
	return bs, err
}

// walkSubBlocks is the SubBlocks walk; with record it also returns what the walk cache
// checks (every directory stamp, taken before the directory is read). The error is the
// listing error (listErr) of the root or, failing that, of the first unlistable directory
// by path (deterministic whatever the walk order); the record of such a walk is invalid.
func walkSubBlocks(root string, record bool) ([]Block, *walkRecord, error) {
	walks.Add(1)
	w := &walker{root: root, sem: make(chan struct{}, walkWorkers()-1), record: record}
	w.wg.Add(1)
	w.walk(root)
	w.wg.Wait()
	if err := w.firstErr(); err != nil {
		w.rec.invalid = true
		return nil, &w.rec, err
	}
	sortBlocks(w.out, func(a, b Block) bool { return a.Path < b.Path })
	return w.out, &w.rec, nil
}

// walker is one SubBlocks walk: sem holds a token per extra goroutine, so at most
// walkWorkers directories are read at once; a directory with no free token is walked
// by the goroutine that found it.
type walker struct {
	root   string
	sem    chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
	out    []Block
	record bool
	rec    walkRecord
	// rootErr is the root's listing error; only the first walk (the root's, before any
	// goroutine starts) sets it. dirErrs are the listing errors of directories below it
	// (guarded by mu).
	rootErr error
	dirErrs []error
}

// fail records dir's listing error (listErr: a directory that is gone is no error).
func (w *walker) fail(dir string, err error) {
	if err = listErr(err); err == nil {
		return
	}
	if dir == w.root {
		w.rootErr = err
		return
	}
	w.mu.Lock()
	w.dirErrs = append(w.dirErrs, err)
	w.mu.Unlock()
}

// firstErr is the root's listing error, else the error of the unlistable directory
// whose path sorts first.
func (w *walker) firstErr() error {
	if w.rootErr != nil {
		return w.rootErr
	}
	var first error
	for _, err := range w.dirErrs {
		if first == nil || UnlistedDir("", err) < UnlistedDir("", first) {
			first = err
		}
	}
	return first
}

// UnlistedDir is the directory a SubBlocks or AllBlocks error could not list: the
// error's path when it carries one (a directory below root), root otherwise.
func UnlistedDir(root string, err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Path != "" {
		return pe.Path
	}
	return root
}

func (w *walker) walk(dir string) {
	defer w.wg.Done()
	var st stamp
	if w.record {
		var err error
		if st, err = statStamp(dir); err != nil {
			w.fail(dir, err)
			w.note(nil, nil, true)
			return
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.fail(dir, err)
		if w.record {
			w.note([]dirEntryStamp{{dir, st}}, nil, !stableErr(err))
		}
		return
	}
	var found []Block
	var dirs []string
	var links []linkKind
	invalid := false
	for _, e := range entries {
		name := e.Name()
		// candidate is the name test of a block file; a symlink passing it is recorded,
		// because its target kind decides whether the walk reads it as one.
		candidate := strings.HasSuffix(name, ".md") && mayBeSubBlock(name)
		if e.Type()&os.ModeSymlink != 0 {
			target, err := os.Stat(join(dir, name))
			isDir := err == nil && target.IsDir()
			if w.record && candidate {
				links = append(links, linkKind{join(dir, name), isDir})
				invalid = invalid || (err != nil && !stableErr(err))
			}
			if isDir {
				continue // os.walk lists it as a directory but does not enter it
			}
		}
		if e.IsDir() {
			if !skipDirs[name] && !strings.HasPrefix(name, ".") {
				dirs = append(dirs, join(dir, name))
			}
			continue
		}
		if candidate {
			path := join(dir, name)
			if corpus, label, ok := subLabel(w.root, path); ok {
				found = append(found, Block{label, path, corpus})
			}
		}
	}
	if len(found) > 0 {
		w.mu.Lock()
		w.out = append(w.out, found...)
		w.mu.Unlock()
	}
	if w.record {
		w.note([]dirEntryStamp{{dir, st}}, links, invalid)
	}
	for _, d := range dirs {
		w.wg.Add(1)
		select {
		case w.sem <- struct{}{}:
			go func() {
				defer func() { <-w.sem }()
				w.walk(d)
			}()
		default:
			w.walk(d)
		}
	}
}

// note adds one directory's share to the walk record.
func (w *walker) note(dirs []dirEntryStamp, links []linkKind, invalid bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rec.dirs = append(w.rec.dirs, dirs...)
	w.rec.links = append(w.rec.links, links...)
	w.rec.invalid = w.rec.invalid || invalid
}

// AllBlocks is all_blocks: the main corpus, then every sub-corpus block. A root that
// cannot be listed is MainBlocks' error (the sub-corpus walk is not run), or SubBlocks'
// when it is locked between the two listings.
func AllBlocks(root string) ([]Block, error) {
	main, err := MainBlocks(root)
	if err != nil {
		return nil, err
	}
	sub, err := SubBlocks(root)
	if err != nil {
		return nil, err
	}
	return append(main, sub...), nil
}
