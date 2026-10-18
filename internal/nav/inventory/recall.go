package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// selftest --recall (recall.py): every top-level class / interface /
// @interface / enum / record declaration of a .java file under
// organized/*/*/vineflower/ (and the original sources under
// docSource/docSource-doc/extracted/, indexed through their vineflower copy)
// must be found by the class lookup in the same file; a declaration whose
// header names extends/implements must have them in its index entry.

// recallFile is one ground-truth file: its path relative to organized/ and
// the path its declarations must have in the class index.
type recallFile struct{ rel, expected string }

// recallReport is class_recall's report.
type recallReport struct {
	files, declarations, withSupertypes, indexEntries int
	missing                                           [][2]string // name, rel
	lostSupertypes                                    []lostSupertypes
	indexBuilt                                        string
}

type lostSupertypes struct {
	name, rel string
	keywords  []string
}

// walkUnsorted is os.walk(top) top-down in directory order (scandir, not
// sorted), not entering symlinked directories; f gets each directory and
// its non-directory entries.
func walkUnsorted(top string, f func(dir string, files []string)) {
	d, err := os.Open(top)
	if err != nil {
		return
	}
	names, err := d.Readdirnames(-1)
	d.Close()
	if err != nil {
		return
	}
	var dirs, files []string
	for _, n := range names {
		if st, err := os.Stat(filepath.Join(top, n)); err == nil && st.IsDir() {
			dirs = append(dirs, n)
		} else {
			files = append(files, n)
		}
	}
	f(top, files)
	for _, n := range dirs {
		p := filepath.Join(top, n)
		if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		walkUnsorted(p, f)
	}
}

// javaFiles is _java_files: the ground-truth files in Python's order.
func javaFiles(organized string) []recallFile {
	var out []recallFile
	for _, module := range sortedDir(organized) {
		mpath := filepath.Join(organized, module)
		if !isDir(mpath) {
			continue
		}
		for _, sub := range sortedDir(mpath) {
			trees := []string{"vineflower"}
			if module == "docSource" && sub == "docSource-doc" {
				trees = append(trees, "extracted")
			}
			for _, tree := range trees {
				walkUnsorted(filepath.Join(mpath, sub, tree), func(dir string, files []string) {
					for _, name := range files {
						if !strings.HasSuffix(name, ".java") {
							continue
						}
						rel, err := filepath.Rel(organized, filepath.Join(dir, name))
						if err != nil {
							continue
						}
						rel = strings.ReplaceAll(filepath.ToSlash(rel), "\\", "/")
						expected := rel
						if tree == "extracted" {
							_, after, _ := strings.Cut(rel, "/extracted/")
							expected = module + "/" + sub + "/vineflower/" + after
						}
						out = append(out, recallFile{rel, expected})
					}
				})
			}
		}
	}
	return out
}

// sortedDir is sorted(os.listdir(dir)); nil when it cannot be read.
func sortedDir(dir string) []string {
	names, err := readDirNames(dir)
	if err != nil {
		return nil
	}
	sort.Strings(names)
	return names
}

// classRecall is class_recall; the error text is what cmd_selftest prints
// after "recall oracle could not run: ".
func classRecall(env *registry.Env, sampleN int) (*recallReport, error) {
	baseDir := env.BaseDir
	organized, tried := env.ResolveOrganized()
	if organized == "" {
		return nil, errors.New("corpus not found: " + tried)
	}
	path := filepath.Join(baseDir, "indexes", "class-index.json")
	v, err := index.LoadPython(path)
	if err != nil {
		return nil, err
	}
	data, _ := v.(*index.Object)
	if data == nil || !data.Has("classes") {
		return nil, errors.New("'classes'")
	}
	classes := data.Object("classes")
	files := javaFiles(organized)
	if sampleN > 0 && sampleN < len(files) {
		files = sample(newPyRandom(0), files, sampleN)
	}
	r := &recallReport{files: len(files), indexBuilt: "?"}
	if meta := data.Object("_meta"); meta != nil {
		if t, ok := meta.Get("timestamp"); ok {
			r.indexBuilt = core.PyStr(t)
		}
	}
	if classes != nil {
		for _, e := range classes.Values() {
			if l, ok := e.([]any); ok {
				r.indexEntries += len(l)
			}
		}
	}
	for _, f := range files {
		names, keywords, err := declarations(filepath.Join(organized, f.rel))
		if err != nil {
			var pe *fs.PathError
			if errors.As(err, &pe) {
				return nil, errors.New(index.OSError(pe.Err, filepath.Join(organized, f.rel)))
			}
			return nil, err
		}
		for i, name := range names {
			r.declarations++
			var found []*index.Object
			if classes != nil {
				if v, ok := classes.Get(name); ok {
					for _, e := range core.Objects(v) {
						if core.Str(e, "path") == f.expected {
							found = append(found, e)
						}
					}
				}
			}
			if len(found) == 0 {
				r.missing = append(r.missing, [2]string{name, f.rel})
				continue
			}
			if len(keywords[i]) > 0 {
				r.withSupertypes++
			}
			var lost []string
			for _, k := range keywords[i] {
				has := false
				for _, e := range found {
					if !core.Truthy(e, "outer_class") && core.Truthy(e, k) {
						has = true
					}
				}
				if !has {
					lost = append(lost, k)
				}
			}
			if len(lost) > 0 {
				r.lostSupertypes = append(r.lostSupertypes, lostSupertypes{name, f.rel, lost})
			}
		}
	}
	return r, nil
}

func runSelftest(env *registry.Env, ns *argparse.Namespace) int {
	w := env.Out
	if !ns.Bool("recall") {
		fmt.Fprintln(w, "Nothing to check: use selftest --recall [--sample N].")
		return core.StatusError
	}
	sampleN, sampled := ns.Get("sample").(int)
	r, err := classRecall(env, sampleN)
	if err != nil {
		fmt.Fprintf(w, "ERROR: recall oracle could not run: %v\n", err)
		return core.StatusError
	}
	checked := r.declarations
	found := checked - len(r.missing)
	pct := 0.0
	if checked > 0 {
		pct = 100.0 * float64(found) / float64(checked)
	}
	mode := ""
	if sampled && sampleN != 0 {
		mode = ", sampled"
	}
	fmt.Fprintf(w, "recall %s%% (%s/%s declarations in %s files%s; 0 excluded; index %s entries built %s)\n",
		core.Fixed(pct, 2), commas(found), commas(checked), commas(r.files), mode, commas(r.indexEntries), r.indexBuilt)
	for _, m := range r.missing[:min(20, len(r.missing))] {
		fmt.Fprintf(w, "  MISSING %s  %s\n", m[0], m[1])
	}
	if len(r.missing) > 20 {
		fmt.Fprintf(w, "  ... and %s more\n", commas(len(r.missing)-20))
	}
	lost := r.lostSupertypes
	fmt.Fprintf(w, "supertypes %s/%s declarations whose header names extends/implements have them in the index\n",
		commas(r.withSupertypes-len(lost)), commas(r.withSupertypes))
	for _, l := range lost[:min(20, len(lost))] {
		fmt.Fprintf(w, "  NO %s %s  %s\n", strings.ToUpper(strings.Join(l.keywords, "/")), l.name, l.rel)
	}
	if len(lost) > 20 {
		fmt.Fprintf(w, "  ... and %s more\n", commas(len(lost)-20))
	}
	if checked > 0 && len(r.missing) == 0 && len(lost) == 0 {
		return 0
	}
	return core.StatusMiss
}

func commas(n int) string { return core.Commas(int64(n)) }
