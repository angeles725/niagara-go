package course

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// miniStation is a hand-written station that exercises every station-side kind.
const miniStation = `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
<p h="1" m="b=baja" t="b:Station">
 <p n="stationName" v="Mini"/>
 <p n="Services" h="2" t="b:ServiceContainer">
  <p n="Svc" h="3" m="t=training" t="t:WorkOrderService"><p n="lastSuccess" v="2026-10-14T11:31:12.108-06:00"/></p>
 </p>
 <p n="Svc2" h="4" t="t:WorkOrderService"/>
 <p n="Tstat" h="5" t="t:Tstat">
  <p n="diff" t="b:StatusNumeric"><p n="value" v="2.0"/></p>
  <p n="Link" t="b:Link"><p n="sourceOrd" v="h:6"/><p n="sourceSlotName" v="out"/><p n="targetSlotName" v="cv"/></p>
  <p n="Link1" t="b:Link"><p n="sourceOrd" v="slot:/Point"/><p n="sourceSlotName" v="out"/><p n="targetSlotName" v="sp"/></p>
 </p>
 <p n="Point" h="6" m="c=control" t="c:NumericWritable"/>
 <p n="Dev" h="7" m="x=lateModule" t="x:Device"><p n="id" v="b"/></p>
 <p n="Dev2" h="8" t="x:Device"/>
 <p n="Net" h="9" m="n=net" t="n:Network">
  <p n="health" t="d:PingHealth"><p n="lastOkTime" v="2026-10-16T15:13:00.000-06:00"/></p>
  <p n="dev" h="b" t="n:Dev"/>
  <p n="tcp" h="a" t="n:TcpConfig"><p n="address" t="n:IpAddress"><p n="ipAddress" v="localhost"/><p n="port" v="23"/></p></p>
 </p>
</p>
</bajaObjectGraph>
`

