package secaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeEnv answers keytool and openssl from canned output and reports the given ports open.
func probeEnv(keytoolErr, keytoolOut, subject string, open map[int]bool) *env {
	return &env{
		run: func(argv []string, stdin []byte, _ time.Duration) (result, error) {
			switch {
			case argv[0] == "keytool" && argv[2] == "-v":
				return result{stdout: []byte(keytoolOut)}, nil
			case argv[0] == "keytool":
				return result{stderr: []byte(keytoolErr), code: 1}, nil
			case argv[0] == "openssl" && argv[1] == "s_client":
				return result{stdout: []byte("-----BEGIN CERTIFICATE-----\n")}, nil
			case argv[0] == "openssl":
				return result{stdout: []byte(subject + "\n")}, nil
			}
			return result{}, &startError{"unexpected " + strings.Join(argv, " ")}
		},
		dial:     func(_ string, port int) bool { return open[port] },
		lookPath: func(string) string { return "" },
		winpath:  toWinpath,
	}
}

func rowsByID(t *testing.T, e *env, home, host string) map[string]row {
	t.Helper()
	rep := &report{}
	if err := e.audit(home, "", host, nil, rep); err != nil {
		t.Fatal(err)
	}
	m := map[string]row{}
	for _, r := range rep.rows {
		m[r.id] = r
	}
	return m
}

func TestKeytoolChecks(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "security"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "security", "truststore.jks"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := rowsByID(t, probeEnv("", "RSA 1024-bit key\nRSA 2048-bit key\nRSA 1024-bit key\n", "", nil), home, "")
	if r := rows["SEC-02"]; r.verdict != "FAIL" || r.observed != "opens with 'changeit'" {
		t.Errorf("SEC-02 = %+v", r)
	}
	if r := rows["SEC-11"]; r.verdict != "FAIL" || r.observed != "key sizes [1024, 2048]; FIPS keystore absent" {
		t.Errorf("SEC-11 = %+v", r)
	}
	rows = rowsByID(t, probeEnv("keytool error: java.io.IOException: Keystore was tampered with", "", "", nil), home, "")
	if r := rows["SEC-02"]; r.verdict != "PASS" || r.observed != "custom password" {
		t.Errorf("SEC-02 custom = %+v", r)
	}
}

func TestLiveProbes(t *testing.T) {
	home := t.TempDir()
	rows := rowsByID(t, probeEnv("", "", "subject=CN = ForRecoveryPurposes", map[int]bool{3011: true, 1911: true}), home, "10.0.0.5")
	if r := rows["SEC-04:443"]; r.verdict != "FAIL" || r.observed != "default self-signed (ForRecoveryPurposes)" {
		t.Errorf("SEC-04 = %+v", r)
	}
	if r := rows["SEC-09"]; r.sev != "high" || r.observed != "3011 open on reachable iface" {
		t.Errorf("SEC-09 = %+v", r)
	}
	if r, ok := rows["SEC-14:1911"]; !ok || r.verdict != "FAIL" {
		t.Errorf("SEC-14:1911 = %+v", r)
	}
	if _, ok := rows["SEC-14:80"]; ok {
		t.Error("SEC-14:80 reported for a closed port")
	}
	rows = rowsByID(t, probeEnv("", "", "subject=CN = station.example", map[int]bool{3011: true}), home, "localhost")
	if r := rows["SEC-04:5011"]; r.verdict != "PASS" || r.observed != "subject=CN = station.example" {
		t.Errorf("SEC-04 custom = %+v", r)
	}
	if r := rows["SEC-09"]; r.sev != "med" || r.observed != "3011 open (loopback only, mitigated)" {
		t.Errorf("SEC-09 loopback = %+v", r)
	}
}

func TestToWinpath(t *testing.T) {
	for in, want := range map[string]string{"/mnt/c/Niagara/security": `C:\Niagara\security`,
		"/home/x/security": "", "/mnt/cd/x": ""} {
		if got := toWinpath(in); got != want {
			t.Errorf("toWinpath(%q) = %q, want %q", in, got, want)
		}
	}
}
