package licensador

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// REF1 artifact format and verification pipeline (licensador.py).
const (
	Scheme     = "REF1"
	SchemeVer  = 1
	pssSaltLen = 32
	skew       = 5 * time.Minute
)

// Pipeline result codes.
const (
	OK             = "OK"
	EParse         = "E_PARSE"
	ESignature     = "E_SIGNATURE"
	EProduct       = "E_PRODUCT"
	EHost          = "E_HOST"
	ETimeNotYet    = "E_TIME_NOT_YET"
	ETimeExpired   = "E_TIME_EXPIRED"
	EClock         = "E_CLOCK"
	EFeatureLocked = "E_FEATURE_LOCKED"
	EFeatureQty    = "E_FEATURE_QTY"
	ERevoked       = "E_REVOKED"
)

const pemFAQ = "Unable to load PEM file. See https://cryptography.io/en/latest/faq/#why-can-t-i-import-my-pem-file for more details. "
const keyDataErr = "Could not deserialize key data. The data may be in an incorrect format, it may be encrypted with an unsupported algorithm, or it may be an unsupported key type (e.g. EC curves with explicit parameters). Details: "

// pemBlocks returns every PEM block, or the cryptography framing error.
func pemBlocks(data []byte) ([]*pem.Block, error) {
	var out []*pem.Block
	for {
		var b *pem.Block
		b, data = pem.Decode(data)
		if b == nil {
			break
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, errors.New(pemFAQ + "MalformedFraming")
	}
	return out, nil
}

// LoadPublicKey mirrors serialization.load_pem_public_key on a file.
func LoadPublicKey(path string) (crypto.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New(pyOSError(err, path))
	}
	blocks, err := pemBlocks(data)
	if err != nil {
		return nil, err
	}
	for _, b := range blocks {
		switch b.Type {
		case "PUBLIC KEY":
			k, err := x509.ParsePKIXPublicKey(b.Bytes)
			if err != nil {
				return nil, errors.New(keyDataErr + err.Error())
			}
			return k, nil
		case "RSA PUBLIC KEY":
			k, err := x509.ParsePKCS1PublicKey(b.Bytes)
			if err != nil {
				return nil, errors.New(keyDataErr + err.Error())
			}
			return k, nil
		}
	}
	return nil, errors.New("Valid PEM but no BEGIN PUBLIC KEY/END PUBLIC KEY delimiters. Are you sure this is a public key?")
}

// LoadPrivateKey mirrors serialization.load_pem_private_key(password=None).
func LoadPrivateKey(path string) (crypto.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New(pyOSError(err, path))
	}
	blocks, err := pemBlocks(data)
	if err != nil {
		return nil, err
	}
	for _, b := range blocks {
		var k any
		switch b.Type {
		case "PRIVATE KEY":
			k, err = x509.ParsePKCS8PrivateKey(b.Bytes)
		case "RSA PRIVATE KEY":
			k, err = x509.ParsePKCS1PrivateKey(b.Bytes)
		case "EC PRIVATE KEY":
			k, err = x509.ParseECPrivateKey(b.Bytes)
		case "ENCRYPTED PRIVATE KEY":
			return nil, errors.New("Password was not given but private key is encrypted")
		default:
			continue
		}
		if err != nil {
			return nil, errors.New(keyDataErr + err.Error())
		}
		if s, ok := k.(crypto.Signer); ok {
			return s, nil
		}
		return nil, errors.New(keyDataErr + "unsupported key type")
	}
	return nil, errors.New("Valid PEM but no BEGIN/END delimiters for a private key found. Are you sure this is a private key?")
}

// sign mirrors licensador._sign (RSA-PSS, MGF1-SHA256, salt 32). PSS is
// randomised: the bytes differ from Python's on every call; verification is
// the parity check.
func sign(key crypto.Signer, payload []byte) (string, error) {
	rk, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("RSA-PSS signing needs an RSA private key")
	}
	h := sha256.Sum256(payload)
	sig, err := rsa.SignPSS(rand.Reader, rk, crypto.SHA256, h[:], &rsa.PSSOptions{SaltLength: pssSaltLen})
	if err != nil {
		return "", err
	}
	return b64u(sig), nil
}

