package bm25

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
)

// fixtures is testdata/fixtures.json (frozen output of the retired testdata/gen.py).
type fixtures struct {
	Log          map[string]family   `json:"log"`
	LogSamples   [][2]string         `json:"log_samples"`
	Log1p        map[string]family   `json:"log1p"`
	Log1pSamples [][2]string         `json:"log1p_samples"`
	RawSum       [][]json.RawMessage `json:"sum"`
	Textsearch   []struct {
		Query      string      `json:"query"`
		RequireAll bool        `json:"require_all"`
		Factors    [][2]string `json:"factors"`
		Ranked     [][2]string `json:"ranked"`
	} `json:"textsearch"`
	ClassRank struct {
		Names     []string          `json:"names"`
		Lengths   []int             `json:"lengths"`
		AvgLen    string            `json:"avg_len"`
		Prior     map[string]string `json:"prior"`
		Importers map[string]int    `json:"importers"`
		Cases     []struct {
			Query    string                    `json:"query"`
			Terms    []string                  `json:"terms"`
			Postings map[string]map[string]int `json:"postings"`
			Scores   map[string]string         `json:"scores"`
		} `json:"cases"`
	} `json:"classrank"`
	FTS5 struct {
		NRows       int                          `json:"n_rows"`
		Tokens      int                          `json:"tokens"`
		NHit        map[string]int               `json:"n_hit"`
		TitleWeight float64                      `json:"title_weight"`
		Found       map[string][]json.RawMessage `json:"found"`
		Cases       []struct {
			Groups [][]string `json:"groups"`
			Mode   string     `json:"mode"`
			Rows   [][]any    `json:"rows"`
		} `json:"cases"`
	} `json:"fts5"`
}

type family struct {
	N      int    `json:"n"`
	SHA256 string `json:"sha256"`
}

