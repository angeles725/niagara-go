package helpbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// moduleXML is a baja.jar module.xml declaring version v.
func moduleXML(v string) string {
	return "<?xml version=\"1.0\"?>\n<module name=\"baja\" vendor=\"Tridium\"\n  vendorVersion = '" + v + "' />\n"
}

// addFixture is an install named folder (under a temp dir) whose baja.jar
// declares version v ("" for no baja.jar), with the docFoo-doc.jar of
// extractFixture, and an empty repository.
func addFixture(t *testing.T, folder, v string) (repo, install string) {
	t.Helper()
	dir := t.TempDir()
	install = filepath.Join(dir, folder)
	writeJar(t, filepath.Join(install, "modules", "docFoo-doc.jar"), [2]string{"META-INF/MANIFEST.MF", "m"},
		[2]string{"doc/", ""}, [2]string{"doc/a.html", "<html>a</html>"}, [2]string{"doc/sub/b.html", "<html>b</html>"})
	if v != "" {
		writeJar(t, filepath.Join(install, "modules", "baja.jar"), [2]string{"META-INF/module.xml", moduleXML(v)})
	}
	return filepath.Join(dir, "repo"), install
}

func addOpts() AddOptions {
	return AddOptions{Rebuild: RebuildToLog(nil), Fulltext: buildingDocs, Now: fixedNow}
}

func addRun(t *testing.T, repo, install string, opt AddOptions) (string, int) {
	t.Helper()
	var out bytes.Buffer
	status, err := AddVersion(repo, install, &out, opt)
	if err != nil {
		t.Fatalf("AddVersion: %v\n%s", err, out.String())
	}
	return out.String(), status
}

