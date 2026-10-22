package nlicense

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Main mirrors the tool's hand-rolled positional CLI.
func Main(args []string, s cli.Stdio) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(s.Out, usage+"\n")
		return 0
	}
	cmd, rest := args[0], args[1:]
	var err error
	code := 0
	switch {
	case cmd == "verify" && len(rest) == 2:
		code, err = cmdVerify(rest[0], rest[1], s)
	case cmd == "sign" && len(rest) == 2:
		err = cmdSign(rest[0], rest[1], s)
	case cmd == "rehost" && len(rest) == 3:
		err = cmdRehost(rest[0], rest[1], rest[2], s)
	case cmd == "gen" && len(rest) == 6:
		err = cmdGen(rest[0], rest[1], rest[2], rest[3], rest[4], rest[5], time.Now(), s)
	default:
		fmt.Fprint(s.Out, usage+"\n")
		return 2
	}
	if err == nil {
		return code
	}
	fmt.Fprintln(s.Err, err)
	if code, ok := cli.ExitCode(err); ok {
		return code
	}
	return 1
}

// readXML mirrors etree.parse on a path: the final line of Python's uncaught
// exception, but exit 2 (input error) rather than its exit 1, which verify uses
// for "invalid".
func readXML(path string) (*document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &cli.Exit{Msg: fmt.Sprintf("OSError: Error reading file '%s': failed to load \"%s\": %s",
			path, path, pycompat.StrerrorOf(err)), Code: 2}
	}
	doc, err := parseXML(data)
	if err != nil {
		return nil, &cli.Exit{Msg: fmt.Sprintf("lxml.etree.XMLSyntaxError: %v", err), Code: 2}
	}
	return doc, nil
}

func loadLicense(path string) (*document, error) {
	doc, err := readXML(path)
	if err != nil {
		return nil, err
	}
	if doc.root.name != "license" {
		return nil, cli.SystemExit(fmt.Sprintf("ERROR: root element is <%s>, expected <license>", doc.root.name))
	}
	return doc, nil
}

func attrOrNone(n *node, name string) string {
	if v, ok := n.get(name); ok {
		return v
	}
	return "None"
}

// cmdVerify checks what NLicenseManager checks offline: the license signature
// against the certificate's key, the certificate's vendor against the license's
// (NLicenseManager.getCertificate, an exact match: NLicenseManager.java:104), and the
// certificate's own signature against the embedded root (certificate.chainProblem).
// VALID only when all three hold; every failed check gets its own FAIL line, exit 1.
func cmdVerify(licensePath, certPath string, s cli.Stdio) (int, error) {
	doc, err := loadLicense(licensePath)
	if err != nil {
		return 0, err
	}
	root := doc.root
	if sigEl := find(root, "signature"); sigEl == nil || elemText(sigEl) == nil || *elemText(sigEl) == "" {
		return 0, cli.SystemExit("ERROR: no <signature> element")
	}
	cert, err := loadCertificate(certPath)
	if err != nil {
		return 0, err
	}
	failure, err := licenseSignatureFailure(root, cert)
	if err != nil {
		return 0, err
	}
	trusted, err := embeddedRoot()
	if err != nil {
		return 0, err
	}
	var fails []string
	if failure != "" {
		fails = append(fails, failure)
	}
	licVendor, licOK := root.get("vendor")
	certVendor, certOK := cert.vendor()
	if !licOK || !certOK || licVendor != certVendor {
		fails = append(fails, fmt.Sprintf("vendor mismatch: license vendor %s, certificate vendor %s "+
			"(the station picks {vendor}.certificate by an exact, case-sensitive match)",
			reprOrNone(licVendor, licOK), reprOrNone(certVendor, certOK)))
	}
	if p := cert.chainProblem(trusted); p != "" {
		fails = append(fails, "certificate chain NOT verified: "+p)
	}
	vendorLine := fmt.Sprintf("     vendor=%s hostId=%s expires=%s generated=%s\n", attrOrNone(root, "vendor"),
		attrOrNone(root, "hostId"), attrOrNone(root, "expiration"), attrOrNone(root, "generated"))
	if len(fails) == 0 {
		fmt.Fprintf(s.Out, "OK   %s: VALID signature (verified offline)\n", licensePath)
		fmt.Fprint(s.Out, vendorLine)
		return 0, nil
	}
	if failure == "" {
		fmt.Fprintf(s.Out, "OK   %s: license signature matches the public key of %s\n", licensePath, certPath)
		fmt.Fprint(s.Out, vendorLine)
	}
	for _, f := range fails {
		fmt.Fprintf(s.Out, "FAIL %s: %s\n", licensePath, f)
	}
	return 1, nil
}

