package secaudit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeEnv runs a fake icacls that prints out/errOut and exits with code (or fails with err).
func fakeEnv(out, errOut []byte, code int, err error) *env {
	return &env{
		run: func(argv []string, _ []byte, _ time.Duration, _ ...string) (result, error) {
			if err != nil {
				return result{}, err
			}
			return result{out, errOut, code}, nil
		},
		dial:     func(string, int) bool { return false },
		lookPath: func(string) string { return "icacls.exe" },
		winpath:  func(string) string { return testWinpath },
	}
}

func sec03Row(t *testing.T, e *env) row {
	t.Helper()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "security"), 0o755); err != nil {
		t.Fatal(err)
	}
	rep := &report{}
	e.sec03(home, rep)
	if len(rep.rows) != 1 {
		t.Fatalf("rows = %v", rep.rows)
	}
	return rep.rows[0]
}

func TestSec03Verdicts(t *testing.T) {
	cases := []struct {
		name              string
		out, err          []byte
		code              int
		verdict, observed string
		contains          string
	}{
		{"generic write", encode(t, testWinpath+" NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n             BUILTIN\\Usuarios:(OI)(CI)(IO)(GW)\r\nSe procesó correctamente 1 archivos\r\n", "cp850"),
			nil, 0, "FAIL", `Modify/Write for BUILTIN\Usuarios`, ""},
		{"lowercase modify", []byte(testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n              BUILTIN\\Users:(OI)(CI)(m)\r\n"),
			nil, 0, "FAIL", `Modify/Write for BUILTIN\Users`, ""},
		{"partly parsed", []byte(testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n              Everyone:(OI)(CI)(special access:)\r\nSuccessfully processed 1 files; Failed processing 0 files\r\n"),
			nil, 0, "SKIP", "", "not understood: 'Everyone:(OI)(CI)(special access:)'"},
		{"summary lines", []byte(testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n\r\nSuccessfully processed 1 files; Failed processing 0 files\r\n"),
			nil, 0, "PASS", "restricted", ""},
		{"loose principal named", []byte("C:\\x Everyone:(OI)(CI)(W)\r\n BUILTIN\\Users:(RX)\r\n"),
			nil, 0, "FAIL", `Modify/Write for C:\x Everyone`, ""},
		{"no ACE lines", encode(t, "¿Qué?\r\n", "cp850"), nil, 0, "SKIP",
			"not checked: icacls.exe printed no ACE lines: '¿Qué?'", ""},
		{"failing icacls", nil, encode(t, "Acceso denegado á\r\n", "cp850"), 5, "SKIP",
			"not checked: icacls.exe exit 5: Acceso denegado á", ""},
	}
	for _, c := range cases {
		r := sec03Row(t, fakeEnv(c.out, c.err, c.code, nil))
		if r.verdict != c.verdict || c.observed != "" && r.observed != c.observed ||
			!strings.Contains(r.observed, c.contains) {
			t.Errorf("%s: %s %q", c.name, r.verdict, r.observed)
		}
	}
}

func TestSec03ProbeFailuresAreSkips(t *testing.T) {
	r := sec03Row(t, fakeEnv(nil, nil, 0, &timeoutError{"Command ['icacls.exe'] timed out after 60 seconds"}))
	if r.verdict != "SKIP" || !strings.Contains(r.observed, "timed out") {
		t.Errorf("timeout: %s %q", r.verdict, r.observed)
	}
	r = sec03Row(t, fakeEnv(nil, nil, 0, &startError{"[Errno 13] Permission denied: 'icacls.exe'"}))
	if r.verdict != "SKIP" || !strings.Contains(r.observed, "Permission denied") {
		t.Errorf("start error: %s %q", r.verdict, r.observed)
	}
	e := fakeEnv(nil, nil, 0, errors.New("unused"))
	e.lookPath = func(string) string { return "" }
	saved := icaclsPaths
	icaclsPaths = []string{filepath.Join(t.TempDir(), "icacls.exe")}
	defer func() { icaclsPaths = saved }()
	r = sec03Row(t, e)
	if r.verdict != "SKIP" || !strings.Contains(r.observed, "icacls.exe not on PATH nor at") {
		t.Errorf("no icacls: %s %q", r.verdict, r.observed)
	}
}

func TestUsersInTheQueriedPathIsNotTheUsersGroup(t *testing.T) {
	winpath := `C:\Users\equipo\Niagara4.14\OptimizerSupervisor\security`
	listing := encode(t, winpath+" NT AUTHORITY\\SYSTEM:(I)(OI)(CI)(F)\r\n     BUILTIN\\Administradores:(I)(OI)(CI)(F)\r\n"+
		"     DESKTOP-X\\equipo:(I)(OI)(CI)(F)\r\n\r\nSe procesaron correctamente 1 archivos; error al procesar 0 archivos\r\n", "cp850")
	e := fakeEnv(listing, nil, 0, nil)
	e.winpath = func(string) string { return winpath }
	if r := sec03Row(t, e); r.verdict != "PASS" || r.observed != "restricted" {
		t.Errorf("%s %q", r.verdict, r.observed)
	}
}
