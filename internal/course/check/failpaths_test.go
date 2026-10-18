package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExamFailPaths runs the exam rows over stations/cert-bad (Limiter min 20 and max 80, an empty
// historyId, history.csv rows in another format, a network pinged but its device never) and an
// install whose websiteDriver-rt has no network class and asks no NETWORK_COMMUNICATION: each of
// those rows FAILs with what it found, every other row still PASSes.
func TestExamFailPaths(t *testing.T) {
	install := t.TempDir()
	if err := os.MkdirAll(filepath.Join(install, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	website := map[string]string{}
	for name, data := range fixtureJars[websiteRT] {
		website[name] = data
	}
	delete(website, "com/certExam/websiteDriver/BWebsiteDriverNetwork.class")
	website["com/certExam/websiteDriver/BWebsiteDriverNetworkHelper.class"] = "stand-in class\n" // a near name does not count
	website["META-INF/module.xml"] = "<module><permissions><name>REFLECTION</name></permissions></module>\n"
	for name, entries := range map[string]map[string]string{certRT: fixtureJars[certRT], websiteRT: website} {
		if err := os.WriteFile(filepath.Join(install, "modules", name+".jar"), buildJar(entries), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fails := map[string]string{
		"CEXAM.2":  "0 of 1 pass, want 25.0: Test/Limiter.min = 20.0 (stored)",
		"CEXAM.3":  "0 of 1 pass, want 75.0: Test/Limiter.max = 80.0 (stored)",
		"CEXAM.9":  `0 of 1 pass, want /Cristian_Angeles_Cert/AuditHistory: Services/HistoryExportService.historyId = "" (stored)`,
		"CEXAM.12": "0 of 1 files pass, want at least 1 line =~ ",
		"CEXAM.16": `0 of 1 pass, want it stored: Drivers/WebsiteDriverNetwork/WebsiteDriverDevice.health.lastOkTime = "" (default)`,
		"CEXAM.18": "modules/websiteDriver-rt.jar: 2 entries match **/websiteDriver/BWebsiteDriver{Network,Device,DeviceFolder}.class, want exactly 3",
		"CEXAM.19": "modules/websiteDriver-rt.jar!META-INF/module.xml lacks <name>NETWORK_COMMUNICATION</name>",
	}
	results := runAll(t, "../testdata/stations/cert-bad", install, RoleCert)
	if len(results) != len(examSpec) {
		t.Fatalf("%d exam results, want %d", len(results), len(examSpec))
	}
	for _, r := range results {
		want, fail := fails[r.ID]
		switch {
		case fail && (r.Verdict != Fail || !strings.HasPrefix(r.Evidence, want)):
			t.Errorf("%s: %s %q, want FAIL %q", r.ID, r.Verdict, r.Evidence, want)
		case !fail && r.Verdict != Pass:
			t.Errorf("%s: %s %q, want PASS", r.ID, r.Verdict, r.Evidence)
		}
	}
}

// CRA2.11 reads the Review2 fallbacks (nested fallback.value): equal values FAIL, and a point whose
// fallback is not stored counts with the default 0.0; a point outside Review2 is not read.
func TestReviewDistinctFallback(t *testing.T) {
	bogText := func(a, b string) string {
		fb := func(v string) string {
			if v == "" {
				return ""
			}
			return `<p n="fallback" t="b:StatusNumeric"><p n="value" v="` + v + `"/></p>`
		}
		return `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
<p h="1" m="b=baja" t="b:Station">
 <p n="Review2" h="2" t="b:Folder">
  <p n="NumericWritable" h="3" m="c=control" t="c:NumericWritable">` + fb(a) + `</p>
  <p n="NumericWritable1" h="4" t="c:NumericWritable">` + fb(b) + `</p>
 </p>
 <p n="Other" h="5" t="c:NumericWritable"><p n="fallback" t="b:StatusNumeric"><p n="value" v="9.0"/></p></p>
</p>
</bajaObjectGraph>
`
	}
	ck := specRow("CRA2.11")
	cases := []struct{ a, b, verdict, evidence string }{
		{"54.0", "33.0", Pass, "2 distinct fallback.value over 2 components: 33.0, 54.0"},
		{"5.0", "5.0", Fail, "1 distinct fallback.value over 2 components, want at least 2: 5.0"},
		{"", "0.0", Fail, "1 distinct fallback.value over 2 components, want at least 2: 0.0"},
		{"", "7.0", Pass, "2 distinct fallback.value over 2 components: 0.0, 7.0"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.bog"), []byte(bogText(c.a, c.b)), 0o644); err != nil {
			t.Fatal(err)
		}
		st, err := Open(dir, "", NeedsNested([]*Check{ck}))
		if err != nil {
			t.Fatal(err)
		}
		if r := st.Run(ck); r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("fallbacks %q, %q: %s %q, want %s %q", c.a, c.b, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}
