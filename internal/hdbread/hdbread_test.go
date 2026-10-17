package hdbread

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Expected strings are Python 3 repr() output.
func TestPyRepr(t *testing.T) {
	cases := map[float64]string{0: "0.0", 1: "1.0", 0.5: "0.5", 1e-4: "0.0001", 1.5e-4: "0.00015",
		1e-5: "1e-05", 123.456: "123.456", 1e15: "1000000000000000.0", 1e16: "1e+16",
		9999999999999998.0: "9999999999999998.0", math.MaxFloat64: "1.7976931348623157e+308",
		5e-324: "5e-324", 2.5e-10: "2.5e-10", 100: "100.0", -12345.678: "-12345.678", 0.1: "0.1",
		1.0 / 3: "0.3333333333333333", math.Inf(1): "inf", math.Inf(-1): "-inf"}
	for v, want := range cases {
		if got := pyRepr(v); got != want {
			t.Errorf("pyRepr(%v) = %q, want %q", v, got, want)
		}
	}
	if got := pyRepr(math.Copysign(0, -1)); got != "-0.0" {
		t.Errorf("pyRepr(-0) = %q", got)
	}
	if got := jsonFloat(math.NaN()); got != "NaN" {
		t.Errorf("jsonFloat(NaN) = %q", got)
	}
}

