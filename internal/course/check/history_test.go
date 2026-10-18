package check

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHistories writes .hdb files into a mini station's history/station/seg0 directory.
func writeHistories(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := writeStation(t)
	seg := filepath.Join(dir, "history", "station", "seg0")
	if err := os.MkdirAll(seg, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(seg, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHistoryKinds(t *testing.T) {
	audit := auditHDB("Mini", []auditRec{
		{"Added", "/Lab10", "NumericWritable1", "", "36,3 {ok} @ 16"},
		{"Changed", "/Services/WorkOrderService", "url", "http://localhost:3001/workOrders", "bad_url"},
		{"Changed", "/Services/WorkOrderService", "url", "bad_url", "http://localhost:3001/workOrders"},
	})
	log := logHDB("Mini", []logRec{
		{"training", 800, "Submitting work order"},
		{"training", 1000, "Work order failed: bad_url"},
		{"sys", 800, "Station started"},
	})
	dir := writeHistories(t, map[string][]byte{
		"AuditHistory.hdb": audit, "LogHistory.hdb": log, "Garbage.hdb": []byte("not a history"),
		"Point1.hdb": trendHDB("Mini", "Point1", []float64{10, 20, 20}),
		"Point2.hdb": trendHDB("Mini", "Point2", []float64{5, math.NaN()}),
	})
	st, err := Open(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	const seg = "history/station/seg0/"
	cases := []struct {
		ck       Check
		verdict  string
		evidence string
	}{
		{Check{Kind: kindAudit, Glob: seg + "AuditHistory.hdb", Hist: &HistMatch{Operation: `^Changed$`, SlotName: `^url$`,
			OldValue: `^bad_url$`}, Op: opEq, N: 1}, Pass,
			"history/station/seg0/AuditHistory.hdb: 1 of 3 audit records match; first 2026-10-15T18:42:34.959Z Changed /Services/WorkOrderService.url = http://localhost:3001/workOrders"},
		{Check{Kind: kindAudit, Glob: seg + "AuditHistory.hdb", Hist: &HistMatch{Operation: `^Added$`, Target: `/Lab10$`,
			SlotName: `^NumericWritable[1-9]$`}, Op: opMin, N: 9}, Fail,
			"history/station/seg0/AuditHistory.hdb: 1 of 3 audit records match, want at least 9 (operation =~ ^Added$, target =~ /Lab10$, slotName =~ ^NumericWritable[1-9]$); first"},
		{Check{Kind: kindAudit, Glob: seg + "AuditHistory.hdb", Hist: &HistMatch{Value: `^Folder$`}, Op: opMin, N: 1}, Fail,
			"history/station/seg0/AuditHistory.hdb: 0 of 3 audit records match, want at least 1 (value =~ ^Folder$)"},
		{Check{Kind: kindLog, Glob: seg + "LogHistory.hdb", Hist: &HistMatch{LogName: `^training$`, Level: `^SEVERE$`,
			Message: `bad_url`}, Op: opMin, N: 1}, Pass,
			"history/station/seg0/LogHistory.hdb: 1 of 3 log records match; first 2026-10-15T18:42:33.959Z SEVERE training: Work order failed: bad_url"},
		{Check{Kind: kindLog, Glob: seg + "LogHistory.hdb", Hist: &HistMatch{Level: `^FINE$`}, Op: opMin, N: 1}, Fail,
			"history/station/seg0/LogHistory.hdb: 0 of 3 log records match, want at least 1 (level =~ ^FINE$)"},
		{Check{Kind: kindTrend, Glob: seg + "Point1.hdb", Hist: &HistMatch{Min: 0, Max: 100}, Op: opMin, N: 3}, Pass,
			"history/station/seg0/Point1.hdb: 3 records, 10..20"},
		{Check{Kind: kindTrend, Glob: seg + "Point1.hdb", Hist: &HistMatch{Min: 0, Max: 100}, Distinct: true, Op: opMin, N: 3}, Fail,
			"history/station/seg0/Point1.hdb: 2 distinct values, 10..20, want at least 3"},
		{Check{Kind: kindTrend, Glob: seg + "Point1.hdb", Hist: &HistMatch{Min: 0, Max: 15}, Op: opMin, N: 1}, Fail,
			"history/station/seg0/Point1.hdb: 3 records, 10..20, want every value in 0..15: 20 at 2026-10-15T18:42:33.959Z"},
		// NaN is never within a range.
		{Check{Kind: kindTrend, Glob: seg + "Point*.hdb", Hist: &HistMatch{Min: 0, Max: 100}, Op: opMin, N: 1}, Fail,
			"history/station/seg0/Point2.hdb: 2 records"},
		{Check{Kind: kindTrend, Glob: seg + "Point1.hdb", Hist: &HistMatch{}, Op: opEq, N: 3}, Pass,
			"history/station/seg0/Point1.hdb: 3 records, 10..20"},
		{Check{Kind: kindAudit, Glob: seg + "Nothing.hdb", Hist: &HistMatch{}, Op: opMin, N: 1}, Fail, "no file history/station/seg0/Nothing.hdb"},
		{Check{Kind: kindAudit, Glob: seg + "Point1.hdb", Hist: &HistMatch{}, Op: opMin, N: 1}, Skip,
			"history/station/seg0/Point1.hdb holds history:NumericTrendRecord records, not audit"},
		{Check{Kind: kindTrend, Glob: seg + "LogHistory.hdb", Hist: &HistMatch{}, Op: opMin, N: 1}, Skip,
			"history/station/seg0/LogHistory.hdb holds history:LogRecord records, not trend"},
		{Check{Kind: kindAudit, Glob: seg + "Garbage.hdb", Hist: &HistMatch{}, Op: opMin, N: 1}, Skip,
			"cannot read history/station/seg0/Garbage.hdb: "},
		{Check{Kind: kindAudit, Glob: seg + "AuditHistory.hdb", Hist: &HistMatch{Target: `(`}, Op: opMin, N: 1}, Skip, "bad pattern ("},
	}
	for _, c := range cases {
		r := st.Run(&c.ck)
		if r.Verdict != c.verdict || !strings.HasPrefix(r.Evidence, c.evidence) {
			t.Errorf("%s %s %+v:\n got %s %q\nwant %s %q", c.ck.Kind, c.ck.Glob, *c.ck.Hist, r.Verdict, r.Evidence, c.verdict, c.evidence)
		}
	}
}

// Audit records are counted over every matching file; the evidence names the file count.
func TestHistoryAuditSeveralFiles(t *testing.T) {
	rec := []auditRec{{"Invoked", "/Lab1/FirstComponent", "execute", "", ""}}
	dir := writeHistories(t, map[string][]byte{"AuditHistory.hdb": auditHDB("Mini", rec), "AuditHistory2.hdb": auditHDB("Mini", rec)})
	st, err := Open(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	r := st.Run(&Check{Kind: kindAudit, Glob: "history/*/*/AuditHistory*.hdb", Hist: &HistMatch{SlotName: `^execute$`}, Op: opEq, N: 2})
	if want := "2 files history/*/*/AuditHistory*.hdb: 2 of 2 audit records match"; r.Verdict != Pass || !strings.HasPrefix(r.Evidence, want) {
		t.Errorf("%s %q, want PASS %q", r.Verdict, r.Evidence, want)
	}
}
