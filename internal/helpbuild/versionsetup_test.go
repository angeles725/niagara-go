package helpbuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// setupFixture is a repo with version 1.0.0.0 registered over the
// extractFixture install.
func setupFixture(t *testing.T) (repo, install string) {
	t.Helper()
	install, _ = extractFixture(t)
	repo = filepath.Join(t.TempDir(), "repo")
	writeRegistry(t, repo, map[string][3]string{"1.0.0.0": {"fixture", install, "versions/1.0.0.0"}})
	return repo, install
}

func fixedNow() time.Time {
	return time.Date(2026, 10, 17, 9, 8, 7, 654321, time.FixedZone("CST", -6*3600))
}

func TestExtractAndIndexWritesTheManifest(t *testing.T) {
	repo, install := setupFixture(t)
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Now: fixedNow})
	if err != nil || res.Status != 0 || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	wantLog := "extracted 3 files (34 bytes)|fidelity 1.0.0.0: jars 1 checked 3 sha256-exact 3 eol-only 0 mismatched 0 missing 0|" +
		"manifest -> " + filepath.FromSlash("versions/1.0.0.0.manifest.json") // os.path.relpath: native separator
	if strings.Join(res.Log, "|") != wantLog {
		t.Fatalf("log %q", res.Log)
	}
	text := readText(t, filepath.Join(repo, "versions", "1.0.0.0.manifest.json"))
	sum, err := fileSHA256(filepath.Join(install, "modules", "docFoo-doc.jar"))
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(filepath.Join(install, "modules", "docFoo-doc.jar"))
	want := "{\n \"version\": \"1.0.0.0\",\n \"label\": \"fixture\",\n \"install_dir\": " +
		pyjson.Dumps(pyjson.String(install), DumpDefault) + ",\n \"data_root\": \"versions/1.0.0.0\",\n" +
		" \"generated_utc\": \"2026-10-17T15:08:07+00:00\",\n \"fidelity\": {\n  \"jars\": 1,\n  \"checked\": 3,\n" +
		"  \"matched\": 3\n },\n \"eol_only_count\": 0,\n \"mismatched\": [],\n \"missing\": [],\n \"jars\": [\n  {\n" +
		"   \"jar\": \"modules/docFoo-doc.jar\",\n   \"bytes\": " + pyjson.Dumps(pyjson.IntValue(st.Size()), DumpDefault) +
		",\n   \"sha256\": \"" + sum + "\",\n   \"mapped_entries\": 3\n  }\n ]\n}"
	if text != want {
		t.Fatalf("manifest\n%s\nwant\n%s", text, want)
	}
}

// The refusals are status 2 with the Python messages; verify-only ignores
// missing files, a full extraction does not.
func TestExtractAndIndexRefusalsAndStatus(t *testing.T) {
	repo, install := setupFixture(t)
	if _, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{})
	if err != nil || res.Status != 2 || !strings.HasSuffix(res.Error, "already holds guides; pass --force to re-extract") {
		t.Fatalf("%+v %v", res, err)
	}
	root := filepath.Join(repo, "versions", "1.0.0.0")
	if err := os.Remove(filepath.Join(root, "guides", "Foo", "a.html")); err != nil {
		t.Fatal(err)
	}
	if res, _ := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{VerifyOnly: true}); res.Status != 0 {
		t.Fatalf("verify-only with a missing file: status %d", res.Status)
	}
	res, err = ExtractAndIndex(repo, "1.0.0.0", SetupOptions{FillMissing: true})
	if err != nil || res.Status != 0 || strings.Join(res.Filled, ",") != "guides/Foo/a.html" {
		t.Fatalf("fill %+v %v", res, err)
	}
	writeRegistry(t, repo, map[string][3]string{"1.0.0.0": {"root", install, "."}, "2.0.0.0": {"gone", install + "-x", "v/2"}})
	if res, _ := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Force: true}); res.Status != 2 ||
		res.Error != "refusing to extract over the repo root; use --verify-only" {
		t.Fatalf("repo root: %+v", res)
	}
	if res, _ := ExtractAndIndex(repo, "2.0.0.0", SetupOptions{}); res.Status != 2 || res.Error != "install dir not found: "+install+"-x" {
		t.Fatalf("not mounted: %+v", res)
	}
	if _, err := ExtractAndIndex(repo, "3.0.0.0", SetupOptions{}); err == nil {
		t.Fatal("an unregistered version is an error (KeyError)")
	}
}