func TestValidVersionID(t *testing.T) {
	for id, want := range map[string]bool{"4.15.6.20": true, "1.2.3.4": true, "١.٢.٣.٤": true, "": false,
		"4.15.6": false, "4.15.6.20.1": false, "4.15.6.x": false, " 4.1.2.3": false, "4.1.2.3\n": false, "a4.1.2.3": false} {
		if got := ValidVersionID(id); got != want {
			t.Errorf("ValidVersionID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestDetectVersion(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, folder, xml, want string
		jar                     bool
	}{
		{"vendorVersion wins", "Niagara-9.9.9.9", moduleXML(" 4.15.6.20 "), "4.15.6.20", true},
		{"double quotes", "x", `<module vendorVersion="1.0.0.7"/>`, "1.0.0.7", true},
		{"invalid kept", "Niagara-9.9.9.9", moduleXML("4.x"), "4.x", true},
		{"no vendorVersion", "EC-Net4-4.3.58.18-copy", "<module/>", "4.3.58.18", true},
		{"no baja.jar", "iC-Niagara-4.13.2.18", "", "4.13.2.18", false},
		{"nothing", "install", "", "", false},
		{"first of the name", "a1.2.3.4.5b", "", "1.2.3.4", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			install := filepath.Join(dir, c.name, c.folder)
			if err := os.MkdirAll(filepath.Join(install, "modules"), 0o755); err != nil {
				t.Fatal(err)
			}
			if c.jar {
				writeJar(t, filepath.Join(install, "modules", "baja.jar"), [2]string{"META-INF/module.xml", c.xml})
			}
			if got, err := DetectVersion(install); err != nil || got != c.want {
				t.Errorf("DetectVersion = %q, %v; want %q", got, err, c.want)
			}
		})
	}
	// A baja.jar that is not a zip, or has no module.xml: the folder name.
	install := filepath.Join(dir, "bad", "N-2.2.2.2")
	writeFile(t, filepath.Join(install, "modules", "baja.jar"), []byte("not a zip"))
	if got, err := DetectVersion(install); err != nil || got != "2.2.2.2" {
		t.Errorf("not a zip: %q %v", got, err)
	}
	writeJar(t, filepath.Join(install, "modules", "baja.jar"), [2]string{"META-INF/MANIFEST.MF", "x"})
	if got, err := DetectVersion(install); err != nil || got != "2.2.2.2" {
		t.Errorf("no module.xml: %q %v", got, err)
	}
}

// Every refusal before registering is status 2 with Python's line, and
// leaves the repository untouched.
func TestAddVersionRefusals(t *testing.T) {
	repo, install := addFixture(t, "install", "")
	empty, bad := "", "4.15"
	cases := []struct {
		name, install, want string
		opt                 AddOptions
	}{
		{"not an install", filepath.Join(install, "modules"), "ERROR: " + filepath.Join(install, "modules") +
			" is not a Niagara install (no modules/ folder)\n", AddOptions{}},
		{"undetectable", install, "ERROR: cannot detect the Niagara version of " + install + " (no modules/baja.jar " +
			"vendorVersion, no a.b.c.d in the folder name); pass --id\n", AddOptions{}},
		{"empty --id", install, "ERROR: cannot detect the Niagara version of " + install + " (no modules/baja.jar " +
			"vendorVersion, no a.b.c.d in the folder name); pass --id\n", AddOptions{ID: &empty}},
		{"bad --id", install, "ERROR: version id '4.15' is not of the form a.b.c.d (digits); it names the data root " +
			"versions/<id>/. Pass --id a.b.c.d\n", AddOptions{ID: &bad}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if out, status := addRun(t, repo, c.install, c.opt); status != 2 || out != c.want {
				t.Errorf("status %d out %q, want %q", status, out, c.want)
			}
			if _, err := os.Stat(repo); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("repository written: %v", err)
			}
		})
	}
	registry := filepath.Join(repo, "versions", "registry.json")
	writeRegistry(t, repo, map[string][3]string{"1.2.3.4": {"Other", "/elsewhere", "versions/1.2.3.4"}})
	before := readText(t, registry)
	id := "1.2.3.4"
	if out, status := addRun(t, repo, install, AddOptions{ID: &id}); status != 2 ||
		out != "ERROR: version 1.2.3.4 is already registered for /elsewhere\n" {
		t.Errorf("other install: %d %q", status, out)
	}
	if got := readText(t, registry); got != before {
		t.Errorf("refused add (other install) rewrote the registry:\n%s", got)
	}
	writeRegistry(t, repo, map[string][3]string{"1.2.3.4": {"Same", install, "versions/1.2.3.4"}})
	before = readText(t, registry)
	if out, status := addRun(t, repo, install+"/.", AddOptions{ID: &id}); status != 2 ||
		out != "ERROR: version 1.2.3.4 is already registered; pass --force to extract and index it again\n" {
		t.Errorf("same install without --force: %d %q", status, out)
	}
	if got := readText(t, registry); got != before {
		t.Errorf("refused add (same install) rewrote the registry:\n%s", got)
	}
}

// Register keeps the other entries and their order, replaces an entry in
// place (dropping its extra keys, as Python does), makes the first version
// the default and ends the file with a newline.
func TestRegister(t *testing.T) {
	repo := t.TempDir()
	reg := filepath.Join(repo, "versions", "registry.json")
	if err := Register(repo, "2.0.0.0", "/i2", "Two", false); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"default\": \"2.0.0.0\",\n  \"versions\": {\n    \"2.0.0.0\": {\n      \"label\": \"Two\",\n" +
		"      \"install_dir\": \"/i2\",\n      \"data_root\": \"versions/2.0.0.0\"\n    }\n  }\n}\n"
	if got := readText(t, reg); got != want {
		t.Fatalf("new registry:\n%s", got)
	}
	writeFile(t, reg, []byte(`{"versions": {"9.0.0.0": {"label": "N", "install_dir": "/n", "data_root": "x", "course_dir": "c"},`+
		` "1.0.0.0": {"label": "Ü"}}, "default": "1.0.0.0"}`))
	if err := Register(repo, "9.0.0.0", "/n2", "N2", false); err != nil {
		t.Fatal(err)
	}
	if err := Register(repo, "3.0.0.0", "/i3", "Three", true); err != nil {
		t.Fatal(err)
	}
	want = "{\n  \"versions\": {\n    \"9.0.0.0\": {\n      \"label\": \"N2\",\n      \"install_dir\": \"/n2\",\n" +
		"      \"data_root\": \"versions/9.0.0.0\"\n    },\n    \"1.0.0.0\": {\n      \"label\": \"\\u00dc\"\n    },\n" +
		"    \"3.0.0.0\": {\n      \"label\": \"Three\",\n      \"install_dir\": \"/i3\",\n      \"data_root\": \"versions/3.0.0.0\"\n" +
		"    }\n  },\n  \"default\": \"3.0.0.0\"\n}\n"
	if got := readText(t, reg); got != want {
		t.Fatalf("updated registry:\n%s", got)
	}
}

