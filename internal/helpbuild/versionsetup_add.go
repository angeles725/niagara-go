package helpbuild

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// `versions add` (version_setup.add_version, detect_version, register,
// _restore_registry, _rollback): detect the version id of an install,
// register it in versions/registry.json (data root versions/<id>/), run
// ExtractAndIndex with the rebuild-all hook, then the two gates and the
// summary of versionreport.go. A failed add puts the registry back as it
// was and removes the data root it created.

// Python's re.search patterns; \d and \s are Unicode classes on a str.
var (
	versionPattern = regexp.MustCompile(`\p{Nd}+\.\p{Nd}+\.\p{Nd}+\.\p{Nd}+`)
	versionFull    = regexp.MustCompile(`^(?:\p{Nd}+\.\p{Nd}+\.\p{Nd}+\.\p{Nd}+)$`)
	vendorPattern  = regexp.MustCompile(pycompat.RE2Space(`vendorVersion\s*=\s*["']([^"']+)["']`))
)

// ValidVersionID is valid_version_id: an a.b.c.d id (decimal digits only),
// the name of the data root versions/<id>/ that extraction may delete and
// rebuild.
func ValidVersionID(vid string) bool { return versionFull.MatchString(vid) }

// DetectVersion is detect_version: the vendorVersion of
// modules/baja.jar's META-INF/module.xml (stripped), else the first a.b.c.d
// in the install folder's name, else "" (None). A jar Python cannot open
// (OSError, BadZipFile) or without the entry (KeyError) falls back to the
// folder name; an entry it cannot read for another reason (encrypted, an
// unsupported method) is an error, as Python raises there.
func DetectVersion(installDir string) (string, error) {
	xml, err := readJarEntry(pycompat.Join(installDir, "modules", "baja.jar"), "META-INF/module.xml")
	if err != nil && !fallsBack(err) {
		return "", err
	}
	if err == nil {
		if m := vendorPattern.FindStringSubmatch(pycompat.DecodeReplace(xml)); m != nil {
			return pycompat.Strip(m[1]), nil
		}
	}
	abs, err := pyAbspath(installDir)
	if err != nil {
		return "", err
	}
	return versionPattern.FindString(pyBasename(abs)), nil
}

// errNoEntry is ZipFile.getinfo's KeyError.
var errNoEntry = errors.New("there is no item named in the archive")

// fallsBack reports whether detect_version's except clause catches err.
func fallsBack(err error) bool {
	var pe *fs.PathError
	return errors.Is(err, errBadZip) || errors.Is(err, errNoEntry) || errors.Is(err, errTruncatedEntry) ||
		errors.As(err, &pe)
}

// readJarEntry is ZipFile(jar).read(name): the last central-directory entry
// of that name (ZipFile.NameToInfo keeps the last one).
func readJarEntry(jar, name string) ([]byte, error) {
	z, err := openZip(jar)
	if err != nil {
		return nil, err
	}
	defer z.f.Close()
	for i := len(z.infos) - 1; i >= 0; i-- {
		if z.infos[i].name == name {
			return z.readEntry(&z.infos[i], true)
		}
	}
	return nil, errNoEntry
}

