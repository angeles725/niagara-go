package licensador

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Each command mirrors a licensador.py subcommand. Flag parsing (argparse)
// is not wired yet: callers fill the option structs directly. Errors that
// Python lets escape to main() become "error: <msg>" on stderr and exit 2.

func finish(s cli.Stdio, code int, err error) int {
	if err != nil {
		fmt.Fprintf(s.Err, "error: %s\n", err)
		return cli.ExitCannotSearch
	}
	return code
}

func readASCII(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New(pyOSError(err, path))
	}
	return decodeASCII(data)
}

func writeText(path, text string) error {
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		return errors.New(pyOSError(err, path))
	}
	return nil
}

// VerifyCmd holds `verify` options (nil = option not given).
type VerifyCmd struct {
	Root, File                      string
	Product, Host, State, Blacklist *string
}

// RunVerify mirrors cmd_verify.
func RunVerify(o VerifyCmd, s cli.Stdio) int {
	code, err := runVerify(o, s)
	return finish(s, code, err)
}

func runVerify(o VerifyCmd, s cli.Stdio) (int, error) {
	root, err := LoadPublicKey(o.Root)
	if err != nil {
		return 0, err
	}
	artifact, err := readASCII(o.File)
	if err != nil {
		return 0, err
	}
	host := ""
	if o.Host != nil {
		host = *o.Host
	}
	if host == "" {
		if host, err = hostIDNow(saltDefault); err != nil {
			return 0, err
		}
	}
	var blacklist *string
	if o.Blacklist != nil {
		if pycompat.Strip(*o.Blacklist) == "" {
			fmt.Fprintln(s.Err, "ERROR: --blacklist: empty path (omit the option for no blacklist)")
			return 2, nil
		}
		bl, err := readASCII(*o.Blacklist)
		if err != nil {
			return 0, err
		}
		blacklist = &bl
	}
	state := ""
	if o.State != nil {
		state = *o.State
	}
	code, claims, err := Verify(root, artifact, VerifyOpts{Product: o.Product, Host: &host, StatePath: state, Blacklist: blacklist})
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(s.Out, "host: %s\n", host)
	get := func(k string) string { v, _ := claims.Get(k); return v.Str() }
	if code != OK {
		fmt.Fprintf(s.Out, "RESULTADO: %s\n", code)
		if claims != nil && claims.Truthy() {
			fmt.Fprintf(s.Out, "claims: serial=%s product=%s hostId=%s expires=%s\n",
				get("serial"), get("product"), get("hostId"), get("expires"))
		}
		return 1, nil
	}
	product, ok := claims.Get("product")
	if !ok {
		return 0, errors.New("'product'")
	}
	licensee := get("licensee")
	serial, ok := claims.Get("serial")
	if !ok {
		return 0, errors.New("'serial'")
	}
	feats, ok := claims.Get("features")
	if !ok {
		feats = pyjson.NewObject()
	}
	names, err := pySorted(feats)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(s.Out, "RESULTADO: %s  product=%s licensee=%s serial=%s features=%s\n",
		OK, product.Str(), licensee, serial.Str(), names.Repr())
	return 0, nil
}