// A full add on the fixture: registered, extracted, indexed, both gates
// run (gates.log), the summary printed; exit 0.
func TestAddVersionHappyPath(t *testing.T) {
	repo, install := addFixture(t, "Niagara-9.9.9.9", "1.2.3.4")
	out, status := addRun(t, repo, install, addOpts())
	root := filepath.Join(repo, "versions", "1.2.3.4")
	head := "registered 1.2.3.4 (Niagara-9.9.9.9) -> versions/1.2.3.4/\nextracted 2 files (28 bytes)\n"
	if !strings.HasPrefix(out, head) {
		t.Fatalf("out:\n%s", out)
	}
	for _, line := range []string{
		"fidelity 1.2.3.4: jars 1 checked 2 sha256-exact 2 eol-only 0 mismatched 0 missing 0\n",
		"manifest -> " + filepath.Join("versions", "1.2.3.4.manifest.json") + "\n",
		"\nSUMMARY 1.2.3.4\n  install:   " + install + "\n  data root: " + filepath.Join("versions", "1.2.3.4") + "\n" +
			"  extracted: 2 files (28 bytes) from 1 doc jars (sha256-exact 2 of 2)\n  classes:   0 in class-index.json\n" +
			"  guide folders: 1\n  searchable full-text files per source kind:\n",
		"    guides-clean             2  guide and module help pages\n",
		"    docs-text                0  PDF manuals  (none: no docs/ PDFs here and no registered version has any)\n",
		"  gates (full reports: " + filepath.Join("versions", "1.2.3.4", "gates.log") + "):\n",
		"    coverage --doc-jars  exit 0  PASS",
		"RESULT: READY - every page is searchable and both gates pass; query it with --niagara-version 1.2.3.4\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %q in:\n%s", line, out)
		}
	}
	if status != 0 {
		t.Errorf("status %d\n%s", status, out)
	}
	gates := readText(t, filepath.Join(root, GatesLog))
	if !strings.Contains(gates, "\nRESULT: PASS") || strings.Count(gates, "RESULT: ") != 2 {
		t.Errorf("gates.log:\n%s", gates)
	}
	if !pycompat.IsFile(filepath.Join(root, "build.log")) || !pycompat.IsFile(filepath.Join(root, "indexes", "fulltext.sqlite")) {
		t.Error("build.log or fulltext.sqlite missing")
	}
	// Again without --force: refused, nothing rolled back; with --force the
	// registered label is kept.
	if out, status := addRun(t, repo, install, addOpts()); status != 2 ||
		out != "ERROR: version 1.2.3.4 is already registered; pass --force to extract and index it again\n" {
		t.Errorf("again: %d %q", status, out)
	}
	opt := addOpts()
	opt.Force = true
	if out, status := addRun(t, repo, install, opt); status != 0 ||
		!strings.HasPrefix(out, "registered 1.2.3.4 (Niagara-9.9.9.9) -> versions/1.2.3.4/\n") {
		t.Errorf("forced: %d\n%s", status, out)
	}
}

