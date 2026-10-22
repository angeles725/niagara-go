package licensador

import (
	"crypto"
	"crypto/rsa"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// selftestFeature is the selftest's check_feature(cl, name, qty)[0]. A failed
// A5 verify leaves cl nil, where Python raises AttributeError on None.get; it
// panics with that error (or CheckFeature's) for RunSelftest to report.
func selftestFeature(cl *pyjson.Value, name string, qty int64) string {
	if cl == nil {
		panic(errors.New("AttributeError: 'NoneType' object has no attribute 'get'"))
	}
	code, _, err := CheckFeature(*cl, name, qty)
	if err != nil {
		panic(err)
	}
	return code
}

// RunSelftest mirrors cmd_selftest: the A1..A10 acceptance matrix with a
// fresh key, a fixed clock and a state file under tmp. Exit 0 when all pass.
// An error raised out of a check (a panic carrying an error) or returned by
// the setup exits 2 before any of the table is printed.
func RunSelftest(tmp string, bits int, s cli.Stdio) (exit int) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				panic(r)
			}
			exit = finish(s, 0, err)
		}
	}()
	if err := pycompat.Makedirs(tmp, 0o777, true); err != nil {
		return finish(s, 0, fmt.Errorf("%s", pycompat.OSErrorText(err)))
	}
	key, err := generateKey(bits)
	if err != nil {
		return finish(s, 0, err)
	}
	e := &selftestEnv{
		key:     key,
		pub:     key.Public(),
		host:    "REF-0123456789AB",
		product: "reflow-oven",
		now:     time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
		state:   filepath.Join(tmp, "state.json"),
	}
	rows, err := e.matrix(tmp)
	if err != nil {
		return finish(s, 0, err)
	}
	return printSelftest(s, rows)
}

// selftestEnv is the fixture cmd_selftest's closures share: the signing key,
// the expected product and host, the fixed clock and the anti-rollback state.
type selftestEnv struct {
	key           *rsa.PrivateKey
	pub           crypto.PublicKey
	host, product string
	now           time.Time
	state         string
}

// art is the selftest's art(claims): the signed artifact of c.
func (e *selftestEnv) art(c pyjson.Value) string {
	a, err := Artifact(e.key, c)
	if err != nil {
		panic(err)
	}
	return a
}

// claims is the selftest's base license (pid qty 4, logging unlimited, a fresh
// serial) with the keys of over replacing or adding to it.
func (e *selftestEnv) claims(over map[string]pyjson.Value) pyjson.Value {
	pid, logging, feats := pyjson.NewObject(), pyjson.NewObject(), pyjson.NewObject()
	pid.Set("qty", pyjson.IntValue(4))
	pid.Set("opts", pyjson.NewObject())
	logging.Set("qty", pyjson.Value{})
	logging.Set("opts", pyjson.NewObject())
	feats.Set("pid", pid)
	feats.Set("logging", logging)
	c := pyjson.NewObject()
	c.Set("ver", pyjson.IntValue(1))
	c.Set("product", pyjson.String(e.product))
	c.Set("licensee", pyjson.String("lab"))
	c.Set("hostId", pyjson.String(e.host))
	c.Set("serial", pyjson.String(newSerial()))
	c.Set("issued", pyjson.String("2026-08-01T00:00:00+00:00"))
	c.Set("expires", pyjson.String("2027-08-01T00:00:00+00:00"))
	c.Set("features", feats)
	for k, v := range over {
		c.Set(k, v)
	}
	return c
}

// verify is the selftest's v(artifact, **kw): Verify against the expected
// product and host at the fixed clock unless o sets another Now.
func (e *selftestEnv) verify(a string, o VerifyOpts) string {
	if o.Now.IsZero() {
		o.Now = e.now
	}
	o.Product, o.Host = &e.product, &e.host
	code, _, err := Verify(e.pub, a, o)
	if err != nil {
		panic(err)
	}
	return code
}

// selftestRow is one line of the matrix: the check, its expected and its
// observed code.
type selftestRow struct{ name, want, got string }

type selftestRows []selftestRow

func (r *selftestRows) expect(name, want, got string) { *r = append(*r, selftestRow{name, want, got}) }

// matrix runs A1..A10 in the Python order; the only error it returns is the
// blacklist write, everything else raises (panics) out of a check.
func (e *selftestEnv) matrix(tmp string) (selftestRows, error) {
	var rows selftestRows
	e.verifyRows(&rows)
	e.featureRows(&rows)
	if err := e.revocationRow(&rows, tmp); err != nil {
		return nil, err
	}
	e.clockAndParseRows(&rows)
	return rows, nil
}