// reprOrNone is Python's repr of an attribute value, None when it is absent.
func reprOrNone(v string, ok bool) string {
	if !ok {
		return "None"
	}
	return pyjson.StrRepr(v)
}

// signInto canonicalises, signs and replaces the <signature> of doc.
func signInto(doc *document, keyPath string) error {
	data, err := canonicalBytes(doc.root)
	if err != nil {
		return err
	}
	key, err := loadDSAPrivateKey(keyPath)
	if err != nil {
		return err
	}
	sig, err := signDSA(key, data)
	if err != nil {
		if errors.Is(err, errNoBit159) {
			return cli.SystemExit(err.Error())
		}
		return err
	}
	setSignature(doc.root, sig)
	return nil
}

// splitext mirrors os.path.splitext.
func splitext(p string) (string, string) {
	base := filepath.Base(p)
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 || strings.Trim(base[:dot], ".") == "" {
		return p, ""
	}
	cut := len(p) - (len(base) - dot)
	return p[:cut], p[cut:]
}

// outPathFor mirrors _out_path_for: a new file next to the input, never the input.
func outPathFor(licensePath, suffix string) (string, error) {
	base, ext := splitext(licensePath)
	out := base + suffix + ext
	a, _ := filepath.Abs(out)
	b, _ := filepath.Abs(licensePath)
	if a == b {
		return "", cli.SystemExit("ERROR: refusing to overwrite input " + licensePath)
	}
	return out, nil
}

func writeLicense(doc *document, path string) error {
	if err := os.WriteFile(path, doc.serialize(), 0o666); err != nil {
		return fmt.Errorf("OSError: %v", err)
	}
	return nil
}

func cmdSign(licensePath, keyPath string, s cli.Stdio) error {
	doc, err := loadLicense(licensePath)
	if err != nil {
		return err
	}
	if err := signInto(doc, keyPath); err != nil {
		return err
	}
	out, err := outPathFor(licensePath, "-signed")
	if err != nil {
		return err
	}
	if err := writeLicense(doc, out); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "OK   signed -> %s  (input %s untouched)\n", out, licensePath)
	return nil
}

func cmdRehost(licensePath, hostID, keyPath string, s cli.Stdio) error {
	doc, err := loadLicense(licensePath)
	if err != nil {
		return err
	}
	if err := checkAttrValue(hostID); err != nil {
		return err
	}
	doc.root.set("hostId", hostID)
	if err := signInto(doc, keyPath); err != nil {
		return err
	}
	out, err := outPathFor(licensePath, "-rehosted")
	if err != nil {
		return err
	}
	if err := writeLicense(doc, out); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "OK   re-hosted to %s and re-signed -> %s  (input untouched)\n", hostID, out)
	return nil
}

type feature struct {
	name  string
	attrs []attr
}

// readFeaturesCSV mirrors read_features_csv: name, then attr=value cells
// (";"-separated pairs); zero features is "ERROR: no features" and exit 2.
func readFeaturesCSV(path string) ([]feature, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, pyOpenError(err, path)
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.IsDir() {
		return nil, pyOpenError(dirOpenErrno(), path)
	}
	r := csv.NewReader(f)
	r.FieldsPerRecord, r.LazyQuotes = -1, true
	var feats []feature
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("_csv.Error: %v", err)
		}
		if len(row) == 0 || pycompat.Strip(row[0]) == "" {
			continue
		}
		ft := feature{name: pycompat.Strip(row[0])}
		for _, cell := range row[1:] {
			for _, pair := range strings.Split(cell, ";") {
				k, v, _ := strings.Cut(pycompat.Strip(pair), "=")
				if k == "" {
					continue
				}
				n := node{attrs: ft.attrs}
				n.set(k, v)
				ft.attrs = n.attrs
			}
		}
		feats = append(feats, ft)
	}
	if len(feats) == 0 {
		return nil, &cli.Exit{Msg: "ERROR: no features in " + path, Code: 2}
	}
	return feats, nil
}

