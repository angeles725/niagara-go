package nlicense

import (
	"bytes"
	//lint:ignore SA1019 DSA is the Niagara license format
	"crypto/dsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

var variants = []string{"crlf", "attrs", "latin1", "ns", "toplevel", "dt1", "dt2", "twosig", "ctrl"}

// TestCanonicalBytesMatchPython compares canonical_bytes_for_verify output
// frozen from Python (testdata/canonical.json) for every fixture.
func TestCanonicalBytesMatchPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/canonical.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		doc, err := loadLicense(filepath.Join("testdata", name))
		got := ""
		if err != nil {
			got = "ERR:SystemExit"
		} else if b, err := canonicalBytes(doc.root); err != nil {
			got = "ERR:" + err.Error()
		} else {
			got = string(b)
		}
		if got != w {
			t.Errorf("%s:\n got %q\nwant %q", name, got, w)
		}
	}
}

var (
	keyOnce sync.Once
	keyDir  string
	keyErr  error
)

// testKey writes a fresh DSA-1024/160 key ("DSA PRIVATE KEY") and a matching
// {vendor}.certificate into a shared temp dir.
func testKey(t *testing.T) (keyPath, certPath string) {
	t.Helper()
	keyOnce.Do(func() {
		var k dsa.PrivateKey
		if keyErr = dsa.GenerateParameters(&k.Parameters, rand.Reader, dsa.L1024N160); keyErr != nil {
			return
		}
		if keyErr = dsa.GenerateKey(&k, rand.Reader); keyErr != nil {
			return
		}
		der, _ := asn1.Marshal(struct{ V, P, Q, G, Y, X *big.Int }{big.NewInt(0), k.P, k.Q, k.G, k.Y, k.X})
		y, _ := asn1.Marshal(k.Y)
		type algo struct {
			OID    asn1.ObjectIdentifier
			Params struct{ P, Q, G *big.Int }
		}
		a := algo{OID: oidDSA}
		a.Params.P, a.Params.Q, a.Params.G = k.P, k.Q, k.G
		spki, err := asn1.Marshal(struct {
			Algo algo
			Key  asn1.BitString
		}{a, asn1.BitString{Bytes: y, BitLength: 8 * len(y)}})
		if err != nil {
			keyErr = err
			return
		}
		if _, err := x509.ParsePKIXPublicKey(spki); err != nil {
			keyErr = err
			return
		}
		keyDir, keyErr = os.MkdirTemp("", "nlicense-key")
		if keyErr != nil {
			return
		}
		_ = os.WriteFile(filepath.Join(keyDir, "vendor.key"), pem.EncodeToMemory(&pem.Block{Type: "DSA PRIVATE KEY", Bytes: der}), 0o600)
		cert := "<certificate><publicKey algorithm=\"DSA\">\n" + base64.StdEncoding.EncodeToString(spki) + "\n</publicKey></certificate>\n"
		keyErr = os.WriteFile(filepath.Join(keyDir, "Test.certificate"), []byte(cert), 0o600)
	})
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	return filepath.Join(keyDir, "vendor.key"), filepath.Join(keyDir, "Test.certificate")
}

func TestMain(m *testing.M) {
	code := m.Run()
	if keyDir != "" {
		os.RemoveAll(keyDir)
	}
	os.Exit(code)
}

var sigText = regexp.MustCompile(`<signature>[^<]*</signature>`)

