package locate

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// checkout makes a directory holding the marker file rel, plus the nested
// folders a start can sit in, and returns its resolved path.
func checkout(t *testing.T, rel string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mkdir(t, filepath.Join(dir, "bin", "deep"))
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rel), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func env(vals map[string]string) func(string) string {
	return func(name string) string { return vals[name] }
}

func cwdAt(dir string) func() (string, error) { return func() (string, error) { return dir, nil } }

func exeAt(path string) func() (string, error) { return func() (string, error) { return path, nil } }

func failing() (string, error) { return "", errors.New("unavailable") }

func TestFindWalksUpFromEachStartInOrder(t *testing.T) {
	t.Parallel()
	a, b := checkout(t, "MARK"), checkout(t, "MARK")
	base := Options{Getenv: env(nil), Marker: HasFile("MARK"),
		Getwd: cwdAt(filepath.Join(a, "bin", "deep")), Executable: exeAt(filepath.Join(b, "bin", "deep", "tool"))}
	tests := []struct {
		name   string
		starts []Start
		want   string
	}{
		{"cwd first", []Start{FromCwd, FromExecutable}, a},
		{"executable first", []Start{FromExecutable, FromCwd}, b},
		{"explicit path", []Start{FromPath(filepath.Join(b, "bin")), FromCwd}, b},
		{"empty explicit path is skipped", []Start{FromPath(""), FromCwd}, a},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base
			o.Starts = tt.starts
			if got, ok := Find(o); !ok || got != tt.want {
				t.Errorf("Find = %q %v, want %q true", got, ok, tt.want)
			}
		})
	}
}

func TestFindSkipsAStartThatCannotBeRead(t *testing.T) {
	t.Parallel()
	a := checkout(t, "MARK")
	o := Options{Getenv: env(nil), Marker: HasFile("MARK"), Starts: []Start{FromCwd, FromExecutable},
		Getwd: failing, Executable: exeAt(filepath.Join(a, "tool"))}
	if got, ok := Find(o); !ok || got != a {
		t.Errorf("unreadable cwd: Find = %q %v, want %q true", got, ok, a)
	}
	o.Getwd, o.Executable = cwdAt(a), failing
	o.Starts = []Start{FromExecutable, FromCwd}
	if got, ok := Find(o); !ok || got != a {
		t.Errorf("unreadable executable: Find = %q %v, want %q true", got, ok, a)
	}
}

func TestFindWithoutTheMarkerFindsNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "MARK")) // a directory, not the marker file
	o := Options{Getenv: env(nil), Marker: HasFile("MARK"), Starts: []Start{FromCwd, FromExecutable},
		Getwd: cwdAt(dir), Executable: exeAt(filepath.Join(dir, "tool"))}
	if got, ok := Find(o); ok || got != "" {
		t.Errorf("Find = %q %v, want \"\" false", got, ok)
	}
	o.Marker = nil
	if got, ok := Find(o); ok || got != "" {
		t.Errorf("nil marker: Find = %q %v, want \"\" false", got, ok)
	}
}

func TestFindTakesTheFirstNonEmptyVariableAsGiven(t *testing.T) {
	t.Parallel()
	a := checkout(t, "MARK")
	o := Options{Env: []string{"PRIMARY", "ALIAS"}, Marker: HasFile("MARK"), Starts: []Start{FromCwd},
		Getwd: cwdAt(a)}
	tests := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"unset", nil, a},
		{"empty is unset", map[string]string{"PRIMARY": "", "ALIAS": ""}, a},
		{"alias", map[string]string{"PRIMARY": "", "ALIAS": "rel/alias"}, "rel/alias"},
		{"primary wins", map[string]string{"PRIMARY": "/no/marker/here", "ALIAS": "x"}, "/no/marker/here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := o
			o.Getenv = env(tt.vars)
			if got, ok := Find(o); !ok || got != tt.want {
				t.Errorf("Find = %q %v, want %q true", got, ok, tt.want)
			}
		})
	}
}