// checkAttrName mirrors lxml's attribute-name validation (an XML NCName).
func checkAttrName(name string) error {
	for i, r := range name {
		if !(r == '_' || unicode.IsLetter(r) || i > 0 && (r == '-' || r == '.' || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r))) {
			return fmt.Errorf("ValueError: Invalid attribute name '%s'", name)
		}
	}
	if name == "" {
		return errors.New("ValueError: Invalid attribute name ''")
	}
	return nil
}

// checkAttrValue mirrors lxml's "XML compatible string" check.
func checkAttrValue(v string) error {
	for _, r := range v {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == utf8.RuneError {
			return errors.New("ValueError: All strings must be XML compatible: Unicode or ASCII, no NULL bytes or control characters")
		}
	}
	return nil
}

func cmdGen(outPath, vendor, hostID, expiration, featuresCSV, keyPath string, today time.Time, s cli.Stdio) error {
	root := &node{kind: elemNode, name: "license"}
	for _, kv := range [][2]string{{"vendor", vendor}, {"expiration", expiration}, {"hostId", hostID},
		{"version", "4.10"}, {"generated", today.Format("2006-01-02")}} {
		if err := checkAttrValue(kv[1]); err != nil {
			return err
		}
		root.set(kv[0], kv[1])
	}
	feats, err := readFeaturesCSV(featuresCSV)
	if err != nil {
		return err
	}
	for _, ft := range feats {
		el := &node{kind: elemNode, name: "feature"}
		el.set("name", ft.name)
		for _, a := range ft.attrs {
			if err := checkAttrName(a.name); err != nil {
				return err
			}
			if err := checkAttrValue(a.value); err != nil {
				return err
			}
			el.set(a.name, a.value)
		}
		root.children = append(root.children, el)
	}
	doc := &document{root: root}
	if err := signInto(doc, keyPath); err != nil {
		return err
	}
	if err := writeLicense(doc, outPath); err != nil {
		return err
	}
	fmt.Fprintf(s.Out, "OK   generated %s\n", outPath)
	return nil
}

// pyOpenError mirrors the last traceback line of an OSError raised by
// Python's open(path): the PEP 3151 subclass, "[Errno N] strerror" and the
// path's repr ("FileNotFoundError: [Errno 2] No such file or directory: 'x'").
func pyOpenError(err error, path string) error {
	cls, code, msg := "OSError", 0, ""
	var errno syscall.Errno
	switch {
	case runtime.GOOS != "windows" && errors.As(err, &errno):
		code, msg = int(errno), pycompat.StrerrorOf(errno)
		switch errno {
		case syscall.ENOENT:
			cls = "FileNotFoundError"
		case syscall.EACCES, syscall.EPERM:
			cls = "PermissionError"
		case syscall.EISDIR:
			cls = "IsADirectoryError"
		case syscall.ENOTDIR:
			cls = "NotADirectoryError"
		case syscall.EEXIST:
			cls = "FileExistsError"
		case syscall.EINTR:
			cls = "InterruptedError"
		}
	case errors.Is(err, fs.ErrNotExist): // Windows: the CRT errno Python reports
		cls, code, msg = "FileNotFoundError", 2, "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		cls, code, msg = "PermissionError", 13, "Permission denied"
	default:
		return fmt.Errorf("OSError: %s", err)
	}
	return fmt.Errorf("%s: [Errno %d] %s: %s", cls, code, msg, pyjson.StrRepr(path))
}

// dirOpenErrno is what Python's open() raises for a directory: EISDIR on
// POSIX (its post-open fstat check), EACCES from the Windows CRT.
func dirOpenErrno() error {
	if runtime.GOOS == "windows" {
		return fs.ErrPermission
	}
	return syscall.EISDIR
}
