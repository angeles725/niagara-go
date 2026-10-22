package secaudit

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// lockedDir is a directory under a temp root that this user cannot list (removed after the test).
func lockedDir(t *testing.T, root, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	d := filepath.Join(root, name)
	if err := os.Mkdir(d, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
	if _, err := os.ReadDir(d); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	return d
}

// A directory os.walk cannot list is named with Python's text: strerror on Linux, the Win32 text of
// FindFirstFileW ("<dir>\\*.*") on Windows, where a missing directory is ERROR_PATH_NOT_FOUND (3) in
// the system language, as Python 3.13 on Windows prints it ("El sistema no puede encontrar la ruta
// especificada" on a Spanish host; Go's own text was "No such file or directory").
func TestScanLogsUnlistableDirUsesPythonsErrorText(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	want := "No such file or directory"
	if runtime.GOOS == "windows" {
		want = pycompat.WinErrorText(3)
	}
	_, _, _, badDirs := scanLogs(missing)
	if len(badDirs) != 1 || badDirs[0].why != want {
		t.Fatalf("badDirs = %v, want one with %q", badDirs, want)
	}
}

// An unlistable root scans nothing: an error that counts it, never a clean verdict.
func TestScanLogsUnreadableRoot(t *testing.T) {
	root := lockedDir(t, t.TempDir(), "logs")
	var out, errb bytes.Buffer
	code := runScanLogs(root, false, cli.Stdio{Out: &out, Err: &errb})
	want := "WARNING: --scan-logs: 1 unreadable directory(ies) not scanned:\n  " + root + ": Permission denied\n" +
		"ERROR: --scan-logs: 0 readable log files (*.log, console/station/nre/daemon *.txt) under " + root + " (1 unreadable)\n"
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != want {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

// Hits found next to an unlistable directory are still printed (text and JSON), the directory is
// named on stderr and the exit says incomplete.
func TestScanLogsHitsBesideAnUnreadableDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nre.log"), []byte("No code signers for entry x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockedDir(t, root, "old")
	hits, nfiles, bad, badDirs := scanLogs(root)
	if len(hits) != 1 || nfiles != 1 || len(bad) != 0 || len(badDirs) != 1 || badDirs[0].path != root+"/old" {
		t.Fatalf("hits %v nfiles %d bad %v badDirs %v", hits, nfiles, bad, badDirs)
	}
	warning := "WARNING: --scan-logs: 1 unreadable directory(ies) not scanned:\n  " + root + "/old: Permission denied\n"
	for _, tc := range []struct {
		name   string
		asJSON bool
		stdout string
	}{
		{"text", false, "\n  Log-IOC harvest — 1 indicator(s) in 1 log file(s) under " + root + "\n\n" +
			"  " + color["high"] + "[high]" + color["off"] + " unsigned module loaded (No code signers)  " +
			root + "/nre.log:1\n         No code signers for entry x\n\n"},
		{"json", true, "[\n  {\n    \"sev\": \"high\",\n    \"ioc\": \"unsigned module loaded (No code signers)\",\n" +
			"    \"file\": \"" + root + "/nre.log\",\n    \"line\": 1,\n    \"text\": \"No code signers for entry x\"\n  }\n]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := runScanLogs(root, tc.asJSON, cli.Stdio{Out: &out, Err: &errb})
			if code != cli.ExitCannotSearch {
				t.Errorf("exit %d, want %d (incomplete)", code, cli.ExitCannotSearch)
			}
			if out.String() != tc.stdout {
				t.Errorf("stdout %q, want %q", out.String(), tc.stdout)
			}
			if errb.String() != warning {
				t.Errorf("stderr %q, want %q", errb.String(), warning)
			}
		})
	}
}

// A log file is named as os.walk names it, joined with the host's os.path.join: a backslash on
// Windows (ntpath), a slash elsewhere, in the subdirectory as at the root.
func TestScanLogsNamesFilesWithTheHostSeparator(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(root, "nre.log"), filepath.Join(root, "sub", "nre.log")} {
		if err := os.WriteFile(p, []byte("No code signers for entry x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hits, _, _, _ := scanLogs(root)
	want := []string{filepath.Join(root, "nre.log"), filepath.Join(root, "sub", "nre.log")}
	if len(hits) != 2 || hits[0].file != want[0] || hits[1].file != want[1] {
		t.Fatalf("hits %+v, want files %q", hits, want)
	}
}

// SEC-15 on a modules/ directory this user cannot list checked no module: a SKIP naming the
// reason ("could not verify"), never the PASS of "0 modules declare it".
func TestSec15UnlistableModulesDirSkips(t *testing.T) {
	home := t.TempDir()
	moddir := lockedDir(t, home, "modules")
	var rep report
	sec15(home, &rep)
	want := row{"SEC-15", "med", "KeyRingPermission wildcard in unsigned module",
		"not checked (cannot list " + moddir + ": Permission denied)",
		"no wildcard KeyRingPermission in an unsigned/attacker module", "SKIP", "B114",
		"run: ls -l <home>/modules (the audit user must be able to list it)"}
	if len(rep.rows) != 1 || rep.rows[0] != want {
		t.Errorf("rows %+v\nwant [%+v]", rep.rows, want)
	}
	// The whole audit prints the SKIP with its observed text and its note, and still exits 0:
	// a check it could not run is reported, not an audit failure.
	code, out := runAudit(t, home)
	for _, line := range []string{"[SKIP  ]\x1b[0m SEC-15 (med) KeyRingPermission wildcard in unsigned module  [B114]\n",
		"           observed: not checked (cannot list " + moddir + ": Permission denied)\n",
		"           note    : run: ls -l <home>/modules (the audit user must be able to list it)\n"} {
		if !strings.Contains(out, line) {
			t.Errorf("audit stdout lacks %q:\n%s", line, out)
		}
	}
	if code != cli.ExitFound {
		t.Errorf("audit exit %d, want %d", code, cli.ExitFound)
	}
}

// runAudit runs the audit of home with no keytool, no network and no Windows tools.
func runAudit(t *testing.T, home string) (int, string) {
	t.Helper()
	e := &env{
		run: func([]string, []byte, time.Duration, ...string) (result, error) {
			t.Fatal("the audit ran a program")
			return result{}, nil
		},
		dial:     func(string, int) bool { return false },
		lookPath: func(string) string { return "" },
		winpath:  func(string) string { return "" },
	}
	var out, errb bytes.Buffer
	code := run(e, home, "", "", "", false, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if errb.Len() != 0 {
		t.Errorf("audit stderr %q", errb.String())
	}
	return code, out.String()
}

// writeJar writes a jar holding META-INF/module.xml (xml) and, when signed, a signature block.
func writeJar(t *testing.T, path, xml string, signed bool) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	members := map[string]string{"META-INF/module.xml": xml}
	if signed {
		members["META-INF/SIGNER.RSA"] = "sig"
	}
	for name, body := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

const wildXML = `<module><permissions><permission class="KeyRingPermission" name="*"/></permissions></module>`

// SEC-15 cannot vouch for a jar it could not open (not a zip, unreadable): with no unsigned
// wildcard holder among the jars it read, the row is a SKIP naming those jars, never the PASS
// of a scan that skipped them; an unsigned wildcard holder it did read is still the FAIL, with
// the jars not read named in the observed text.
func TestSec15UnreadableJarIsNotChecked(t *testing.T) {
	home := t.TempDir()
	moddir := filepath.Join(home, "modules")
	if err := os.Mkdir(moddir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moddir, "broken-rt.jar"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJar(t, filepath.Join(moddir, "plain-rt.jar"), "<module/>", false)
	writeJar(t, filepath.Join(moddir, "signedWild-rt.jar"), wildXML, true)
	var rep report
	sec15(home, &rep)
	want := row{"SEC-15", "med", "KeyRingPermission wildcard in unsigned module",
		"not checked (cannot read 1 jar(s): broken-rt.jar); 1 modules declare it, 1 wildcard(name=*), 0 of those UNSIGNED",
		"no wildcard KeyRingPermission in an unsigned/attacker module", "SKIP", "B114",
		"run: unzip -t <home>/modules/<jar> on each jar not read (the audit user must be able to read it)"}
	if len(rep.rows) != 1 || rep.rows[0] != want {
		t.Errorf("rows %+v\nwant [%+v]", rep.rows, want)
	}
	code, out := runAudit(t, home)
	for _, line := range []string{"[SKIP  ]\x1b[0m SEC-15 (med) KeyRingPermission wildcard in unsigned module  [B114]\n",
		"           observed: " + want.observed + "\n", "           note    : " + want.note + "\n"} {
		if !strings.Contains(out, line) {
			t.Errorf("audit stdout lacks %q:\n%s", line, out)
		}
	}
	if code != cli.ExitFound {
		t.Errorf("audit exit %d, want %d", code, cli.ExitFound)
	}

	writeJar(t, filepath.Join(moddir, "wildOpen-rt.jar"), wildXML, false)
	rep = report{}
	sec15(home, &rep)
	want.observed = "2 modules declare it, 2 wildcard(name=*), 1 of those UNSIGNED; 1 jar(s) not checked (cannot read: broken-rt.jar)"
	want.verdict, want.note = "FAIL", "unsigned wildcard holders: wildOpen-rt.jar"
	if len(rep.rows) != 1 || rep.rows[0] != want {
		t.Errorf("with an unsigned wildcard holder: rows %+v\nwant [%+v]", rep.rows, want)
	}
}
