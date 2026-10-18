package navbuild

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

func TestNormalizeVolatile(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"indented",
			"{\n  \"_meta\": {\n    \"source\": \"/a/organized\",\n    \"n\": 1,\n    \"build_time_sec\": 12.3,\n    \"timestamp\": \"2026-10-17T10:00:00\"\n  },\n  \"x\": {\"timestamp\": \"keep\"}\n}",
			"{\n  \"_meta\": {\n    \"source\": \"<SOURCE>\",\n    \"n\": 1,\n    \"build_time_sec\": 0,\n    \"timestamp\": \"<TIMESTAMP>\"\n  },\n  \"x\": {\"timestamp\": \"keep\"}\n}"},
		{"compact with a nested meta object and a space-separated timestamp",
			`{"_meta":{"source_root":"C:\\o \"x\"","d":{"1":2},"build_time_sec":0.0,"timestamp":"2026-10-17 10:00:00"},"source":"keep"}`,
			`{"_meta":{"source_root":"<SOURCE>","d":{"1":2},"build_time_sec":0,"timestamp":"<TIMESTAMP>"},"source":"keep"}`},
		{"no leading _meta", `{"a":{"timestamp":"t"}}`, `{"a":{"timestamp":"t"}}`},
		{"unterminated _meta", `{"_meta":{"timestamp":"t"`, `{"_meta":{"timestamp":"t"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(NormalizeVolatile([]byte(tc.in))); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestRound1(t *testing.T) {
	// Python: round(x, 1) for each x.
	for x, want := range map[float64]float64{0: 0, 0.04: 0, 0.05: 0.1, 0.25: 0.2, 0.35: 0.3, 2.675: 2.7, 12.349: 12.3, 7.0: 7.0} {
		if got := round1(x); got != want {
			t.Errorf("round1(%v) = %v, want %v", x, got, want)
		}
	}
}

// The streamed layout must equal pyjson.Dumps of the same tree, in both layouts.
func TestJSONWriterMatchesDumps(t *testing.T) {
	inner := object(Field{"a", strList([]string{"x", "é\n"})}, Field{"b", pyjson.NewObject()},
		Field{"c", pyjson.Value{Kind: pyjson.Array}}, Field{"d", optStr("")})
	whole := object(Field{"_meta", object(Field{"k", integer(1)})},
		Field{"items", object(Field{"one", inner}, Field{"two", inner})}, Field{"empty", pyjson.NewObject()})
	for name, o := range map[string]pyjson.DumpOpts{"indented": Indented, "compact": Compact} {
		t.Run(name, func(t *testing.T) {
			w := &jsonWriter{o: o}
			keys := []string{"_meta", "items", "empty"}
			w.object(0, 3, func(i int) string { return keys[i] }, func(i, level int) {
				switch i {
				case 0:
					w.fields(level, []Field{{"k", integer(1)}})
				case 1:
					w.object(level, 2, func(j int) string { return []string{"one", "two"}[j] },
						func(_, l int) { w.value(inner, l) })
				default:
					w.object(level, 0, nil, nil)
				}
			})
			if got, want := w.b.String(), pyjson.Dumps(whole, o); got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestStampedMeta(t *testing.T) {
	at := time.Date(2026, 10, 17, 9, 5, 7, 0, time.Local)
	got := stamped([]Field{{"description", str("d")}}, 1260*time.Millisecond, at)
	want := `{"description":"d","build_time_sec":1.3,"timestamp":"2026-10-17T09:05:07"}`
	if s := pyjson.Dumps(object(got...), Compact); s != want {
		t.Errorf("got %s, want %s", s, want)
	}
}

func TestScanPackagesSkipsSymlinkedDirs(t *testing.T) {
	root := t.TempDir()
	vf := filepath.Join(root, "vineflower")
	mustWrite(t, filepath.Join(vf, "com", "acme", "x", "A.java"), "class A {}")
	mustWrite(t, filepath.Join(root, "elsewhere", "org", "B.java"), "class B {}")
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(vf, "linked")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(filepath.Join(root, "missing.java"), filepath.Join(vf, "Dangling.java")); err != nil {
		t.Fatal(err)
	}
	count, pkgs, third := scanPackages(vf)
	// os.walk lists the dangling link as a file and never enters linked/.
	if count != 2 || !reflect.DeepEqual(pkgs, []string{"com.acme"}) || !reflect.DeepEqual(third, []string{"com.acme"}) {
		t.Errorf("got %d %v %v", count, pkgs, third)
	}
	if c, p, _ := scanPackages(filepath.Join(root, "absent")); c != 0 || len(p) != 0 {
		t.Errorf("absent vineflower: %d %v", c, p)
	}
}

func TestScanInventoryRejectsNonObjectRecon(t *testing.T) {
	org := t.TempDir()
	mustWrite(t, filepath.Join(org, "m", "m-rt", "pipeline", "fase1-recon.json"), "[1, 2]")
	_, err := ScanInventory(org)
	var notObject *reconNotObject
	if !errors.As(err, &notObject) {
		t.Fatalf("err = %v, want a reconNotObject (Python dies with AttributeError)", err)
	}
}

func TestBuildInventoryWritesIntoBaseDir(t *testing.T) {
	base := t.TempDir()
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local) }
	inv, err := BuildInventory(base, syntheticCorpus(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Submodules) != 6 || len(inv.NoRecon) != 4 || len(inv.UnreadableRecon) != 2 ||
		!reflect.DeepEqual(inv.Skipped, []string{"alpha/docs"}) {
		t.Errorf("submodules %d, no recon %v, unreadable %v, skipped %v",
			len(inv.Submodules), inv.NoRecon, inv.UnreadableRecon, inv.Skipped)
	}
	data, err := os.ReadFile(IndexPath(base, InventoryFile))
	if err != nil {
		t.Fatal(err)
	}
	v, err := pyjson.Loads([]rune(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := v.Get("_meta")
	if ts, _ := meta.Get("timestamp"); ts.S != "2026-01-02T03:04:05" {
		t.Errorf("timestamp %q", ts.S)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
