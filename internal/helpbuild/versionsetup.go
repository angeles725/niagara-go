package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The raw-extraction steps of version_setup.extract_and_index for one
// registered version: (1) unzip every doc jar into the data root, (3) the
// sha256 fidelity check and (4) the tracked manifest
// versions/<id>.manifest.json. Step (2), rebuild-all, is the Rebuild hook
// (RebuildToLog runs RebuildAll with its stdout in build.log); the CLI
// wiring of extract_version.py (--force, --verify-only, --fill-missing) is
// S10. Like
// the rest of the package nothing is printed: the lines Python passes to
// out() are returned in SetupResult.Log, in order.

// RawDirs are the data-root folders extraction writes; DerivedDirs the
// ones the builders write. A re-extraction (Force) removes both.
var (
	RawDirs     = []string{"bajadoc", "devguide", "devguide-rc", "devguide-examples", "guides", "jdk", "source"}
	DerivedDirs = []string{"bajadoc-clean", "devguide-clean", "guides-clean", "indexes"}
)

// BuilderIndexes are the JSON indexes of maintenance.BUILDERS, in builder
// order, without the OPTIONAL_ARTIFACTS (jdk-class-index.json): the files
// _empty_indexes writes as {} when a skipped builder left them absent.
// call-index.json is retired (decision I3-5): Python's list still holds
// it, so _empty_indexes writes an empty one; EmptyIndexes does not
// (intended difference).
var BuilderIndexes = []string{"class-index.json", "inheritance.json", "method-index.json", "source-index.json",
	"slots-index.json", "xref-index.json", "guides-index.json", "devguide-toc.json", "pdf-index.json"}

// DumpManifest is the version manifest's json.dumps(indent=1).
var DumpManifest = pyjson.DumpOpts{EnsureASCII: true, Indent: 1, ItemSep: ",", KeySep: ": "}

// SetupOptions are extract_and_index's switches. FillMissing wins over
// VerifyOnly, VerifyOnly over a full extraction; Force lets a full
// extraction replace existing raw folders. Rebuild, when set, runs after
// a full extraction as rebuild-all on the data root, writing root/build.log
// and returning its "  SKIP", "  FAIL" and "Ran " lines; ExtractAndIndex
// writes the indexes/pdf-index.json stub before it and the empty indexes
// after it. RebuildToLog(os.Stderr) is _rebuild's hook; a nil Rebuild
// skips the step (extract_and_index no_index=True).
// Now stamps generated_utc (nil = time.Now).
type SetupOptions struct {
	Force, VerifyOnly, FillMissing bool
	Rebuild                        func(root string) ([]string, error)
	Now                            func() time.Time
}

// SetupResult is extract_and_index's (status, stats) and its out() lines.
// Status 0 is extracted and verified, 1 a failed fidelity check, 2 a
// refusal with Error set (install not mounted, the repo-root corpus, an
// existing corpus without Force).
type SetupResult struct {
	Status        int
	Error         string
	Install, Root string
	Extracted     *ExtractStats
	Filled        []string
	Build         []string
	Fidelity      *FidelityReport
	ManifestPath  string
	Log           []string
}

func (r *SetupResult) out(format string, a ...any) { r.Log = append(r.Log, fmt.Sprintf(format, a...)) }

func (r *SetupResult) refuse(msg string) *SetupResult {
	r.Status, r.Error = 2, msg
	return r
}

// commas is format(n, ",").
func commas(n any) string {
	s, err := pycompat.Format(n, ",")
	if err != nil {
		return fmt.Sprint(n)
	}
	return s
}

