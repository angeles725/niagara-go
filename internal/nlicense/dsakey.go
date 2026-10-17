package nlicense

import (
	//lint:ignore SA1019 the Niagara license format is DSA/SHA-1 by definition
	"crypto/dsa"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA-1 is what the platform verifies
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// b64decode mirrors base64.b64decode(s) (non-strict, Python 3.15): bytes
// outside the standard alphabet are skipped and a trailing 2-3 character
// group decodes to whole bytes; a lone trailing character is binascii.Error.
func b64decode(s string) ([]byte, error) {
	val := func(c byte) int {
		switch {
		case c >= 'A' && c <= 'Z':
			return int(c - 'A')
		case c >= 'a' && c <= 'z':
			return int(c-'a') + 26
		case c >= '0' && c <= '9':
			return int(c-'0') + 52
		case c == '+':
			return 62
		case c == '/':
			return 63
		}
		return -1
	}
	var out []byte
	quad, left := 0, 0
	for i := 0; i < len(s); i++ {
		v := val(s[i])
		if v < 0 {
			continue
		}
		switch quad {
		case 0:
			quad, left = 1, v
		case 1:
			out = append(out, byte(left<<2|v>>4))
			quad, left = 2, v&0x0F
		case 2:
			out = append(out, byte(left<<4|v>>2))
			quad, left = 3, v&0x03
		case 3:
			out = append(out, byte(left<<6|v))
			quad = 0
		}
	}
	if quad == 1 {
		return nil, fmt.Errorf("binascii.Error: Invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4", len(out)/3*4+1)
	}
	return out, nil
}

type dsaSig struct{ R, S *big.Int }

// verifyDSA mirrors DSAPublicKey.verify(der, data, SHA1()): strict DER,
// 0 < r, s < q.
func verifyDSA(pub *dsa.PublicKey, der, data []byte) bool {
	var sig dsaSig
	rest, err := asn1.Unmarshal(der, &sig)
	if err != nil || len(rest) != 0 || sig.R.Sign() <= 0 || sig.S.Sign() <= 0 {
		return false
	}
	h := sha1.Sum(data)
	return dsa.Verify(pub, h[:], sig.R, sig.S)
}

// errNoBit159 is the tool's SystemExit after 40 unlucky signatures.
var errNoBit159 = errors.New("ERROR: could not produce a bit159-clear DSA signature after 40 tries")

// signDSA mirrors _sign_bytes: DSA/SHA-1, retried until r and s both have
// bit 159 clear (Sun JDK8 reads a set top bit as negative), emitted as
// SEQUENCE { INTEGER r(20 bytes), INTEGER s(20 bytes) } in base64.
func signDSA(priv *dsa.PrivateKey, data []byte) (string, error) {
	h := sha1.Sum(data)
	for try := 0; try < 40; try++ {
		r, s, err := dsa.Sign(rand.Reader, priv, h[:])
		if err != nil {
			return "", err
		}
		if r.BitLen() > 160 || s.BitLen() > 160 {
			return "", errors.New("OverflowError: int too big to convert")
		}
		if r.BitLen() < 160 && s.BitLen() < 160 {
			body := append([]byte{0x02, 0x14}, r.FillBytes(make([]byte, 20))...)
			body = append(body, 0x02, 0x14)
			body = append(body, s.FillBytes(make([]byte, 20))...)
			der := append([]byte{0x30, byte(len(body))}, body...)
			return base64.StdEncoding.EncodeToString(der), nil
		}
	}
	return "", errNoBit159
}

type dsaParams struct{ P, Q, G *big.Int }

type pkcs8 struct {
	Version int
	Algo    struct {
		OID    asn1.ObjectIdentifier
		Params dsaParams
	}
	Key []byte
}

var oidDSA = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 1}

// loadDSAPrivateKey reads a PEM DSA key ("DSA PRIVATE KEY" or PKCS#8), the
// forms `openssl dgst -sign` accepts without a passphrase.
func loadDSAPrivateKey(path string) (*dsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the DSA private key %s: %s", path, pycompat.StrerrorOf(err))
	}
	for {
		var b *pem.Block
		b, data = pem.Decode(data)
		if b == nil {
			return nil, fmt.Errorf("no DSA private key in %s", path)
		}
		switch b.Type {
		case "DSA PRIVATE KEY":
			var k struct{ Version, P, Q, G, Y, X *big.Int }
			if _, err := asn1.Unmarshal(b.Bytes, &k); err != nil {
				return nil, fmt.Errorf("bad DSA private key in %s: %v", path, err)
			}
			return &dsa.PrivateKey{PublicKey: dsa.PublicKey{Parameters: dsa.Parameters{P: k.P, Q: k.Q, G: k.G}, Y: k.Y}, X: k.X}, nil
		case "PRIVATE KEY":
			var k pkcs8
			if _, err := asn1.Unmarshal(b.Bytes, &k); err != nil || !k.Algo.OID.Equal(oidDSA) {
				return nil, fmt.Errorf("no DSA private key in %s", path)
			}
			x := new(big.Int)
			if _, err := asn1.Unmarshal(k.Key, &x); err != nil {
				return nil, fmt.Errorf("bad DSA private key in %s: %v", path, err)
			}
			p := k.Algo.Params
			y := new(big.Int).Exp(p.G, x, p.P)
			return &dsa.PrivateKey{PublicKey: dsa.PublicKey{Parameters: dsa.Parameters{P: p.P, Q: p.Q, G: p.G}, Y: y}, X: x}, nil
		}
	}
}

// parseSPKI mirrors serialization.load_der_public_key.
func parseSPKI(der []byte) (any, error) {
	k, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("ValueError: Could not deserialize key data. The data may be in an incorrect format, it may be encrypted with an unsupported algorithm, or it may be an unsupported key type (e.g. EC curves with explicit parameters). Details: %v", err)
	}
	return k, nil
}
