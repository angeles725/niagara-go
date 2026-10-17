package helpbuild

import (
	"errors"
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
		"manifest -> versions/1.0.0.0.manifest.json"
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