// registryEntry reads what extract_and_index takes from the registry
// entry; a missing key is Python's KeyError, a non-string value (Python
// would fail later, or copy a label of another type) an error too.
func registryEntry(repo, vid string) (install, label, dataRoot string, err error) {
	reg, err := index.LoadRegistry(repo)
	if err != nil {
		return "", "", "", err
	}
	e, ok := reg.Entry(vid)
	if !ok {
		return "", "", "", fmt.Errorf("version %q is not in %s", vid, index.RegistryRel)
	}
	keys := []string{"install_dir", "data_root", "label"}
	for i, dst := range []*string{&install, &dataRoot, &label} {
		v, isStr := e.Raw.String(keys[i])
		if !isStr {
			return "", "", "", fmt.Errorf("%s: %s of version %s is missing or not a string", index.RegistryRel, keys[i], vid)
		}
		*dst = v
	}
	return install, label, dataRoot, nil
}

// rmtreeIgnore is shutil.rmtree(p, ignore_errors=True): only a real
// folder is removed (rmtree refuses a symlink or a file, and the error is
// ignored).
func rmtreeIgnore(p string) {
	if st, err := os.Lstat(p); err == nil && st.IsDir() {
		_ = os.RemoveAll(p)
	}
}

// ExtractAndIndex runs the raw steps of extract_and_index for registered
// version vid of the repository at repo. Errors are what Python raises
// (no registry, unknown version, unreadable jar, I/O); the result carries
// the Log lines written up to that point.
func ExtractAndIndex(repo, vid string, opt SetupOptions) (*SetupResult, error) {
	res := &SetupResult{}
	install, label, dataRootRel, err := registryEntry(repo, vid)
	if err != nil {
		return res, err
	}
	root, err := index.DataRoot(repo, vid)
	if err != nil {
		return res, err
	}
	res.Install, res.Root = install, root
	if !isDir(install) {
		return res.refuse("install dir not found: " + install), nil
	}
	cache, err := CachePath(repo, install)
	if err != nil {
		return res, err
	}
	ls := NewListings(cache)
	switch {
	case opt.FillMissing:
		if res.Filled, err = FillMissing(install, root, ls); err != nil {
			return res, err
		}
		for _, rel := range res.Filled {
			res.out("  filled %s", rel)
		}
		res.out("fill-missing: wrote %d file(s)", len(res.Filled))
	case !opt.VerifyOnly:
		if done, err := extractStep(res, repo, root, dataRootRel, install, ls, opt); done || err != nil {
			return res, err
		}
	}
	rep, err := Fidelity(install, root, ls)
	if err != nil {
		return res, err
	}
	if err := ls.Save(); err != nil {
		return res, err
	}
	res.Fidelity = rep
	res.out("fidelity %s: jars %d checked %s sha256-exact %s eol-only %s mismatched %d missing %d",
		vid, rep.Jars, commas(rep.Checked), commas(rep.Matched), commas(len(rep.EOLOnly)),
		len(rep.Mismatched), len(rep.Missing))
	if err := writeManifest(res, repo, vid, label, dataRootRel, opt.Now); err != nil {
		return res, err
	}
	if len(rep.Mismatched) > 0 || (!opt.VerifyOnly && len(rep.Missing) > 0) {
		res.Status = 1
	}
	return res, nil
}

// isRepoRoot is extract_and_index's data_root in (".", "") for every
// spelling: root (index.DataRoot: absolute, cleaned) is the repository
// itself when it is the same path once cleaned, or the same folder once
// symlinks are followed ("./", "x/..", the absolute repo path, a link to
// it). Python matches only the two literal values.
func isRepoRoot(repo, root string) (bool, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return false, err
	}
	if filepath.Clean(root) == abs {
		return true, nil
	}
	rs, err := os.Stat(root)
	if err != nil {
		return false, nil // absent: it cannot be the repository
	}
	as, err := os.Stat(abs)
	if err != nil {
		return false, err
	}
	return os.SameFile(rs, as), nil
}

