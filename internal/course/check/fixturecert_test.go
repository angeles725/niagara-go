package check

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test-only generator of PKCS#7 signature blocks. The keys are Ed25519 keys from fixed seeds and
// Ed25519 signatures are deterministic, so the same certificates and blocks come out on every run:
// the fixture jars and testdata/certificates/code_signing.pem can be checked for drift. The
// SignerInfo's signature is a stand-in (the kind does not verify the jar signature; it verifies
// only the certificate chain). The names are the course's (Lab 5): a "Tridium University Root
// Certification Authority" root, a "(CodeSigning)" signer, a DigiCert-named TSA; nothing here is a
// real certificate.

var (
	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidData   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
)

// fixtureCA is one generated certificate and its key.
type fixtureCA struct {
	cert *x509.Certificate
	key  ed25519.PrivateKey
}

// fixtureCert issues a certificate for subject from a seed, signed by parent (nil: self-signed).
func fixtureCert(seed byte, serial int64, subject pkix.Name, ca bool, parent *fixtureCA) *fixtureCA {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: subject,
		NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), NotAfter: time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC),
		BasicConstraintsValid: true, IsCA: ca, KeyUsage: x509.KeyUsageDigitalSignature}
	if ca {
		tmpl.KeyUsage |= x509.KeyUsageCertSign
	}
	parentCert, signKey := tmpl, key
	if parent != nil {
		parentCert, signKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(nil, tmpl, parentCert, key.Public(), signKey)
	if err != nil {
		panic(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	return &fixtureCA{c, key}
}

// tlv is one DER element of the given class and tag around the concatenated parts.
func tlv(class, tag int, compound bool, parts ...[]byte) []byte {
	b, err := asn1.Marshal(asn1.RawValue{Class: class, Tag: tag, IsCompound: compound, Bytes: bytes.Join(parts, nil)})
	if err != nil {
		panic(err)
	}
	return b
}

func mustMarshal(v any) []byte {
	b, err := asn1.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// fixtureSignedData is a ContentInfo holding a SignedData with the certificates, one SignerInfo
// naming signer by issuer and serial number, and (when token is not nil) the timestamp token as an
// unsigned attribute.
func fixtureSignedData(certs []*x509.Certificate, signer *x509.Certificate, token []byte) []byte {
	var unsigned []byte
	if token != nil {
		attr := tlv(classUniversal, tagSequence, true, mustMarshal(oidTimeStampToken), tlv(classUniversal, tagSet, true, token))
		unsigned = tlv(classContext, 1, true, attr)
	}
	return fixtureSignedDataSI(certs, fixtureSignerInfo(fixtureSid(signer), unsigned))
}

// fixtureSid is an issuerAndSerialNumber signer identifier.
func fixtureSid(c *x509.Certificate) []byte {
	return tlv(classUniversal, tagSequence, true, c.RawIssuer, mustMarshal(c.SerialNumber))
}

// fixtureSignerInfo is a SignerInfo with the identifier, a stand-in signature and the unsigned
// attributes element (nil: none).
func fixtureSignerInfo(sid, unsigned []byte) []byte {
	alg := mustMarshal(pkix.AlgorithmIdentifier{Algorithm: oidSHA256})
	si := [][]byte{mustMarshal(1), sid, alg, alg, tlv(classUniversal, tagOctetString, false, []byte("fixture: not a signature"))}
	if unsigned != nil {
		si = append(si, unsigned)
	}
	return tlv(classUniversal, tagSequence, true, si...)
}

// fixtureSignedDataSI is a ContentInfo holding a SignedData with the certificates and one SignerInfo.
func fixtureSignedDataSI(certs []*x509.Certificate, si []byte) []byte {
	alg := mustMarshal(pkix.AlgorithmIdentifier{Algorithm: oidSHA256})
	var raw [][]byte
	for _, c := range certs {
		raw = append(raw, c.Raw)
	}
	sd := tlv(classUniversal, tagSequence, true,
		mustMarshal(1),
		tlv(classUniversal, tagSet, true, alg),
		tlv(classUniversal, tagSequence, true, mustMarshal(oidData)),
		tlv(classContext, 0, true, raw...),
		tlv(classUniversal, tagSet, true, si))
	return tlv(classUniversal, tagSequence, true, mustMarshal(oidSignedData), tlv(classContext, 0, true, sd))
}

// The fixture PKI: the course root, the CA-issued code-signing certificate (L5.17-L5.18), a
// self-signed one (the keystore before L5.15, L5.14), and the TSA.
var (
	fixtureRoot = fixtureCert(1, 1, pkix.Name{CommonName: "Tridium University Root Certification Authority",
		Organization: []string{"Tridium University"}, Country: []string{"US"}}, true, nil)
	fixtureSigner = fixtureCert(2, 0x45e7, pkix.Name{CommonName: "(CodeSigning)", Organization: []string{"Fixture"},
		Country: []string{"XX"}}, false, fixtureRoot)
	fixtureSelfSigned = fixtureCert(3, 7, pkix.Name{CommonName: "(CodeSigning)", Organization: []string{"Fixture"},
		Country: []string{"XX"}}, false, nil)
	fixtureTSARoot = fixtureCert(4, 2, pkix.Name{CommonName: "Fixture TSA Root", Organization: []string{"DigiCert, Inc."}}, true, nil)
	fixtureTSA     = fixtureCert(5, 3, pkix.Name{CommonName: "Fixture Timestamp Responder", Organization: []string{"DigiCert, Inc."}}, false, fixtureTSARoot)
	fixtureToken   = fixtureSignedData([]*x509.Certificate{fixtureTSA.cert, fixtureTSARoot.cert}, fixtureTSA.cert, nil)
	// fixtureBlock is the signature block of the fixture's training-rt and review jars.
	fixtureBlock = fixtureSignedData([]*x509.Certificate{fixtureSigner.cert, fixtureRoot.cert}, fixtureSigner.cert, fixtureToken)
)

// fixturePEM is testdata/certificates/code_signing.pem: the signer, then the root (the chain the
// instructor returns at L5.16).
func fixturePEM() []byte {
	var b bytes.Buffer
	for _, c := range []*x509.Certificate{fixtureSigner.cert, fixtureRoot.cert} {
		if err := pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw}); err != nil {
			panic(err)
		}
	}
	return b.Bytes()
}

const fixturePEMPath = "../testdata/certificates/code_signing.pem"

// TestFixtureCerts: the committed PEM is the generator's (COURSE_UPDATE_FIXTURES=1 rewrites it).
func TestFixtureCerts(t *testing.T) {
	if os.Getenv("COURSE_UPDATE_FIXTURES") == "1" {
		if err := os.WriteFile(filepath.FromSlash(fixturePEMPath), fixturePEM(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filepath.FromSlash(fixturePEMPath))
	if err != nil || !bytes.Equal(got, fixturePEM()) {
		t.Errorf("%s drifted from fixturePEM (regenerate: COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureCerts): %v", fixturePEMPath, err)
	}
}
