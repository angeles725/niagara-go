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
