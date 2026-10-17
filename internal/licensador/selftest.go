package licensador

import (
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
	if err := os.MkdirAll(tmp, 0o777); err != nil {
		return finish(s, 0, fmt.Errorf("%s", pycompat.OSErrorText(err)))
	}
	key, err := generateKey(bits)
	if err != nil {
		return finish(s, 0, err)
	}
	pub := key.Public()
	host := "REF-0123456789AB"
	product := "reflow-oven"
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	state := filepath.Join(tmp, "state.json")

	art := func(c pyjson.Value) string {
		a, err := Artifact(key, c)
		if err != nil {
			panic(err)
		}
		return a
	}
	claims := func(over map[string]pyjson.Value) pyjson.Value {
		pid, logging, feats := pyjson.NewObject(), pyjson.NewObject(), pyjson.NewObject()
		pid.Set("qty", pyjson.IntValue(4))
		pid.Set("opts", pyjson.NewObject())
		logging.Set("qty", pyjson.Value{})
		logging.Set("opts", pyjson.NewObject())
		feats.Set("pid", pid)
		feats.Set("logging", logging)
		c := pyjson.NewObject()
		c.Set("ver", pyjson.IntValue(1))
		c.Set("product", pyjson.String(product))
		c.Set("licensee", pyjson.String("lab"))
		c.Set("hostId", pyjson.String(host))
		c.Set("serial", pyjson.String(newSerial()))
		c.Set("issued", pyjson.String("2026-08-01T00:00:00+00:00"))
		c.Set("expires", pyjson.String("2027-08-01T00:00:00+00:00"))
		c.Set("features", feats)
		for k, v := range over {
			c.Set(k, v)
		}
		return c
	}
	v := func(a string, o VerifyOpts) string {
		if o.Now.IsZero() {
			o.Now = now
		}
		o.Product, o.Host = &product, &host
		code, _, err := Verify(pub, a, o)
		if err != nil {
			panic(err)
		}
		return code
	}
	type row struct{ name, want, got string }
	var rows []row
	expect := func(name, want, got string) { rows = append(rows, row{name, want, got}) }

	a1 := art(claims(nil))
	expect("A1 valida", OK, v(a1, VerifyOpts{StatePath: state}))
	expect("A2 host ajeno", EHost, v(art(claims(map[string]pyjson.Value{"hostId": pyjson.String("REF-FFFFFFFFFFFF")})), VerifyOpts{}))
	bad := strings.Split(a1, ".")
	payload, _ := b64uDecode(bad[1])
	payload[4] ^= 0x01
	bad[1] = b64u(payload)
	expect("A3 tamper payload", ESignature, v(strings.Join(bad, "."), VerifyOpts{}))
	expect("A4 expirada", ETimeExpired, v(art(claims(map[string]pyjson.Value{"expires": pyjson.String("2026-01-01T00:00:00+00:00")})), VerifyOpts{}))
	_, cl, err := Verify(pub, art(claims(nil)), VerifyOpts{Product: &product, Host: &host, Now: now})
	if err != nil {
		panic(err)
	}
	expect("A5 feature faltante", EFeatureLocked, selftestFeature(cl, "nope", 1))
	expect("A6 qty insuficiente", EFeatureQty, selftestFeature(cl, "pid", 9))
	expect("A6b qty justa", OK, selftestFeature(cl, "pid", 4))
	expect("A6c qty ilimitada", OK, selftestFeature(cl, "logging", 999))
	rev := claims(map[string]pyjson.Value{"serial": pyjson.String("DEADBEEF001")})
	bl := art(revocation([]string{"DEADBEEF001"}, "2026-08-01T00:00:00+00:00"))
	if err := writeText(filepath.Join(tmp, "bl.refl"), bl+"\n"); err != nil {
		return finish(s, 0, err)
	}
	expect("A7 revocada", ERevoked, v(art(rev), VerifyOpts{Blacklist: &bl}))
	v(art(claims(nil)), VerifyOpts{Now: now.Add(30 * 24 * time.Hour), StatePath: state})
	expect("A8 rollback reloj", EClock, v(art(claims(nil)), VerifyOpts{StatePath: state}))
	expect("A9 producto ajeno", EProduct, v(art(claims(map[string]pyjson.Value{"product": pyjson.String("other")})), VerifyOpts{}))
	expect("A10 corrupto", EParse, v("REF1.not-base64!!.sig", VerifyOpts{}))
	expect("A10b formato", EParse, v("garbage", VerifyOpts{}))

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
