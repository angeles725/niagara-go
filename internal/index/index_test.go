package index

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeKeepsPythonDictOrder(t *testing.T) {
	v, err := Decode([]byte(`{"b": 1, "a": {"z": [true, null, "s"], "y": 2.5}, "b": 3}`))
	if err != nil {
		t.Fatal(err)
	}
	o := v.(*Object)
	if got := o.Keys(); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Errorf("keys %q, want first-insertion order", got)
	}
	if b, _ := o.Get("b"); b != json.Number("3") {
		t.Errorf("duplicate key: got %v, want the last value 3", b)
	}
	inner := o.Object("a")
	if got := inner.Keys(); !reflect.DeepEqual(got, []string{"z", "y"}) {
		t.Errorf("nested keys %q", got)
	}
	z, _ := inner.Get("z")
	if !reflect.DeepEqual(z, []any{true, nil, "s"}) {
		t.Errorf("array %#v", z)
	}
	if o.Len() != 2 || !o.Has("a") || o.Has("c") || o.Object("b") != nil {
		t.Error("accessors")
	}
}

func TestDecodeErrors(t *testing.T) {
	for name, text := range map[string]string{
		"extra data": `{} {}`,
		"truncated":  `{"a": [1,`,
		"empty":      ``,
		"nan":        `NaN`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(text)); err == nil {
				t.Errorf("%q decoded without error", text)
			}
		})
	}
}

func TestLoadObjectRejectsNonObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	writeFile(t, path, `[1]`)
	if _, err := LoadObject(path); err == nil {
		t.Error("an array loaded as an object")
	}
}

const registryJSON = `{"default": "4.15.6.20", "versions": {
 "4.14.0.162": {"label": "Old", "install_dir": "/i/414", "data_root": "."},
 "4.15.6.20": {"label": "New", "install_dir": "/i/415", "data_root": "versions/4.15.6.20", "course_dir": "x"},
 "4.13.2.18": {"label": "Ic", "install_dir": "/i/413", "data_root": "/abs/413/"}}}`

func repoWithRegistry(t *testing.T, body string) string {
	t.Helper()
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), body)
	return repo
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func env(vals map[string]string) func(string) string {
	return func(k string) string { return vals[k] }
}

func TestResolveVersion(t *testing.T) {
	repo := repoWithRegistry(t, registryJSON)
	tests := []struct {
		name, requested, env, want, err string
	}{
		{name: "registry default", want: "4.15.6.20"},
		{name: "env beats default", env: "4.14.0.162", want: "4.14.0.162"},
		{name: "request beats env", requested: "4.13.2.18", env: "4.14.0.162", want: "4.13.2.18"},
		{name: "unique prefix", requested: "4.14", want: "4.14.0.162"},
		{name: "ambiguous prefix", requested: "4",
			err: "Ambiguous Niagara version '4': matches 4.13.2.18, 4.14.0.162, 4.15.6.20"},
		{name: "unknown", requested: "9.9",
			err: "Unknown Niagara version '9.9'. Known versions: 4.13.2.18, 4.14.0.162, 4.15.6.20"},
		{name: "prefix needs a dot boundary", requested: "4.1",
			err: "Unknown Niagara version '4.1'. Known versions: 4.13.2.18, 4.14.0.162, 4.15.6.20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveVersion(repo, tt.requested, env(map[string]string{EnvVersion: tt.env}))
			if tt.err != "" {
				var uv *UnknownVersionError
				if !errors.As(err, &uv) || err.Error() != tt.err {
					t.Fatalf("err %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestResolveWithoutRegistry(t *testing.T) {
	repo := t.TempDir()
	id, root, err := Resolve(repo, "", env(nil))
	if err != nil || id != "" || root != repo {
		t.Errorf("no request: %q %q %v", id, root, err)
	}
	_, err = ResolveVersion(repo, "4.15", env(nil))
	if err == nil || err.Error() != "Unknown Niagara version '4.15': no versions/registry.json found" {
		t.Errorf("request without registry: %v", err)
	}
}

// absRegistry is registryJSON with the absolute data_root of 4.13.2.18
// replaced by abs plus a trailing separator, so the test holds on Windows
// (where "/abs/413/" is not absolute) as well as on POSIX.
func absRegistry(t *testing.T, abs string) string {
	t.Helper()
	q, err := json.Marshal(abs + string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Replace(registryJSON, `"/abs/413/"`, string(q), 1)
}

func TestDataRootAndExtraction(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "abs413")
	repo := repoWithRegistry(t, absRegistry(t, abs))
	for id, want := range map[string]string{
		"4.14.0.162": repo,
		"4.15.6.20":  filepath.Join(repo, "versions", "4.15.6.20"),
		"4.13.2.18":  abs,
		"":           repo,
	} {
		if got, err := DataRoot(repo, id); err != nil || got != want {
			t.Errorf("DataRoot(%q) = %q, %v; want %q", id, got, err, want)
		}
	}
	if IsExtracted(repo) {
		t.Error("extracted before class-index.json exists")
	}
	writeFile(t, Path(repo, "class-index.json"), "{}")
	if !IsExtracted(repo) || Dir(repo) != filepath.Join(repo, "indexes") {
		t.Error("IsExtracted/Dir after class-index.json exists")
	}
	reg, err := LoadRegistry(repo)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := reg.Entry("4.15.6.20")
	if !ok || e.Label != "New" || e.InstallDir != "/i/415" || !e.Raw.Has("course_dir") {
		t.Errorf("entry %+v", e)
	}
	if !reflect.DeepEqual(reg.IDs(), []string{"4.13.2.18", "4.14.0.162", "4.15.6.20"}) {
		t.Errorf("ids %q", reg.IDs())
	}
}

func TestMalformedRegistry(t *testing.T) {
	for name, body := range map[string]string{
		"no versions": `{"default": "1"}`,
		"bad json":    `{"versions": `,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadRegistry(repoWithRegistry(t, body)); err == nil {
				t.Error("no error")
			}
		})
	}
	repo := repoWithRegistry(t, `{"versions": {"1.0": {"data_root": "."}}}`)
	if _, err := ResolveVersion(repo, "", env(nil)); err == nil {
		t.Error("no default and no request resolved")
	}

}

func TestDataRootRequiresAString(t *testing.T) {
	repo := repoWithRegistry(t, `{"default": "1", "versions": {
 "1": {"label": "null root", "data_root": null},
 "2": {"label": "number root", "data_root": 7},
 "3": {"label": "no root"}}}`)
	for id, want := range map[string]string{
		"1": "versions/registry.json: data_root of version 1 is not a string",
		"2": "versions/registry.json: data_root of version 2 is not a string",
		"3": "versions/registry.json: version 3 has no data_root",
	} {
		want = strings.Replace(want, "versions/registry.json", RegistryRel, 1)
		if got, err := DataRoot(repo, id); err == nil || err.Error() != want {
			t.Errorf("DataRoot(%q) = %q, %v; want error %q", id, got, err, want)
		}
	}
}
