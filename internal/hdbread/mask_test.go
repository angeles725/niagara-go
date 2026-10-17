package hdbread

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// utf is a Java writeUTF string: a 2-byte big-endian length and the bytes.
func utf(s string) string { return string([]byte{byte(len(s) >> 8), byte(len(s))}) + s }

// maskedStrings is the --strings --mask output for the record section rec.
func maskedStrings(rec []byte) []string { return stringsLines(rec, true) }

// verbatimNotOp returns the first --mask line that is neither a mask token nor an AuditEvent
// operation constant: the rule v5 invariant is that no other text ever prints verbatim.
func verbatimNotOp(lines []string) (string, bool) {
	for _, l := range lines {
		if l != userMasked && l != valueMasked && !isAuditOperation(l) {
			return l, true
		}
	}
	return "", false
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
	if n := strings.Count(out.String(), userMasked+"\n"); n != 11 {
		t.Errorf("want 11 %s lines (one per identity field), got %d:\n%s", userMasked, n, out.String())
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")[2:] // past the header
	if l, bad := verbatimNotOp(lines); bad {
		t.Errorf("rule v5: only operation constants print verbatim, got %q:\n%s", l, out.String())
	}
	for _, keep := range []string{"Login Failure\n", "Logout (Timeout)\n", "Flags Changed\n"} {
		if !strings.Contains(out.String(), keep) {
			t.Errorf("framed operation %q was masked", keep)
		}
	}
}

func TestIdentityRangesFraming(t *testing.T) {
	rec := utf("Login Failure") + utf("x") + utf("") + utf("") + utf("eve") + utf("root") +
		utf("Changed") + utf("/a") + utf("out") + utf("1") + utf("2") + utf("ann")
	got := planMask([]byte(rec)).ids
	var names []string
	for _, r := range got {
		names = append(names, rec[r[0]:r[1]])
	}
	if want := "x,eve,root,ann"; strings.Join(names, ",") != want {
		t.Fatalf("identity fields = %q, want %q", strings.Join(names, ","), want)
	}
}

// assertNoLeak fails when a --mask line contains user, or text other than an operation constant
// prints verbatim.
func assertNoLeak(t *testing.T, name string, rec []byte, user string) {
	t.Helper()
	out := maskedStrings(rec)
	for _, l := range out {
		if strings.Contains(l, user) {
			t.Errorf("%s: %q leaks under --mask: %q", name, user, out)
		}
	}
	if l, bad := verbatimNotOp(out); bad {
		t.Errorf("%s: %q prints verbatim but is no operation constant: %q", name, l, out)
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
				head + utf("o\xffut") + utf("1.0") + utf("2.0") + utf("trudy"), "trudy"},
		{"role account in a truncated record", valid + head + utf("out") + utf("1.0") + utf("2.0") + "\x00\x10admin",
			"admin"},
		{"length byte reads as a slot path", valid + head + utf("o\xffut") + utf("1.0") + utf("2.0") + utf(slash47),
			"mallory"},
	}
	for _, c := range cases {
		assertNoLeak(t, c.name, []byte(c.rec), c.user)
	}
}

// Under --mask only operation constants at framed operation fields print; slot paths, slotNames
// and values are masked too, and there is no role-account exception.
func TestStringsMaskAuditAllowlist(t *testing.T) {
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	tail := "\x00\x01Logout\x00\x01admin\x00\x01Removed\x00\x01Changedx\x00\x01admins\x00\x01/a/b"
	got := strings.Join(maskedStrings([]byte(valid+tail)), ",")
	want := "Changed,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<USER-MASKED>," +
		"<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>"
	if got != want {
		t.Errorf("audit section = %q, want %q", got, want)
	}
}

// Rule v5 applies to every section: no audit detection, no old Python mask() fallback.
func TestStringsMaskNonAuditSection(t *testing.T) {
	rec := []byte("\x00\x01mallory\x00\x01QWxhZGRpbjpvcGVuIHNlc2FtZQ==\x00\x01Change\x00\x01admin")
	want := "<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>,<VALUE-MASKED>"
	if got := strings.Join(maskedStrings(rec), ","); got != want {
		t.Errorf("non-audit section = %q, want %q", got, want)
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
		assertNoLeak(t, c.name, []byte(c.rec), c.user)
	}
	// A framed operation field still prints; its slot-path target does not.
	rec := utf("Login") + utf("/Config/R1") + utf("") + utf("") + utf("") + utf("jdoe")
	if got, want := strings.Join(maskedStrings([]byte(rec)), ","), "Login,<VALUE-MASKED>,<USER-MASKED>"; got != want {
		t.Errorf("framed login = %q, want %q", got, want)
	}
}

