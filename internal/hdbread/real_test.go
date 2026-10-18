package hdbread

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// RealHDBEnv names a real .hdb history for TestRealHarborHistory. The data never enters the
// repo; `make parity` sets it to the HARBOR UMA14 Ret history of the niagara-research checkout:
//
//	clients/distech-merida-harbor/HM_BMS_backup_08_09_2026/HM_BMS/history/station/seg1/UMA14$20Ret.hdb
const RealHDBEnv = "HDBREAD_REAL_HDB"

// TestRealHarborHistory is the Go form of niagara-research
// pre-go-retire:tools/tests/test_t6c_hdb_trend.py RealHarborHistoryTest: internal consistency of
// the real UMA14 Ret history (no other copy of its values exists), plus its pinned record count.
// Skipped when HDBREAD_REAL_HDB is unset; set, a missing or undecodable file fails.
func TestRealHarborHistory(t *testing.T) {
	path := strings.TrimSpace(os.Getenv(RealHDBEnv))
	if path == "" {
		t.Skipf("%s unset: real HARBOR UMA14 Ret history not checked (make parity sets it)", RealHDBEnv)
	}
	h, err := ReadRecords(path)
	if err != nil {
		t.Fatalf("ReadRecords(%s): %v", path, err)
	}
	if h.RecordType != "history:NumericTrendRecord" || h.ID != "/HM_BMS/UMA14$20Ret" {
		t.Fatalf("id/type = %q %q, want /HM_BMS/UMA14$20Ret history:NumericTrendRecord", h.ID, h.RecordType)
	}
	const want = 682
	if len(h.Records) != want || h.Declared != want {
		t.Fatalf("records = %d (pages declare %d), want %d", len(h.Records), h.Declared, want)
	}
	ms := make([]int64, len(h.Records))
	for i, r := range h.Records {
		ms[i] = r.Millis
		v, ok := r.Trend.Value.(float64)
		if !ok || v <= 0 || v >= 60 { // return-air degC
			t.Errorf("record %d value = %#v, want a float64 in (0, 60)", i, r.Trend.Value)
		}
	}
	if !slices.IsSorted(ms) {
		t.Error("record timestamps are not in ascending order")
	}
	if got := h.Records[0].Timestamp; got != "2018-09-26T17:30:00.008Z" {
		t.Errorf("first timestamp = %s, want 2018-09-26T17:30:00.008Z", got)
	}
	steps := make([]int64, len(ms)-1)
	for i := range steps {
		steps[i] = ms[i+1] - ms[i]
	}
	slices.Sort(steps)
	// The config interval is 15 minutes; Python took steps[len // 2] within 1 s of it.
	if med := steps[len(steps)/2]; med < 900000-1000 || med > 900000+1000 {
		t.Errorf("median step = %d ms, want 900000 +- 1000 (the 15-minute interval)", med)
	}
}
