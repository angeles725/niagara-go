package check

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
)

// A minimal reader of the PKCS#7 SignedData (RFC 2315, RFC 5652 §5) a signed jar keeps in its
// META-INF/<alias>.RSA (.DSA, .EC) block: the certificates, the signer's certificate and the
// RFC 3161 timestamp token jarsigner adds as an unsigned attribute. The Go standard library has no
// PKCS#7, so the structure is walked element by element with encoding/asn1, after berToDER has
// rewritten BER indefinite lengths as definite ones (the 4.15 build's blocks open their ContentInfo,
// SignedData and certificate set with indefinite lengths); nothing is verified but the certificate
// chain (chainOf). Every malformed input is an error, never a panic (FuzzParseSignedData).

var (
	oidSignedData     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidTimeStampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14} // id-aa-timeStampToken
)

// ASN.1 classes and the universal tags the reader checks.
const (
	classUniversal = 0
	classContext   = 2
	tagInteger     = 2
	tagOctetString = 4
	tagSequence    = 16
	tagSet         = 17
)

// signedData is what the reader keeps of one SignedData.
type signedData struct {
	certs     []*x509.Certificate
	signer    *x509.Certificate // the certificate the first SignerInfo names
	timestamp *signedData       // the timestamp token's SignedData; nil when the signer has none
}

// elements splits the contents of a constructed element into its DER elements.
func elements(b []byte) ([]asn1.RawValue, error) {
	var out []asn1.RawValue
	for len(b) > 0 {
		var rv asn1.RawValue
		rest, err := asn1.Unmarshal(b, &rv)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
		b = rest
	}
	return out, nil
}

func is(rv asn1.RawValue, class, tag int, compound bool) bool {
	return rv.Class == class && rv.Tag == tag && rv.IsCompound == compound
}

// parseSignedData reads a BER or DER ContentInfo holding a SignedData (a signature block, or the
// timestamp token inside one).
func parseSignedData(ber []byte) (*signedData, error) {
	der, rest, err := berToDER(ber, 0)
	if err != nil {
		return nil, fmt.Errorf("not DER: %v", err)
	}
	if len(rest) > 0 {
		return nil, errors.New("trailing data after the ContentInfo")
	}
	return parseContentInfo(der, 0)
}

// maxBERDepth bounds the nesting berToDER follows (a signature block nests about 15 deep).
const maxBERDepth = 64

// berToDER re-encodes the first BER element of b with definite, minimal lengths, its constructed
// contents element by element (an indefinite length ends at the end-of-contents octets 00 00); a
// primitive's contents, and so every DER certificate, are copied as they are. It returns the
// element and the bytes after it.
func berToDER(b []byte, depth int) (out, rest []byte, err error) {
	if depth > maxBERDepth {
		return nil, nil, errors.New("nested too deep")
	}
	if len(b) < 2 {
		return nil, nil, errors.New("truncated element")
	}
	id := 1
	if b[0]&0x1f == 0x1f { // a high tag number: base-128 octets up to one without the top bit
		for id < len(b) && b[id]&0x80 != 0 {
			id++
		}
		id++
		if id >= len(b) || id > 6 {
			return nil, nil, errors.New("bad tag")
		}
	}
	compound := b[0]&0x20 != 0
	l, n := int(b[id]), id+1
	var content []byte
	switch {
	case l == 0x80: // indefinite: constructed only
		if !compound {
			return nil, nil, errors.New("indefinite length on a primitive element")
		}
		var parts [][]byte
		for p := b[n:]; ; {
			if len(p) >= 2 && p[0] == 0 && p[1] == 0 {
				content, rest = bytes.Join(parts, nil), p[2:]
				break
			}
			if len(p) == 0 {
				return nil, nil, errors.New("no end-of-contents")
			}
			el, r, err := berToDER(p, depth+1)
			if err != nil {
				return nil, nil, err
			}
			parts, p = append(parts, el), r
		}
		return append(append(append([]byte{}, b[:id]...), derLength(len(content))...), content...), rest, nil
	case l > 0x80:
		k := l & 0x7f
		if k > 4 || n+k > len(b) {
			return nil, nil, errors.New("bad length")
		}
		l = 0
		for _, c := range b[n : n+k] {
			l = l<<8 | int(c)
		}
		n += k
	}
	if l < 0 || l > len(b)-n {
		return nil, nil, errors.New("length past the end")
	}
	content, rest = b[n:n+l], b[n+l:]
	if compound {
		var parts [][]byte
		for p := content; len(p) > 0; {
			el, r, err := berToDER(p, depth+1)
			if err != nil {
				return nil, nil, err
			}
			parts, p = append(parts, el), r
		}
		content = bytes.Join(parts, nil)
	}
	return append(append(append([]byte{}, b[:id]...), derLength(len(content))...), content...), rest, nil
}