// registryFile is the registry as add_version reads it: nil when the file
// does not exist (load_registry's None).
func registryFile(repo string) (*pyjson.Value, error) {
	path := pycompat.Join(repo, index.RegistryRel)
	if !pycompat.IsFile(path) {
		return nil, nil
	}
	v, err := loadJSON(path)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// registryVersions is (reg or {}).get("versions") or {}: nil for none; an
// entry table that is not an object is an error (Python would test
// membership in a list or a string).
func registryVersions(reg *pyjson.Value) (*pyjson.Value, error) {
	if reg == nil || !reg.Truthy() {
		return nil, nil
	}
	if reg.Kind != pyjson.Object {
		return nil, fmt.Errorf("%s: the top level is not an object", index.RegistryRel)
	}
	vs, ok := reg.Get("versions")
	if !ok || !vs.Truthy() {
		return nil, nil
	}
	if vs.Kind != pyjson.Object {
		return nil, fmt.Errorf("%s: \"versions\" is not an object", index.RegistryRel)
	}
	return &vs, nil
}

// entryString is known[vid][key] for a str value.
func entryString(e pyjson.Value, vid, key string) (string, error) {
	v, ok := e.Get(key)
	if !ok || v.Kind != pyjson.Str {
		return "", fmt.Errorf("%s: %s of version %s is missing or not a string", index.RegistryRel, key, vid)
	}
	return v.S, nil
}

// Register is register: vid added (or replaced, in place) in
// versions/registry.json with its label, install and data root
// versions/<vid>, created as {"default": null, "versions": {}} when absent
// or empty; it becomes the default with makeDefault or when there is none.
// The file is json.dumps(indent=2) and a newline, written atomically
// (Python appends the newline in a second write).
func Register(repo, vid, install, label string, makeDefault bool) error {
	reg, err := registryFile(repo)
	if err != nil {
		return err
	}
	if reg == nil || !reg.Truthy() {
		fresh := pyjson.NewObject()
		fresh.Set("default", pyjson.Value{Kind: pyjson.Null})
		fresh.Set("versions", pyjson.NewObject())
		reg = &fresh
	}
	if reg.Kind != pyjson.Object {
		return fmt.Errorf("%s: the top level is not an object", index.RegistryRel)
	}
	vs, ok := reg.Get("versions")
	if !ok || vs.Kind != pyjson.Object {
		return fmt.Errorf("%s: \"versions\" is missing or not an object", index.RegistryRel)
	}
	entry := pyjson.NewObject()
	entry.Set("label", pyjson.String(label))
	entry.Set("install_dir", pyjson.String(install))
	entry.Set("data_root", pyjson.String("versions/"+vid))
	vs.Set(vid, entry)
	reg.Set("versions", vs)
	if def, _ := reg.Get("default"); makeDefault || !def.Truthy() {
		reg.Set("default", pyjson.String(vid))
	}
	return atomicfile.WriteText(pycompat.Join(repo, index.RegistryRel), pyjson.Dumps(*reg, pyjson.Indent2)+"\n")
}

// AddOptions are add_version's arguments past the install, and the hooks
// of the steps it runs.
type AddOptions struct {
	Label          string  // --label; "" (falsy): the registered label, else the folder name
	Default, Force bool    // --default, --force
	ID             *string // --id; nil: DetectVersion (an explicit "" is validated, never detected)
	Rebuild        func(root string) ([]string, error)
	Extract        func(repo, vid string, opt SetupOptions) (*SetupResult, error) // nil: ExtractAndIndex
	Fulltext       FulltextDocs                                                   // the gates and the summary; nil: FreshFulltextDocs
	Now            func() time.Time
}

// AddVersion is add_version: it writes its lines to w and returns the exit
// status (0 ready, 1 a gate or the fidelity check failed, 2 cannot add). An
// error is what Python raises (a traceback); the registry entry and the
// data root it created are rolled back first when extraction raised.
// opt.Rebuild is the extraction's rebuild-all hook (RebuildToLog in nh);
// nil skips it.
func AddVersion(repo, install string, w io.Writer, opt AddOptions) (int, error) {
	out := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	install, err := pyAbspath(install)
	if err != nil {
		return 2, err
	}
	if !pycompat.IsDir(pycompat.Join(install, "modules")) {
		out("ERROR: %s is not a Niagara install (no modules/ folder)", install)
		return 2, nil
	}
	var vid string
	if opt.ID != nil {
		vid = *opt.ID
	} else if vid, err = DetectVersion(install); err != nil {
		return 2, err
	}
	if vid == "" {
		out("ERROR: cannot detect the Niagara version of %s (no modules/baja.jar vendorVersion, no a.b.c.d in "+
			"the folder name); pass --id", install)
		return 2, nil
	}
	if !ValidVersionID(vid) {
		out("ERROR: version id %s is not of the form a.b.c.d (digits); it names the data root versions/<id>/. "+
			"Pass --id a.b.c.d", pycompat.Repr(vid))
		return 2, nil
	}
	label, status, err := checkKnown(repo, vid, install, opt, out)
	if err != nil || status != 0 {
		return status, err
	}
	if label == "" {
		label = pyBasename(install)
	}
	regPath := pycompat.Join(repo, index.RegistryRel)
	var before []byte
	if pycompat.IsFile(regPath) {
		if before, err = os.ReadFile(regPath); err != nil {
			return 2, err
		}
	}
	root := pycompat.Join(repo, "versions", vid)
	_, lerr := os.Lstat(root)
	rootExisted := lerr == nil
	if err := Register(repo, vid, install, label, opt.Default); err != nil {
		return 2, err
	}
	out("registered %s (%s) -> versions/%s/", vid, label, vid)
	extract := opt.Extract
	if extract == nil {
		extract = ExtractAndIndex
	}
	res, err := extract(repo, vid, SetupOptions{Force: opt.Force, Rebuild: opt.Rebuild, Now: opt.Now})
	if res != nil {
		for _, line := range res.Log {
			out("%s", line)
		}
	}
	if err == nil && res == nil {
		err = fmt.Errorf("extract %s returned no result", vid)
	}
	if err != nil {
		rollback(regPath, before, root, rootExisted, out)
		return 2, err
	}
	if res.Error != "" {
		how := "rollback incomplete, see above"
		if rollback(regPath, before, root, rootExisted, out) {
			how = "registry entry rolled back"
		}
		out("ERROR: %s (%s)", res.Error, how)
		return 2, nil
	}
	gates, err := RunGates(repo, res.Root, vid, opt.Fulltext)
	if err != nil {
		return 1, err
	}
	if err := PrintSummary(w, repo, vid, res.Root, res, gates, opt.Fulltext); err != nil {
		return 1, err
	}
	return max(res.Status, gates.DocJars.Status, gates.Sweep.Status), nil
}

// checkKnown is add_version's test of a registered vid: another install,
// or the same one without Force, refuses (status 2); with Force the
// registered label is the default label.
func checkKnown(repo, vid, install string, opt AddOptions, out func(string, ...any)) (label string, status int, err error) {
	reg, err := registryFile(repo)
	if err != nil {
		return "", 2, err
	}
	known, err := registryVersions(reg)
	if err != nil || known == nil {
		return opt.Label, 0, err
	}
	e, ok := known.Get(vid)
	if !ok {
		return opt.Label, 0, nil
	}
	if e.Kind != pyjson.Object {
		return "", 2, fmt.Errorf("%s: version %s is not an object", index.RegistryRel, vid)
	}
	dir, err := entryString(e, vid, "install_dir")
	if err != nil {
		return "", 2, err
	}
	abs, err := pyAbspath(dir)
	if err != nil {
		return "", 2, err
	}
	if abs != install {
		out("ERROR: version %s is already registered for %s", vid, dir)
		return "", 2, nil
	}
	if !opt.Force {
		out("ERROR: version %s is already registered; pass --force to extract and index it again", vid)
		return "", 2, nil
	}
	if opt.Label != "" {
		return opt.Label, 0, nil
	}
	label, err = entryString(e, vid, "label")
	return label, 0, err
}

// rollback is _rollback: the registry put back (removed when it did not
// exist), the data root removed when this add created it. A failure is
// reported through out, never returned; true when everything was undone.
func rollback(regPath string, before []byte, root string, rootExisted bool, out func(string, ...any)) bool {
	ok := true
	var err error
	if before == nil {
		if pycompat.IsFile(regPath) {
			err = os.Remove(regPath)
		}
	} else {
		err = atomicfile.WriteBytes(regPath, before)
	}
	if err != nil {
		ok = false
		out("ERROR: the registry entry could not be rolled back (%s); remove it from %s by hand",
			pycompat.OSErrorText(err), regPath)
	}
	if !rootExisted && pycompat.IsDir(root) {
		if err := os.RemoveAll(root); err != nil {
			ok = false
			out("ERROR: the partial data root %s could not be removed (%s); remove it by hand",
				root, pycompat.OSErrorText(err))
		}
	}
	return ok
}
