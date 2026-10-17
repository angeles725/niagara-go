package nlicense

import (
	//lint:ignore SA1019 the Niagara license format is DSA by definition
	"crypto/dsa"
	"crypto/rsa"
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

// exitError is a Python SystemExit: message to stderr, then the code.
type exitError struct {
	msg  string
	code int
}

func (e exitError) Error() string { return e.msg }

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
	var ee exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

func sysExit(msg string) error { return exitError{msg, 1} }

// readXML mirrors etree.parse on a path; failures are uncaught exceptions.
func readXML(path string) (*document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("OSError: Error reading file '%s': failed to load \"%s\": %s", path, path, pycompat.StrerrorOf(err))
	}
	doc, err := parseXML(data)
	if err != nil {
		return nil, fmt.Errorf("lxml.etree.XMLSyntaxError: %v", err)
	}
	return doc, nil
}

func loadLicense(path string) (*document, error) {
	doc, err := readXML(path)
	if err != nil {
		return nil, err
	}
	if doc.root.name != "license" {
		return nil, sysExit(fmt.Sprintf("ERROR: root element is <%s>, expected <license>", doc.root.name))
	}
	return doc, nil
}

func loadCertPublicKey(path string) (any, error) {
	doc, err := readXML(path)
	if err != nil {
		return nil, err
	}
	pk := find(doc.root, "publicKey")
	if pk == nil || elemText(pk) == nil || *elemText(pk) == "" {
		return nil, sysExit("ERROR: no <publicKey> in certificate")
	}
	der, err := b64decode(strings.Join(pycompat.Fields(*elemText(pk)), ""))
	if err != nil {
		return nil, err
	}
	return parseSPKI(der)
}

func attrOrNone(n *node, name string) string {
	if v, ok := n.get(name); ok {
		return v
	}
	return "None"
}

func cmdVerify(licensePath, certPath string, s cli.Stdio) (int, error) {
	doc, err := loadLicense(licensePath)
	if err != nil {
		return 0, err
	}
	root := doc.root
	sigEl := find(root, "signature")
	if sigEl == nil || elemText(sigEl) == nil || *elemText(sigEl) == "" {
		return 0, sysExit("ERROR: no <signature> element")
	}
	sig, err := b64decode(strings.Join(pycompat.Fields(*elemText(sigEl)), ""))
	if err != nil {
		return 0, err
	}
	pub, err := loadCertPublicKey(certPath)
	if err != nil {
		return 0, err
	}
	data, err := canonicalBytes(root)
	if err != nil {
		return 0, err
	}
	failure := ""
	switch k := pub.(type) {
	case *dsa.PublicKey:
		if !verifyDSA(k, sig, data) {
			failure = "InvalidSignature: "
		}
	case *rsa.PublicKey:
		failure = "TypeError: RSAPublicKey.verify() missing 1 required positional argument: 'algorithm'"
	default:
		failure = fmt.Sprintf("TypeError: unsupported public key type %T", pub)
	}
	if failure != "" {
		fmt.Fprintf(s.Out, "FAIL %s: %s\n", licensePath, failure)
		return 1, nil
	}
	fmt.Fprintf(s.Out, "OK   %s: VALID signature (verified offline)\n", licensePath)
	fmt.Fprintf(s.Out, "     vendor=%s hostId=%s expires=%s generated=%s\n", attrOrNone(root, "vendor"),
		attrOrNone(root, "hostId"), attrOrNone(root, "expiration"), attrOrNone(root, "generated"))
	return 0, nil
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
			return sysExit(err.Error())
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
		return "", sysExit("ERROR: refusing to overwrite input " + licensePath)
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
		return nil, exitError{"ERROR: no features in " + path, 2}
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
