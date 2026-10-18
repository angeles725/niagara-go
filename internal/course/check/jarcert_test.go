package check

import (
	"bytes"
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSignedData(t *testing.T) {
	sd, err := parseSignedData(fixtureBlock)
	if err != nil {
		t.Fatal(err)
	}
	if len(sd.certs) != 2 || !bytes.Equal(sd.signer.Raw, fixtureSigner.cert.Raw) {
		t.Fatalf("certs %d, signer %v", len(sd.certs), sd.signer.Subject)
	}
	if sd.timestamp == nil || !bytes.Equal(sd.timestamp.signer.Raw, fixtureTSA.cert.Raw) {
		t.Errorf("timestamp token: %+v", sd.timestamp)
	}
	if path := sd.chainOf(); len(path) != 2 || path[1] != sd.certs[1] {
		t.Errorf("chain %d", len(path))
	}
	// The self-signed keystore certificate of L5.14: one certificate, no timestamp.
	self, err := parseSignedData(fixtureSignedData([]*x509.Certificate{fixtureSelfSigned.cert}, fixtureSelfSigned.cert, nil))
	if err != nil || len(self.chainOf()) != 1 || self.timestamp != nil {
		t.Errorf("self-signed: %v %+v", err, self)
	}
	// A root in the block that did not sign the signer is not its issuer, even with the right name.
	other := fixtureCert(9, 1, fixtureRoot.cert.Subject, true, nil)
	wrong, err := parseSignedData(fixtureSignedData([]*x509.Certificate{fixtureSigner.cert, other.cert}, fixtureSigner.cert, nil))
	if err != nil || len(wrong.chainOf()) != 1 {
		t.Errorf("a same-named root that did not sign: %v chain %d", err, len(wrong.chainOf()))
	}
}

// indefinite re-encodes a DER constructed element (one-byte tag) with an indefinite length, as the
// 4.15 build writes the outer elements of its blocks.
func indefinite(der []byte) []byte {
	_, rest, _ := berToDER(der, 0) // der is one element
	n := len(der) - len(rest)
	hl := 2
	if der[1] > 0x80 {
		hl += int(der[1] & 0x7f)
	}
	return append(append([]byte{der[0], 0x80}, der[hl:n]...), 0, 0)
}

// A BER block (indefinite lengths, as the live jars) reads as its DER form; berToDER leaves DER as it is.
func TestBERToDER(t *testing.T) {
	ber := indefinite(fixtureBlock)
	if bytes.Equal(ber, fixtureBlock) {
		t.Fatal("indefinite did not change the block")
	}
	der, rest, err := berToDER(ber, 0)
	if err != nil || len(rest) != 0 || !bytes.Equal(der, fixtureBlock) {
		t.Fatalf("berToDER: %v, %d left, equal %v", err, len(rest), bytes.Equal(der, fixtureBlock))
	}
	if sd, err := parseSignedData(ber); err != nil || !bytes.Equal(sd.signer.Raw, fixtureSigner.cert.Raw) {
		t.Errorf("BER block: %v", err)
	}
	deep := bytes.Repeat([]byte{0x30, 0x80}, maxBERDepth+2)
	for name, b := range map[string][]byte{
		"indefinite primitive": {0x04, 0x80, 0x00, 0x00},
		"no end-of-contents":   {0x30, 0x80, 0x02, 0x01, 0x01},
		"length past the end":  {0x30, 0x05, 0x02, 0x01},
		"long length":          {0x30, 0x85, 1, 2, 3, 4, 5},
		"too deep":             deep,
		"bad tag":              {0x1f, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81},
	} {
		if _, _, err := berToDER(b, 0); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// Malformed blocks are errors with a reason, never a panic or a signer.
func TestParseSignedDataMalformed(t *testing.T) {
	noSigner := fixtureSignedData([]*x509.Certificate{fixtureRoot.cert}, fixtureSigner.cert, nil)
	cases := map[string][]byte{
		"empty":         nil,
		"not DER":       []byte("stand-in signature block\n"),
		"truncated":     fixtureBlock[:len(fixtureBlock)/2],
		"trailing":      append(append([]byte{}, fixtureBlock...), 0),
		"other content": tlv(classUniversal, tagSequence, true, mustMarshal(oidData), tlv(classContext, 0, true, mustMarshal(1))),
		"signer absent": noSigner,
		"token not SD":  fixtureSignedData([]*x509.Certificate{fixtureSigner.cert}, fixtureSigner.cert, mustMarshal(1)),
	}
	for name, der := range cases {
		if sd, err := parseSignedData(der); err == nil {
			t.Errorf("%s: no error (%+v)", name, sd)
		}
	}
}

func FuzzParseSignedData(f *testing.F) {
	f.Add(fixtureBlock)
	f.Add(fixtureToken)
	f.Add(fixtureBlock[:100])
	f.Add(indefinite(fixtureBlock))
	f.Fuzz(func(t *testing.T, der []byte) {
		sd, err := parseSignedData(der)
		if err == nil && sd.signer == nil {
			t.Fatal("no error and no signer")
		}
		if err == nil {
			_ = sd.chainOf()
		}
	})
}

// writeInstall writes modules/<part>.jar for each part, with the given signature block.
func writeInstall(t *testing.T, blocks map[string][]byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "modules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for part, block := range blocks {
		entries := map[string]string{"META-INF/CODESIGN.SF": "Signature-Version: 1.0\n"}
		if block != nil {
			entries["META-INF/CODESIGN.RSA"] = string(block)
		}
		if err := os.WriteFile(filepath.Join(dir, part+".jar"), buildJar(entries), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Dir(dir)
}

func TestJarCertKind(t *testing.T) {
	selfBlock := fixtureSignedData([]*x509.Certificate{fixtureSelfSigned.cert}, fixtureSelfSigned.cert, nil)
	install := writeInstall(t, map[string][]byte{"good": fixtureBlock, "copy": fixtureBlock, "self": selfBlock,
		"unsigned": nil, "garbage": []byte("stand-in signature block\n")})
	labs := writeTree(t, map[string]string{"certificates/code_signing.pem": string(fixturePEM()), "dev/": ""})
	st, err := Open(writeStation(t), install, false)
	if err != nil {
		t.Fatal(err)
	}
	st.labsrc = filepath.Join(labs, "dev")
	defer st.Close()
	root := `^CN=Tridium University Root Certification Authority,`
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Jar: "good", Fact: certSubject, Match: `^CN=\(CodeSigning\),`}, Pass, "modules/good.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX"},
		{Check{Jar: "good", Fact: certIssuer, Match: root}, Pass, "modules/good.jar!META-INF/CODESIGN.RSA: issuer CN=Tridium University Root Certification Authority,O=Tridium University,C=US"},
		{Check{Jar: "self", Fact: certIssuer, Match: root}, Fail, "modules/self.jar!META-INF/CODESIGN.RSA: issuer CN=(CodeSigning),O=Fixture,C=XX, want " + root},
		{Check{Jar: "good", Fact: certChain, Op: opEq, N: 2}, Pass, "modules/good.jar!META-INF/CODESIGN.RSA: 2 of 2 certificates in the chain: CN=(CodeSigning),O=Fixture,C=XX -> CN=Tridium University Root Certification Authority,O=Tridium University,C=US (self-signed)"},
		{Check{Jar: "self", Fact: certChain, Op: opEq, N: 2}, Fail, "modules/self.jar!META-INF/CODESIGN.RSA: 1 of 1 certificates in the chain: CN=(CodeSigning),O=Fixture,C=XX (self-signed), want exactly 2"},
		{Check{Jar: "good", Fact: certTimestamp, Match: `O=DigiCert`}, Pass, "modules/good.jar!META-INF/CODESIGN.RSA: timestamped by CN=Fixture Timestamp Responder,O=DigiCert\\, Inc."},
		{Check{Jar: "self", Fact: certTimestamp, Match: `O=DigiCert`}, Fail, "modules/self.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 7) has no timestamp token"},
		{Check{Jar: "copy", Fact: certSame, CertOf: "good"}, Pass, "modules/copy.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 45e7), the same as modules/good.jar!META-INF/CODESIGN.RSA"},
		{Check{Jar: "self", Fact: certSame, CertOf: "good"}, Fail, "modules/self.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 7), modules/good.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 45e7)"},
		{Check{Jar: "good", Fact: certSame, CertOf: "none"}, Fail, "missing modules/none.jar under "},
		{Check{Jar: "good", Fact: certPEM, Root: rootLabsrc, Glob: "../certificates/code_signing.pem"}, Pass, "modules/good.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 45e7), the first certificate of ../certificates/code_signing.pem"},
		{Check{Jar: "self", Fact: certPEM, Root: rootLabsrc, Glob: "../certificates/code_signing.pem"}, Fail, "modules/self.jar!META-INF/CODESIGN.RSA: signer CN=(CodeSigning),O=Fixture,C=XX (serial 7) is not the first certificate"},
		{Check{Jar: "good", Fact: certPEM, Root: rootLabsrc, Glob: "../certificates/none.pem"}, Fail, "missing ../certificates/none.pem under "},
		{Check{Jar: "unsigned", Fact: certSubject, Match: `.`}, Fail, "modules/unsigned.jar: no signature block (.RSA/.DSA/.EC)"},
		{Check{Jar: "garbage", Fact: certSubject, Match: `.`}, Skip, "cannot read modules/garbage.jar!META-INF/CODESIGN.RSA: not DER"},
		{Check{Jar: "none", Fact: certSubject, Match: `.`}, Fail, "missing modules/none.jar under "},
		{Check{Jar: "good", Fact: certSubject, Match: `(`}, Skip, "bad pattern ("},
	}
	for _, c := range cases {
		c.ck.Kind = kindJarCert
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s:\n got %s %q\nwant %s %q", c.ck.Jar, c.ck.Fact, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
	st.install = ""
	if r := st.Run(&Check{Kind: kindJarCert, Jar: "good", Fact: certChain, Op: opEq, N: 2}); r.Verdict != Skip {
		t.Errorf("no install: %s %q", r.Verdict, r.Evidence)
	}
}
