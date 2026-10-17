package secaudit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// probeEnv answers keytool and openssl from canned output and reports the given ports open;
// keytoolErr "" means the store opens with "changeit".
func probeEnv(keytoolErr, keytoolOut, subject string, open map[int]bool) *env {
	return &env{
		run: func(argv []string, stdin []byte, _ time.Duration, _ ...string) (result, error) {
			switch {
			case argv[0] == "keytool" && argv[2] == "-v":
				return result{stdout: []byte(keytoolOut)}, nil
			case argv[0] == "keytool" && keytoolErr == "":
				return result{}, nil // the store opened with "changeit"
			case argv[0] == "keytool":
				return result{stdout: []byte(keytoolErr), code: 1}, nil // keytool prints errors on stdout
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
	rows = rowsByID(t, probeEnv("keytool error: java.io.IOException: Keystore was tampered with, or password was incorrect", "", "", nil), home, "")
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

// keytoolEnv answers both keytool runs with one canned result.
func keytoolEnv(r result) *env {
	e := probeEnv("", "", "", nil)
	e.run = func(argv []string, _ []byte, _ time.Duration, _ ...string) (result, error) {
		if argv[0] != "keytool" {
			return result{}, &startError{"unexpected " + strings.Join(argv, " ")}
		}
		return r, nil
	}
	return e
}

// homeWithTruststore is a Niagara home holding only security/truststore.jks.
func homeWithTruststore(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "security"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "security", "truststore.jks"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// A keytool that fails never yields a PASS: SEC-02 says "custom password" only for keytool's
// wrong-password refusal (printed on stdout, as the real keytool does), SEC-11 reads key sizes
// only from a run that exited 0, and any other failure is a SKIP naming the keytool error.
func TestKeytoolFailureIsNeverAPass(t *testing.T) {
	home := homeWithTruststore(t)
	const (
		jksWrong = "keytool error: java.io.IOException: Keystore was tampered with, or password was incorrect\n"
		p12Wrong = "keytool error: java.io.IOException: keystore password was incorrect\n"
		badFmt   = "keytool error: java.security.KeyStoreException: Unrecognized keystore format. Please load it with a specified type\n"
	)
	for _, tc := range []struct {
		name          string
		r             result
		sec02, sec02v string
		sec11, sec11v string
	}{
		{"jks wrong password", result{stdout: []byte(jksWrong), code: 1},
			"custom password", "PASS",
			"key sizes not read (keytool exit 1: " + strings.TrimSpace(jksWrong) + "); FIPS keystore absent", "SKIP"},
		{"pkcs12 wrong password", result{stdout: []byte(p12Wrong), code: 1},
			"custom password", "PASS",
			"key sizes not read (keytool exit 1: " + strings.TrimSpace(p12Wrong) + "); FIPS keystore absent", "SKIP"},
		{"not a keystore", result{stdout: []byte(badFmt), code: 1},
			"not verified (keytool exit 1: " + head(strings.TrimSpace(badFmt), 120) + ")", "SKIP",
			"key sizes not read (keytool exit 1: " + head(strings.TrimSpace(badFmt), 120) + "); FIPS keystore absent", "SKIP"},
		{"stack trace after the error line", result{stdout: []byte(p12Wrong + "java.io.IOException: keystore password was incorrect\n\tat x\n"), code: 1},
			"custom password", "PASS",
			"key sizes not read (keytool exit 1: " + strings.TrimSpace(p12Wrong) + "); FIPS keystore absent", "SKIP"},
		{"silent failure", result{stderr: []byte("  \n"), code: 3},
			"not verified (keytool exit 3: )", "SKIP",
			"key sizes not read (keytool exit 3: ); FIPS keystore absent", "SKIP"},
		{"wrong-password words with exit 0", result{stdout: []byte("Keystore type: JKS\nalias: password was incorrect\n")},
			"opens with 'changeit'", "FAIL", "key sizes n/a; FIPS keystore absent", "PASS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := rowsByID(t, keytoolEnv(tc.r), home, "")
			if r := rows["SEC-02"]; r.observed != tc.sec02 || r.verdict != tc.sec02v {
				t.Errorf("SEC-02 = %s %q, want %s %q", r.verdict, r.observed, tc.sec02v, tc.sec02)
			}
			if r := rows["SEC-11"]; r.observed != tc.sec11 || r.verdict != tc.sec11v {
				t.Errorf("SEC-11 = %s %q, want %s %q", r.verdict, r.observed, tc.sec11v, tc.sec11)
			}
		})
	}
}

// The default store password reaches keytool through its environment (-storepass:env), never
// its argv: a command line is readable by every local user in the process list.
func TestKeytoolPasswordStaysOffTheCommandLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake keytool is a shell script")
	}
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "argv")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done >>" + log + "\n" +
		"printf 'env=%s\\n' \"$" + storepassVar + "\" >>" + log + "\n"
	if err := os.WriteFile(filepath.Join(bin, "keytool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	home := homeWithTruststore(t)
	e := defaultEnv()
	e.dial = func(string, int) bool { return false }
	rows := rowsByID(t, e, home, "")
	if r := rows["SEC-02"]; r.verdict != "FAIL" || r.observed != "opens with 'changeit'" {
		t.Errorf("SEC-02 = %s %q", r.verdict, r.observed)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	ts := filepath.Join(home, "security", "truststore.jks")
	want := "-list\n-keystore\n" + ts + "\n-storepass:env\n" + storepassVar + "\nenv=changeit\n" +
		"-list\n-v\n-keystore\n" + ts + "\n-storepass:env\n" + storepassVar + "\nenv=changeit\n"
	if string(data) != want {
		t.Errorf("keytool saw:\n%s\nwant:\n%s", data, want)
	}
	if strings.Contains(strings.ReplaceAll(string(data), "env=changeit", ""), "changeit") {
		t.Error("the password is on the keytool command line")
	}
	if v, ok := os.LookupEnv(storepassVar); ok {
		t.Errorf("%s=%q leaked into the auditor's own environment", storepassVar, v)
	}
}

// SEC-11 on a store keytool cannot open is a SKIP whether or not the FIPS keystore exists; the
// observed text names which.
func TestSec11FailureWithFIPSKeystoreIsASkip(t *testing.T) {
	home := homeWithTruststore(t)
	secdir := filepath.Join(home, "jre", "lib", "security")
	if err := os.MkdirAll(secdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secdir, "cacerts.bcfks"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := rowsByID(t, keytoolEnv(result{stdout: []byte("keytool error: boom\n"), code: 1}), home, "")
	want := "key sizes not read (keytool exit 1: keytool error: boom); FIPS keystore present"
	if r := rows["SEC-11"]; r.verdict != "SKIP" || r.observed != want {
		t.Errorf("SEC-11 = %s %q, want SKIP %q", r.verdict, r.observed, want)
	}
}

// The failure text keeps the first 120 code points (not bytes) of keytool's first output line.
func TestKeytoolFailureTextIsCutAt120CodePoints(t *testing.T) {
	long := strings.Repeat("é", 119) + "xyz" // 122 code points, 241 bytes
	exact := strings.Repeat("é", 120)
	for _, tc := range []struct{ stdout, want string }{
		{"  " + long + "\nsecond line\n", "exit 1: " + strings.Repeat("é", 119) + "x"},
		{exact + "\n", "exit 1: " + exact},
	} {
		o, err := keytoolEnv(result{stdout: []byte(tc.stdout), code: 1}).keytoolList("ts.jks", false)
		if err != nil || o.failure != tc.want {
			t.Errorf("failure = %q, %v; want %q", o.failure, err, tc.want)
		}
	}
}