// verifyRows are A1..A4: a valid license (which seeds the state file), a
// foreign host, a payload with one flipped bit and an expired license.
func (e *selftestEnv) verifyRows(rows *selftestRows) {
	a1 := e.art(e.claims(nil))
	rows.expect("A1 valida", OK, e.verify(a1, VerifyOpts{StatePath: e.state}))
	rows.expect("A2 host ajeno", EHost, e.verify(e.art(e.claims(map[string]pyjson.Value{"hostId": pyjson.String("REF-FFFFFFFFFFFF")})), VerifyOpts{}))
	bad := strings.Split(a1, ".")
	payload, _ := b64uDecode(bad[1])
	payload[4] ^= 0x01
	bad[1] = b64u(payload)
	rows.expect("A3 tamper payload", ESignature, e.verify(strings.Join(bad, "."), VerifyOpts{}))
	rows.expect("A4 expirada", ETimeExpired, e.verify(e.art(e.claims(map[string]pyjson.Value{"expires": pyjson.String("2026-01-01T00:00:00+00:00")})), VerifyOpts{}))
}

// featureRows are A5..A6c: check_feature on the claims of a freshly verified
// license (no state file) for a missing feature, too high, exact and
// unlimited quantities.
func (e *selftestEnv) featureRows(rows *selftestRows) {
	_, cl, err := Verify(e.pub, e.art(e.claims(nil)), VerifyOpts{Product: &e.product, Host: &e.host, Now: e.now})
	if err != nil {
		panic(err)
	}
	rows.expect("A5 feature faltante", EFeatureLocked, selftestFeature(cl, "nope", 1))
	rows.expect("A6 qty insuficiente", EFeatureQty, selftestFeature(cl, "pid", 9))
	rows.expect("A6b qty justa", OK, selftestFeature(cl, "pid", 4))
	rows.expect("A6c qty ilimitada", OK, selftestFeature(cl, "logging", 999))
}

// revocationRow is A7: a serial listed in a signed blacklist, which is also
// written to tmp/bl.refl as Python does.
func (e *selftestEnv) revocationRow(rows *selftestRows, tmp string) error {
	rev := e.claims(map[string]pyjson.Value{"serial": pyjson.String("DEADBEEF001")})
	bl := e.art(revocation([]string{"DEADBEEF001"}, "2026-08-01T00:00:00+00:00"))
	if err := writeText(filepath.Join(tmp, "bl.refl"), bl+"\n"); err != nil {
		return err
	}
	rows.expect("A7 revocada", ERevoked, e.verify(e.art(rev), VerifyOpts{Blacklist: &bl}))
	return nil
}

// clockAndParseRows are A8..A10b: a verify 30 days ahead advances the state
// file so the next one at the fixed clock is a rollback, then a foreign
// product and two malformed artifacts.
func (e *selftestEnv) clockAndParseRows(rows *selftestRows) {
	e.verify(e.art(e.claims(nil)), VerifyOpts{Now: e.now.Add(30 * 24 * time.Hour), StatePath: e.state})
	rows.expect("A8 rollback reloj", EClock, e.verify(e.art(e.claims(nil)), VerifyOpts{StatePath: e.state}))
	rows.expect("A9 producto ajeno", EProduct, e.verify(e.art(e.claims(map[string]pyjson.Value{"product": pyjson.String("other")})), VerifyOpts{}))
	rows.expect("A10 corrupto", EParse, e.verify("REF1.not-base64!!.sig", VerifyOpts{}))
	rows.expect("A10b formato", EParse, e.verify("garbage", VerifyOpts{}))
}

// printSelftest prints the matrix and the PASS count; exit 0 only when every
// row got its expected code.
func printSelftest(s cli.Stdio, rows selftestRows) int {
	fmt.Fprintf(s.Out, "%-22s%-18s%-18s  ok\n", "test", "esperado", "obtenido")
	pass := 0
	for _, r := range rows {
		mark := "FAIL"
		if r.want == r.got {
			mark = "PASS"
			pass++
		}
		fmt.Fprintf(s.Out, "%-22s%-18s%-18s  %s\n", r.name, r.want, r.got, mark)
	}
	fmt.Fprintf(s.Out, "\n%d/%d PASS\n", pass, len(rows))
	if pass == len(rows) {
		return 0
	}
	return 1
}
