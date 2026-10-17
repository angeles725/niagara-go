package licensador

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

func stdio() (cli.Stdio, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb}, &out, &errb
}

func TestRefHostIDMatchesPython(t *testing.T) {
	// Expected values computed with licensador.ref_host_id's hashing in Python.
	for ids, want := range map[[2]string]string{
		{"3ad8e86cff544806ad53b27ca1dcf2c3x", "4931002881628"}: "REF-ED12FC4DDD15",
		{"1234ABCD", "5"}: "REF-7BFFCE2AD758",
	} {
		if got := RefHostID([]byte(saltDefault), ids[:]); got != want {
			t.Errorf("RefHostID(%v) = %s, want %s", ids, got, want)
		}
	}
}

// TestIssueVerifyRoundTrip covers genkeys -> issue -> verify -> revoke with
// the Go signer (PSS is randomised, so parity is checked by verification;
// the Python-signed fixtures in testdata cover the other direction).
func TestIssueVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, out, errb := stdio()
	if code := RunGenkeys(dir, 2048, s); code != 0 {
		t.Fatalf("genkeys exit %d: %s", code, errb)
	}
	if !strings.HasPrefix(out.String(), "raiz generada: "+filepath.Join(dir, "root-private.pem")+" (PRIVADA") {
		t.Errorf("genkeys output %q", out)
	}
	priv, pub := filepath.Join(dir, "root-private.pem"), filepath.Join(dir, "root-public.pem")
	lic := filepath.Join(dir, "lic.refl")
	s, out, errb = stdio()
	code := RunIssue(IssueCmd{Key: priv, Product: "p", Host: "REF-0123456789AB", Out: lic, Licensee: "lab",
		Features: []string{"pid:4", "logging", "pid: 2 "}}, s)
	if code != 0 || !strings.Contains(out.String(), "host=REF-0123456789AB  expires=None") {
		t.Fatalf("issue exit %d out %q err %q", code, out, errb)
	}
	s, out, _ = stdio()
	if code := RunVerify(VerifyCmd{Root: pub, File: lic, Host: sp("REF-0123456789AB"), Product: sp("p")}, s); code != 0 {
		t.Fatalf("verify exit %d: %s", code, out)
	}
	if !strings.Contains(out.String(), "licensee=lab serial=") || !strings.HasSuffix(out.String(), "features=['logging', 'pid']\n") {
		t.Errorf("verify output %q", out)
	}
	serial := strings.Fields(strings.SplitAfter(out.String(), "serial=")[1])[0]
	bl := filepath.Join(dir, "bl.refl")
	s, _, _ = stdio()
	if code := RunRevoke(priv, serial, bl, s); code != 0 {
		t.Fatal("revoke failed")
	}
	s, out, _ = stdio()
	if code := RunVerify(VerifyCmd{Root: pub, File: lic, Host: sp("REF-0123456789AB"), Blacklist: sp(bl)}, s); code != 1 ||
		!strings.Contains(out.String(), "RESULTADO: E_REVOKED") {
		t.Errorf("revoked verify exit %d: %q", code, out)
	}
}

func TestIssueBadQuantity(t *testing.T) {
	dir := t.TempDir()
	s, _, _ := stdio()
	RunGenkeys(dir, 1024, s)
	s, _, errb := stdio()
	code := RunIssue(IssueCmd{Key: filepath.Join(dir, "root-private.pem"), Product: "p", Host: "h",
		Out: filepath.Join(dir, "x.refl"), Features: []string{"pid:abc"}}, s)
	if code != 2 || errb.String() != "error: invalid literal for int() with base 10: 'abc'\n" {
		t.Errorf("exit %d err %q", code, errb)
	}
}

func TestGenkeysRejectsSmallKeys(t *testing.T) {
	s, _, errb := stdio()
	if code := RunGenkeys(t.TempDir(), 512, s); code != 2 || errb.String() != "error: key_size must be at least 1024-bits.\n" {
		t.Errorf("exit %d err %q", code, errb)
	}
}

func TestStateFileAntiRollback(t *testing.T) {
	key, err := generateKey(1024)
	if err != nil {
		t.Fatal(err)
	}
	c := pyjson.NewObject()
	c.Set("ver", pyjson.IntValue(1))
	c.Set("issued", pyjson.String("2026-08-01T00:00:00+00:00"))
	c.Set("serial", pyjson.String("S"))
	a, _ := Artifact(key, c)
	state := filepath.Join(t.TempDir(), "state.json")
	at := func(day int) string {
		code, _, err := Verify(key.Public(), a, VerifyOpts{Now: time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC), StatePath: state})
		if err != nil {
			t.Fatal(err)
		}
		return code
	}
	if at(20) != OK || at(2) != EClock || at(21) != OK {
		t.Fatal("anti-rollback sequence")
	}
	got, _ := os.ReadFile(state)
	if string(got) != `{"lastSeen": "2026-08-21T12:00:00+00:00"}` {
		t.Errorf("state file %q", got)
	}
	if err := os.WriteFile(state, []byte(`{"lastSeen": "2026-08-01T00:00:00"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Verify(key.Public(), a, VerifyOpts{StatePath: state}); err == nil || err.Error() != errNaiveAware.Error() {
		t.Errorf("naive lastSeen: %v", err)
	}
}

func TestCheckFeature(t *testing.T) {
	f, _ := pyjson.Loads([]rune(`{"features":{"pid":{"qty":4},"log":{"qty":null},"odd":7}}`))
	for _, tc := range []struct {
		name string
		qty  int64
		want string
	}{{"nope", 1, EFeatureLocked}, {"pid", 9, EFeatureQty}, {"pid", 4, OK}, {"log", 999, OK}, {"odd", 1, OK}} {
		if code, _, _ := CheckFeature(f, tc.name, tc.qty); code != tc.want {
			t.Errorf("%s/%d: %s want %s", tc.name, tc.qty, code, tc.want)
		}
	}
}