// verifySig mirrors licensador._verify_sig: any failure is false.
func verifySig(pub crypto.PublicKey, payload []byte, sigB64 string) bool {
	rk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return false
	}
	sig, err := b64uDecode(sigB64)
	if err != nil {
		return false
	}
	h := sha256.Sum256(payload)
	return rsa.VerifyPSS(rk, crypto.SHA256, h[:], sig, &rsa.PSSOptions{SaltLength: pssSaltLen}) == nil
}

// Artifact builds "REF1.<b64u(canonical payload)>.<b64u(signature)>".
func Artifact(key crypto.Signer, claims pyjson.Value) (string, error) {
	payload := []byte(pyjson.Dumps(claims, pyjson.Canonical))
	sig, err := sign(key, payload)
	if err != nil {
		return "", err
	}
	return Scheme + "." + b64u(payload) + "." + sig, nil
}

// VerifyOpts are the optional inputs of the pipeline (licensador.verify).
type VerifyOpts struct {
	Product   *string
	Host      *string
	Now       time.Time // zero = time.Now()
	StatePath string
	Blacklist *string // artifact CONTENT; nil = no blacklist configured
}

// Verify runs PARSE -> SIGNATURE -> PRODUCT -> HOST -> TIME -> REVOCATION ->
// anti-clock. It returns the code, the claims (when parsed) and a non-nil
// error only where Python raises out of verify() (exit 2 in the CLI).
func Verify(pub crypto.PublicKey, artifact string, o VerifyOpts) (string, *pyjson.Value, error) {
	now := awareUTC(o.Now)
	if o.Now.IsZero() {
		now = awareUTC(time.Now())
	}
	parts := strings.Split(strings.TrimSpace(artifact), ".")
	if len(parts) != 3 || parts[0] != Scheme {
		return EParse, nil, nil
	}
	raw, err := b64uDecode(parts[1])
	if err != nil {
		return EParse, nil, nil
	}
	text, err := decodeUTF8(raw, false)
	if err != nil {
		return EParse, nil, nil
	}
	payload, err := pyjson.Loads(text)
	if err != nil || payload.Kind != pyjson.Object {
		return EParse, nil, nil
	}
	if ver, ok := payload.Get("ver"); !ok || !pyjson.Equal(ver, pyjson.IntValue(SchemeVer)) {
		return EParse, nil, nil
	}
	if !verifySig(pub, []byte(pyjson.Dumps(payload, pyjson.Canonical)), parts[2]) {
		return ESignature, nil, nil
	}
	if o.Product != nil {
		if p, _ := payload.Get("product"); !pyjson.Equal(p, pyjson.String(*o.Product)) {
			return EProduct, &payload, nil
		}
	}
	if o.Host != nil {
		if h, _ := payload.Get("hostId"); !pyjson.Equal(h, pyjson.String(*o.Host)) {
			return EHost, &payload, nil
		}
	}
	iv, ok := payload.Get("issued")
	if !ok || iv.Kind != pyjson.Str {
		return EParse, &payload, nil
	}
	issued, err := parseISO(iv.S)
	if err != nil {
		return EParse, &payload, nil
	}
	var expires *pyTime
	if ev, _ := payload.Get("expires"); ev.Truthy() {
		if ev.Kind != pyjson.Str {
			return EParse, &payload, nil
		}
		e, err := parseISO(ev.S)
		if err != nil {
			return EParse, &payload, nil
		}
		expires = &e
	}
	c, err := issued.cmp(now.add(skew))
	if err != nil {
		return "", nil, err
	}
	if c > 0 {
		return ETimeNotYet, &payload, nil
	}
	if expires != nil {
		c, err := expires.cmp(now.add(-skew))
		if err != nil {
			return "", nil, err
		}
		if c < 0 {
			return ETimeExpired, &payload, nil
		}
	}
	if o.Blacklist != nil {
		code, bl, err := Verify(pub, *o.Blacklist, VerifyOpts{Now: now.T})
		if err != nil {
			return "", nil, err
		}
		if code != OK || bl == nil {
			return ERevoked, &payload, nil
		}
		if t, _ := bl.Get("type"); !pyjson.Equal(t, pyjson.String("revocations")) {
			return ERevoked, &payload, nil
		}
		serial, _ := payload.Get("serial")
		revoked, ok := bl.Get("revoked")
		if !ok {
			revoked = pyjson.Value{Kind: pyjson.Array}
		}
		in, err := pyContains(revoked, serial)
		if err != nil {
			return "", nil, err
		}
		if in {
			return ERevoked, &payload, nil
		}
	}
	if o.StatePath != "" {
		if code, err := checkClock(o.StatePath, now); err != nil || code != OK {
			return code, &payload, err
		}
	}
	return OK, &payload, nil
}

