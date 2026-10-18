package helpbuild

import (
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The registry lookups of niagara_help_lib/versions.py and course.py that
// the PDF and course builders make from a data root: identify,
// install_dir and course_dir. They read versions/registry.json with
// Python's json (pyjson) so a malformed entry takes the branch Python
// takes: what Python catches is skipped, what makes it raise is an error.

// loadRegistry is versions.load_registry: ok is false when the file does
// not exist (None).
func loadRegistry(repo string) (reg pyjson.Value, ok bool, err error) {
	path := filepath.Join(repo, index.RegistryRel)
	if !isRegular(path) {
		return pyjson.Value{}, false, nil
	}
	reg, err = loadJSON(path)
	return reg, err == nil, err
}

// registryShapeError is the AttributeError or TypeError Python raises on a
// registry of an unexpected shape.
type registryShapeError struct{ path, what string }

func (e *registryShapeError) Error() string { return e.path + ": " + e.what }

func registryError(repo, what string) error {
	return &registryShapeError{filepath.Join(repo, index.RegistryRel), what}
}

// pyNormJoin is os.path.normpath(os.path.join(base, rel)) for an absolute
// base: an absolute rel replaces it.
func pyNormJoin(base, rel string) string {
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel)
	}
	return filepath.Join(base, rel)
}

// candidateRepos are the two places versions.py looks for the registry of
// a data root: the root itself (the 4.14 repo root) and two levels up
// (versions/<id>/ -> repo root).
func candidateRepos(target string) []string {
	return []string{target, filepath.Dir(filepath.Dir(target))}
}

// absTarget is os.path.normpath(os.path.abspath(base)).
func absTarget(base string) (string, error) { return filepath.Abs(base) }

// identify is versions.identify: the registered version id whose
// data_root is target (absolute, clean), "" (None) when none is.
func identify(target string) (string, error) {
	for _, repo := range candidateRepos(target) {
		reg, ok, err := loadRegistry(repo)
		if err != nil {
			return "", err
		}
		if !ok || !reg.Truthy() {
			continue
		}
		if reg.Kind != pyjson.Object {
			return "", registryError(repo, "the registry is not a JSON object")
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
			return "", registryError(repo, `"versions" is not iterable`)
		}
		for _, vid := range vers.Keys {
			entry := vers.M[vid]
			rel, has := entry.Get("data_root") // KeyError and TypeError are caught
			if !has || rel.Kind != pyjson.Str {
				continue
			}
			if pyNormJoin(repo, rel.S) == target {
				return vid, nil
			}
		}
	}
	return "", nil
}

// installDir is versions.install_dir: the install directory registered
// for the version whose data root is base, "" when there is none or the
// value is falsy (pdf_extractor then has no docs dir).
func installDir(base string) (string, error) {
	target, err := absTarget(base)
	if err != nil {
		return "", err
	}
	vid, err := identify(target)
	if err != nil || vid == "" {
		return "", err
	}
	for _, repo := range candidateRepos(target) {
		reg, ok, err := loadRegistry(repo)
		if err != nil {
			return "", err
		}
		if !ok || !reg.Truthy() {
			continue
		}
		vers, _ := reg.Get("versions")
		if vers.Kind != pyjson.Object {
			continue // identify proved this shape harmless
		}
		entry, has := vers.Get(vid)
		if !has {
			continue
		}
		if entry.Kind != pyjson.Object {
			return "", registryError(repo, "version "+vid+" is not an object")
		}
		dir, _ := entry.Get("install_dir")
		if !dir.Truthy() {
			return "", nil
		}
		if dir.Kind != pyjson.Str {
			return "", registryError(repo, "install_dir of version "+vid+" is not a string")
		}
		return dir.S, nil
	}
	return "", nil
}

// courseDir is course.course_dir: the absolute course tree bound to the
// version whose data root is base, "" (None) when none is.
func courseDir(base string) (string, error) {
	target, err := absTarget(base)
	if err != nil {
		return "", err
	}
	vid, err := identify(target)
	if err != nil {
		return "", err
	}
	for _, repo := range candidateRepos(target) {
		reg, ok, err := loadRegistry(repo)
		if err != nil {
			return "", err
		}
		if !ok || !reg.Truthy() {
			continue // (reg or {}).get(...) is None
		}
		vers, has := reg.Get("versions")
		if !has {
			continue
		}
		if vers.Kind != pyjson.Object {
			return "", registryError(repo, `"versions" is not an object`)
		}
		entry, has := vers.Get(vid)
		if vid == "" || !has || !entry.Truthy() {
			continue
		}
		if entry.Kind != pyjson.Object {
			return "", registryError(repo, "version "+vid+" is not an object")
		}
		dir, _ := entry.Get("course_dir")
		if !dir.Truthy() {
			continue
		}
		if dir.Kind != pyjson.Str {
			return "", registryError(repo, "course_dir of version "+vid+" is not a string")
		}
		return pyNormJoin(repo, dir.S), nil
	}
	return "", nil
}
