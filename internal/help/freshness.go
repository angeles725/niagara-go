package help

// Corpus freshness (maintenance.py): freshness compares the raw corpus with
// the manifest the last rebuild stamped; freshness --all prints one line
// per registered version with its index status and the drift of its
// install's doc jars. Stamping and rebuilding stay in Python.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// freshnessDirs are SOURCE_DIRS: the raw corpus directories whose change
// makes the indexes stale.
var freshnessDirs = []string{"bajadoc", "source", "guides", "devguide", "docs"}

// freshnessExt is SOURCE_EXT: the only extension a directory counts.
var freshnessExt = map[string]string{"docs": ".pdf"}

// dirPrint is a _fingerprint_dir result: file count and total bytes.
type dirPrint struct{ files, bytes int64 }

// fingerprintDir is _fingerprint_dir: nil for an absent directory, or when
// ext is given and no file has it. A file whose size cannot be read (a
// dangling link) is not counted.
func fingerprintDir(path, ext string) *dirPrint {
	if !pycompat.IsDir(path) {
		return nil
	}
	fp := &dirPrint{}
	fts.WalkFiles(path, func(p string) {
		if ext != "" && !strings.HasSuffix(pycompat.Lower(filepath.Base(p)), ext) {
			return
		}
		if st, err := os.Stat(p); err == nil {
			fp.bytes += st.Size()
			fp.files++
		}
	})
	if ext != "" && fp.files == 0 {
		return nil
	}
	return fp
}

// computeFingerprint is compute_fingerprint: every source dir of baseDir.
// The directories are walked concurrently (the counts do not depend on
// the order).
func computeFingerprint(baseDir string) map[string]*dirPrint {
	prints := make([]*dirPrint, len(freshnessDirs))
	var wg sync.WaitGroup
	for i, d := range freshnessDirs {
		wg.Go(func() { prints[i] = fingerprintDir(filepath.Join(baseDir, d), freshnessExt[d]) })
	}
	wg.Wait()
	out := map[string]*dirPrint{}
	for i, d := range freshnessDirs {
		out[d] = prints[i]
	}
	return out
}

// loadManifest is _load_manifest: indexes/.manifest.json, nil when absent.
func loadManifest(baseDir string) (*index.Object, error) {
	path := index.Path(baseDir, ".manifest.json")
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil, nil
	}
	return index.LoadObject(path)
}

// recordedPrint is manifest["sources"].get(d) as a fingerprint; ok is false
// for a recorded value that is no {"files", "bytes"} pair of integers
// (never equal to a computed one).
func recordedPrint(sources *index.Object, d string) (fp *dirPrint, ok bool) {
	if sources == nil {
		return nil, true
	}
	v, present := sources.Get(d)
	if !present || v == nil {
		return nil, true
	}
	o, isObj := v.(*index.Object)
	if !isObj || o.Len() != 2 {
		return nil, false
	}
	files, ok1 := index.Int(get(o, "files", nil))
	size, ok2 := index.Int(get(o, "bytes", nil))
	if !ok1 || !ok2 {
		return nil, false
	}
	return &dirPrint{files, size}, true
}