// Every spelling of a data_root that resolves to the repository is
// refused before anything is removed, even with Force: Python matches
// only "." and "", and "./" or the absolute repo path would make it
// remove the raw and derived folders of the repository itself.
func TestExtractAndIndexRefusesEverySpellingOfTheRepoRoot(t *testing.T) {
	repo, install := setupFixture(t)
	abs, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	spellings := []string{".", "", "./", ".//", "x/..", "versions/..", abs, abs + string(filepath.Separator)}
	link := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(abs, link); err != nil {
		t.Logf("no symlinks here (%v): link spellings not checked", err)
	} else if err := os.Symlink(abs, filepath.Join(repo, "versions", "self")); err != nil {
		t.Fatal(err)
	} else {
		spellings = append(spellings, link, "versions/self")
	}
	keep := []string{"guides/keep.txt", "source/keep.txt", "indexes/keep.json", "bajadoc-clean/keep.txt"}
	for _, k := range keep {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(k)), []byte("repository file"))
	}
	for _, spelling := range spellings {
		writeRegistry(t, repo, map[string][3]string{"1.0.0.0": {"fixture", install, spelling}})
		res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Force: true})
		if err != nil || res.Status != 2 || res.Error != "refusing to extract over the repo root; use --verify-only" {
			t.Errorf("data_root %q: %+v %v", spelling, res, err)
		}
		for _, k := range keep {
			if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(k))); err != nil {
				t.Fatalf("data_root %q removed %s: %v", spelling, k, err)
			}
		}
	}
}

// Status 1 is a failed fidelity check: a mismatched file in any mode, a
// missing one only outside verify-only (here fill-missing, which never
// replaces a dangling symlink). The manifest is still written.
func TestExtractAndIndexStatusOne(t *testing.T) {
	repo, _ := setupFixture(t)
	if res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{}); err != nil || res.Status != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	root := filepath.Join(repo, "versions", "1.0.0.0")
	manifest := filepath.Join(repo, "versions", "1.0.0.0.manifest.json")
	writeFile(t, filepath.Join(root, "guides", "Foo", "a.html"), []byte("tampered"))
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{VerifyOnly: true})
	if err != nil || res.Status != 1 || strings.Join(res.Fidelity.Mismatched, ",") != "guides/Foo/a.html" ||
		!strings.Contains(readText(t, manifest), `"mismatched": [
  "guides/Foo/a.html"
 ]`) {
		t.Fatalf("mismatched: %+v %v", res, err)
	}
	writeFile(t, filepath.Join(root, "guides", "Foo", "a.html"), []byte("<html>a</html>"))
	gone := filepath.Join(root, "guides", "Foo", "sub", "b.html")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nowhere", gone); err != nil {
		t.Skipf("no symlinks here (%v): missing-file status not checked", err)
	}
	res, err = ExtractAndIndex(repo, "1.0.0.0", SetupOptions{FillMissing: true})
	if err != nil || res.Status != 1 || len(res.Filled) != 0 || strings.Join(res.Fidelity.Missing, ",") != "guides/Foo/sub/b.html" {
		t.Fatalf("missing after fill: %+v %v", res, err)
	}
	if res, _ := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{VerifyOnly: true}); res.Status != 0 {
		t.Fatalf("verify-only ignores missing: status %d", res.Status)
	}
}

// Force re-extracts over an existing corpus: the raw and derived folders
// are removed (a stray file in them goes), anything else in the data root
// stays, and the result is verified.
func TestExtractAndIndexForceReextracts(t *testing.T) {
	repo, _ := setupFixture(t)
	if _, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repo, "versions", "1.0.0.0")
	writeFile(t, filepath.Join(root, "guides", "Foo", "a.html"), []byte("tampered"))
	writeFile(t, filepath.Join(root, "guides", "stray.txt"), []byte("x"))
	writeFile(t, filepath.Join(root, "indexes", "old.json"), []byte("{}"))
	writeFile(t, filepath.Join(root, "keep.txt"), []byte("kept"))
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Force: true})
	if err != nil || res.Status != 0 || res.Error != "" || res.Extracted.Files != 3 || res.Fidelity.Matched != 3 {
		t.Fatalf("%+v %v", res, err)
	}
	for _, gone := range []string{"guides/stray.txt", "indexes"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Errorf("%s survived --force: %v", gone, err)
		}
	}
	if readText(t, filepath.Join(root, "guides", "Foo", "a.html")) != "<html>a</html>" || readText(t, filepath.Join(root, "keep.txt")) != "kept" {
		t.Fatal("force did not re-extract, or removed a file outside the raw and derived folders")
	}
}

