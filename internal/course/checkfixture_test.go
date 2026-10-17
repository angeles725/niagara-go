package course

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckZippedBog runs the spec over the good station stored as a config.bog ZIP (the
// form a real station writes); every verdict and evidence must equal the plain-XML run.
func TestCheckZippedBog(t *testing.T) {
	dir := t.TempDir()
	xml, err := os.ReadFile(filepath.Join(goodStation, "config.bog"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "config.bog"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("file.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(xml); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	csvs, err := filepath.Glob(filepath.Join(goodStation, "shared", "*.csv"))
	if err != nil || len(csvs) == 0 {
		t.Fatalf("fixture CSVs: %d, %v", len(csvs), err)
	}
	if err := os.Mkdir(filepath.Join(dir, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range csvs {
		if err := os.WriteFile(filepath.Join(dir, "shared", filepath.Base(c)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plain, zipped := runAll(t, goodStation, fixtureInstall), runAll(t, dir, fixtureInstall)
	if len(plain) == 0 || len(plain) != len(zipped) {
		t.Fatalf("plain station gave %d results, zipped %d", len(plain), len(zipped))
	}
	for i := range plain {
		if plain[i].Verdict != verdictPass || plain[i].Verdict != zipped[i].Verdict || plain[i].Evidence != zipped[i].Evidence {
			t.Errorf("%s: plain %s %q, zipped %s %q", plain[i].ID, plain[i].Verdict, plain[i].Evidence,
				zipped[i].Verdict, zipped[i].Evidence)
		}
	}
}