// A refusal of the extraction (a data root that already holds a raw
// folder) restores the registry byte for byte, or removes it when the add
// created it, and keeps the data root it did not create.
func TestAddVersionRollsBackARefusal(t *testing.T) {
	repo, install := addFixture(t, "Niagara-1.2.3.4", "")
	root := filepath.Join(repo, "versions", "1.2.3.4")
	writeFile(t, filepath.Join(root, "bajadoc", "keep.html"), nil)
	out, status := addRun(t, repo, install, addOpts())
	want := "registered 1.2.3.4 (Niagara-1.2.3.4) -> versions/1.2.3.4/\nERROR: " + root +
		" already holds bajadoc; pass --force to re-extract (registry entry rolled back)\n"
	if status != 2 || out != want {
		t.Fatalf("status %d out %q", status, out)
	}
	if pycompat.IsFile(filepath.Join(repo, "versions", "registry.json")) || !pycompat.IsFile(filepath.Join(root, "bajadoc", "keep.html")) {
		t.Error("registry not removed, or the existing data root removed")
	}
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte("{\"default\": \"x\", \"versions\": {}}"))
	if _, status := addRun(t, repo, install, addOpts()); status != 2 {
		t.Fatalf("status %d", status)
	}
	if got := readText(t, filepath.Join(repo, "versions", "registry.json")); got != "{\"default\": \"x\", \"versions\": {}}" {
		t.Errorf("registry not restored: %q", got)
	}
}

// An extraction that raises rolls back the registry and the data root it
// created, then returns the error.
func TestAddVersionRollsBackAnError(t *testing.T) {
	repo, install := addFixture(t, "Niagara-1.2.3.4", "")
	stop := errors.New("builder crashed")
	opt := addOpts()
	opt.Rebuild = func(string) ([]string, error) { return nil, stop }
	var out bytes.Buffer
	if _, err := AddVersion(repo, install, &out, opt); !errors.Is(err, stop) {
		t.Fatalf("err %v", err)
	}
	if !strings.HasPrefix(out.String(), "registered 1.2.3.4 (Niagara-1.2.3.4) -> versions/1.2.3.4/\nextracted 2 files") {
		t.Errorf("out %q", out.String())
	}
	if _, err := os.Lstat(filepath.Join(repo, "versions", "1.2.3.4")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("data root kept: %v", err)
	}
	if pycompat.IsFile(filepath.Join(repo, "versions", "registry.json")) {
		t.Error("registry kept")
	}
}

// An extraction that returns neither a result nor an error is an error too:
// the registry entry and the data root it left behind are rolled back, and
// only the registered line was written.
func TestAddVersionRollsBackANilResult(t *testing.T) {
	repo, install := addFixture(t, "Niagara-1.2.3.4", "")
	root := filepath.Join(repo, "versions", "1.2.3.4")
	opt := addOpts()
	opt.Force = true // must reach SetupOptions.Force through the Extract seam
	called := false
	opt.Extract = func(r, vid string, so SetupOptions) (*SetupResult, error) {
		called = true
		if r != repo || vid != "1.2.3.4" || !so.Force || so.Rebuild == nil || so.Now == nil {
			t.Errorf("Extract(%q, %q, %+v)", r, vid, so)
		}
		writeFile(t, filepath.Join(root, "bajadoc", "partial.html"), nil)
		return nil, nil
	}
	var out bytes.Buffer
	status, err := AddVersion(repo, install, &out, opt)
	if !called || status != 2 || err == nil || err.Error() != "extract 1.2.3.4 returned no result" {
		t.Fatalf("called %v status %d err %v", called, status, err)
	}
	if want := "registered 1.2.3.4 (Niagara-1.2.3.4) -> versions/1.2.3.4/\n"; out.String() != want {
		t.Errorf("out %q, want %q", out.String(), want)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("data root kept: %v", err)
	}
	if pycompat.IsFile(filepath.Join(repo, "versions", "registry.json")) {
		t.Error("registry kept")
	}
}

func TestResultLine(t *testing.T) {
	for text, want := range map[string]string{"": "(no RESULT line)", "a\nRESULT: one\nRESULT: two\nx": "two",
		" RESULT: no\n": "(no RESULT line)", "RESULT: cr\rRESULT: last\r\n": "last"} {
		if got := resultLine(text); got != want {
			t.Errorf("resultLine(%q) = %q, want %q", text, got, want)
		}
	}
}
