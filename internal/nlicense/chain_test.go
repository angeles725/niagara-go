package nlicense

import (
	//lint:ignore SA1019 DSA is the Niagara license format
	"crypto/dsa"
	"crypto/rand"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmbeddedRootVerifiesGenuineCertificate: testdata/Tridium.certificate is the
// certificate every N4 install ships (security/certificates); its own signature
// verifies against the root key baja.jar embeds. Tridium-tampered.certificate is the
// same file with expiration="never" changed to "2099-01-01": its public key still
// signs real licenses, but its signature no longer verifies against the root.
func TestEmbeddedRootVerifiesGenuineCertificate(t *testing.T) {
	root, err := embeddedRoot()
	if err != nil {
		t.Fatal(err)
	}
	genuine, err := loadCertificate("testdata/Tridium.certificate")
	if err != nil {
		t.Fatal(err)
	}
	if p := genuine.chainProblem(root); p != "" {
		t.Errorf("genuine Tridium.certificate: %s", p)
	}
	tampered, err := loadCertificate("testdata/Tridium-tampered.certificate")
	if err != nil {
		t.Fatal(err)
	}
	if p := tampered.chainProblem(root); !strings.Contains(p, "does not verify against the embedded baja root key") {
		t.Errorf("tampered certificate: chain problem %q", p)
	}
}

// TestChainProblems covers the certificate states CertificateFile.load rejects
// before or instead of a root signature check.
func TestChainProblems(t *testing.T) {
	root, err := embeddedRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	genuine, err := os.ReadFile("testdata/Tridium.certificate")
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct{ old, new, want string }{
		"no-signature": {"signature>", "sig>", "no <signature> in certificate"},
		"no-version":   {` version="1.0"`, "", "no version attribute"},
		"version-2":    {`version="1.0"`, `version="2.0"`, "version 2.0 certificate"},
		"bad-version":  {`version="1.0"`, `version="1.x"`, `invalid version "1.x"`},
		"not-cert":     {"certificate", "cert", "root element is <cert>, expected <certificate>"},
	} {
		p := filepath.Join(dir, name+".certificate")
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(genuine), tc.old, tc.new)), 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := loadCertificate(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := c.chainProblem(root); !strings.Contains(got, tc.want) {
			t.Errorf("%s: chain problem %q, want it to contain %q", name, got, tc.want)
		}
	}
}

// writeDSAKey writes a new DSA key on params as "DSA PRIVATE KEY" PEM.
func writeDSAKey(t *testing.T, params dsa.Parameters, path string) *dsa.PrivateKey {
	t.Helper()
	k := dsa.PrivateKey{PublicKey: dsa.PublicKey{Parameters: params}}
	if err := dsa.GenerateKey(&k, rand.Reader); err != nil {
		t.Fatal(err)
	}
	der, err := asn1.Marshal(struct{ V, P, Q, G, Y, X *big.Int }{big.NewInt(0), k.P, k.Q, k.G, k.Y, k.X})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "DSA PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return &k
}

// chainFixture is a vendor key, a {vendor}.certificate for it signed by a test
// root key, and a license for vendor signed by the vendor key.
type chainFixture struct {
	dir, cert, license string
	root               *dsa.PublicKey
}