// derLength is the minimal DER encoding of a length.
func derLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var be []byte
	for ; n > 0; n >>= 8 {
		be = append([]byte{byte(n)}, be...)
	}
	return append([]byte{0x80 | byte(len(be))}, be...)
}

func parseContentInfo(der []byte, depth int) (*signedData, error) {
	var ci asn1.RawValue
	rest, err := asn1.Unmarshal(der, &ci)
	if err != nil {
		return nil, fmt.Errorf("not DER: %v", err)
	}
	if len(rest) > 0 {
		return nil, errors.New("trailing data after the ContentInfo")
	}
	if !is(ci, classUniversal, tagSequence, true) {
		return nil, errors.New("not a ContentInfo SEQUENCE")
	}
	els, err := elements(ci.Bytes)
	if err != nil {
		return nil, err
	}
	var oid asn1.ObjectIdentifier
	if len(els) != 2 || !is(els[1], classContext, 0, true) {
		return nil, errors.New("ContentInfo is not a content type and a [0] content")
	}
	if _, err := asn1.Unmarshal(els[0].FullBytes, &oid); err != nil || !oid.Equal(oidSignedData) {
		return nil, fmt.Errorf("content type %v is not SignedData", oid)
	}
	inner, err := elements(els[1].Bytes)
	if err != nil {
		return nil, err
	}
	if len(inner) != 1 || !is(inner[0], classUniversal, tagSequence, true) {
		return nil, errors.New("SignedData is not a SEQUENCE")
	}
	return parseSignedDataSeq(inner[0].Bytes, depth)
}

// parseSignedDataSeq reads SignedData ::= SEQUENCE { version, digestAlgorithms SET,
// encapContentInfo SEQUENCE, certificates [0] IMPLICIT OPTIONAL, crls [1] IMPLICIT OPTIONAL,
// signerInfos SET }.
func parseSignedDataSeq(b []byte, depth int) (*signedData, error) {
	els, err := elements(b)
	if err != nil {
		return nil, err
	}
	if len(els) < 4 || !is(els[0], classUniversal, tagInteger, false) || !is(els[1], classUniversal, tagSet, true) ||
		!is(els[2], classUniversal, tagSequence, true) {
		return nil, errors.New("SignedData lacks its version, digest algorithms or content")
	}
	sd := &signedData{}
	i := 3
	if i < len(els) && is(els[i], classContext, 0, true) {
		certEls, err := elements(els[i].Bytes)
		if err != nil {
			return nil, err
		}
		for _, ce := range certEls {
			if !is(ce, classUniversal, tagSequence, true) {
				continue // an attribute certificate or another choice: not an X.509 certificate
			}
			c, err := x509.ParseCertificate(ce.FullBytes)
			if err != nil {
				return nil, fmt.Errorf("certificate %d: %v", len(sd.certs)+1, err)
			}
			sd.certs = append(sd.certs, c)
		}
		i++
	}
	if i < len(els) && is(els[i], classContext, 1, true) {
		i++ // crls
	}
	if i != len(els)-1 || !is(els[i], classUniversal, tagSet, true) {
		return nil, errors.New("SignedData does not end with its signerInfos SET")
	}
	signers, err := elements(els[i].Bytes)
	if err != nil {
		return nil, err
	}
	if len(signers) == 0 {
		return nil, errors.New("no SignerInfo")
	}
	if err := sd.readSigner(signers[0], depth); err != nil {
		return nil, err
	}
	return sd, nil
}

