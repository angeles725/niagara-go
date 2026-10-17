package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInvokedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the $PATH lookup checks POSIX execute bits")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, name), nil, mode); err != nil {
			t.Fatal(err)
		}
	}
	write("tool", 0o755)
	write("plain", 0o644)
	if err := os.Mkdir(filepath.Join(bin, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	fallback := func() (string, error) { return "/proc/self/exe", nil }

	tests := []struct {
		name, arg0, pathEnv, want string
	}{
		{"absolute argv[0] is kept", "/opt/x/tool", bin, "/opt/x/tool"},
		{"relative argv[0] with a separator is made absolute", "bin/tool", "", filepath.Join(dir, "bin", "tool")},
		{"bare name is found on $PATH", "tool", "/nonexistent" + string(filepath.ListSeparator) + bin, filepath.Join(bin, "tool")},
		{"non-executable file is skipped", "plain", bin, "/proc/self/exe"},
		{"directory is skipped", "subdir", bin, "/proc/self/exe"},
		{"name missing from $PATH falls back to the executable", "absent", bin, "/proc/self/exe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := invokedPath(tt.arg0, tt.pathEnv, fallback)
			if err != nil || got != tt.want {
				t.Errorf("invokedPath(%q, %q) = %q, %v; want %q", tt.arg0, tt.pathEnv, got, err, tt.want)
			}
		})
	}
}

func TestInvokedPathEmptyEntryFindsCurrentDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the $PATH lookup checks POSIX execute bits")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tool"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	got, err := invokedPath("tool", string(filepath.ListSeparator), func() (string, error) { return "unused", nil })
	if want := filepath.Join(dir, "tool"); err != nil || got != want {
		t.Errorf("invokedPath = %q, %v; want %q", got, err, want)
	}
}