// Rule v5 closes the inputs that leaked a user name through rule v4 (adversarial review):
// slot-path allowlist evasions, forged operation positions and audit-detection evasion.
func TestStringsMaskV4Leaks(t *testing.T) {
	record := func(op, target, user string) string {
		return utf(op) + utf(target) + utf("out") + utf("1") + utf("2") + utf(user)
	}
	// A record that fails to frame (bad modified UTF-8 in its slotName) whose userName is user.
	damaged := func(user string) string {
		return utf("Changed") + utf("/a") + utf("o\xffut") + utf("1") + utf("2") + utf(user)
	}
	cases := []struct{ name, rec, user string }{
		{"user Login heads a forged record after a damaged one",
			damaged("Login") + record("Changed", "/b", "ann"), "Login"},
		{"user Changed heads a forged record after a damaged one",
			damaged("Changed") + record("Changed", "/b", "ann"), "Changed"},
		{"user as part of a path segment", record("Changed", "/Users/jdoe_home", "ann"), "jdoe"},
		{"user as a path segment, never an identity", record("Changed", "/Config/Owners/jdoe", "ann"), "jdoe"},
		{"user as a middle segment", record("Changed", "/x/eve/ops", "ann"), "eve"},
		{"path cut by a non-ASCII byte", record("Changed", "/x/jdoe\xc3\xbc", "ann"), "jdoe"},
		{"user under a Users folder", record("Changed", "/Services/Users/jdoe", "ann"), "jdoe"},
		{"path with a space", record("Changed", "/x/jdoe home", "ann"), "jdoe"},
		{"path cut by a NUL byte", record("Changed", "/x/jdoe\x00z", "ann"), "jdoe"},
		{"schema off and damaged operation names", record("Chamged", "/a", "jdoe") + utf("Chamged"), "jdoe"},
		{"non-ASCII user in a non-audit section", "\x00\x07J\xc3\xbcrgen\x00\x01x", "rgen"},
	}
	for _, c := range cases {
		assertNoLeak(t, c.name, []byte(c.rec), c.user)
	}
	// The damaged record's range ends at its declared userName end: a record after it prints.
	rec := damaged("ann") + record("Changed", "/b", "bob")
	if got := maskedStrings([]byte(rec)); strings.Count(strings.Join(got, ","), "Changed") != 1 {
		t.Errorf("the record after a damaged one must print its operation: %q", got)
	}
}

// maskedLineRE matches every line --mask may print besides --strings tokens: the file path
// argument, header numbers, the masked history id, record_type and schema field names (type-system
// identifiers), --ops lines (the name is checked separately) and the --ops framing error.
var maskedLineRE = regexp.MustCompile(`^(# fixture\.hdb|` +
	`magic=A106F11E version=\d+ data_offset=\d+ data_section=\d+ bytes|history_id=<VALUE-MASKED>|` +
	`record_type=history:AuditRecord|schema \(7 fields\): timestamp, operation, target, slotName, ` +
	`oldValue, value, userName|op (.*\S) *= \d+(  \(\+\d+ with an empty, non-slot or 4096\+ byte target, ` +
	`not counted\))?|--ops: 0 operation strings decoded from \d+ record bytes \(schema has 'operation'\): ` +
	`the record framing did not match)$`)

// verbatimNotAllowed returns the first output line that prints something verbatim other than an
// auditOperations constant or a non-user field (maskedLineRE).
func verbatimNotAllowed(out string) (string, bool) {
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if l == userMasked || l == valueMasked || isAuditOperation(l) {
			continue
		}
		m := maskedLineRE.FindStringSubmatch(l)
		if m == nil || m[2] != "" && m[2] != valueMasked && !isAuditOperation(m[2]) {
			return l, true
		}
	}
	return "", false
}

// maskCombos are the --mask output modes: every non-empty combination of --strings, --ops and
// --schema.
var maskCombos = func() []Options {
	var c []Options
	for bits := 1; bits < 8; bits++ {
		c = append(c, Options{Path: "fixture.hdb", Strings: bits&1 != 0, Ops: bits&2 != 0,
			Schema: bits&4 != 0, Mask: true})
	}
	return c
}()

