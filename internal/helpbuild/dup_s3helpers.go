package helpbuild

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// DUPLICATES, to be removed when S3 lands: the subset of
// feat/i3-helpbuild-s3 (commit 360a4ef, pyfs.go pyWalk / loadJSON /
// JSONLoadError and registry.go loadRegistry / identify / installDir) that
// S6 needs, renamed with a "dup" prefix so both branches compile together.
// The bodies are S3's; when both are merged, replace each dupX by its S3
// original and delete this file.

// dupWalk is os.walk(top) top-down without followlinks: visit gets each
// directory (joined like os.path.join), its subdirectory names (a symlink
// to a directory counts as one) and the other names, both sorted, and
// returns the subdirectories to descend; a symlinked one is not descended.
// A directory that cannot be read is skipped silently (onerror=None).
func dupWalk(top string, visit func(dir string, dirs, files []string) []string) {
	ents, err := os.ReadDir(top)
	if err != nil {
		return
	}
	var dirs, files []string
	for _, e := range ents {
		isDirEntry := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			st, err := os.Stat(pyJoin(top, e.Name()))
			isDirEntry = err == nil && st.IsDir()
		}
		if isDirEntry {
			dirs = append(dirs, e.Name())
		} else {
			files = append(files, e.Name())
		}
	}
	for _, d := range visit(top, dirs, files) {
		sub := pyJoin(top, d)
		if st, err := os.Lstat(sub); err == nil && st.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		dupWalk(sub, visit)
	}
}

// dupJSONLoadError is an error json.load(open(path, encoding="utf-8"))
// raises that callers catch: an OSError (OS true) or a ValueError (a
// UnicodeDecodeError, a JSONDecodeError or the int digit limit).
type dupJSONLoadError struct {
	OS  bool
	Msg string
}

func (e *dupJSONLoadError) Error() string { return e.Msg }

// dupLoadJSON is json.load(open(path, encoding="utf-8")): strict UTF-8,
// universal newlines, then Python's decoder. A RecursionError is returned
// as it is (Python does not catch it either).
func dupLoadJSON(path string) (pyjson.Value, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return pyjson.Value{}, &dupJSONLoadError{OS: true, Msg: index.OSError(pe.Err, path)}
		}
		return pyjson.Value{}, err
	}
	if err := index.CheckUTF8(raw); err != nil {
		return pyjson.Value{}, &dupJSONLoadError{Msg: err.Error()}
	}
	v, err := pyjson.Loads([]rune(pycompat.UniversalNewlines(string(raw))))
	var rec *pyjson.RecursionError
	if err != nil && !errors.As(err, &rec) {
		return pyjson.Value{}, &dupJSONLoadError{Msg: err.Error()}
	}
	return v, err
}

// dupRegistryShapeError is the AttributeError or TypeError Python raises
// on a registry of an unexpected shape.
type dupRegistryShapeError struct{ path, what string }

func (e *dupRegistryShapeError) Error() string { return e.path + ": " + e.what }

func dupRegistryError(repo, what string) error {
	return &dupRegistryShapeError{filepath.Join(repo, index.RegistryRel), what}
}

// dupLoadRegistry is versions.load_registry: ok is false when the file
// does not exist (None).
func dupLoadRegistry(repo string) (reg pyjson.Value, ok bool, err error) {
	path := filepath.Join(repo, index.RegistryRel)
	if !isRegular(path) {
		return pyjson.Value{}, false, nil
	}
	reg, err = dupLoadJSON(path)
	return reg, err == nil, err
}

// dupNormJoin is os.path.normpath(os.path.join(base, rel)) for an absolute
// base.
func dupNormJoin(base, rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Join(base, rel)
}

// dupCandidateRepos: the data root itself (the 4.14 repo root) and two
// levels up (versions/<id>/ -> repo root).
func dupCandidateRepos(target string) []string {
	return []string{target, filepath.Dir(filepath.Dir(target))}
}

// dupIdentify is versions.identify for an absolute, clean target: the
// registered version id whose data_root is target, "" (None) when none is.
func dupIdentify(target string) (string, error) {
	for _, repo := range dupCandidateRepos(target) {
		reg, ok, err := dupLoadRegistry(repo)
		if err != nil {
			return "", err
		}
		if !ok || !reg.Truthy() {
			continue
		}
		if reg.Kind != pyjson.Object {
			return "", dupRegistryError(repo, "the registry is not a JSON object")
		}
		vers, _ := reg.Get("versions")
		if !vers.Truthy() {
			continue
		}
		switch vers.Kind {
		case pyjson.Array, pyjson.Str: // data_root's indexing raises TypeError: caught
			continue
		case pyjson.Object:
		default:
			return "", dupRegistryError(repo, `"versions" is not iterable`)
		}
		for _, vid := range vers.Keys {
			rel, has := vers.M[vid].Get("data_root") // KeyError and TypeError are caught
			if !has || rel.Kind != pyjson.Str {
				continue
			}
			if dupNormJoin(repo, rel.S) == target {
				return vid, nil
			}
		}
	}
	return "", nil
}

// dupInstallDir is versions.install_dir: the install directory registered
// for the version whose data root is base, "" when there is none or the
// value is falsy.
func dupInstallDir(base string) (string, error) {
	target, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	vid, err := dupIdentify(target)
	if err != nil || vid == "" {
		return "", err
	}
	for _, repo := range dupCandidateRepos(target) {
		reg, ok, err := dupLoadRegistry(repo)
		if err != nil {
			return "", err
		}
		if !ok || !reg.Truthy() {
			continue
		}
		vers, _ := reg.Get("versions")
		if vers.Kind != pyjson.Object {
			continue // dupIdentify proved this shape harmless
		}
		entry, has := vers.Get(vid)
		if !has {
			continue
		}
		if entry.Kind != pyjson.Object {
			return "", dupRegistryError(repo, "version "+vid+" is not an object")
		}
		dir, _ := entry.Get("install_dir")
		if !dir.Truthy() {
			return "", nil
		}
		if dir.Kind != pyjson.Str {
			return "", dupRegistryError(repo, "install_dir of version "+vid+" is not a string")
		}
		return dir.S, nil
	}
	return "", nil
}