func masked(b []byte) string {
	return sigText.ReplaceAllString(string(b), "<signature>SIG</signature>")
}

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSignMatchesPythonSerialisation signs each fixture with Go and compares
// the file with the Python output (signature text masked: DSA is randomised),
// then verifies the Go signature.
func TestSignMatchesPythonSerialisation(t *testing.T) {
	key, cert := testKey(t)
	dir := t.TempDir()
	for _, v := range variants {
		in := filepath.Join(dir, v+".license")
		copyFile(t, filepath.Join("testdata", v+".license"), in)
		code, out, errOut := run("sign", in, key)
		wantOut := "OK   signed -> " + filepath.Join(dir, v+"-signed.license") + "  (input " + in + " untouched)\n"
		if code != 0 || out != wantOut || errOut != "" {
			t.Fatalf("%s: exit %d out %q err %q", v, code, out, errOut)
		}
		got, _ := os.ReadFile(filepath.Join(dir, v+"-signed.license"))
		want, _ := os.ReadFile(filepath.Join("testdata", v+"-signed.license"))
		if masked(got) != masked(want) {
			t.Errorf("%s serialisation:\n got %q\nwant %q", v, masked(got), masked(want))
		}
		verifyWithRetry(t, filepath.Join(dir, v+"-signed.license"), cert, func() { run("sign", in, key) })
	}
}

// verifyWithRetry verifies a Go-signed file. Like the Python tool, the signer
// pads r and s to 20 bytes, so about 1 in 256 signatures carries a
// non-minimal leading zero that strict DER verification (Python's too)
// rejects; re-signing is how an operator recovers, so the test does the same.
func verifyWithRetry(t *testing.T, path, cert string, resign func()) {
	t.Helper()
	for i := 0; i < 8; i++ {
		if code, out, _ := run("verify", path, cert); code == 0 && strings.HasPrefix(out, "OK   ") {
			return
		}
		resign()
	}
	t.Errorf("%s never verified", path)
}

func TestRehostAndGenMatchPython(t *testing.T) {
	key, cert := testKey(t)
	dir := t.TempDir()
	for _, v := range []string{"crlf", "attrs"} {
		in := filepath.Join(dir, v+".license")
		copyFile(t, filepath.Join("testdata", v+".license"), in)
		host := `Qnx-NEW-&"<1>`
		code, out, _ := run("rehost", in, host, key)
		if code != 0 || out != "OK   re-hosted to "+host+" and re-signed -> "+filepath.Join(dir, v+"-rehosted.license")+"  (input untouched)\n" {
			t.Fatalf("rehost %s: %d %q", v, code, out)
		}
		got, _ := os.ReadFile(filepath.Join(dir, v+"-rehosted.license"))
		want, _ := os.ReadFile(filepath.Join("testdata", "py-out", v+"-rehosted.license"))
		if masked(got) != masked(want) {
			t.Errorf("rehost %s:\n got %q\nwant %q", v, masked(got), masked(want))
		}
	}
	out := filepath.Join(dir, "gen-out.license")
	var o, e bytes.Buffer
	day := time.Date(2026, 10, 17, 9, 0, 0, 0, time.Local)
	err := cmdGen(out, "Ac&me", `Qnx-G "1"`, "2027-12-31", "testdata/features-ok.csv", key, day, cli.Stdio{Out: &o, Err: &e})
	if err != nil || o.String() != "OK   generated "+out+"\n" {
		t.Fatalf("gen: %v %q", err, o.String())
	}
	got, _ := os.ReadFile(out)
	want, _ := os.ReadFile("testdata/py-out/gen-out.license")
	if masked(got) != masked(want) {
		t.Errorf("gen:\n got %q\nwant %q", masked(got), masked(want))
	}
	verifyWithRetry(t, out, cert, func() {
		_ = cmdGen(out, "Ac&me", `Qnx-G "1"`, "2027-12-31", "testdata/features-ok.csv", key, day, cli.Stdio{Out: &o, Err: &e})
	})
}

