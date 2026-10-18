package check

import (
	"archive/zip"
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// The jar-cert kind: the signer certificate of a jar's signature block (META-INF/*.RSA, .DSA or
// .EC: a PKCS#7 SignedData, pkcs7.go), as Tools > Module Info shows it (L5.18, L5.25). Fact is what
// the row reads:
const (
	certSubject   = "subject"   // the signer's subject DN (RFC 2253 order: CN first) matches Match
	certIssuer    = "issuer"    // the signer's issuer DN matches Match
	certChain     = "chain"     // the signer's path inside the block (chainOf) has N certificates (Op)
	certTimestamp = "timestamp" // the signer carries a timestamp token whose signer's subject matches Match
	certSame      = "same"      // the signer certificate is the one of the jar CertOf (the same DER)
	certPEM       = "pem"       // the signer certificate is the first CERTIFICATE of the PEM file Glob under Root
)

// maxSignatureBlock bounds the read of a signature block (a real one is about 9 KiB).
const maxSignatureBlock = 1 << 20

var signatureBlock = regexp.MustCompile(`^META-INF/[^/]+\.(?:RSA|DSA|EC)$`)

// jarSigner reads the first signature block of a jar. skip is set when the check cannot run (no
// install, an unreadable jar or block); fail when the jar is missing or holds no block.
func (s *Station) jarSigner(part string) (sd *signedData, block, skip, fail string) {
	rel := "modules/" + part + ".jar"
	if s.install == "" {
		return nil, "", "no --install DIR: " + rel + " not checked", ""
	}
	zr, err := s.jar(part)
	if os.IsNotExist(err) {
		return nil, "", "", "missing " + rel + " under " + s.install
	}
	if err != nil {
		return nil, "", fmt.Sprintf("cannot read %s: %v", rel, err), ""
	}
	blocks := entries(zr, signatureBlock)
	if len(blocks) == 0 {
		return nil, "", "", rel + ": no signature block (.RSA/.DSA/.EC)"
	}
	block = rel + "!" + blocks[0].Name
	data, err := readEntry(blocks[0])
	if err != nil {
		return nil, block, fmt.Sprintf("cannot read %s: %v", block, err), ""
	}
	if sd, err = parseSignedData(data); err != nil {
		return nil, block, fmt.Sprintf("cannot read %s: %v", block, err), ""
	}
	return sd, block, "", ""
}

func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxSignatureBlock+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSignatureBlock {
		return nil, fmt.Errorf("larger than %d bytes", maxSignatureBlock)
	}
	return data, nil
}

func (s *Station) checkJarCert(ck *Check) (string, string) {
	sd, block, skip, fail := s.jarSigner(ck.Jar)
	switch {
	case skip != "":
		return Skip, skip
	case fail != "":
		return Fail, fail
	}
	signer := fmt.Sprintf("signer %s (serial %x)", sd.signer.Subject, sd.signer.SerialNumber)
	match := func(what, dn string) (string, string) {
		re, err := regexp.Compile(ck.Match)
		if err != nil {
			return Skip, fmt.Sprintf("bad pattern %s: %v", ck.Match, err)
		}
		ev := fmt.Sprintf("%s: %s %s", block, what, dn)
		if !re.MatchString(dn) {
			return Fail, ev + ", want " + ck.Match
		}
		return Pass, ev
	}
	switch ck.Fact {
	case certSubject:
		return match("signer", sd.signer.Subject.String())
	case certIssuer:
		return match("issuer", sd.signer.Issuer.String())
	case certChain:
		path := sd.chainOf()
		names := make([]string, len(path))
		for i, c := range path {
			names[i] = c.Subject.String()
		}
		ev := fmt.Sprintf("%s: %d of %d certificates in the chain: %s", block, len(path), len(sd.certs), strings.Join(names, " -> "))
		if last := path[len(path)-1]; bytes.Equal(last.RawSubject, last.RawIssuer) {
			ev += " (self-signed)"
		}
		if !countOK(ck.Op, len(path), ck.N) {
			return Fail, ev + ", want " + want(ck.Op, ck.N)
		}
		return Pass, ev
	case certTimestamp:
		if sd.timestamp == nil {
			return Fail, block + ": " + signer + " has no timestamp token"
		}
		return match("timestamped by", sd.timestamp.signer.Subject.String())
	case certSame:
		other, oblock, skip, fail := s.jarSigner(ck.CertOf)
		switch {
		case skip != "":
			return Skip, skip
		case fail != "":
			return Fail, fail
		}
		if !bytes.Equal(sd.signer.Raw, other.signer.Raw) {
			return Fail, fmt.Sprintf("%s: %s, %s: signer %s (serial %x)", block, signer, oblock, other.signer.Subject, other.signer.SerialNumber)
		}
		return Pass, fmt.Sprintf("%s: %s, the same as %s", block, signer, oblock)
	case certPEM:
		return s.certMatchesPEM(ck, sd, block, signer)
	}
	return Skip, "unknown jar-cert fact " + ck.Fact
}

// certMatchesPEM compares the signer with the first CERTIFICATE block of the PEM file Glob under
// the row's root (the code_signing.pem L5.17 imports into the keystore).
func (s *Station) certMatchesPEM(ck *Check, sd *signedData, block, signer string) (string, string) {
	dir, skip := s.rootDir(ck)
	if skip != "" {
		return Skip, skip
	}
	data, err := readText(s.rootPath(ck, ck.Glob))
	if os.IsNotExist(err) {
		return Fail, "missing " + ck.Glob + " under " + dir
	}
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", ck.Glob, err)
	}
	for rest := []byte(data); ; {
		var b *pem.Block
		if b, rest = pem.Decode(rest); b == nil {
			return Fail, ck.Glob + " holds no CERTIFICATE block"
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		if !bytes.Equal(b.Bytes, sd.signer.Raw) {
			return Fail, fmt.Sprintf("%s: %s is not the first certificate of %s", block, signer, ck.Glob)
		}
		return Pass, fmt.Sprintf("%s: %s, the first certificate of %s", block, signer, ck.Glob)
	}
}