func load(t *testing.T) fixtures {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func fl(t *testing.T, s string) float64 {
	t.Helper()
	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func hx(x float64) string { return strconv.FormatFloat(x, 'x', -1, 64) }

// lcg, logFamilies and log1pFamilies rebuild the retired gen.py's inputs exactly.
func lcg(n int, seed uint64) []uint64 {
	out := make([]uint64, n)
	x := seed
	for i := range out {
		x = x*6364136223846793005 + 1442695040888963407
		out[i] = x
	}
	return out
}

func logFamilies() map[string][]float64 {
	var idf []float64
	for n := 1; n <= 1200; n++ {
		for df := 0; df <= n; df++ {
			r := (float64(n-df) + 0.5) / (float64(df) + 0.5)
			idf = append(idf, r, 1+r)
		}
	}
	var bits []float64
	for _, b := range lcg(300000, 1) {
		if x := math.Float64frombits(b & 0x7FEFFFFFFFFFFFFF); x > 0 {
			bits = append(bits, x)
		}
	}
	var near []float64
	for k := -(1 << 28); k < 1<<28; k += 997 {
		near = append(near, 1+float64(k)*0x1p-32)
	}
	return map[string][]float64{"idf": idf, "bits": bits, "near_one": near}
}

func log1pFamilies() map[string][]float64 {
	var ints, pos, neg, small []float64
	for n := 0; n <= 300000; n++ {
		ints = append(ints, float64(n))
	}
	for _, b := range lcg(200000, 2) {
		pos = append(pos, math.Float64frombits(b%0x43B0000000000000))
	}
	for _, b := range lcg(200000, 3) {
		neg = append(neg, -math.Float64frombits(b%0x3FF0000000000000))
	}
	for k := -20000; k < 20000; k += 7 {
		small = append(small, float64(k)*0x1p-40)
	}
	return map[string][]float64{"ints": ints, "pos": pos, "neg": neg, "small": small}
}

func digest(xs []float64, f func(float64) float64) string {
	h := sha256.New()
	var b [8]byte
	for _, x := range xs {
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(f(x)))
		h.Write(b[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func checkFamilies(t *testing.T, name string, fams map[string][]float64, want map[string]family, f func(float64) float64) {
	t.Helper()
	if len(fams) != len(want) {
		t.Fatalf("%s: %d families, fixture %d", name, len(fams), len(want))
	}
	for k, xs := range fams {
		w := want[k]
		if len(xs) != w.N {
			t.Fatalf("%s/%s: %d inputs, Python %d (input rebuild drifted)", name, k, len(xs), w.N)
		}
		if got := digest(xs, f); got != w.SHA256 {
			t.Errorf("%s/%s: results differ from CPython over %d inputs", name, k, w.N)
		}
	}
}

func TestLogMatchesCPython(t *testing.T) {
	f := load(t)
	for _, s := range f.LogSamples {
		if got := hx(Log(fl(t, s[0]))); got != hx(fl(t, s[1])) {
			t.Errorf("Log(%s) = %s, CPython %s", s[0], got, s[1])
		}
	}
	checkFamilies(t, "log", logFamilies(), f.Log, Log)
}

func TestLog1pMatchesCPython(t *testing.T) {
	f := load(t)
	for _, s := range f.Log1pSamples {
		if got := hx(Log1p(fl(t, s[0]))); got != hx(fl(t, s[1])) {
			t.Errorf("Log1p(%s) = %s, CPython %s", s[0], got, s[1])
		}
	}
	checkFamilies(t, "log1p", log1pFamilies(), f.Log1p, Log1p)
}

func TestLogSpecialValues(t *testing.T) {
	if !math.IsInf(Log(0), -1) || !math.IsNaN(Log(-1)) || !math.IsInf(Log(math.Inf(1)), 1) || Log(1) != 0 {
		t.Error("Log special values")
	}
	if !math.IsInf(Log1p(-1), -1) || !math.IsNaN(Log1p(-2)) || Log1p(0) != 0 || !math.IsInf(Log1p(math.Inf(1)), 1) {
		t.Error("Log1p special values")
	}
}

func TestPythonSum(t *testing.T) {
	for _, c := range load(t).RawSum {
		var xs []string
		var want string
		if err := json.Unmarshal(c[0], &xs); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(c[1], &want); err != nil {
			t.Fatal(err)
		}
		in := make([]float64, len(xs))
		for i, s := range xs {
			in[i] = fl(t, s)
		}
		if got := PythonSum(in); hx(got) != hx(fl(t, want)) {
			t.Errorf("PythonSum(%v) = %s, Python %s", xs, hx(got), want)
		}
	}
}

// fixtureDocs mirrors the retired gen.py's DOCS.
var fixtureDocs = []Doc[string]{
	{"d00", "Alarm service", "The alarm service routes alarms to recipients and consoles."},
	{"d01", "Alarm console", "Acknowledge alarms in the console; alarm ack and alarm class."},
	{"d02", "History service", "Histories record point values; the history service collects them."},
	{"d03", "BACnet network", "Configure the BACnet network, devices and points discovery."},
	{"d04", "Modbus network", "Configure Modbus devices, registers and polling."},
	{"d05", "Schedules", "Weekly schedules, holidays and calendar events control points."},
	{"d06", "Configuración de alarmas", "Cómo configurar las alarmas de temperatura alta en la estación."},
	{"d07", "Estación", "La estación guarda la configuración y los históricos."},
	{"d08", "Alarm service", "The alarm service routes alarms to recipients and consoles."},
	{"d09", "Point", "A control point has a value, a status and an override."},
	{"d10", "", "Temperature setpoint and high temperature alarm limits for a cold room."},
	{"d11", "Defrost", "Defrost cycles of the evaporator; coil temperature terminates defrost."},
	{"d12", "Compressor staging", "Suction pressure setpoint and differential stage the compressors."},
	{"d13", "Empty", ""},
	{"d14", "Servlet", "Register a web servlet in the module; BWebServlet handles GET and POST."},
	{"d15", "Users and roles", "User passwords, roles and permissions; authentication of sessions."},
	{"d16", "Point", "A control point has a value, a status and an override."},
	{"d17", "Network tuning", "Tuning policies: poll frequency, retry and timeout of the network."},
	{"d18", "Graphics", "Px graphics views bind to points; widgets and palettes."},
	{"d19", "Alarm alarm alarm", "alarm alarm alarm alarm alarm alarm alarm alarm alarm alarm"},
}

func TestIndexMatchesTextsearchBM25(t *testing.T) {
	f := load(t)
	ix := NewIndex(fixtureDocs, Textsearch, 8)
	for _, c := range f.Textsearch {
		var factors []Factor
		for _, p := range c.Factors {
			factors = append(factors, Factor{p[0], fl(t, p[1])})
		}
		got := ix.Rank(c.Query, c.RequireAll, factors)
		if len(got) != len(c.Ranked) {
			t.Errorf("%q all=%v: %d hits, Python %d", c.Query, c.RequireAll, len(got), len(c.Ranked))
			continue
		}
		for i, h := range got {
			if h.Key != c.Ranked[i][1] || hx(h.Score) != hx(fl(t, c.Ranked[i][0])) {
				t.Errorf("%q all=%v #%d: (%s, %s), Python (%s, %s)", c.Query, c.RequireAll, i,
					hx(h.Score), h.Key, c.Ranked[i][0], c.Ranked[i][1])
			}
		}
	}
}

// TestClassRankScores proves Params.Rank with ClassRank, times the importer
// prior 1 + 0.3 * log1p(importers), gives class_rank.rank_all's scores.
func TestClassRankScores(t *testing.T) {
	cr := load(t).ClassRank
	for name, want := range cr.Prior {
		if got := 1 + float64(0.3*Log1p(float64(cr.Importers[name]))); hx(got) != hx(fl(t, want)) {
			t.Errorf("prior %s = %s, Python %s", name, hx(got), want)
		}
	}
	for _, c := range cr.Cases {
		st := Stats{NDocs: len(cr.Names), AvgLen: fl(t, cr.AvgLen), DF: map[string]int{},
			Post: map[string]map[int]int{}, Len: cr.Lengths}
		for term, p := range c.Postings {
			st.DF[term] = len(p)
			st.Post[term] = map[int]int{}
			for d, n := range p {
				i, _ := strconv.Atoi(d)
				st.Post[term][i] = n
			}
		}
		got := map[string]string{}
		for _, r := range ClassRank.Rank(c.Terms, st, false, nil) {
			name := cr.Names[r.Doc]
			got[name] = hx(r.Score * fl(t, cr.Prior[name]))
		}
		if len(got) != len(c.Scores) {
			t.Errorf("%q: %d scored classes, Python %d", c.Query, len(got), len(c.Scores))
		}
		for name, want := range c.Scores {
			if got[name] != hx(fl(t, want)) {
				t.Errorf("%q %s: %s, Python %s", c.Query, name, got[name], want)
			}
		}
	}
}

// TestFTS5Scores proves FTS5 Weight/Norm/Term reproduce fulltext_scan rows():
// every phrase of every group adds idf * f * (k1+1) / (f + norm), in order.
func TestFTS5Scores(t *testing.T) {
	fx := load(t).FTS5
	avg := float64(fx.Tokens) / float64(fx.NRows)
	type doc struct {
		length int
		tf     map[string][2]int
	}
	docs := map[int]doc{}
	for id, v := range fx.Found {
		var d doc
		if err := json.Unmarshal(v[0], &d.length); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(v[1], &d.tf); err != nil {
			t.Fatal(err)
		}
		i, _ := strconv.Atoi(id)
		docs[i] = d
	}
	ids := make([]int, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, c := range fx.Cases {
		want := map[int]string{}
		for _, r := range c.Rows {
			want[int(r[0].(float64))] = r[1].(string)
		}
		got := 0
		for _, id := range ids {
			d := docs[id]
			matched := 0
			for _, g := range c.Groups {
				for _, a := range g {
					if _, ok := d.tf[a]; ok {
						matched++
						break
					}
				}
			}
			if matched == 0 || c.Mode == "and" && matched < len(c.Groups) {
				continue
			}
			got++
			score := 0.0
			norm := FTS5.Norm(float64(d.length), avg)
			for _, g := range c.Groups {
				for _, a := range g {
					if tf, ok := d.tf[a]; ok {
						f := float64(float64(tf[0])*fx.TitleWeight) + float64(tf[1])
						score += FTS5.Term(FTS5.Weight(fx.NRows, fx.NHit[a]), f, norm)
					}
				}
			}
			if w, ok := want[id]; !ok || hx(score) != hx(fl(t, w)) {
				t.Errorf("%v %s doc %d: %s, Python %q", c.Groups, c.Mode, id, hx(score), w)
			}
		}
		if got != len(want) {
			t.Errorf("%v %s: %d rows, Python %d", c.Groups, c.Mode, got, len(want))
		}
	}
}