// pyContains mirrors `item in container` for JSON values.
func pyContains(container, item pyjson.Value) (bool, error) {
	switch container.Kind {
	case pyjson.Array:
		for _, x := range container.A {
			if pyjson.Equal(x, item) {
				return true, nil
			}
		}
		return false, nil
	case pyjson.Str:
		if item.Kind != pyjson.Str {
			return false, fmt.Errorf("'in <string>' requires string as left operand, not %s", item.TypeName())
		}
		return strings.Contains(container.S, item.S), nil
	case pyjson.Object:
		if item.Kind == pyjson.Array || item.Kind == pyjson.Object {
			return false, fmt.Errorf("unhashable type: '%s'", item.TypeName())
		}
		if item.Kind != pyjson.Str {
			return false, nil
		}
		_, ok := container.M[item.S]
		return ok, nil
	}
	return false, fmt.Errorf("argument of type '%s' is not a container or iterable", container.TypeName())
}

// checkClock is the anti-rollback high-water mark (state file "lastSeen").
func checkClock(path string, now pyTime) (string, error) {
	last := readLastSeen(path)
	if last != nil {
		c, err := now.cmp(last.add(-skew))
		if err != nil {
			return "", err
		}
		if c < 0 {
			return EClock, nil
		}
	}
	keep := now
	if last != nil {
		if c, err := last.cmp(now); err != nil {
			return "", err
		} else if c >= 0 {
			keep = *last
		}
	}
	data := `{"lastSeen": ` + pyjson.EncodeString(keep.dropMicros().isoformat(), true) + `}`
	if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
		return "", errors.New(pyOSError(err, path))
	}
	return OK, nil
}

func readLastSeen(path string) *pyTime {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text, err := decodeUTF8(data, false)
	if err != nil {
		return nil
	}
	v, err := pyjson.Loads(text)
	if err != nil {
		return nil
	}
	ls, ok := v.Get("lastSeen")
	if !ok || ls.Kind != pyjson.Str {
		return nil
	}
	t, err := parseISO(ls.S)
	if err != nil {
		return nil
	}
	return &t
}

// CheckFeature mirrors check_feature: the code and the remaining quantity
// (nil = unlimited). The limit is compared like Python's `qty > fqty`: exactly
// against any int, float (5.0, inf, nan) or bool; any other JSON type is
// Python's TypeError, returned as the error.
func CheckFeature(claims pyjson.Value, name string, qty int64) (string, *pyjson.Value, error) {
	feats, _ := claims.Get("features")
	f, ok := feats.Get(name)
	if !feats.Truthy() || !ok {
		zero := pyjson.IntValue(0)
		return EFeatureLocked, &zero, nil
	}
	fq, _ := f.Get("qty")
	var rest pyjson.Value
	over := false
	switch fq.Kind {
	case pyjson.Null:
		return OK, nil, nil
	case pyjson.Bool, pyjson.Int:
		limit := fq.I
		if fq.Kind == pyjson.Bool {
			limit = big.NewInt(0)
			if fq.B {
				limit = big.NewInt(1)
			}
		}
		left := new(big.Int).Sub(limit, big.NewInt(qty))
		over = left.Sign() < 0
		rest = pyjson.Value{Kind: pyjson.Int, I: left}
	case pyjson.Float:
		// int > float is exact in Python (no rounding of qty); nan is never
		// exceeded and inf never runs out, as in Python.
		if !math.IsNaN(fq.F) {
			if math.IsInf(fq.F, 0) {
				over = fq.F < 0
			} else {
				over = new(big.Float).SetInt64(qty).Cmp(big.NewFloat(fq.F)) > 0
			}
		}
		rest = pyjson.Value{Kind: pyjson.Float, F: fq.F - float64(qty)}
	default:
		return "", nil, fmt.Errorf("TypeError: '>' not supported between instances of 'int' and '%s'", fq.TypeName())
	}
	if over {
		return EFeatureQty, &fq, nil
	}
	return OK, &rest, nil
}