func samePrint(a, b *dirPrint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// fmtPrint is _fmt.
func fmtPrint(fp *dirPrint) string {
	if fp == nil {
		return "(absent)"
	}
	return fmt.Sprintf("%s files, %s bytes", pycompat.Commas(int(fp.files)), pycompat.Commas(int(fp.bytes)))
}

// fmtRecorded is _fmt of a recorded value that is not a plain fingerprint.
func fmtRecorded(sources *index.Object, d string) string {
	if fp, ok := recordedPrint(sources, d); ok {
		return fmtPrint(fp)
	}
	o, _ := get(sources, d, nil).(*index.Object)
	return fmt.Sprintf("%s files, %s bytes", pyStr(get(o, "files", nil)), pyStr(get(o, "bytes", nil)))
}

// Freshness is the `freshness` command (cmd_admin.run_freshness).
func (e *Env) Freshness(all bool) error {
	if all {
		return e.freshnessAll()
	}
	vid := e.VersionID
	if vid == "" {
		vid = "None"
	}
	fmt.Fprintf(e.Out, "Niagara version: %s  (%s)\n", vid, e.BaseDir)
	return e.freshness()
}

// freshness is maintenance.cmd_freshness.
func (e *Env) freshness() error {
	current := computeFingerprint(e.BaseDir)
	manifest, err := loadManifest(e.BaseDir)
	if err != nil {
		return err
	}
	fmt.Fprintln(e.Out, "CORPUS FRESHNESS")
	fmt.Fprintln(e.Out, strings.Repeat("=", 60))
	if manifest == nil {
		fmt.Fprintln(e.Out, "UNKNOWN — no baseline manifest recorded.")
		fmt.Fprintln(e.Out)
		fmt.Fprintln(e.Out, "Current corpus:")
		for _, d := range freshnessDirs {
			fmt.Fprintf(e.Out, "  %s %s\n", pycompat.LJust(d, 12, ' '), fmtPrint(current[d]))
		}
		fmt.Fprintln(e.Out)
		fmt.Fprintln(e.Out, "Run 'rebuild-all --stamp-only' to record the current indexes")
		fmt.Fprintln(e.Out, "as the trusted baseline, or 'rebuild-all' to rebuild + stamp.")
		return nil
	}
	recorded := getObj(manifest, "sources")
	var stale []string
	for _, d := range freshnessDirs {
		now := current[d]
		was, plain := recordedPrint(recorded, d)
		status := "OK"
		switch {
		case plain && samePrint(now, was):
		case now == nil:
			status = "SOURCE REMOVED"
		case plain && was == nil:
			status = "NEW SOURCE"
		default:
			status = "CHANGED"
		}
		if status != "OK" {
			stale = append(stale, d)
		}
		fmt.Fprintf(e.Out, "  %s %s now: %s\n", pycompat.LJust(status, 14, ' '), pycompat.LJust(d, 12, ' '), fmtPrint(now))
		if status == "CHANGED" {
			fmt.Fprintf(e.Out, "  %s %s was: %s\n", pycompat.LJust("", 14, ' '), pycompat.LJust("", 12, ' '), fmtRecorded(recorded, d))
		}
	}
	fmt.Fprintln(e.Out)
	fmt.Fprintf(e.Out, "Baseline stamped: %s\n", pyStr(get(manifest, "stamped_utc", "?")))
	if len(stale) > 0 {
		fmt.Fprintf(e.Out, "RESULT: STALE — %d source(s) changed: %s\n", len(stale), strings.Join(stale, ", "))
		fmt.Fprintln(e.Out, "Reindex with 'rebuild-all' (then the manifest re-stamps).")
	} else {
		fmt.Fprintln(e.Out, "RESULT: FRESH — corpus matches the recorded baseline.")
	}
	return nil
}

// indexStatus is index_status: FRESH, STALE or UNKNOWN for one data root.
func indexStatus(root string) (string, error) {
	manifest, err := loadManifest(root)
	if err != nil {
		return "", err
	}
	if manifest == nil {
		return "UNKNOWN", nil
	}
	current := computeFingerprint(root)
	recorded := getObj(manifest, "sources")
	if recorded == nil || recorded.Len() != len(freshnessDirs) {
		return "STALE", nil
	}
	for _, d := range freshnessDirs {
		if !recorded.Has(d) {
			return "STALE", nil
		}
		was, plain := recordedPrint(recorded, d)
		if !plain || !samePrint(current[d], was) {
			return "STALE", nil
		}
	}
	return "FRESH", nil
}

// javadocJar is doc_jars.JAVADOC_JAR.
const javadocJar = "niagaraJavadoc.jar"

// isDocJar is doc_jars.is_doc_jar.
func isDocJar(name string) bool {
	return strings.HasSuffix(name, "-doc.jar") || name == "docDeveloper-ux.jar" ||
		name == "docDeveloper-rt.jar" ||
		(strings.HasPrefix(name, "doc") && strings.HasSuffix(name, ".jar") && !strings.Contains(name, "-"))
}

// docJarSizes is {relpath(j, install): getsize(j)} over doc_jars.list_doc_jars.
func docJarSizes(install string) (map[string]int64, error) {
	out := map[string]int64{}
	modules := filepath.Join(install, "modules")
	if pycompat.IsDir(modules) {
		entries, err := os.ReadDir(modules)
		if err != nil {
			return nil, err
		}
		for _, d := range entries {
			if !isDocJar(d.Name()) {
				continue
			}
			st, err := os.Stat(filepath.Join(modules, d.Name()))
			if err != nil {
				return nil, err
			}
			out["modules/"+d.Name()] = st.Size()
		}
	}
	javadoc := filepath.Join(install, "javadoc", javadocJar)
	if st, err := os.Stat(javadoc); err == nil && st.Mode().IsRegular() {
		out["javadoc/"+javadocJar] = st.Size()
	}
	return out, nil
}

// installStatus is install_status: the install's doc jars against the
// tracked versions/<id>.manifest.json, by relative path and size.
func installStatus(repoRoot, vid, install string) (string, error) {
	path := filepath.Join(repoRoot, "versions", vid+".manifest.json")
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return "no jar manifest", nil
	}
	if install == "" || !pycompat.IsDir(install) {
		return "install absent", nil
	}
	m, err := index.LoadObject(path)
	if err != nil {
		return "", err
	}
	recorded := map[string]int64{}
	jars, _ := get(m, "jars", nil).([]any)
	for _, j := range jars {
		o, _ := j.(*index.Object)
		size, _ := index.Int(get(o, "bytes", nil))
		recorded[getStr(o, "jar", "")] = size
	}
	current, err := docJarSizes(install)
	if err != nil {
		return "", err
	}
	added, removed, changed := 0, 0, 0
	for j, size := range current {
		if was, ok := recorded[j]; !ok {
			added++
		} else if was != size {
			changed++
		}
	}
	for j := range recorded {
		if _, ok := current[j]; !ok {
			removed++
		}
	}
	if added+removed+changed > 0 {
		return fmt.Sprintf("INSTALL CHANGED (+%d -%d ~%d jars)", added, removed, changed), nil
	}
	return "install jars match", nil
}