// Property: no byte mutation of the audit fixture plus planted users (overwrite, insert, delete,
// truncate, '\x00\x02/x' and '\x00\x05Login' inserts; fixed seed), under any --mask output mode
// (maskCombos, one per iteration in turn) and with a user in the history id, prints anything verbatim but an AuditEvent
// operation constant or a non-user field (the rule v5 structural invariant), so no planted user
// prints.
func TestMaskNeverPrintsAUserOfAMutatedFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/inputs/audit.hdb")
	if err != nil {
		t.Fatal(err)
	}
	f, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
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
	rng := rand.New(rand.NewSource(20261017))
	special := []byte{0x00, 0x01, '/', '.', ' ', 'A', 0x7f, 0x80, 0xc0, 0xff}
	insert := func(rec []byte, k int, b []byte) []byte { return append(rec[:k:k], append(b, rec[k:]...)...) }
	for i := 0; i < 20000; i++ {
		rec := []byte(section)
		if rng.Intn(2) == 0 { // a '/x' string after a user makes it read as a countOps operation
			u := users[rng.Intn(len(users))]
			if k := bytes.Index(rec, []byte(u)); k >= 0 {
				rec = insert(rec, k+len(u), []byte("\x00\x02/x"))
			}
		}
		if rng.Intn(2) == 0 { // an operation-shaped string anywhere
			rec = insert(rec, rng.Intn(len(rec)+1), []byte("\x00\x05Login"))
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
				rec = insert(rec, pos, []byte{b})
			default:
				rec[pos] = b
			}
		}
		lines := maskedStrings(rec)
		if l, bad := verbatimNotOp(lines); bad {
			t.Fatalf("iteration %d: %q prints verbatim for section %q:\n%s", i, l, rec, strings.Join(lines, "\n"))
		}
		for _, l := range lines {
			for _, u := range users {
				if strings.Contains(l, u) {
					t.Fatalf("iteration %d: %q leaks for section %q", i, u, rec)
				}
			}
		}
		{ // every mode in turn through report (all of them per iteration would be 7x slower)
			o := maskCombos[i%len(maskCombos)]
			var out, errb bytes.Buffer
			report(o, &file{version: 2, offset: 1600, xml: auditXML("/S/JDoe"), section: rec},
				cli.Stdio{Out: &out, Err: &errb})
			all := out.String() + errb.String()
			if l, bad := verbatimNotAllowed(all); bad {
				t.Fatalf("iteration %d %+v: %q prints verbatim for section %q:\n%s", i, o, l, rec, all)
			}
			for _, u := range users {
				if strings.Contains(all, u) {
					t.Fatalf("iteration %d %+v: %q leaks for section %q:\n%s", i, o, u, rec, all)
				}
			}
		}
	}
}

