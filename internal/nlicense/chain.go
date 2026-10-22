package nlicense

import (
	//lint:ignore SA1019 the Niagara license format is DSA by definition
	"crypto/dsa"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// masterRootDER is LicenseUtil.masterPublicKeyData, the DSA root key baja.jar embeds
// and verifies every version-1 certificate against (organized/baja/baja/decompiled/
// com/tridium/sys/license/LicenseUtil.java:223, read by getMasterPublicKey at :208;
// sha256 aed58673c799ab1e57b3c4cc51fe6dfba3c7de79badd0859d790530bf604957f).
const masterRootDER = "" +
	"MIIBuDCCASwGByqGSM44BAEwggEfAoGBAP1/U4EddRIpUt9KnC7s5Of2EbdSPO9EAMMeP4C2USZp" +
	"RV1AIlH7WT2NWPq/xfW6MPbLm1Vs14E7gB00b/JmYLdrmVClpJ+f6AR7ECLCT7up1/63xhv4O1fn" +
	"xqimFQ8E+4P208UewwI1VBNaFpEy9nXzrith1yrv8iIDGZ3RSAHHAhUAl2BQjxUjC8yykrmCouuE" +
	"C/BYHPUCgYEA9+GghdabPd7LvKtcNrhXuXmUr7v6OuqC+VdMCz0HgmdRWVeOutRZT+ZxBxCBgLRJ" +
	"FnEj6EwoFhO3zwkyjMim4TwWeotUfI0o4KOuHiuzpnWRbqN/C/ohNWLx+2J6ASQ7zKTxvqhRkImo" +
	"g9/hWuWfBpKLZl6Ae1UlZAFMO/7PSSoDgYUAAoGBAIvoE0ZQ86VbId9jvWGwAd7fGbAbefVHGrqC" +
	"q6GWrC2390ep1XFGxeiGHzpkA6Fb5ZidiRDWVdy9msxOo6FeQzXeDfNIx41SOFI86WJxgMiXUfw/" +
	"tVEDJiZn0N+UgP8ZU5DDOVREougPsNSW7TcFDihLIyrvLzXxTTiWwWT1oI6W"

// embeddedRoot is the trusted root key; tests replace it.
var embeddedRoot = func() (*dsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(masterRootDER)
	if err != nil {
		return nil, err
	}
	k, err := parseSPKI(der)
	if err != nil {
		return nil, err
	}
	root, ok := k.(*dsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("embedded root key is %T, not DSA", k)
	}
	return root, nil
}

// certificate is a parsed {vendor}.certificate (CertificateFile.load).
type certificate struct {
	path string
	doc  *document
	pub  any
}

// loadCertificate reads path and its public key. A file that cannot be read or
// parsed, or whose key cannot be decoded, is an input error (exit 2); a missing
// <publicKey> is the tool's SystemExit (exit 1).
func loadCertificate(path string) (*certificate, error) {
	doc, err := readXML(path)
	if err != nil {
		return nil, err
	}
	pk := find(doc.root, "publicKey")
	if pk == nil || elemText(pk) == nil || *elemText(pk) == "" {
		return nil, cli.SystemExit("ERROR: no <publicKey> in certificate")
	}
	der, err := b64decode(strings.Join(textfold.Fields(*elemText(pk)), ""))
	if err != nil {
		return nil, &cli.Exit{Msg: err.Error(), Code: 2}
	}
	pub, err := parseSPKI(der)
	if err != nil {
		return nil, &cli.Exit{Msg: err.Error(), Code: 2}
	}
	return &certificate{path: path, doc: doc, pub: pub}, nil
}

// vendor is the certificate's vendor attribute, ok false when it has none.
func (c *certificate) vendor() (string, bool) { return c.doc.root.get("vendor") }

// chainProblem is "" when the certificate's own signature verifies against root,
// otherwise why the chain is not verified.
// It follows CertificateFile.load (organized/baja/baja/decompiled/com/tridium/sys/
// license/CertificateFile.java): the root must be <certificate> (:44), the signature
// is the <signature> text (:78-84), and the signed bytes are LicenseUtil.encode of the
// root without it, its <publicKey> text included (:85-86), verified by LicenseUtil.verify(xml, sig, Version) (:89),
// which uses the master root for every version but 2.0 (LicenseUtil.java:183-190).
// A version-2.0 certificate verifies against the ECDSA version2PublicKey instead,
// which this tool does not check, so its chain is reported as not verified. The
// certificate expiration (:63-77) is not checked here.
func (c *certificate) chainProblem(root *dsa.PublicKey) string {
	r := c.doc.root
	if r.name != "certificate" {
		return fmt.Sprintf("root element is <%s>, expected <certificate>", r.name)
	}
	version, ok := r.get("version")
	if !ok {
		return "no version attribute (CertificateFile parses new Version(null))"
	}
	parts, ok := parseVersion(version)
	if !ok {
		return fmt.Sprintf("invalid version %q", version)
	}
	if len(parts) == 2 && parts[0] == 2 && parts[1] == 0 {
		return "version 2.0 certificate: its ECDSA root (LicenseUtil.version2PublicKeyData) is not checked by this tool"
	}
	sigEl := find(r, "signature")
	if sigEl == nil || elemText(sigEl) == nil || *elemText(sigEl) == "" {
		return "no <signature> in certificate"
	}
	sig, err := b64decode(strings.Join(textfold.Fields(*elemText(sigEl)), ""))
	if err != nil {
		return "certificate <signature> is not base64: " + err.Error()
	}
	data, err := certCanonicalBytes(r)
	if err != nil {
		return "cannot encode the certificate: " + err.Error()
	}
	if !verifyDSA(root, sig, data) {
		return c.path + " signature does not verify against the embedded baja root key (LicenseUtil.masterPublicKeyData)"
	}
	return ""
}

// parseVersion mirrors javax.baja.util.Version(String): "."-separated non-negative
// ints (empty tokens skipped, as StringTokenizer does); ok false where it throws.
func parseVersion(s string) ([]int, bool) {
	var parts []int
	for _, tok := range strings.Split(s, ".") {
		if tok == "" {
			continue
		}
		n, err := strconv.ParseInt(tok, 10, 32) // Integer.parseInt: optional sign, int range
		if err != nil || n < 0 {
			return nil, false
		}
		parts = append(parts, int(n))
	}
	return parts, true
}

// licenseSignatureFailure is "" when the license root's <signature> verifies against
// the certificate's public key, otherwise the failure.
func licenseSignatureFailure(lic *node, c *certificate) (string, error) {
	sigEl := find(lic, "signature")
	if sigEl == nil || elemText(sigEl) == nil || *elemText(sigEl) == "" {
		return "", cli.SystemExit("ERROR: no <signature> element")
	}
	sig, err := b64decode(strings.Join(textfold.Fields(*elemText(sigEl)), ""))
	if err != nil {
		return "", err
	}
	data, err := canonicalBytes(lic)
	if err != nil {
		return "", err
	}
	switch k := c.pub.(type) {
	case *dsa.PublicKey:
		if !verifyDSA(k, sig, data) {
			return "InvalidSignature: the license signature does not verify against the public key of " + c.path, nil
		}
		return "", nil
	case *rsa.PublicKey:
		return "TypeError: RSAPublicKey.verify() missing 1 required positional argument: 'algorithm'", nil
	}
	return fmt.Sprintf("TypeError: unsupported public key type %T", c.pub), nil
}