// holdsRepo reports whether root is a folder above the repository (intended difference: Python
// only refuses the repository itself), comparing the cleaned paths and, where both exist, the
// paths with symlinks resolved.
func holdsRepo(repo, root string) (bool, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return false, err
	}
	if under(filepath.Clean(root), abs) {
		return true, nil
	}
	r, err1 := filepath.EvalSymlinks(root)
	a, err2 := filepath.EvalSymlinks(abs)
	return err1 == nil && err2 == nil && under(r, a), nil
}

// under reports whether path lies strictly below dir.
func under(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// extractStep is the full extraction: refusals, removal of the raw and
// derived folders, Extract, then the optional rebuild. done is true after
// a refusal (res holds it).
func extractStep(res *SetupResult, repo, root, dataRootRel, install string, ls *Listings, opt SetupOptions) (done bool, err error) {
	same, err := isRepoRoot(repo, root)
	if err != nil {
		return false, err
	}
	if same || dataRootRel == "." || dataRootRel == "" {
		res.refuse("refusing to extract over the repo root; use --verify-only")
		return true, nil
	}
	above, err := holdsRepo(repo, root)
	if err != nil {
		return false, err
	}
	if above { // a forced extraction removes fixed folder names there, the repository's among them
		res.refuse("refusing to extract over a folder that holds the repo; use --verify-only")
		return true, nil
	}
	var present []string
	for _, d := range RawDirs {
		if isDir(pycompat.Join(root, d)) {
			present = append(present, d)
		}
	}
	if len(present) > 0 && !opt.Force {
		res.refuse(fmt.Sprintf("%s already holds %s; pass --force to re-extract", root, strings.Join(present, ", ")))
		return true, nil
	}
	for _, d := range append(append([]string{}, RawDirs...), DerivedDirs...) {
		rmtreeIgnore(pycompat.Join(root, d))
	}
	if err := os.MkdirAll(root, 0o777); err != nil {
		return false, err
	}
	ext, err := Extract(install, root, ls)
	if err != nil {
		return false, err
	}
	res.Extracted = ext
	res.out("extracted %s files (%s bytes)", commas(ext.Files), commas(ext.Bytes))
	for i, c := range ext.Collisions {
		if i == 20 {
			break
		}
		res.out("  COLLISION %s (%s overwritten by %s)", c.Rel, c.Owner, c.By)
	}
	if opt.Rebuild == nil {
		return false, nil
	}
	if res.Build, err = rebuild(root, opt.Rebuild); err != nil {
		return false, err
	}
	for _, line := range res.Build {
		res.out("%s", line)
	}
	return false, nil
}

// rebuild is _rebuild around the hook: indexes/ and the pdf-index.json
// stub first (pdf_extractor replaces it when docs/ exists), the empty
// indexes after. The stub is written atomically (Python's open("w") is
// not), so a reader never sees a half-written index.
func rebuild(root string, hook func(string) ([]string, error)) ([]string, error) {
	if err := os.MkdirAll(pycompat.Join(root, "indexes"), 0o777); err != nil {
		return nil, err
	}
	if pdf := pycompat.Join(root, "indexes", "pdf-index.json"); !isRegular(pdf) {
		if err := WriteText(pdf, "{}\n"); err != nil {
			return nil, err
		}
	}
	tail, err := hook(root)
	if err != nil {
		return nil, err
	}
	empty, err := EmptyIndexes(root)
	return append(tail, empty...), err
}

// EmptyIndexes writes {} for every BuilderIndexes file absent from
// root/indexes (_empty_indexes): an install without javadoc, devguide or
// source still answers lookups with an honest miss. It returns one
// "  EMPTY <name> ..." line per file written.
func EmptyIndexes(root string) ([]string, error) {
	var lines []string
	for _, name := range BuilderIndexes {
		path := pycompat.Join(root, "indexes", name)
		if isRegular(path) {
			continue
		}
		if err := WriteJSON(path, pyjson.NewObject(), DumpDefault); err != nil {
			return lines, err
		}
		lines = append(lines, fmt.Sprintf("  EMPTY %s (its source is not in this install)", name))
	}
	return lines, nil
}
