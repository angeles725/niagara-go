package helpbuild

import (
	"os"
	"path/filepath"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The tracked version manifest versions/<id>.manifest.json
// (version_setup.jar_manifest and the manifest dict of extract_and_index).

// jarRow is one jar_manifest row: the jar relative to the install, its
// size and sha256, and how many of its central-directory entries destPath
// maps (duplicates, directories and source archives counted as destPath
// counts them).
type jarRow struct {
	Jar           string
	Bytes         int64
	SHA256        string
	MappedEntries int
}

// jarManifest lists the doc jars of installDir (listDocJars without help
// jars) with their size, sha256 and mapped-entry count.
func jarManifest(installDir string) ([]jarRow, error) {
	rules, err := Default()
	if err != nil {
		return nil, err
	}
	jars, err := listDocJars(installDir, false, nil)
	if err != nil {
		return nil, err
	}
	base, err := pyAbspath(installDir)
	if err != nil {
		return nil, err
	}
	rows := []jarRow{}
	for _, jar := range jars {
		name := pyBasename(jar)
		infos, err := zipfile.ReadCentralDir(jar)
		if err != nil {
			return nil, err
		}
		row := jarRow{}
		for _, x := range infos {
			if rel, ok := destPath(name, x.Name, rules); ok && rel != "" {
				row.MappedEntries++
			}
		}
		abs, err := pyAbspath(jar)
		if err != nil {
			return nil, err
		}
		if row.Jar, err = filepath.Rel(base, abs); err != nil {
			return nil, err
		}
		row.Jar = filepath.ToSlash(row.Jar)
		st, err := os.Stat(jar)
		if err != nil {
			return nil, err
		}
		row.Bytes = st.Size()
		if row.SHA256, err = pycompat.FileSHA256(jar); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// manifestValue is the manifest dict in Python's key order.
func manifestValue(vid, label, install, dataRoot string, now time.Time, rep *FidelityReport, rows []jarRow) pyjson.Value {
	m := pyjson.NewObject()
	m.Set("version", pyjson.String(vid))
	m.Set("label", pyjson.String(label))
	m.Set("install_dir", pyjson.String(install))
	m.Set("data_root", pyjson.String(dataRoot))
	// isoformat(timespec="seconds") of an aware UTC datetime.
	m.Set("generated_utc", pyjson.String(now.UTC().Format("2006-01-02T15:04:05+00:00")))
	fid := pyjson.NewObject()
	fid.Set("jars", pyjson.IntValue(int64(rep.Jars)))
	fid.Set("checked", pyjson.IntValue(int64(rep.Checked)))
	fid.Set("matched", pyjson.IntValue(int64(rep.Matched)))
	m.Set("fidelity", fid)
	m.Set("eol_only_count", pyjson.IntValue(int64(len(rep.EOLOnly))))
	m.Set("mismatched", stringsValue(rep.Mismatched))
	m.Set("missing", stringsValue(rep.Missing))
	jars := make([]pyjson.Value, len(rows))
	for i, r := range rows {
		j := pyjson.NewObject()
		j.Set("jar", pyjson.String(r.Jar))
		j.Set("bytes", pyjson.IntValue(r.Bytes))
		j.Set("sha256", pyjson.String(r.SHA256))
		j.Set("mapped_entries", pyjson.IntValue(int64(r.MappedEntries)))
		jars[i] = j
	}
	m.Set("jars", pyjson.Value{Kind: pyjson.Array, A: jars})
	return m
}

func stringsValue(ss []string) pyjson.Value {
	a := make([]pyjson.Value, len(ss))
	for i, s := range ss {
		a[i] = pyjson.String(s)
	}
	return pyjson.Value{Kind: pyjson.Array, A: a}
}

// writeManifest writes versions/<vid>.manifest.json under repo and logs
// "manifest -> <path relative to repo>".
func writeManifest(res *SetupResult, repo, vid, label, dataRoot string, now func() time.Time) error {
	rows, err := jarManifest(res.Install)
	if err != nil {
		return err
	}
	if now == nil {
		now = time.Now
	}
	path := pycompat.Join(repo, "versions", vid+".manifest.json")
	v := manifestValue(vid, label, res.Install, dataRoot, now(), res.Fidelity, rows)
	if err := pyfs.WriteJSON(path, v, dumpManifest); err != nil {
		return err
	}
	res.ManifestPath = path
	absRepo, err1 := pyAbspath(repo)
	absPath, err2 := pyAbspath(path)
	rel, err3 := filepath.Rel(absRepo, absPath)
	if err1 != nil || err2 != nil || err3 != nil {
		rel = path
	}
	res.out("manifest -> %s", rel)
	return nil
}