func writeStation(t *testing.T, csvs ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.bog"), []byte(miniStation), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range csvs {
		if err := os.WriteFile(filepath.Join(dir, "shared", c), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckKinds(t *testing.T) {
	st, err := openStation(writeStation(t, "Mini_a.csv", "Mini_b.csv", "Other_c.csv"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Kind: kindType, Type: "training:WorkOrderService", Op: opEq, N: 2}, verdictPass, "found 2: Services/Svc, Svc2"},
		{Check{Kind: kindType, Type: "training:WorkOrderService", Within: "baja:ServiceContainer", Op: opEq, N: 2},
			verdictFail, "found 1 under baja:ServiceContainer (2 in total), want exactly 2: Services/Svc"},
		{Check{Kind: kindType, Type: "training:Add", Op: opMin, N: 1}, verdictFail, "found 0, want at least 1"},
		{Check{Kind: kindSlot, Type: "training:Tstat", Slot: "diff", Want: "2.0", Default: "0.0"}, verdictPass, "Tstat.diff = 2.0 (stored)"},
		{Check{Kind: kindSlot, Type: "training:Tstat", Slot: "action", Want: "reverse", Default: "direct"},
			verdictFail, "0 of 1 pass, want reverse: Tstat.action = direct (default)"},
		{Check{Kind: kindSlot, Type: "training:WorkOrderService", Slot: "lastSuccess", Set: true},
			verdictFail, "1 of 2 pass, want it stored: Svc2.lastSuccess = \"\" (default)"},
		{Check{Kind: kindSlot, Type: "lateModule:Device", Slot: "id", Default: "A", Distinct: true, Op: opMin, N: 2},
			verdictPass, "2 distinct id over 2 components: A, b"},
		{Check{Kind: kindSlot, Type: "training:Add", Slot: "out", Set: true}, verdictFail, "no training:Add component (searched 11 components)"},
		{Check{Kind: kindSlot, Type: "net:TcpConfig", Slot: "address.port", Want: "23", Default: "0"}, verdictPass, "Net/tcp.address.port = 23 (stored)"},
		{Check{Kind: kindSlot, Type: "net:TcpConfig", Slot: "address.sessionId", Want: "5", Default: "-1"},
			verdictFail, "0 of 1 pass, want 5: Net/tcp.address.sessionId = -1 (default)"},
		{Check{Kind: kindSlot, Type: "net:Network", Slot: "health.lastOkTime", Set: true}, verdictPass,
			"Net.health.lastOkTime = 2026-10-16T15:13:00.000-06:00 (stored)"},
		{Check{Kind: kindSlot, Type: "net:Network", Slot: "health.down", Want: "false", Default: "false"}, verdictPass,
			"Net.health.down = false (default)"},
		// down = false on a component never pinged is the default: Stored requires lastOkTime too.
		{Check{Kind: kindSlot, Type: "net:Network", Slot: "health.down", Want: "false", Default: "false",
			Stored: []string{"health.lastOkTime"}}, verdictPass, "Net.health.down = false (default)"},
		{Check{Kind: kindSlot, Type: "net:Dev", Slot: "health.down", Want: "false", Default: "false",
			Stored: []string{"health.lastOkTime"}}, verdictFail,
			"0 of 1 pass, want false with health.lastOkTime stored: Net/dev.health.lastOkTime = \"\" (default)"},
		{Check{Kind: kindLink, From: "control:NumericWritable", FromSlot: "out", Type: "training:Tstat", Slot: "cv", Op: opMin, N: 1},
			verdictPass, "found 1: Point.out -> Tstat.cv"},
		{Check{Kind: kindLink, From: "control:NumericWritable", FromSlot: "out", Type: "training:Tstat", Slot: "sp", Op: opMin, N: 1},
			verdictPass, "found 1: Point.out -> Tstat.sp"},
		{Check{Kind: kindLink, From: "control:NumericWritable", FromSlot: "out", Type: "training:Tstat", Slot: "diff", Op: opMin, N: 1},
			verdictFail, "found 0, want at least 1 (searched 2 links)"},
		{Check{Kind: kindFiles, Glob: "shared/{station}_*.csv", Op: opEq, N: 2}, verdictPass, "found 2 shared/Mini_*.csv: Mini_a.csv … Mini_b.csv"},
		{Check{Kind: kindFiles, Glob: "shared/{station}_*.csv", Op: opEq, N: 3}, verdictFail, "found 2 shared/Mini_*.csv, want exactly 3: Mini_a.csv … Mini_b.csv"},
		{Check{Kind: kindJar, Jar: trainingRT}, verdictSkip, "no --install DIR: modules/training-rt.jar not checked"},
	}
	for _, c := range cases {
		r := st.run(&c.ck)
		if r.Verdict != c.verdict || r.Evidence != c.evidence {
			t.Errorf("%s %s.%s: %s %q, want %s %q", c.ck.Kind, c.ck.Type, c.ck.Slot, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
	if st.name != "Mini" {
		t.Errorf("station name %q, want the stationName slot Mini", st.name)
	}
}

// TestCheckJarKinds runs the jar kinds over the synthetic install: a missing jar is FAIL, an
// unreadable one is SKIP (the check could not run), never PASS.
func TestCheckJarKinds(t *testing.T) {
	install := t.TempDir()
	if err := os.MkdirAll(filepath.Join(install, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, j := range []string{"training-rt", "trafficLightServer-rt"} {
		data, err := os.ReadFile(filepath.Join(fixtureInstall, "modules", j+".jar"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(install, "modules", j+".jar"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(install, "modules", "broken-rt.jar"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := openStation(writeStation(t), install, false)
	if err != nil {
		t.Fatal(err)
	}
	defer st.close()
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Kind: kindJar, Jar: trainingRT}, verdictPass, "modules/training-rt.jar (9 entries)"},
		{Check{Kind: kindJar, Jar: "training-wb"}, verdictFail, "missing modules/training-wb.jar under " + install},
		{Check{Kind: kindJar, Jar: "broken-rt"}, verdictSkip, "cannot read modules/broken-rt.jar: zip: not a valid zip file"},
		{Check{Kind: kindJarSigned, Jar: trainingRT}, verdictPass, "modules/training-rt.jar: META-INF/CODESIGN.SF, META-INF/CODESIGN.RSA"},
		{Check{Kind: kindJarSigned, Jar: "trafficLightServer-rt"}, verdictPass,
			"modules/trafficLightServer-rt.jar: META-INF/CODESIGN.SF, META-INF/CODESIGN.EC"},
		{Check{Kind: kindJarEntry, Jar: trainingRT, Glob: "com/**/*.class", Op: opMin, N: 2}, verdictPass,
			"modules/training-rt.jar: 2 entries match com/**/*.class"},
		{Check{Kind: kindJarEntry, Jar: trainingRT, Glob: "com/**/*.class", Op: opMin, N: 3}, verdictFail,
			"modules/training-rt.jar: 2 entries match com/**/*.class, want at least 3 (searched 9 entries)"},
		{Check{Kind: kindJarText, Jar: trainingRT, Glob: "META-INF/module.xml", Contains: []string{`name="net-rt"`, "<name>X</name>"}},
			verdictFail, "modules/training-rt.jar!META-INF/module.xml lacks <name>X</name> (1 of 2 found)"},
		{Check{Kind: kindJarText, Jar: trainingRT, Glob: "nothing.txt", Contains: []string{"a"}},
			verdictFail, "modules/training-rt.jar: no entry nothing.txt (searched 9 entries)"},
	}
	for _, c := range cases {
		r := st.run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s %s: %s %q, want %s %q", c.ck.Kind, c.ck.Jar, c.ck.Glob, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}

// A dotted slot needs the nested values: a station read without them SKIPs the check instead of
// passing it on the type default.
func TestCheckNestedNeedsNestedLoad(t *testing.T) {
	st, err := openStation(writeStation(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	r := st.run(&Check{Kind: kindSlot, Type: "net:Network", Slot: "health.down", Want: "false", Default: "false"})
	if r.Verdict != verdictSkip || r.Evidence != "health.down is a nested slot: the station was read without nested values" {
		t.Errorf("%s %q", r.Verdict, r.Evidence)
	}
	if !needsNested([]*Check{{Kind: kindJar}, {Kind: kindSlot, Slot: "a.b"}}) || needsNested([]*Check{{Kind: kindSlot, Slot: "diff"}}) ||
		!needsNested([]*Check{{Kind: kindSlot, Slot: "diff", Stored: []string{"a.b"}}}) {
		t.Error("needsNested: want true only for a dotted slot (Slot or Stored)")
	}
}

func TestGlobRe(t *testing.T) {
	cases := []struct {
		glob, name string
		match      bool
	}{
		{"org/mockito/**", "org/mockito/a/B.class", true},
		{"org/mockito/**", "org/mockitox/B.class", false},
		{"**/B*.class", "B.class", true},
		{"**/B*.class", "a/b/Bx$1.class", true},
		{"META-INF/*.SF", "META-INF/X.SF", true},
		{"META-INF/*.SF", "META-INF/a/X.SF", false},
		{"module.palette", "moduleXpalette", false},
	}
	for _, c := range cases {
		if got := globRe(c.glob).MatchString(c.name); got != c.match {
			t.Errorf("globRe(%q) on %q = %v, want %v", c.glob, c.name, got, c.match)
		}
	}
}

// A station whose directory or name holds glob meta characters still matches only its own files.
func TestFileCheckEscapesStationPath(t *testing.T) {
	name, files := "St[1]*?", []string{"St[1]*?_a.csv", "S1_a.csv", "Sx_a.csv"}
	if runtime.GOOS == "windows" {
		// '*' and '?' are illegal in Windows file names; an unescaped '[1]' would match both St1_ files.
		name, files = "St[1]", []string{"St[1]_a.csv", "St1_a.csv", "St1_b.csv"}
	}
	base := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(base, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range files {
		if err := os.WriteFile(filepath.Join(base, "shared", n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &station{dir: base, name: name}
	v, ev := s.checkFiles(&Check{Kind: kindFiles, Glob: "shared/{station}_*.csv", Op: opEq, N: 1})
	if v != verdictPass {
		t.Errorf("verdict %s, evidence %q", v, ev)
	}
}
