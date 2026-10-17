package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RegistryRel is the registry path relative to the repository root
// (versions.py REGISTRY_REL).
var RegistryRel = filepath.Join("versions", "registry.json")

// EnvVersion names the variable that selects the version when no explicit
// request is given (versions.py ENV_VAR).
const EnvVersion = "NIAGARA_HELP_VERSION"

// UnknownVersionError is versions.py UnknownVersionError: the requested version
// is not registered, or a prefix matches several versions. Its text is what the
// CLI prints after "ERROR: ".
type UnknownVersionError struct{ Msg string }

func (e *UnknownVersionError) Error() string { return e.Msg }

// Registry is versions/registry.json: the default version id and, in file
// order, one entry object per registered version id.
type Registry struct {
	Default    string // "" when the file has no "default" key
	hasDefault bool
	Versions   *Object // version id -> entry object (label, install_dir, data_root, ...)
}

// Entry is one registered version.
type Entry struct {
	Label      string
	InstallDir string
	DataRoot   string  // relative to the repository root ("." for the 4.14 corpus)
	Raw        *Object // every key of the entry, including optional ones (course_dir)
}

// LoadRegistry reads <repoRoot>/versions/registry.json. It returns (nil, nil)
// when the file does not exist, like versions.load_registry returning None.
func LoadRegistry(repoRoot string) (*Registry, error) {
	path := filepath.Join(repoRoot, RegistryRel)
	if !isFile(path) {
		return nil, nil
	}
	o, err := LoadObject(path)
	if err != nil {
		return nil, err
	}
	vers := o.Object("versions")
	if vers == nil {
		return nil, fmt.Errorf("%s: \"versions\" is missing or not an object", path)
	}
	r := &Registry{Versions: vers}
	r.Default, r.hasDefault = o.String("default")
	return r, nil
}

// IDs returns the registered version ids sorted as strings (Python sorted()).
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	ids := append([]string(nil), r.Versions.Keys()...)
	sort.Strings(ids)
	return ids
}

// Entry returns the entry of a registered version id.
func (r *Registry) Entry(id string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	o := r.Versions.Object(id)
	if o == nil {
		return Entry{}, false
	}
	e := Entry{Raw: o}
	e.Label, _ = o.String("label")
	e.InstallDir, _ = o.String("install_dir")
	e.DataRoot, _ = o.String("data_root")
	return e, true
}

// Match maps a requested id or unique dotted prefix ("4.15") to a registered
// id (versions.py _match), with Python's exact error texts.
func (r *Registry) Match(requested string) (string, error) {
	ids := r.IDs()
	if r.Versions.Has(requested) {
		return requested, nil
	}
	var hits []string
	for _, v := range ids {
		if strings.HasPrefix(v, requested+".") {
			hits = append(hits, v)
		}
	}
	switch {
	case len(hits) == 1:
		return hits[0], nil
	case len(hits) > 1:
		return "", &UnknownVersionError{fmt.Sprintf("Ambiguous Niagara version '%s': matches %s",
			requested, strings.Join(hits, ", "))}
	}
	return "", &UnknownVersionError{fmt.Sprintf("Unknown Niagara version '%s'. Known versions: %s",
		requested, strings.Join(ids, ", "))}
}

// ResolveVersion returns the version id to use: the explicit request, else the
// EnvVersion variable read through getenv, else the registry default
// (versions.py resolve_version; empty strings count as absent). Without a
// registry it returns "" unless a version was requested.
func ResolveVersion(repoRoot, requested string, getenv func(string) string) (string, error) {
	reg, err := LoadRegistry(repoRoot)
	if err != nil {
		return "", err
	}
	if reg == nil {
		if requested != "" {
			return "", &UnknownVersionError{fmt.Sprintf("Unknown Niagara version '%s': no %s found",
				requested, RegistryRel)}
		}
		return "", nil
	}
	wanted := requested
	if wanted == "" {
		wanted = getenv(EnvVersion)
	}
	if wanted == "" {
		if !reg.hasDefault {
			return "", errors.New(RegistryRel + " has no \"default\" version")
		}
		wanted = reg.Default
	}
	return reg.Match(wanted)
}

// DataRoot returns the absolute data folder of a registered version
// (versions.py data_root). Without a registry or a version id it is the
// absolute repository root.
func DataRoot(repoRoot, id string) (string, error) {
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	reg, err := LoadRegistry(repoRoot)
	if err != nil {
		return "", err
	}
	if reg == nil || id == "" {
		return abs, nil
	}
	e, ok := reg.Entry(id)
	if !ok || !e.Raw.Has("data_root") {
		return "", fmt.Errorf("%s: version %s has no data_root", RegistryRel, id)
	}
	if _, isStr := e.Raw.String("data_root"); !isStr {
		// Python's os.path.join raises TypeError on null or a number.
		return "", fmt.Errorf("%s: data_root of version %s is not a string", RegistryRel, id)
	}
	if filepath.IsAbs(e.DataRoot) { // os.path.join drops abs when rel is absolute
		return filepath.Clean(e.DataRoot), nil
	}
	return filepath.Join(abs, e.DataRoot), nil
}

// Resolve returns the version id and its absolute data root (versions.py
// resolve). Without a registry the id is "" and the root is the repository.
func Resolve(repoRoot, requested string, getenv func(string) string) (id, root string, err error) {
	if id, err = ResolveVersion(repoRoot, requested, getenv); err != nil {
		return "", "", err
	}
	root, err = DataRoot(repoRoot, id)
	return id, root, err
}

// Dir is the index directory of a data root.
func Dir(root string) string { return filepath.Join(root, "indexes") }

// Path is the path of one index file of a data root.
func Path(root, name string) string { return filepath.Join(root, "indexes", name) }

// IsExtracted reports whether a data root holds a usable corpus: its
// indexes/class-index.json exists (versions.py is_extracted).
func IsExtracted(root string) bool { return isFile(Path(root, "class-index.json")) }

// isFile is os.path.isfile: true for a regular file, following symlinks.
func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