func TestDecodeReplaceMaximalSubpart(t *testing.T) {
	cases := map[string]string{"\xe2\x82A": "�A", "\xf0\x9f\x98": "�", "\xc0\xaf": "��",
		"\xf4\x90\x80\x80": "����", "ok\xffz": "ok�z", "\xe0\x80\x80": "���",
		"caf\xc3\xa9": "café"}
	for in, want := range cases {
		if got := decodeReplace([]byte(in)); got != want {
			t.Errorf("decodeReplace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortFloat32(t *testing.T) {
	if got := shortFloat32(math.Float32bits(36.0813064575)); got != 36.081306 {
		t.Errorf("got %v, want 36.081306", got)
	}
	if got := shortFloat32(math.Float32bits(20.25)); got != 20.25 {
		t.Errorf("got %v, want 20.25", got)
	}
}

func TestIsoUTC(t *testing.T) {
	for ms, want := range map[int64]string{1537983000008: "2018-09-26T17:30:00.008Z",
		-86400001: "1969-12-30T23:59:59.999Z", -62135596800000: "0001-01-01T00:00:00.000Z"} {
		if got, err := isoUTC(ms); err != nil || got != want {
			t.Errorf("isoUTC(%d) = %q, %v; want %q", ms, got, err, want)
		}
	}
	if _, err := isoUTC(1<<48 - 1); err == nil {
		t.Error("a year past 9999 must be an error (Python raises OverflowError)")
	}
}

func TestValidModifiedUTF8(t *testing.T) {
	for in, want := range map[string]bool{"abc": true, "\xc0\x80x": true, "\xed\xa0\x80": true,
		"\xc0\xaf": false, "\xff": false, "\xe2\x82": false} {
		if got := validModifiedUTF8([]byte(in)); got != want {
			t.Errorf("validModifiedUTF8(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestOpAtFraming(t *testing.T) {
	cases := []struct {
		in      string
		op      string
		counted bool
		ok      bool
	}{
		{"\x00\x07Changed\x00\x05/a/bc", "Changed", true, true},
		{"\x00\x07Changed\x01\x00/", "Changed", true, true}, // a 256-byte target length
		{"\x00\x05Changed\x00\x05/a", "Changed", false, true},
		{"\x00\x07Changed\x00\x00/", "", false, false}, // zero-length target
		{"\x00\x07Changed\x10\x00/", "", false, false}, // 4096+ target
		{"\x00\x07changed\x00\x05/", "", false, false},
		{"\x00\x07Changed\x00\x05x", "", false, false},
	}
	for _, c := range cases {
		op, counted, ok := opAt([]byte(c.in), 0)
		if op != c.op || counted != c.counted || ok != c.ok {
			t.Errorf("opAt(%q) = %q,%v,%v want %q,%v,%v", c.in, op, counted, ok, c.op, c.counted, c.ok)
		}
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"admin": "admin", "jdoe": "jdoe",
		"QWxhZGRpbjpvcGVuIHNlc2FtZQ==": "<VALUE-MASKED>", "adminadminadminadmin": "<VALUE-MASKED>"} {
		if got := mask(in); got != want {
			t.Errorf("mask(%q) = %q, want %q", in, got, want)
		}
	}
}

// A version-2 record whose 8-byte timestamp form runs past the data section is an exit-1 error,
// not a panic (Python raises IndexError/struct.error there).
func TestRecordPastSectionIsAnError(t *testing.T) {
	sect := make([]byte, 8)
	if _, err := decodeRecord(sect, 0, 2, 4, "history:NumericTrendRecord"); err == nil {
		t.Fatal("want an error for a record past the section")
	}
	binary.BigEndian.PutUint32(sect, 0x01000000)
	if _, err := decodeRecord(sect, 6, 2, 4, "history:NumericTrendRecord"); err == nil {
		t.Fatal("want an error when the status bytes are missing")
	}
}

func TestRunMissingFileIsExit2(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{Path: "testdata/no-such.hdb"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || !strings.Contains(errb.String(), "No such file or directory") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
}

// utf is a Java writeUTF string: a 2-byte big-endian length and the bytes.
func utf(s string) string { return string([]byte{byte(len(s) >> 8), byte(len(s))}) + s }

// maskedStrings is the --strings --mask output for the record section rec; auditSchema says
// the file schema names an audit history.
func maskedStrings(rec []byte, auditSchema bool) []string {
	return stringsLines(rec, true, auditSchema)
}

// --strings --mask must not print any user identity: the userName of every framed audit record
// and the identity fields of a login-family record (here jdoe, admin, guest, station, root and the
// "bad" login attempt), role accounts included. Python's mask() let all of them through.
func TestStringsMaskHidesUserIdentities(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{Path: "testdata/inputs/audit.hdb", Strings: true, Mask: true},
		cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitFound {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	lines := strings.Split(out.String(), "\n")
	for _, id := range []string{"jdoe", "admin", "guest", "station", "root", "bad"} {
		for _, l := range lines {
			if strings.Contains(l, id) {
				t.Errorf("identity %q leaks under --mask: line %q", id, l)
			}
		}
	}
	if n := strings.Count(out.String(), userMasked+"\n"); n != 11 {
		t.Errorf("want 11 %s lines (one per identity field), got %d:\n%s", userMasked, n, out.String())
	}
	// Only operation names and slot-path targets stay readable (rule v3 allowlist).
	for _, keep := range []string{"Login Failure\n", "Logout (Timeout)\n", "/Config/Rooms/R1\n"} {
		if !strings.Contains(out.String(), keep) {
			t.Errorf("%q was masked but is allowlisted", keep)
		}
	}
	if strings.Contains(out.String(), "setpoint") {
		t.Errorf("a slotName is not allowlisted:\n%s", out.String())
	}
}

func TestIdentityRangesFraming(t *testing.T) {
	rec := utf("Login Failure") + utf("x") + utf("") + utf("") + utf("eve") + utf("root") +
		utf("Changed") + utf("/a") + utf("out") + utf("1") + utf("2") + utf("ann")
	got := planMask([]byte(rec), true).ids
	var names []string
	for _, r := range got {
		names = append(names, rec[r[0]:r[1]])
	}
	if want := "x,eve,root,ann"; strings.Join(names, ",") != want {
		t.Fatalf("identity fields = %q, want %q", strings.Join(names, ","), want)
	}
}

// Rule v3 closes the inputs that leaked a user name through rule v2 (independent review), plus
// a writeUTF length byte 0x2f ('/') that makes a 47-byte userName read as a slot path.
func TestStringsMaskV2Leaks(t *testing.T) {
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	valid2 := utf("Changed") + utf("/a/d") + utf("out") + utf("1.0") + utf("2.0") + utf("bob")
	head := utf("Changed") + utf("/a/c")
	slash47 := "mallory" + strings.Repeat("-x", 20)
	cases := []struct{ name, rec, user string }{
		{"false record framed at an oldValue operation name",
			valid + head + utf("o\xffut") + utf("Removed") + utf("x") + utf("mallory") + valid2, "mallory"},
		{"no framed record", head + utf("out") + utf("1.0") + utf("2.0") + "\x00\x10mallory", "mallory"},
		{"every record has bad modified UTF-8",
			head + utf("o\xffut") + utf("1.0") + utf("2.0") + utf("mallory") +
				head + utf("o\xffut") + utf("1.0") + utf("2.0") + utf("trudy"), "mallory"},
		{"role account in a truncated record", valid + head + utf("out") + utf("1.0") + utf("2.0") + "\x00\x10admin",
			"admin"},
		{"length byte reads as a slot path", valid + head + utf("o\xffut") + utf("1.0") + utf("2.0") + utf(slash47),
			"mallory"},
	}
	for _, c := range cases {
		for _, schema := range []bool{false, true} {
			out := strings.Join(maskedStrings([]byte(c.rec), schema), "|")
			if strings.Contains(out, c.user) || strings.Contains(out, "trudy") {
				t.Errorf("%s (audit schema %v): user leaks under --mask: %s", c.name, schema, out)
			}
		}
	}
}

// In an audit section only operation names and framed slot-path targets print; text inside a
// framed record (slotName, values) is masked too, and there is no role-account exception.
func TestStringsMaskAuditAllowlist(t *testing.T) {
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	tail := "\x00\x01Logout\x00\x01admin\x00\x01Removed\x00\x01Changedx\x00\x01admins\x00\x01/a/b"
	got := strings.Join(maskedStrings([]byte(valid+tail), false), ",")
	// Rule v4: an operation name prints only at a framed record's operation field.
	want := "Changed,/a/b,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<USER-MASKED>," +
		"<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>"
	if got != want {
		t.Errorf("audit section = %q, want %q", got, want)
	}
}

// A section with no operation name anywhere keeps the old Python mask() rule, unless the schema
// names an audit history. Any operation name, even without a length prefix, makes it an audit
// section (rule v4).
func TestStringsMaskNonAuditSection(t *testing.T) {
	rec := []byte("\x00\x01mallory\x00\x01QWxhZGRpbjpvcGVuIHNlc2FtZQ==\x00\x01Change")
	if got, want := strings.Join(maskedStrings(rec, false), ","), "mallory,<VALUE-MASKED>,Change"; got != want {
		t.Errorf("non-audit section = %q, want the old rule %q", got, want)
	}
	if got, want := strings.Join(maskedStrings(rec, true), ","),
		"<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>"; got != want {
		t.Errorf("audit schema = %q, want %q", got, want)
	}
	rec = append(rec, "\x00\x01xLogins"...)
	if got, want := strings.Join(maskedStrings(rec, false), ","),
		"<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>"; got != want {
		t.Errorf("plain operation name = %q, want an audit section %q", got, want)
	}
}

// Rule v4 closes the inputs that leaked a user name through rule v3 (adversarial verifier).
func TestStringsMaskV3Leaks(t *testing.T) {
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	cases := []struct{ name, rec, user string }{
		{"A1 framed slotName decodes as an operation",
			utf("Changed") + utf("/a") + utf("Mallory") + utf("/x") + utf("2") + utf("ann"), "Mallory"},
		{"A1b unframed tail decodes as an operation", valid + "\x00\x07Mallory\x00\x02/x", "Mallory"},
		{"A1c framed userName with an operation-shaped copy",
			valid[:len(valid)-5] + utf("Admin") + "\x00\x05Admin\x00\x02/q", "Admin"},
		{"A1d login slotName decodes as an operation",
			utf("Login") + utf("x") + utf("JDoe") + utf("/x") + utf("") + utf("JDoe"), "JDoe"},
		{"A2 truncated record whose userName is Login",
			valid + utf("Changed") + utf("/a/c") + utf("out") + utf("1") + utf("2") + "\x00\x10Login", "Login"},
		{"A3 user under a UserService path",
			utf("Logout") + utf("/Services/UserService/jdoe") + utf("") + utf("") + utf("") + utf("jdoe"), "jdoe"},
		{"A3c any segment under UserService",
			utf("Changed") + utf("/Services/UserService/x") + utf("") + utf("") + utf("") + utf("ann"), "UserService/"},
		{"A3b user as a slot-path segment", utf("Login") + utf("/JDOE") + utf("") + utf("") + utf("") + utf("jdoe"),
			"JDOE"},
		{"A4 damaged lengths and no audit schema", "\x00\x09Login\x00\x04jdoe\x00\x01admin", "jdoe"},
		{"A4b damaged lengths, role account", "\x00\x09Logout\x00\x01admin", "admin"},
	}
	for _, c := range cases {
		for _, schema := range []bool{false, true} {
			out := maskedStrings([]byte(c.rec), schema)
			for _, l := range out {
				if strings.Contains(l, c.user) {
					t.Errorf("%s (audit schema %v): %q leaks under --mask: %q", c.name, schema, c.user, out)
				}
			}
		}
	}
	// A framed operation field still prints, and a slot path free of identities too.
	rec := utf("Login") + utf("/Config/R1") + utf("") + utf("") + utf("") + utf("jdoe")
	if got, want := strings.Join(maskedStrings([]byte(rec), false), ","), "Login,/Config/R1,<USER-MASKED>"; got != want {
		t.Errorf("framed login = %q, want %q", got, want)
	}
}

// Property: no byte mutation of the audit fixture plus planted capitalized users (overwrite,
// insert, delete, truncate, a '/x' string after a user; fixed seed) makes --mask print a user.
func TestStringsMaskNeverPrintsAUserOfAMutatedFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/inputs/audit.hdb")
	if err != nil {
		t.Fatal(err)
	}
	f, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	_, rt, fields := schemaOf(f.xml)
	// Capitalized users (rule v3 leaks A1-A3): a slotName followed by a '/' value, a user as a
	// UserService path segment, and a last userName "Login" that can never head a framed record.
	section := string(f.section) +
		utf("Changed") + utf("/Config/Rooms/R1") + utf("Mallory") + utf("/x") + utf("2") + utf("Admin") +
		utf("Login") + utf("/Services/UserService/JDoe") + utf("") + utf("") + utf("") + utf("JDoe") +
		utf("Changed") + utf("/a") + utf("out") + utf("1") + utf("2") + utf("Login")
	users := []string{"admin", "jdoe", "guest", "station", "root", "bad", "abcdefghijklmnopqrstu",
		"Admin", "JDoe", "Mallory"}
	for _, u := range users {
		if !strings.Contains(section, u) {
			t.Fatalf("fixture has no planted user %q", u)
		}
	}
	// "Login" is also an operation: the userName leaks when more "Login" lines print than the
	// unmutated section prints (one per framed Login record).
	logins := func(out []string) (n int) {
		for _, l := range out {
			if l == "Login" {
				n++
			}
		}
		return n
	}
	baseLogins := logins(maskedStrings([]byte(section), auditSchema(rt, fields)))
	rng := rand.New(rand.NewSource(20261017))
	special := []byte{0x00, 0x01, '/', '.', ' ', 'A', 0x7f, 0x80, 0xc0, 0xff}
	for i := 0; i < 5000; i++ {
		rec := []byte(section)
		if rng.Intn(2) == 0 { // a '/x' string after a user makes it read as a countOps operation
			u := users[rng.Intn(len(users))]
			if k := bytes.Index(rec, []byte(u)); k >= 0 {
				k += len(u)
				rec = append(rec[:k:k], append([]byte("\x00\x02/x"), rec[k:]...)...)
			}
		}
		for n := rng.Intn(5); n > 0 && len(rec) > 0; n-- {
			pos, b := rng.Intn(len(rec)), byte(rng.Intn(256))
			if rng.Intn(2) == 0 {
				b = special[rng.Intn(len(special))]
			}
			switch rng.Intn(8) {
			case 0:
				rec = rec[:pos]
			case 1, 2:
				rec = append(rec[:pos:pos], rec[pos+1:]...)
			case 3, 4:
				rec = append(rec[:pos:pos], append([]byte{b}, rec[pos:]...)...)
			default:
				rec[pos] = b
			}
		}
		lines := maskedStrings(rec, auditSchema(rt, fields))
		out := strings.Join(lines, "\n")
		for _, u := range users {
			if strings.Contains(out, u) {
				t.Fatalf("iteration %d: %q leaks under --mask for section %q:\n%s", i, u, rec, out)
			}
		}
		if n := logins(lines); n > baseLogins {
			t.Fatalf("iteration %d: userName \"Login\" leaks (%d > %d Login lines) for section %q:\n%s",
				i, n, baseLogins, rec, out)
		}
	}
}
