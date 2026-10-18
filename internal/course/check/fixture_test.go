package check

import (
	"archive/zip"
	"io/fs"
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
	// Every other file of the station (shared/ CSVs, history/ files) is copied as it is.
	err = filepath.WalkDir(goodStation, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "config.bog" {
			return err
		}
		rel, err := filepath.Rel(goodStation, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	plain, zipped := runAll(t, goodStation, fixtureInstall, RoleTraining), runAll(t, dir, fixtureInstall, RoleTraining)
	if len(plain) == 0 || len(plain) != len(zipped) {
		t.Fatalf("plain station gave %d results, zipped %d", len(plain), len(zipped))
	}
	for i := range plain {
		if plain[i].Verdict != Pass || plain[i].Verdict != zipped[i].Verdict || plain[i].Evidence != zipped[i].Evidence {
			t.Errorf("%s: plain %s %q, zipped %s %q", plain[i].ID, plain[i].Verdict, plain[i].Evidence,
				zipped[i].Verdict, zipped[i].Evidence)
		}
	}
}