// TestTracebackPaths covers the paths where Python dies with an uncaught
// exception (traceback, exit 1): Go prints the final "<Type>: <msg>" line.
func TestTracebackPaths(t *testing.T) {
	key, _ := testKey(t)
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"verify", filepath.Join(dir, "missing.license"), "x"}, "OSError: Error reading file"},
		{[]string{"verify", write("bad.license", "<license><a></license>"), "x"}, "lxml.etree.XMLSyntaxError: Opening and ending tag mismatch"},
		{[]string{"verify", "testdata/crlf-signed.license", write("bad.certificate", "<c><publicKey>AAAA</publicKey></c>")}, "ValueError: Could not deserialize key data"},
		{[]string{"sign", write("c.license", "<license><!-- c --></license>"), key}, "TypeError: string argument expected"},
		{[]string{"sign", "testdata/crlf.license", filepath.Join(dir, "no.key")}, "cannot read the DSA private key"},
		{[]string{"gen", filepath.Join(dir, "g.license"), "A", "H", "never", "testdata/features.csv", key}, "ValueError: Invalid attribute name 'b '"},
	} {
		code, _, errOut := run(tc.args...)
		if code != 1 || !strings.HasPrefix(errOut, tc.want) {
			t.Errorf("%v: exit %d stderr %q, want prefix %q", tc.args, code, errOut, tc.want)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("an error path wrote a file: %v", entries)
	}
}

func TestSplitext(t *testing.T) {
	for in, want := range map[string][2]string{
		"a.license": {"a", ".license"}, "dir.d/a": {"dir.d/a", ""}, ".license": {".license", ""},
		"..x.y": {"..x", ".y"}, "a.b.c": {"a.b", ".c"}, "/p/q.": {"/p/q", "."},
	} {
		if b, e := splitext(in); b != want[0] || e != want[1] {
			t.Errorf("splitext(%q) = %q %q, want %q", in, b, e, want)
		}
	}
}

func TestReadFeaturesCSV(t *testing.T) {
	feats, err := readFeaturesCSV("testdata/features-ok.csv")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range feats {
		parts := []string{f.name}
		for _, a := range f.attrs {
			parts = append(parts, a.name+"="+a.value)
		}
		got = append(got, strings.Join(parts, "|"))
	}
	want := "bacnet|device.limit=50|point.limit=500 fox quoted, name|a=1|c= bacnet|device.limit=7"
	if strings.Join(got, " ") != want {
		t.Errorf("got %q", strings.Join(got, " "))
	}
}

// TestReadFeaturesCSVStripsLikePython pins str.strip() on the U+001C..U+001F separators,
// which Python treats as whitespace and strings.TrimSpace does not. Python 3.15 skips the
// first row and reads ('bacnet', {'device.limit': '5', 'a': '1'}).
func TestReadFeaturesCSVStripsLikePython(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.csv")
	if err := os.WriteFile(path, []byte("\x1c\x1d\n\x1fbacnet\x1e,\x1c device.limit=5\x1f;\x1ca=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	feats, err := readFeaturesCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range feats {
		parts := []string{f.name}
		for _, a := range f.attrs {
			parts = append(parts, a.name+"="+a.value)
		}
		got = append(got, strings.Join(parts, "|"))
	}
	if want := "bacnet|device.limit=5|a=1"; strings.Join(got, " ") != want {
		t.Errorf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// TestPKCS8Key signs with the same key re-encoded as PKCS#8 ("PRIVATE KEY").
func TestPKCS8Key(t *testing.T) {
	keyPath, cert := testKey(t)
	k, err := loadDSAPrivateKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := asn1.Marshal(k.X)
	var p pkcs8
	p.Algo.OID = oidDSA
	p.Algo.Params = dsaParams{k.P, k.Q, k.G}
	p.Key = x
	der, err := asn1.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pk := filepath.Join(dir, "pkcs8.key")
	if err := os.WriteFile(pk, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "crlf.license")
	copyFile(t, "testdata/crlf.license", in)
	if code, _, e := run("sign", in, pk); code != 0 {
		t.Fatalf("sign with PKCS#8: %s", e)
	}
	verifyWithRetry(t, filepath.Join(dir, "crlf-signed.license"), cert, func() { run("sign", in, pk) })
}