// Collisions across jars: the later jar overwrites, every collision is in
// ExtractStats, the log shows only the first 20 (extract_and_index's
// [:20]); a same-jar duplicate is skipped and recorded as (rel, jar, jar).
func TestExtractAndIndexCollisions(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	var first, second [][2]string
	for i := range 25 {
		first = append(first, [2]string{fmt.Sprintf("doc/f%02d.html", i), "first"})
		second = append(second, [2]string{fmt.Sprintf("doc/f%02d.html", i), "second"})
	}
	first = append(first, [2]string{"doc/f00.html", "same-jar duplicate"})
	writeJar(t, filepath.Join(install, "modules", "docFoo-doc.jar"), first...)
	writeJar(t, filepath.Join(install, "modules", "docFoo.jar"), second...)
	repo := filepath.Join(dir, "repo")
	writeRegistry(t, repo, map[string][3]string{"1.0.0.0": {"collide", install, "v"}})
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{})
	if err != nil || res.Extracted == nil {
		t.Fatalf("%+v %v", res, err)
	}
	cs := res.Extracted.Collisions
	if len(cs) != 26 || cs[0] != (Collision{"guides/Foo/f00.html", "docFoo-doc.jar", "docFoo-doc.jar"}) {
		t.Fatalf("collisions %d %+v", len(cs), cs[:min(len(cs), 2)])
	}
	for i, c := range cs[1:] {
		if c != (Collision{fmt.Sprintf("guides/Foo/f%02d.html", i), "docFoo-doc.jar", "docFoo.jar"}) {
			t.Fatalf("collision %d: %+v", i+1, c)
		}
	}
	var logged []string
	for _, l := range res.Log {
		if strings.HasPrefix(l, "  COLLISION ") {
			logged = append(logged, l)
		}
	}
	if len(logged) != 20 || logged[0] != "  COLLISION guides/Foo/f00.html (docFoo-doc.jar overwritten by docFoo-doc.jar)" ||
		logged[19] != "  COLLISION guides/Foo/f18.html (docFoo-doc.jar overwritten by docFoo.jar)" {
		t.Fatalf("logged %d: %q", len(logged), logged)
	}
	if got := readText(t, filepath.Join(repo, "v", "guides", "Foo", "f24.html")); got != "second" {
		t.Fatalf("the later jar did not overwrite: %q", got)
	}
}

// The Rebuild hook runs after a full extraction: the pdf stub is there
// before it, its lines and the EMPTY lines follow in the log, a hook
// error stops the run.
func TestExtractAndIndexRebuildHook(t *testing.T) {
	repo, _ := setupFixture(t)
	var sawStub bool
	hook := func(root string) ([]string, error) {
		sawStub = readText(t, filepath.Join(root, "indexes", "pdf-index.json")) == "{}\n"
		writeFile(t, filepath.Join(root, "indexes", "class-index.json"), []byte(`{"a": 1}`))
		return []string{"  SKIP  bajadoc_parser.py", "Ran 1 builders"}, nil
	}
	res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Rebuild: hook})
	if err != nil || !sawStub {
		t.Fatalf("%v, stub seen %v", err, sawStub)
	}
	want := []string{"  SKIP  bajadoc_parser.py", "Ran 1 builders"}
	for _, n := range BuilderIndexes[1:] {
		if n != "pdf-index.json" {
			want = append(want, "  EMPTY "+n+" (its source is not in this install)")
		}
	}
	if strings.Join(res.Build, "|") != strings.Join(want, "|") || strings.Join(res.Log[1:1+len(want)], "|") != strings.Join(want, "|") {
		t.Fatalf("build %q\nlog %q", res.Build, res.Log)
	}
	stop := errors.New("builder crashed")
	if _, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Force: true,
		Rebuild: func(string) ([]string, error) { return nil, stop }}); !errors.Is(err, stop) {
		t.Fatalf("hook error: %v", err)
	}
}

// writeRegistry writes versions/registry.json of repo.
func writeRegistry(t *testing.T, repo string, versions map[string][3]string) {
	t.Helper()
	vs := pyjson.NewObject()
	for _, id := range pyjson.SortedKeys(keysOf(versions)) {
		e := pyjson.NewObject()
		e.Set("label", str(versions[id][0]))
		e.Set("install_dir", str(versions[id][1]))
		e.Set("data_root", str(versions[id][2]))
		vs.Set(id, e)
	}
	reg := pyjson.NewObject()
	reg.Set("default", str(pyjson.SortedKeys(keysOf(versions))[0]))
	reg.Set("versions", vs)
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(pyjson.Dumps(reg, DumpDefault)))
}

func keysOf(m map[string][3]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// A data root above the repository (".." and its other spellings) holds the repository, so a forced
// extraction there could remove it when the repository folder has one of the raw or derived names:
// refused like the repository itself (review-aa2a16b40c7c2120).
func TestExtractAndIndexRefusesAFolderAboveTheRepo(t *testing.T) {
	repo, install := setupFixture(t)
	abs, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(abs)
	keep := filepath.Join(parent, "source", "keep.txt")
	writeFile(t, keep, []byte("outside file"))
	for _, spelling := range []string{"..", "../", "versions/../..", parent, filepath.Dir(parent)} {
		writeRegistry(t, repo, map[string][3]string{"1.0.0.0": {"fixture", install, spelling}})
		res, err := ExtractAndIndex(repo, "1.0.0.0", SetupOptions{Force: true})
		if err != nil || res.Status != 2 || res.Error != "refusing to extract over a folder that holds the repo; use --verify-only" {
			t.Errorf("data_root %q: %+v %v", spelling, res, err)
		}
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("data_root %q removed %s: %v", spelling, keep, err)
		}
	}
}