func newChainFixture(t *testing.T, vendor, certVendor string) chainFixture {
	t.Helper()
	vendorKey, testCert := testKey(t)
	k, err := loadDSAPrivateKey(vendorKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root := writeDSAKey(t, k.Parameters, filepath.Join(dir, "root.key"))
	doc, err := readXML(testCert)
	if err != nil {
		t.Fatal(err)
	}
	doc.root.set("version", "1.0")
	doc.root.set("vendor", certVendor)
	doc.root.set("expiration", "never")
	data, err := certCanonicalBytes(doc.root)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signDSA(root, data)
	if err != nil {
		t.Fatal(err)
	}
	setSignature(doc.root, sig)
	f := chainFixture{dir: dir, cert: filepath.Join(dir, certVendor+".certificate"), root: &root.PublicKey}
	if err := writeLicense(doc, f.cert); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "v.license")
	body := `<license vendor="` + vendor + `" hostId="H1" expiration="never"><feature name="x"/></license>`
	if err := os.WriteFile(in, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, e := run("sign", in, vendorKey); code != 0 {
		t.Fatalf("sign: %s", e)
	}
	f.license = filepath.Join(dir, "v-signed.license")
	// A padded r or s fails strict DER about 1 in 256 times (see verifyWithRetry).
	for i := 0; i < 8 && !licenseSignatureOK(t, f.license, f.cert); i++ {
		run("sign", in, vendorKey)
	}
	return f
}

// useRoot makes cmdVerify trust root instead of the key baja.jar embeds.
func useRoot(t *testing.T, root *dsa.PublicKey) {
	t.Helper()
	saved := embeddedRoot
	embeddedRoot = func() (*dsa.PublicKey, error) { return root, nil }
	t.Cleanup(func() { embeddedRoot = saved })
}

// TestVerifyRequiresTrustedChain: a license signed by a certificate's key is VALID
// only when that certificate itself verifies against the trusted root; otherwise
// verify says the chain is NOT verified, never VALID, and exits 1.
func TestVerifyRequiresTrustedChain(t *testing.T) {
	f := newChainFixture(t, "Test", "Test")
	useRoot(t, f.root)
	code, out, errOut := run("verify", f.license, f.cert)
	if code != 0 || !strings.HasPrefix(out, "OK   "+f.license+": VALID signature (verified offline)\n") || errOut != "" {
		t.Fatalf("trusted chain: exit %d stdout %q stderr %q", code, out, errOut)
	}
	other := writeDSAKey(t, f.root.Parameters, filepath.Join(f.dir, "other-root.key"))
	useRoot(t, &other.PublicKey)
	code, out, _ = run("verify", f.license, f.cert)
	if code != 1 || strings.Contains(out, "VALID") || !strings.Contains(out, "FAIL "+f.license+": certificate chain NOT verified: ") {
		t.Errorf("untrusted chain: exit %d stdout %q", code, out)
	}
	if !strings.Contains(out, "OK   "+f.license+": license signature matches the public key of "+f.cert) {
		t.Errorf("untrusted chain: the license signature result is missing: %q", out)
	}
}

// TestVerifyNamesVendorMismatch: NLicenseManager picks {vendor}.certificate by an
// exact, case-sensitive vendor match, so a certificate of another vendor is named as
// such, not reported as a bare "InvalidSignature: ".
func TestVerifyNamesVendorMismatch(t *testing.T) {
	f := newChainFixture(t, "acme", "Acme")
	useRoot(t, f.root)
	code, out, _ := run("verify", f.license, f.cert)
	want := "FAIL " + f.license + ": vendor mismatch: license vendor 'acme', certificate vendor 'Acme'"
	if code != 1 || strings.Contains(out, "VALID") || !strings.Contains(out, want) {
		t.Errorf("exit %d stdout %q, want a line starting %q", code, out, want)
	}
}

// TestVerifyInvalidSignatureIsExplained: the failure line says what did not verify.
func TestVerifyInvalidSignatureIsExplained(t *testing.T) {
	code, out, _ := run("verify", "testdata/crlf-signed.license", "testdata/Other.certificate")
	want := "FAIL testdata/crlf-signed.license: InvalidSignature: the license signature does not verify " +
		"against the public key of testdata/Other.certificate\n"
	if code != 1 || !strings.HasPrefix(out, want) {
		t.Errorf("exit %d stdout %q, want prefix %q", code, out, want)
	}
}

// TestVerifyUnreadableInputExits2: exit 1 means "invalid"; a license or certificate
// that cannot be read or parsed is not a verdict and exits 2.
func TestVerifyUnreadableInputExits2(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.xml")
	if err := os.WriteFile(bad, []byte("<license><a></license>"), 0o600); err != nil {
		t.Fatal(err)
	}
	badKey := filepath.Join(dir, "badkey.certificate")
	if err := os.WriteFile(badKey, []byte("<certificate><publicKey>AAAA</publicKey></certificate>"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"verify", filepath.Join(dir, "missing.license"), "testdata/Acme.certificate"},
		{"verify", bad, "testdata/Acme.certificate"},
		{"verify", "testdata/crlf-signed.license", filepath.Join(dir, "missing.certificate")},
		{"verify", "testdata/crlf-signed.license", bad},
		{"verify", "testdata/crlf-signed.license", badKey},
	} {
		if code, out, errOut := run(args...); code != 2 || out != "" || errOut == "" {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, errOut)
		}
	}
}

// TestUsageShowsEscapedNewlines: the docstring quotes LicenseUtil.encode's ">\n"
// and "</root>\n" literally; they are not line breaks.
func TestUsageShowsEscapedNewlines(t *testing.T) {
	_, out, _ := run("--help")
	if want := `document order, ">\n", children, "</root>\n")`; !strings.Contains(out, want) {
		t.Errorf("--help lacks %q:\n%s", want, out)
	}
}

// licenseSignatureOK reports whether path's signature verifies against cert's key,
// whatever the vendor and chain checks say.
func licenseSignatureOK(t *testing.T, path, cert string) bool {
	t.Helper()
	doc, err := loadLicense(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadCertificate(cert)
	if err != nil {
		t.Fatal(err)
	}
	failure, err := licenseSignatureFailure(doc.root, c)
	return err == nil && failure == ""
}
