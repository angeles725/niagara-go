package secaudit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Truth tests: the audit never reads as a full check of something it did not look at.

// auditJSON runs the --json audit of home with no programs, no network and no Windows tools.
func auditJSON(t *testing.T, home string) (int, map[string]map[string]string, string) {
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
	code := run(e, home, "", "", "", true, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	rows := map[string]map[string]string{}
	if out.Len() > 0 {
		var list []map[string]string
		if err := json.Unmarshal(out.Bytes(), &list); err != nil {
			t.Fatalf("stdout is not the JSON rows: %v\n%s", err, out.String())
		}
		for _, r := range list {
			rows[r["id"]] = r
		}
	}
	return code, rows, errb.String()
}

func mkHome(t *testing.T, files map[string]string, dirs ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range files {
		p := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestAuditRefusesADirectoryThatIsNotANiagaraHome(t *testing.T) {
	home := mkHome(t, map[string]string{"notes.txt": "hello"}, "docs")
	code, rows, errOut := auditJSON(t, home)
	if code != cli.ExitCannotSearch {
		t.Errorf("exit = %d, want %d", code, cli.ExitCannotSearch)
	}
	if len(rows) != 0 {
		t.Errorf("printed %d rows for a non-Niagara directory", len(rows))
	}
	if !strings.Contains(errOut, "not a Niagara home") {
		t.Errorf("stderr %q does not name the refusal", errOut)
	}
}

func TestAuditReportsEveryCheckItCouldNotRun(t *testing.T) {
	props := "niagara.moduleVerificationMode=high\nprogram.requireSigning=true\n"
	home := mkHome(t, map[string]string{"defaults/system.properties": props})
	_, rows, errOut := auditJSON(t, home)
	if errOut != "" {
		t.Errorf("stderr %q", errOut)
	}
	want := map[string]struct{ verdict, observed string }{
		"SEC-02": {"SKIP", "no security/truststore.jks"},
		"SEC-03": {"SKIP", "no security/ directory"},
		"SEC-04": {"MANUAL", "no --host"},
		"SEC-06": {"SKIP", "no security/licenses/ directory"},
		"SEC-09": {"MANUAL", "no --host"},
		"SEC-12": {"MANUAL", "no --station"},
		"SEC-14": {"MANUAL", "no --host"},
		"SEC-15": {"SKIP", "no modules/ directory"},
	}
	for id, w := range want {
		r, ok := rows[id]
		if !ok {
			t.Errorf("%s: no row; a check that did not run must say so", id)
			continue
		}
		if r["verdict"] != w.verdict || !strings.Contains(r["observed"], w.observed) {
			t.Errorf("%s = %s %q, want %s containing %q", id, r["verdict"], r["observed"], w.verdict, w.observed)
		}
	}
	if rows["SEC-01"]["verdict"] != "PASS" {
		t.Errorf("SEC-01 = %v, want PASS from defaults/system.properties", rows["SEC-01"])
	}
}

func TestAuditWithoutPropertiesSkipsThePropertyChecks(t *testing.T) {
	home := mkHome(t, nil, "modules")
	_, rows, _ := auditJSON(t, home)
	for _, id := range []string{"SEC-01", "SEC-05", "SEC-07"} {
		r := rows[id]
		if r["verdict"] != "SKIP" || !strings.Contains(r["observed"], "system.properties") {
			t.Errorf("%s = %v, want SKIP naming the missing system.properties", id, r)
		}
	}
}

func TestAuditExitSaysWhetherAnythingFailed(t *testing.T) {
	secure := "niagara.moduleVerificationMode=high\nprogram.requireSigning=true\n" +
		"niagara.commandLinePropertyBlacklist=skipModuleValidation,commissioning.ignoreVerificationMode\n"
	insecure := "niagara.moduleVerificationMode=low\n"
	tests := []struct {
		name  string
		props string
		args  []string
		code  int
	}{
		// SEC-16 FAILs on every home (unsigned data by design): it alone does not fail the run.
		{"only the architectural SEC-16", secure, nil, cli.ExitFound},
		{"a critical FAIL", insecure, nil, 1},
		{"a critical FAIL under --json", insecure, []string{"--json"}, 1},
		{"a critical FAIL with --exit-zero", insecure, []string{"--exit-zero"}, cli.ExitFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := mkHome(t, map[string]string{"defaults/system.properties": tt.props}, "security/licenses", "modules")
			var out, errb bytes.Buffer
			code := Main(append([]string{home}, tt.args...), cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			if code != tt.code {
				t.Errorf("exit = %d, want %d\n%s%s", code, tt.code, out.String(), errb.String())
			}
		})
	}
}

func TestAuditNotADirectoryIsAUsageExit(t *testing.T) {
	var out, errb bytes.Buffer
	code := Main([]string{filepath.Join(t.TempDir(), "gone")}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch {
		t.Errorf("exit = %d, want %d (1 now means FAIL findings)", code, cli.ExitCannotSearch)
	}
}
