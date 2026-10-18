package navbuild

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The module inventory (build_module_inventory.py): one entry per submodule of
// the corpus, indexes/module-inventory.json.

// InventoryFile is the index the inventory builder writes.
const InventoryFile = "module-inventory.json"

// niagaraPrefixes are the Niagara/Honeywell package prefixes: any other
// package of two or more segments is third-party.
var niagaraPrefixes = []string{"javax.baja", "com.tridium", "com.honeywell", "com.vykon", "com.niagara"}

// Submodule is one inventory entry. ClassCount and Bytecode are the values of
// pipeline/fase1-recon.json as decoded (0 without them), so any JSON number
// round-trips as Python writes it back.
type Submodule struct {
	Name, Module, Type, Jar    string
	ClassCount, Bytecode       pyjson.Value
	JavaFiles                  int
	ZKM, HasVineflower, HasCFR bool
	HasCode, Recon             bool
	Packages, ThirdParty       []string
}

// Inventory is what build_inventory returns, plus the print-side facts.
type Inventory struct {
	// Submodules in dict order: corpus order, a repeated submodule name keeping
	// the position of its first occurrence and the value of its last.
	Submodules []Submodule
	// NoRecon names the submodules without a readable fase1-recon.json.
	NoRecon []string
	// UnreadableRecon lists the fase1-recon.json files that exist but could not be
	// read or decoded (the --verbose WARN lines of Python).
	UnreadableRecon []string
	// Skipped lists organized/<module>/<dir> entries that are not submodules.
	Skipped []string
	// Elapsed is the scan time, the source of build_time_sec.
	Elapsed time.Duration
}

// ScanInventory is build_inventory(organized): every submodule under
// organized, sorted. It fails only on a fase1-recon.json that decodes to
// something other than an object, where Python dies with AttributeError.
func ScanInventory(organized string) (*Inventory, error) {
	t0 := time.Now()
	inv := &Inventory{}
	pos := map[string]int{}
	for _, s := range core.IterSubmodules(organized, &inv.Skipped) {
		reconPath := filepath.Join(s.Path, "pipeline", "fase1-recon.json")
		recon, err := readRecon(reconPath)
		var notObject *reconNotObject
		switch {
		case errors.As(err, &notObject):
			return nil, err
		case err != nil:
			if pycompat.IsFile(reconPath) {
				inv.UnreadableRecon = append(inv.UnreadableRecon, reconPath)
			}
			inv.NoRecon = append(inv.NoRecon, s.Name)
			recon = pyjson.NewObject()
		}
		vineflower := filepath.Join(s.Path, "vineflower")
		count, packages, thirdParty := scanPackages(vineflower)
		get := func(key string, def pyjson.Value) pyjson.Value {
			if v, ok := recon.Get(key); ok {
				return v
			}
			return def
		}
		entry := Submodule{
			Name:          s.Name,
			Module:        s.Module,
			Type:          core.SubmoduleType(s.Module, s.Name),
			Jar:           s.Name + ".jar",
			ClassCount:    get("class_count", integer(0)),
			JavaFiles:     count,
			ZKM:           get("ofuscador_detectado", boolean(false)).Truthy(),
			Bytecode:      get("bytecode_major_version", integer(0)),
			HasVineflower: pycompat.IsDir(vineflower),
			HasCFR:        pycompat.IsDir(filepath.Join(s.Path, "decompiled")),
			HasCode:       get("tiene_codigo_java", boolean(count > 0)).Truthy(),
			Recon:         recon.Truthy(),
			Packages:      packages,
			ThirdParty:    thirdParty,
		}
		if i, ok := pos[s.Name]; ok {
			inv.Submodules[i] = entry
			continue
		}
		pos[s.Name] = len(inv.Submodules)
		inv.Submodules = append(inv.Submodules, entry)
	}
	inv.Elapsed = time.Since(t0)
	return inv, nil
}

// reconNotObject is a fase1-recon.json that is valid JSON but not an object.
type reconNotObject struct{ path string }

func (e *reconNotObject) Error() string {
	return e.path + ": fase1-recon.json is not a JSON object"
}

