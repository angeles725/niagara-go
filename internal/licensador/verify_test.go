package licensador

import (
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestVerifyPipelineEdges pins the PARSE, TIME and REVOCATION exits of
// licensador.verify that the round trips do not reach: which code each
// returns, whether the claims come back, and the errors Python raises out of
// verify().
func TestVerifyPipelineEdges(t *testing.T) {
	key, err := generateKey(1024)
	if err != nil {
		t.Fatal(err)
	}
	signed := func(members ...any) string {
		c := pyjson.NewObject()
		c.Set("ver", pyjson.IntValue(1))
		for i := 0; i < len(members); i += 2 {
			c.Set(members[i].(string), members[i+1].(pyjson.Value))
		}
		a, err := Artifact(key, c)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	issued := pyjson.String("2026-08-01T00:00:00+00:00")
	revocations := func(revoked pyjson.Value) *string {
		bl := signed("issued", issued, "type", pyjson.String("revocations"), "revoked", revoked)
		return &bl
	}
	noRevoked := signed("issued", issued, "type", pyjson.String("revocations"))
	naiveBlacklist := signed("issued", pyjson.String("2026-08-01T00:00:00"))
	now := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, artifact string
		blacklist      *string
		code           string
		claims         bool
		err            string
	}{
		{name: "non-ASCII payload", artifact: Scheme + ".é.x", code: EParse},
		{name: "payload not an object", artifact: Scheme + "." + b64u([]byte("[1]")) + ".x", code: EParse},
		{name: "no issued", artifact: signed(), code: EParse, claims: true},
		{name: "unparseable issued", artifact: signed("issued", pyjson.String("nope")), code: EParse, claims: true},
		{name: "non-string expires", artifact: signed("issued", issued, "expires", pyjson.IntValue(5)), code: EParse, claims: true},
		{name: "unparseable expires", artifact: signed("issued", issued, "expires", pyjson.String("nope")), code: EParse, claims: true},
		{name: "naive expires", artifact: signed("issued", issued, "expires", pyjson.String("2026-09-01T00:00:00")),
			err: errNaiveAware.Error()},
		{name: "blacklist raises", artifact: signed("issued", issued), blacklist: &naiveBlacklist, err: errNaiveAware.Error()},
		{name: "revoked not a container", artifact: signed("issued", issued, "serial", pyjson.String("S")),
			blacklist: revocations(pyjson.IntValue(5)), err: "argument of type 'int' is not a container or iterable"},
		{name: "serial revoked", artifact: signed("issued", issued, "serial", pyjson.String("S")),
			blacklist: revocations(pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{pyjson.String("S")}}), code: ERevoked, claims: true},
		{name: "no revoked member", artifact: signed("issued", issued, "serial", pyjson.String("S")),
			blacklist: &noRevoked, code: OK, claims: true},
		{name: "serial not revoked", artifact: signed("issued", issued, "serial", pyjson.String("S")),
			blacklist: revocations(pyjson.Value{Kind: pyjson.Array}), code: OK, claims: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, claims, err := Verify(key.Public(), tc.artifact, VerifyOpts{Now: now, Blacklist: tc.blacklist})
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if code != tc.code || (claims != nil) != tc.claims || gotErr != tc.err {
				t.Errorf("Verify = %q, claims %v, err %q; want %q, claims %v, err %q",
					code, claims != nil, gotErr, tc.code, tc.claims, tc.err)
			}
		})
	}
}