// readSigner reads SignerInfo ::= SEQUENCE { version, sid, digestAlgorithm, signedAttrs [0]
// OPTIONAL, signatureAlgorithm, signature OCTET STRING, unsignedAttrs [1] OPTIONAL }: the signer's
// certificate (by issuer and serial number, or by subject key identifier) and the timestamp token.
func (sd *signedData) readSigner(si asn1.RawValue, depth int) error {
	if !is(si, classUniversal, tagSequence, true) {
		return errors.New("SignerInfo is not a SEQUENCE")
	}
	els, err := elements(si.Bytes)
	if err != nil {
		return err
	}
	if len(els) < 5 {
		return errors.New("SignerInfo is too short")
	}
	sid := els[1]
	switch {
	case is(sid, classUniversal, tagSequence, true):
		parts, err := elements(sid.Bytes)
		if err != nil {
			return err
		}
		var serial *big.Int
		if len(parts) != 2 || !is(parts[0], classUniversal, tagSequence, true) {
			return errors.New("issuerAndSerialNumber is not an issuer and a serial number")
		}
		if _, err := asn1.Unmarshal(parts[1].FullBytes, &serial); err != nil {
			return fmt.Errorf("serial number: %v", err)
		}
		for _, c := range sd.certs {
			if bytes.Equal(c.RawIssuer, parts[0].FullBytes) && c.SerialNumber.Cmp(serial) == 0 {
				sd.signer = c
				break
			}
		}
	case is(sid, classContext, 0, false):
		for _, c := range sd.certs {
			if len(c.SubjectKeyId) > 0 && bytes.Equal(c.SubjectKeyId, sid.Bytes) {
				sd.signer = c
				break
			}
		}
	default:
		return errors.New("SignerInfo has no signer identifier")
	}
	if sd.signer == nil {
		return errors.New("the signer's certificate is not in the block")
	}
	last := els[len(els)-1]
	if !is(last, classContext, 1, true) {
		if !is(last, classUniversal, tagOctetString, false) {
			return errors.New("SignerInfo does not end with its signature")
		}
		return nil // no unsigned attributes: no timestamp
	}
	attrs, err := elements(last.Bytes)
	if err != nil {
		return err
	}
	for _, a := range attrs {
		parts, err := elements(a.Bytes)
		if err != nil {
			return err
		}
		var oid asn1.ObjectIdentifier
		if len(parts) != 2 || !is(parts[1], classUniversal, tagSet, true) {
			return errors.New("an unsigned attribute is not a type and a SET of values")
		}
		if _, err := asn1.Unmarshal(parts[0].FullBytes, &oid); err != nil || !oid.Equal(oidTimeStampToken) {
			continue
		}
		if depth > 0 {
			return errors.New("a timestamp token inside a timestamp token")
		}
		vals, err := elements(parts[1].Bytes)
		if err != nil || len(vals) != 1 {
			return errors.New("the timestamp token attribute does not hold one token")
		}
		if sd.timestamp, err = parseContentInfo(vals[0].FullBytes, depth+1); err != nil {
			return fmt.Errorf("timestamp token: %v", err)
		}
	}
	return nil
}

// chainOf is the signer's certificate path inside the block: each next certificate is the
// issuer of the one before (its subject is that one's issuer, and it signed it), up to a
// self-signed root or the last issuer the block holds.
func (sd *signedData) chainOf() []*x509.Certificate {
	path := []*x509.Certificate{sd.signer}
	for cur := sd.signer; len(path) <= len(sd.certs); {
		if bytes.Equal(cur.RawSubject, cur.RawIssuer) {
			break // self-signed: the root
		}
		var next *x509.Certificate
		for _, c := range sd.certs {
			if c != cur && bytes.Equal(c.RawSubject, cur.RawIssuer) && cur.CheckSignatureFrom(c) == nil {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		path = append(path, next)
		cur = next
	}
	return path
}