// readRecon is json.load(open(path, encoding="utf-8-sig")): any OSError or
// ValueError (undecodable UTF-8, invalid JSON) is an error.
func readRecon(path string) (pyjson.Value, error) {
	text, err := pycompat.ReadText(path)
	if err != nil {
		return pyjson.Value{}, err
	}
	v, err := pyjson.Loads([]rune(strings.TrimPrefix(text, "\ufeff")))
	if err != nil {
		return pyjson.Value{}, err
	}
	if v.Kind != pyjson.Object {
		return pyjson.Value{}, &reconNotObject{path}
	}
	return v, nil
}

// scanPackages is scan_packages(vineflower_dir): the .java file count, the
// sorted top-level packages (first two segments) of the directories holding
// .java files, and those of them that are third-party.
func scanPackages(vineflower string) (count int, packages, thirdParty []string) {
	pkgs, third := map[string]bool{}, map[string]bool{}
	var walk func(dir, rel string)
	walk = func(dir, rel string) {
		dirs, files := listDir(dir)
		java := 0
		for _, f := range files {
			if strings.HasSuffix(f, ".java") {
				java++
			}
		}
		count += java
		if java > 0 && rel != "" {
			pkg := strings.ReplaceAll(rel, "/", ".")
			parts := strings.Split(pkg, ".")
			top := strings.Join(parts[:min(2, len(parts))], ".")
			pkgs[top] = true
			if !hasAnyPrefix(pkg, niagaraPrefixes) && len(parts) >= 2 {
				third[top] = true
			}
		}
		for _, d := range dirs {
			sub := d
			if rel != "" {
				sub = rel + "/" + d
			}
			walk(filepath.Join(dir, d), sub)
		}
	}
	walk(vineflower, "")
	return count, slices.Sorted(maps.Keys(pkgs)), slices.Sorted(maps.Keys(third))
}

// listDir splits dir's entries as os.walk does: directories (or links to
// one) and everything else; a symlinked directory is listed but, with
// followlinks=False, not entered, so it is left out of dirs. An unreadable
// dir has no entries (os.walk ignores the error).
func listDir(dir string) (dirs, files []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir():
			dirs = append(dirs, name)
		case e.Type()&os.ModeSymlink != 0 && pycompat.IsDir(filepath.Join(dir, name)):
			// a link to a directory: a dir name, never walked into
		default:
			files = append(files, name)
		}
	}
	return dirs, files
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// Document is the module-inventory.json text for this inventory: _meta
// (source = organized, the volatile members from Elapsed and at) and modules.
func (inv *Inventory) Document(organized string, at time.Time) string {
	meta := stamped([]Field{
		{"description", str("Module inventory for Niagara N4 decompiled modules")},
		{"source", str(organized)},
		{"total_submodules", integer(len(inv.Submodules))},
	}, inv.Elapsed, at)
	w := &jsonWriter{o: Indented}
	w.object(0, 2, func(i int) string { return []string{"_meta", "modules"}[i] }, func(i, level int) {
		if i == 0 {
			w.fields(level, meta)
			return
		}
		w.object(level, len(inv.Submodules), func(j int) string { return inv.Submodules[j].Name },
			func(j, l int) { w.value(inv.Submodules[j].value(), l) })
	})
	return w.b.String()
}

func (s Submodule) value() pyjson.Value {
	return object(
		Field{"module", str(s.Module)},
		Field{"type", str(s.Type)},
		Field{"jar", str(s.Jar)},
		Field{"class_count", s.ClassCount},
		Field{"java_files", integer(s.JavaFiles)},
		Field{"zkm", boolean(s.ZKM)},
		Field{"bytecode", s.Bytecode},
		Field{"has_vineflower", boolean(s.HasVineflower)},
		Field{"has_cfr", boolean(s.HasCFR)},
		Field{"has_code", boolean(s.HasCode)},
		Field{"recon", boolean(s.Recon)},
		Field{"packages", strList(s.Packages)},
		Field{"third_party", strList(s.ThirdParty)},
	)
}

// BuildInventory scans organized and writes <baseDir>/indexes/module-inventory.json
// (build_module_inventory.py main without the printed summary).
func BuildInventory(baseDir, organized string) (*Inventory, error) {
	inv, err := ScanInventory(organized)
	if err != nil {
		return nil, err
	}
	if err := writeIndex(baseDir, InventoryFile, inv.Document(organized, now())); err != nil {
		return nil, fmt.Errorf("writing %s: %w", InventoryFile, err)
	}
	return inv, nil
}