// pySorted mirrors sorted(x) for the iterables a features claim can be
// (str items, or numbers; mixed types raise like Python's TypeError).
func pySorted(v pyjson.Value) (pyjson.Value, error) {
	var items []pyjson.Value
	switch v.Kind {
	case pyjson.Object:
		for _, k := range v.Keys {
			items = append(items, pyjson.String(k))
		}
	case pyjson.Str:
		for _, r := range pyjson.Runes(v.S) {
			items = append(items, pyjson.String(pyjson.WTF8([]rune{r})))
		}
	case pyjson.Array:
		items = append(items, v.A...)
	default:
		return pyjson.Value{}, fmt.Errorf("'%s' object is not iterable", v.TypeName())
	}
	less := func(a, b pyjson.Value) (bool, error) {
		if a.Kind == pyjson.Str && b.Kind == pyjson.Str {
			return a.S < b.S, nil
		}
		na, okA := a.Numeric()
		nb, okB := b.Numeric()
		if okA && okB {
			return na.Cmp(nb) < 0, nil
		}
		return false, fmt.Errorf("'<' not supported between instances of '%s' and '%s'", a.TypeName(), b.TypeName())
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			lt, err := less(items[j], items[j-1])
			if err != nil {
				return pyjson.Value{}, err
			}
			if !lt {
				break
			}
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return pyjson.Value{Kind: pyjson.Array, A: items}, nil
}

// RunInspect mirrors cmd_inspect: the claims without verification.
func RunInspect(file string, s cli.Stdio) int {
	text, err := readASCII(file)
	if err != nil {
		return finish(s, 0, err)
	}
	parts := strings.Split(pycompat.Strip(text), ".")
	if len(parts) != 3 || parts[0] != Scheme {
		fmt.Fprintln(s.Out, "no es un artefacto REF1 valido")
		return 1
	}
	claims, err := inspectPayload(parts[1])
	if err != nil {
		fmt.Fprintf(s.Out, "payload ilegible: %s\n", err)
		return 1
	}
	fmt.Fprintln(s.Out, pyjson.Dumps(claims, pyjson.DumpOpts{Indent: 2, ItemSep: ",", KeySep: ": "}))
	return 0
}

func inspectPayload(b64 string) (pyjson.Value, error) {
	raw, err := b64uDecode(b64)
	if err != nil {
		return pyjson.Value{}, err
	}
	text, err := decodeJSONBytes(raw)
	if err != nil {
		return pyjson.Value{}, err
	}
	return pyjson.Loads(text)
}

// IssueCmd holds `issue` options.
type IssueCmd struct {
	Key, Product, Host, Out string
	Licensee                string
	Expires                 *string
	Features                []string // NAME or NAME:QTY
}

// RunIssue mirrors cmd_issue.
func RunIssue(o IssueCmd, s cli.Stdio) int {
	key, err := LoadPrivateKey(o.Key)
	if err != nil {
		return finish(s, 0, err)
	}
	feats, err := parseFeatures(o.Features)
	if err != nil {
		return finish(s, 0, err)
	}
	expires := pyjson.Value{}
	if o.Expires != nil {
		expires = pyjson.String(*o.Expires)
	}
	serial := newSerial()
	claims := pyjson.NewObject()
	claims.Set("ver", pyjson.IntValue(SchemeVer))
	claims.Set("product", pyjson.String(o.Product))
	claims.Set("licensee", pyjson.String(o.Licensee))
	claims.Set("hostId", pyjson.String(o.Host))
	claims.Set("serial", pyjson.String(serial))
	claims.Set("issued", pyjson.String(nowISO()))
	claims.Set("expires", expires)
	claims.Set("features", feats)
	if err := writeArtifact(o.Out, key, claims); err != nil {
		return finish(s, 0, err)
	}
	fmt.Fprintf(s.Out, "licencia emitida: %s  serial=%s  host=%s  expires=%s\n", o.Out, serial, o.Host, expires.Str())
	return 0
}

func writeArtifact(path string, key crypto.Signer, claims pyjson.Value) error {
	art, err := Artifact(key, claims)
	if err != nil {
		return err
	}
	return writeText(path, art+"\n")
}

func newSerial() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0F | 0x40
	b[8] = b[8]&0x3F | 0x80
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

func nowISO() string { return awareUTC(time.Now()).dropMicros().isoformat() }

func parseFeatures(items []string) (pyjson.Value, error) {
	feats := pyjson.NewObject()
	for _, it := range items {
		name, qty, _ := strings.Cut(it, ":")
		q := pyjson.Value{}
		if qty != "" {
			n, err := pyInt(qty)
			if err != nil {
				return pyjson.Value{}, err
			}
			q = pyjson.Value{Kind: pyjson.Int, I: n}
		}
		f := pyjson.NewObject()
		f.Set("qty", q)
		f.Set("opts", pyjson.NewObject())
		feats.Set(name, f)
	}
	return feats, nil
}

func revocation(serials []string, issued string) pyjson.Value {
	list := pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{}}
	for _, k := range pyjson.SortedKeys(serials) {
		list.A = append(list.A, pyjson.String(k))
	}
	v := pyjson.NewObject()
	v.Set("ver", pyjson.IntValue(SchemeVer))
	v.Set("type", pyjson.String("revocations"))
	v.Set("revoked", list)
	v.Set("issued", pyjson.String(issued))
	return v
}

// RunRevoke mirrors cmd_revoke: a signed one-serial blacklist.
func RunRevoke(key, serial, out string, s cli.Stdio) int {
	k, err := LoadPrivateKey(key)
	if err != nil {
		return finish(s, 0, err)
	}
	if err := writeArtifact(out, k, revocation([]string{serial}, nowISO())); err != nil {
		return finish(s, 0, err)
	}
	fmt.Fprintf(s.Out, "blacklist firmada: %s  revoked=%s\n", out, serial)
	return 0
}

// RunGenkeys mirrors cmd_genkeys (unencrypted PKCS#8 + SubjectPublicKeyInfo).
func RunGenkeys(outDir string, bits int, s cli.Stdio) int {
	if err := os.MkdirAll(outDir, 0o777); err != nil {
		return finish(s, 0, errors.New(pyOSError(err, outDir)))
	}
	key, err := generateKey(bits)
	if err != nil {
		return finish(s, 0, err)
	}
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	priv := filepath.Join(outDir, "root-private.pem")
	pub := filepath.Join(outDir, "root-public.pem")
	if err := writeText(priv, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))); err != nil {
		return finish(s, 0, err)
	}
	if err := writeText(pub, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))); err != nil {
		return finish(s, 0, err)
	}
	fmt.Fprintf(s.Out, "raiz generada: %s (PRIVADA — no distribuir) / %s (publica — embebida)\n", priv, pub)
	return 0
}

func generateKey(bits int) (*rsa.PrivateKey, error) {
	if bits < 1024 {
		return nil, errors.New("key_size must be at least 1024-bits.")
	}
	return rsa.GenerateKey(rand.Reader, bits)
}

// RunHostID mirrors cmd_hostid.
func RunHostID(salt string, s cli.Stdio) int {
	id, err := hostIDNow(salt)
	if err != nil {
		return finish(s, 0, err)
	}
	fmt.Fprintln(s.Out, id)
	return 0
}
