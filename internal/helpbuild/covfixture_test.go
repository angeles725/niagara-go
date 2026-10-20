package helpbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The synthetic data root of the coverage report tests.

// covReportBase writes a data root for the coverage report: classes of
// several modules (ties on the percentage, a module without classes
// documented, non-ASCII names), bajadoc-clean texts, methods and slots.
func covReportBase(t *testing.T, dir string) string {
	t.Helper()
	base := filepath.Join(dir, "report")
	files := map[string]string{
		"indexes/source-index.json": `{"BAlarm": {"module": "alarm-rt"}, "BAlarmUx": {"module": "alarm-ux"},` +
			` "BAlarmWb": {"module": "Alarm-WB"}, "BZeta": {"module": "zeta-rt"}, "BAlpha": {"module": "alpha-rt"},` +
			` "BNoMod": {}, "BÉcole": {"module": "école-rt"}, "BOrphan": {"module": "orphan"}, "Bx.txt": {"module": "zeta-rt"}}`,
		"indexes/class-index.json": `{"BAlarmWb": {"package": "javax.baja.alarm.wb"}, "BAlarmUx": {"name": "x"}}`,
		"indexes/method-index.json": `{"ack": [{"class": "BAlarm", "section": "methods"}, {"class": "BAlarm", "section": "fields"},` +
			` {"class": "BAlarmUx", "section": "methods"}, {"class": "BOther", "section": "methods"}, {"section": "methods"}],` +
			` "none": [], "close": [{"class": "BAlarmWb", "section": "methods"}]}`,
		"indexes/slots-index.json":       `{"BAlarm": {}, "BZeta": {}}`,
		"bajadoc-clean/alarm/BAlarm.txt": "x",
		"bajadoc-clean/zeta/BZeta.txt":   "x",
		"bajadoc-clean/alpha/BAlpha.txt": "x",
		"bajadoc-clean/BÉcole.txt":       "x",
		"bajadoc-clean/Bx.txt.txt":       "x",
		"bajadoc-clean/BNoMod.TXT":       "x",
		"guides/Spy/a.html":              "a",
		"guides/Spy/b.HTM":               "b",
		"guides/Spy/deep/c.html":         "c",
		"guides/Spy/d.txt":               "d",
		"guides/Ok/a.html":               "a",
		"guides/Ünï/a.htm":               "a",
		"guides/stray.html":              "s",
		"guides-clean/Spy/a.txt":         "a",
		"guides-clean/Ok/a.txt":          "a",
		"guides-clean/Ok/extra.txt":      "a",
		"guides-clean/Ünï/a.htm.txt":     "a",
	}
	for rel, text := range files {
		testkit.WriteFile(t, filepath.Join(base, rel), []byte(text))
	}
	if err := os.MkdirAll(filepath.Join(base, "guides", "Empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}
