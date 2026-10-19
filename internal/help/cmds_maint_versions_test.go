package help

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The Go-only cases of `versions extract` (D46): testdata/go/golden.
func TestGoldenGoVersions(t *testing.T) { golden.RunGo(t, Main) }

// writeTestJar writes a zip holding the (name, data) pairs.
func writeTestJar(t *testing.T, path string, files ...[2]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, e := range files {
		w, err := z.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// versionsInstall is an install whose baja.jar declares 1.2.3.4, with one
// guide jar, and a checkout with an empty registry; it returns the checkout.
func versionsInstall(t *testing.T) (dir, install string) {
	t.Helper()
	dir = repo(t, `{"default": null, "versions": {}}`)
	install = filepath.Join(t.TempDir(), "Niagara-9.9.9.9")
	writeTestJar(t, filepath.Join(install, "modules", "baja.jar"),
		[2]string{"META-INF/module.xml", `<module name="baja" vendorVersion="1.2.3.4"/>`})
	writeTestJar(t, filepath.Join(install, "modules", "docFoo-doc.jar"),
		[2]string{"doc/a.html", "<html><title>A</title>quokka</html>"}, [2]string{"doc/b.html", "<html>b</html>"})
	return dir, install
}

// `versions add` end to end: registered, extracted, indexed, the full-text
// index built through scope (its notice on stderr), both gates passed,
// exit 0; the same add again is refused with exit 2.
func TestVersionsAddEndToEnd(t *testing.T) {
	dir, install := versionsInstall(t)
	out, errs, code := run(t, dir, "versions", "add", install, "--label", "Nine")
	root := filepath.Join(dir, "versions", "1.2.3.4")
	for _, want := range []string{"registered 1.2.3.4 (Nine) -> versions/1.2.3.4/\nextracted 2 files",
		"\nSUMMARY 1.2.3.4\n", "    guides-clean             2  guide and module help pages\n",
		"RESULT: READY - every page is searchable and both gates pass; query it with --niagara-version 1.2.3.4\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if code != 0 || !strings.Contains(errs, "building the full-text index for "+root) {
		t.Errorf("code %d stderr %q", code, errs)
	}
	for _, f := range []string{"build.log", "gates.log", filepath.Join("indexes", "fulltext.sqlite")} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Error(err)
		}
	}
	out, _, code = run(t, dir, "versions", "add", install)
	if code != 2 || out != "ERROR: version 1.2.3.4 is already registered; pass --force to extract and index it again\n" {
		t.Errorf("again: code %d out %q", code, out)
	}
	out, _, code = run(t, dir, "versions", "add", install, "--id", "")
	if code != 2 || !strings.HasPrefix(out, "ERROR: cannot detect the Niagara version of ") {
		t.Errorf("empty --id: code %d out %q", code, out)
	}
	out, _, code = run(t, dir, "versions", "list")
	if code != 0 || !strings.HasPrefix(out, "1.2.3.4 (default)  Nine\n") || !strings.Contains(out, "(extracted)") {
		t.Errorf("list: code %d out %q", code, out)
	}
}

// registerNine registers version 1.2.3.4 (Nine) on install in the checkout
// at dir. The install path goes through json.Marshal: a raw Windows path
// (backslashes) inside a JSON string literal is invalid JSON.
func registerNine(t *testing.T, dir, install string) {
	t.Helper()
	installJSON, err := json.Marshal(install)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "versions", "registry.json"), `{"default": "1.2.3.4", "versions": {"1.2.3.4": `+
		`{"label": "Nine", "install_dir": `+string(installJSON)+`, "data_root": "versions/1.2.3.4"}}}`)
}

