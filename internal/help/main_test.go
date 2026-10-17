package help

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func TestPopVersionArg(t *testing.T) {
	tests := []struct {
		name      string
		argv      []string
		value     string
		rest      []string
		errSubstr string
	}{
		{name: "before the command", argv: []string{"--niagara-version", "4.14", "class", "X"},
			value: "4.14", rest: []string{"class", "X"}},
		{name: "after the command", argv: []string{"class", "X", "--niagara-version=4.15"},
			value: "4.15", rest: []string{"class", "X"}},
		{name: "last one wins", argv: []string{"--niagara-version=1", "a", "--niagara-version", "2"},
			value: "2", rest: []string{"a"}},
		{name: "empty value", argv: []string{"--niagara-version=", "a"}, value: "", rest: []string{"a"}},
		{name: "abbreviation is left to argparse", argv: []string{"--niagara-v", "1"},
			rest: []string{"--niagara-v", "1"}},
		{name: "missing value", argv: []string{"class", "--niagara-version"},
			errSubstr: "--niagara-version needs a value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, rest, err := popVersionArg(tt.argv)
			if tt.errSubstr != "" {
				if err == nil || err.Error() != tt.errSubstr {
					t.Fatalf("err %v, want %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil || value != tt.value || !reflect.DeepEqual(rest, tt.rest) {
				t.Fatalf("got %q %q %v; want %q %q", value, rest, err, tt.value, tt.rest)
			}
		})
	}
}

// repo builds a niagara-help checkout: a registry with the given versions
// (id -> data_root) and class-index.json in every extracted data root.
func repo(t *testing.T, registryJSON string, extracted ...string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "versions", "registry.json"), registryJSON)
	if err := os.MkdirAll(filepath.Join(dir, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, root := range extracted {
		write(t, filepath.Join(dir, root, "indexes", "class-index.json"), "{}")
	}
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run calls Main in dir with no NIAGARA_HELP_* variables and no executable walk-up.
func run(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv(EnvDir, "")
	t.Setenv(envVersionForTest, "")
	t.Setenv("COLUMNS", "100")
	defer func(f func() (string, error)) { executable = f }(executable)
	executable = func() (string, error) { return "", nil }
	t.Chdir(dir)
	var out, errb bytes.Buffer
	code = Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

const twoVersions = `{"default": "2.0.0.1", "versions": {
 "2.0.0.1": {"label": "Two", "install_dir": "/nonexistent-install", "data_root": "versions/2.0.0.1"},
 "1.0.0.1": {"label": "One", "install_dir": ".", "data_root": "."}}}`

func TestVersionResolutionErrors(t *testing.T) {
	dir := repo(t, twoVersions, ".")
	out, _, code := run(t, dir, "class", "X") // default 2.0.0.1 is not extracted
	want := "ERROR: Niagara version 2.0.0.1 is registered but not extracted at " +
		filepath.Join(dir, "versions", "2.0.0.1") + ".\nRun: python3 tools/extract_version.py 2.0.0.1\n"
	if code != 2 || out != want {
		t.Errorf("not extracted: code %d out %q", code, out)
	}
	out, _, code = run(t, dir, "class", "X", "--niagara-version")
	if code != 2 || out != "ERROR: --niagara-version needs a value\n" {
		t.Errorf("missing value: code %d out %q", code, out)
	}
	t.Setenv(envVersionForTest, "7")
	t.Chdir(dir)
	var o bytes.Buffer
	defer func(f func() (string, error)) { executable = f }(executable)
	executable = func() (string, error) { return "", nil }
	if code := Main([]string{"class", "X"}, cli.Stdio{Out: &o, Err: &o}); code != 2 ||
		o.String() != "ERROR: Unknown Niagara version '7'. Known versions: 1.0.0.1, 2.0.0.1\n" {
		t.Errorf("env version: code %d out %q", code, o.String())
	}
}

func TestVersionsList(t *testing.T) {
	dir := repo(t, twoVersions, ".")
	out, _, code := run(t, dir, "versions", "list")
	want := "1.0.0.1  One\n" +
		"    install:   .\n" +
		"    data root: .  (extracted)\n" +
		"2.0.0.1 (default)  Two\n" +
		"    install:   /nonexistent-install  (not mounted)\n" +
		"    data root: versions/2.0.0.1  (not extracted)\n"
	if code != 0 || out != want {
		t.Errorf("code %d out:\n%s", code, out)
	}
	empty := repo(t, `{"default": "", "versions": {}}`)
	out, _, code = run(t, empty, "versions", "list")
	if code != 1 || out != "No version registered (versions/registry.json); add one with 'versions add <install>'.\n" {
		t.Errorf("empty registry: code %d out %q", code, out)
	}
}

func TestVersionsAddRefusesNonInstall(t *testing.T) {
	dir := repo(t, twoVersions)
	out, _, code := run(t, dir, "versions", "add", "nope")
	if code != 2 || out != "ERROR: "+filepath.Join(dir, "nope")+" is not a Niagara install (no modules/ folder)\n" {
		t.Errorf("code %d out %q", code, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "inst", "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, dir, "versions", "add", "inst")
	if code != 2 || out != "" || errs != "nh: 'versions add' is not ported yet\n" {
		t.Errorf("install: code %d out %q err %q", code, out, errs)
	}
}

func TestRepoRoot(t *testing.T) {
	dir := repo(t, twoVersions, ".")
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func(f func() (string, error)) { executable = f }(executable)

	t.Setenv(EnvDir, "/from/env")
	if got := repoRoot(); got != "/from/env" {
		t.Errorf("env: %q", got)
	}
	t.Setenv(EnvDir, "")
	executable = func() (string, error) { return filepath.Join(sub, "nh"), nil }
	t.Chdir(t.TempDir())
	if got := repoRoot(); got != dir {
		t.Errorf("executable walk-up: %q, want %q", got, dir)
	}
	executable = func() (string, error) { return "", nil }
	t.Chdir(sub)
	if got := repoRoot(); got != dir {
		t.Errorf("cwd walk-up: %q, want %q", got, dir)
	}
	t.Chdir(t.TempDir())
	if got := repoRoot(); got != "" {
		t.Errorf("no checkout: %q", got)
	}
	var out bytes.Buffer
	if code := Main(nil, cli.Stdio{Out: &out}); code != 1 ||
		out.String() != "ERROR: Cannot find niagara-help directory.\nSet NIAGARA_HELP_DIR or run from tools/ directory.\n" {
		t.Errorf("no checkout Main: code %d out %q", code, out.String())
	}
}