// freshnessAll is maintenance.cmd_freshness_all.
func (e *Env) freshnessAll() error {
	reg, err := index.LoadRegistry(e.RepoRoot)
	if err != nil {
		return err
	}
	if reg == nil {
		fmt.Fprintln(e.Out, "No versions/registry.json; use plain 'freshness'.")
		return nil
	}
	fmt.Fprintf(e.Out, "FRESHNESS PER NIAGARA VERSION (default %s)\n", reg.Default)
	fmt.Fprintln(e.Out, strings.Repeat("=", 60))
	ids := reg.IDs()
	sort.Strings(ids)
	// Each version's status is computed concurrently, then printed in order.
	statuses := make([]string, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, vid := range ids {
		wg.Go(func() { statuses[i], errs[i] = e.versionFreshness(reg, vid) })
	}
	wg.Wait()
	for i, vid := range ids {
		if errs[i] != nil {
			return errs[i]
		}
		entry, _ := reg.Entry(vid)
		fmt.Fprintf(e.Out, "  %s %s %s\n", pycompat.LJust(vid, 12, ' '), pycompat.LJust(entry.DataRoot, 20, ' '), statuses[i])
	}
	return nil
}

// versionFreshness is the status column of freshness --all for one version.
func (e *Env) versionFreshness(reg *index.Registry, vid string) (string, error) {
	entry, _ := reg.Entry(vid)
	root, err := index.DataRoot(e.RepoRoot, vid)
	if err != nil {
		return "", err
	}
	if !index.IsExtracted(root) {
		return fmt.Sprintf("NOT EXTRACTED (nh versions extract %s)", vid), nil
	}
	idx, err := indexStatus(root)
	if err != nil {
		return "", err
	}
	inst, err := installStatus(e.RepoRoot, vid, getStr(entry.Raw, "install_dir", ""))
	if err != nil {
		return "", err
	}
	return idx + "; " + inst, nil
}