// `versions extract` (extract_version.py): a refusal on stderr with exit
// 2, --force re-extracts, --verify-only and --fill-missing verify, a
// changed file fails the fidelity check (exit 1).
func TestVersionsExtract(t *testing.T) {
	dir, install := versionsInstall(t)
	registerNine(t, dir, install)
	root := filepath.Join(dir, "versions", "1.2.3.4")
	out, errs, code := run(t, dir, "versions", "extract", "1.2", "--no-index")
	if code != 0 || errs != "" || out != "extracted 2 files (49 bytes)\n"+
		"fidelity 1.2.3.4: jars 1 checked 2 sha256-exact 2 eol-only 0 mismatched 0 missing 0\n"+
		"manifest -> "+filepath.Join("versions", "1.2.3.4.manifest.json")+"\n" {
		t.Fatalf("extract: code %d out %q err %q", code, out, errs)
	}
	out, errs, code = run(t, dir, "versions", "extract", "1.2.3.4")
	if code != 2 || out != "" || errs != "ERROR: "+root+" already holds guides; pass --force to re-extract\n" {
		t.Errorf("refusal: code %d out %q err %q", code, out, errs)
	}
	out, _, code = run(t, dir, "versions", "extract", "1.2.3.4", "--force")
	if code != 0 || !strings.Contains(out, "Ran ") || !strings.Contains(out, "  EMPTY class-index.json") {
		t.Errorf("--force: code %d out %q", code, out)
	}
	write(t, filepath.Join(root, "guides", "Foo", "a.html"), "changed")
	out, _, code = run(t, dir, "versions", "extract", "1.2.3.4", "--verify-only")
	if code != 1 || !strings.HasPrefix(out, "fidelity 1.2.3.4: jars 1 checked 2 sha256-exact 1 eol-only 0 mismatched 1 missing 0\n") {
		t.Errorf("--verify-only: code %d out %q", code, out)
	}
	if err := os.Remove(filepath.Join(root, "guides", "Foo", "b.html")); err != nil {
		t.Fatal(err)
	}
	out, _, code = run(t, dir, "versions", "extract", "1.2.3.4", "--fill-missing")
	if code != 1 || !strings.HasPrefix(out, "  filled guides/Foo/b.html\nfill-missing: wrote 1 file(s)\n") {
		t.Errorf("--fill-missing: code %d out %q", code, out)
	}
	_, errs, code = run(t, dir, "versions", "extract", "7")
	if code != 2 || errs != "ERROR: Unknown Niagara version '7'. Known versions: 1.2.3.4\n" {
		t.Errorf("unknown: code %d err %q", code, errs)
	}
}

// `versions extract` errors that are not a refusal: a registry that cannot
// be read (not an UnknownVersionError) is one ERROR line on stderr, exit 2,
// nothing on stdout; an error after ExtractAndIndex logged lines keeps those
// lines on stdout, then the ERROR line on stderr, exit 2.
func TestVersionsExtractGenericErrors(t *testing.T) {
	t.Run("unreadable registry", func(t *testing.T) {
		dir := repo(t, `{"default": null}`)
		out, errs, code := run(t, dir, "versions", "extract", "1.2.3.4")
		// The registry path is the repository root as located, so only its
		// tail is pinned (a symlinked temp dir resolves differently).
		if code != 2 || out != "" || !strings.HasPrefix(errs, "ERROR: ") || strings.Count(errs, "\n") != 1 ||
			!strings.HasSuffix(errs, "registry.json: \"versions\" is missing or not an object\n") {
			t.Errorf("code %d out %q err %q", code, out, errs)
		}
	})
	t.Run("error after log lines", func(t *testing.T) {
		dir, install := versionsInstall(t)
		registerNine(t, dir, install)
		// The manifest path is a non-empty directory: writing it fails after
		// the extract and fidelity lines were logged.
		write(t, filepath.Join(dir, "versions", "1.2.3.4.manifest.json", "keep"), "x")
		out, errs, code := run(t, dir, "versions", "extract", "1.2.3.4", "--no-index")
		wantOut := "extracted 2 files (49 bytes)\n" +
			"fidelity 1.2.3.4: jars 1 checked 2 sha256-exact 2 eol-only 0 mismatched 0 missing 0\n"
		// The ERROR line names the manifest it could not write.
		if code != 2 || out != wantOut || !strings.HasPrefix(errs, "ERROR: ") ||
			strings.Count(errs, "\n") != 1 || !strings.HasSuffix(errs, "\n") ||
			!strings.Contains(errs, "1.2.3.4.manifest.json") {
			t.Errorf("code %d out %q err %q", code, out, errs)
		}
	})
}