func TestFindDefaultsToTheProcess(t *testing.T) {
	a := checkout(t, "MARK")
	t.Setenv("LOCATE_TEST_ROOT", "")
	t.Chdir(filepath.Join(a, "bin"))
	got, ok := Find(Options{Env: []string{"LOCATE_TEST_ROOT"}, Marker: HasFile("MARK"), Starts: []Start{FromCwd}})
	if !ok || got != a {
		t.Errorf("Find from the process cwd = %q %v, want %q true", got, ok, a)
	}
	t.Setenv("LOCATE_TEST_ROOT", "from-env")
	if got, _ := Find(Options{Env: []string{"LOCATE_TEST_ROOT"}}); got != "from-env" {
		t.Errorf("Find from the process env = %q, want from-env", got)
	}
}

func TestFindResolvesASymlinkedExecutable(t *testing.T) {
	t.Parallel()
	a := checkout(t, "MARK")
	real := filepath.Join(a, "bin", "deep", "tool")
	if err := os.WriteFile(real, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "tool")
	if err := os.Symlink(real, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need a privilege on Windows: ", err)
		}
		t.Fatal(err)
	}
	o := Options{Getenv: env(nil), Marker: HasFile("MARK"), Starts: []Start{FromExecutable}, Executable: exeAt(link)}
	if got, ok := Find(o); !ok || got != a {
		t.Errorf("symlinked executable: Find = %q %v, want %q true", got, ok, a)
	}
	// a path that cannot be resolved (it does not exist) is walked as given
	o.Executable = exeAt(filepath.Join(a, "bin", "missing", "tool"))
	if got, ok := Find(o); !ok || got != a {
		t.Errorf("unresolvable executable: Find = %q %v, want %q true", got, ok, a)
	}
}

func TestFirstEnv(t *testing.T) {
	t.Parallel()
	get := env(map[string]string{"B": "b", "C": "c"})
	if name, val := FirstEnv(get, "A", "B", "C"); name != "B" || val != "b" {
		t.Errorf("FirstEnv = %q %q, want B b", name, val)
	}
	if name, val := FirstEnv(get, "A"); name != "" || val != "" {
		t.Errorf("FirstEnv unset = %q %q, want empty", name, val)
	}
}

func TestWalkUp(t *testing.T) {
	t.Parallel()
	a := checkout(t, filepath.Join("sub", "MARK"))
	deep := filepath.Join(a, "bin", "deep")
	if got, ok := WalkUp(deep, HasFile(filepath.Join("sub", "MARK"))); !ok || got != a {
		t.Errorf("WalkUp = %q %v, want %q true", got, ok, a)
	}
	if got, ok := WalkUp(deep+string(filepath.Separator)+"..", HasDir("deep")); !ok || got != filepath.Join(a, "bin") {
		t.Errorf("WalkUp of an unclean path = %q %v, want %q true", got, ok, filepath.Join(a, "bin"))
	}
	if got, ok := WalkUp(deep, HasFile("no-such-marker")); ok || got != "" {
		t.Errorf("WalkUp to the root = %q %v, want \"\" false", got, ok)
	}
}

func TestMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "d"))
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		marker func(string) bool
		want   bool
	}{
		{"HasFile file", HasFile("f"), true},
		{"HasFile dir", HasFile("d"), false},
		{"HasFile absent", HasFile("x"), false},
		{"HasDir dir", HasDir("d"), true},
		{"HasDir file", HasDir("f"), false},
		{"Exists file", Exists("f"), true},
		{"Exists dir", Exists("d"), true},
		{"Exists absent", Exists("x"), false},
		{"All true", All(HasFile("f"), HasDir("d")), true},
		{"All false", All(HasFile("f"), HasDir("x")), false},
		{"All empty", All(), true},
	}
	for _, tt := range tests {
		if got := tt.marker(dir); got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
		}
	}
}