// auditXML is a HistoryConfig whose history id is id, with the AuditRecord schema.
func auditXML(id string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
<p m="h=history" t="h:HistoryConfig">
 <p n="id" t="h:HistoryId" v="` + id + `"/>
 <p n="recordType" t="b:TypeSpec" v="history:AuditRecord"/>
 <p n="schema" t="h:HistorySchema" v="timestamp,baja:AbsTime;operation,baja:String;target,baja:String;slotName,baja:String;oldValue,baja:String;value,baja:String;userName,baja:String"/>
</p>
</bajaObjectGraph>`
}

// writeHdb writes a version-2 .hdb with the audit config of history id and the record section.
func writeHdb(t *testing.T, id string, section []byte) string {
	t.Helper()
	xml := auditXML(id)
	head := make([]byte, 12)
	copy(head, magic)
	binary.BigEndian.PutUint32(head[4:], 2)
	binary.BigEndian.PutUint32(head[8:], uint32(12+len(xml)))
	path := t.TempDir() + "/audit.hdb"
	if err := os.WriteFile(path, append(append(head, xml...), section...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runOut(t *testing.T, o Options) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(o, cli.Stdio{Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

// Under --mask, --ops prints only auditOperations constants verbatim: every other decoded name
// folds into one "op <VALUE-MASKED>" line after the constant lines (adversarial finding on v5).
func TestOpsMaskFoldsNonConstantNames(t *testing.T) {
	valid := utf("Changed") + utf("/a/b") + utf("out") + utf("1.0") + utf("2.0") + utf("ann")
	rec := valid + "\x00\x07Mallory\x00\x02/x" + "\x00\x05Trudy\x00\x02/y" + "\x00\x05Trudy\x00\x02/z"
	path := writeHdb(t, "/S/Test", []byte(rec))
	out, errs, code := runOut(t, Options{Path: path, Ops: true, Mask: true})
	if code != cli.ExitFound {
		t.Fatalf("code %d, stderr %q", code, errs)
	}
	want := "op Changed        = 1\nop <VALUE-MASKED> = 3\n"
	if !strings.HasSuffix(out, want) {
		t.Errorf("--ops --mask output:\n%s\nwant suffix:\n%s", out, want)
	}
	for _, o := range []Options{{Path: path, Ops: true, Mask: true}, {Path: path, Ops: true, Strings: true, Mask: true}} {
		out, _, _ := runOut(t, o)
		for _, u := range []string{"Mallory", "Trudy"} {
			if strings.Contains(out, u) {
				t.Errorf("%+v: %q leaks:\n%s", o, u, out)
			}
		}
	}
	// Without --mask the names print as Python does.
	if out, _, _ := runOut(t, Options{Path: path, Ops: true}); !strings.Contains(out, "op Mallory        = 1\n") {
		t.Errorf("unmasked --ops lost a name:\n%s", out)
	}
}

// Under --mask, --schema prints history_id=<VALUE-MASKED>; record_type and field names stay.
func TestSchemaMaskHidesHistoryID(t *testing.T) {
	path := writeHdb(t, "/S/JDoe", []byte(utf("Changed")+utf("/a")))
	out, _, code := runOut(t, Options{Path: path, Schema: true, Mask: true})
	if code != cli.ExitFound {
		t.Fatalf("code %d", code)
	}
	for _, want := range []string{"history_id=<VALUE-MASKED>\n", "record_type=history:AuditRecord\n",
		"schema (7 fields): timestamp, operation, target, slotName, oldValue, value, userName\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("--schema --mask lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "JDoe") {
		t.Errorf("history id leaks under --mask:\n%s", out)
	}
	if out, _, _ := runOut(t, Options{Path: path, Schema: true}); !strings.Contains(out, "history_id=/S/JDoe\n") {
		t.Errorf("unmasked --schema lost the history id:\n%s", out)
	}
}

// hostileSection is n framed login records, each followed by a Login candidate that fails to
// frame (its sixth string is not modified UTF-8): every framed record is checked against every
// failed range, and every printable run against every identity field.
func hostileSection(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(utf("Login") + utf("u"+strconv.Itoa(i)) + utf("slot") + utf("old") + utf("new") + utf("user"))
		b.WriteString(utf("Login") + utf("a") + utf("b") + utf("c") + utf("d") + "\x00\x01\xff")
	}
	return []byte(b.String())
}

// --mask stays near-linear in the record count: four times the hostile records may cost about four
// times as long, not sixteen (the quadratic planMask took 14 s for 20k). A ratio of two sizes, best of
// three runs each, instead of a wall-clock cap, so a slow or -race build does not flake.
func TestMaskScalesToManyRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("timing ratio test; run without -short")
	}
	best := func(n int) time.Duration {
		rec := hostileSection(n)
		var min time.Duration
		for i := 0; i < 3; i++ {
			start := time.Now()
			lines := maskedStrings(rec)
			if d := time.Since(start); i == 0 || d < min {
				min = d
			}
			if l, bad := verbatimNotOp(lines); bad {
				t.Fatalf("rule v5: %q printed verbatim", l)
			}
		}
		return min
	}
	small, large := best(5000), best(20000)
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("--strings --mask: 5k records %s, 20k records %s (ratio %.1f > 8: not near-linear)", small, large, ratio)
	}
}

// spans.any answers exactly what the quadratic scans of planMask and maskRun answered.
func TestSpansMatchesTheLinearScan(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	for round := 0; round < 200; round++ {
		ranges := make([][2]int, rng.Intn(12))
		for i := range ranges {
			a := rng.Intn(40)
			ranges[i] = [2]int{a, a + rng.Intn(10)}
		}
		sp := newSpans(ranges)
		for lo := -1; lo < 52; lo++ {
			for hi := lo; hi < lo+6; hi++ {
				want := false
				for _, r := range ranges {
					want = want || r[0] < hi && lo < r[1]
				}
				if got := sp.any(lo, hi); got != want {
					t.Fatalf("ranges %v: any(%d, %d) = %v, want %v", ranges, lo, hi, got, want)
				}
			}
		}
	}
}
