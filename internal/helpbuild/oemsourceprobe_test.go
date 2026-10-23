package helpbuild

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
)

// TestOEMSourceDryProbe lists, read-only, what extraction would take from
// a real install's module jars as Java source: per jar the .java entries,
// the ones mapped to source/<jar stem>/, those left to docSource (same
// path) and those an exclude rule keeps out (third-party packages). It
// writes nothing. Set NH_OEM_SOURCE_PROBE to the install folder (the one
// holding modules/) to run it, with -v to see the table.
func TestOEMSourceDryProbe(t *testing.T) {
	install := os.Getenv("NH_OEM_SOURCE_PROBE")
	if install == "" {
		t.Skip("NH_OEM_SOURCE_PROBE is not set (an install folder to probe read-only)")
	}
	jars, err := listDocJars(install, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	rules := mustDefault(t)
	owners := newSourceOwners()
	type row struct{ java, taken, kept, excluded int }
	rows := map[string]*row{}
	get := func(name string) *row {
		if rows[name] == nil {
			rows[name] = &row{}
		}
		return rows[name]
	}
	listing := func(jar string) []string {
		infos, err := zipfile.ReadCentralDir(jar)
		if err != nil {
			t.Fatalf("%s: %v", jar, err)
		}
		names := make([]string, len(infos))
		for i, x := range infos {
			names[i] = x.Name
		}
		return names
	}
	// The extraction pass: the jars extract reads, in its order.
	for _, jar := range jars {
		name := pyBasename(jar)
		for _, m := range mappedEntries(name, listing(jar), rules) {
			if _, take := owners.take(name, m.Rel); !m.Collides && !take {
				get(name).kept++
			}
		}
	}
	// Every module jar, help jar or not: a jar whose .java are all
	// third-party is no help jar, so extract never opens it.
	modules, err := lexiconJars(install)
	if err != nil {
		t.Fatal(err)
	}
	for _, jar := range modules {
		name := pyBasename(jar)
		if !isModuleJar(name) {
			continue
		}
		for _, e := range listing(jar) {
			if !strings.HasSuffix(e, ".java") || strings.HasPrefix(e, "META-INF/") {
				continue
			}
			r := get(name)
			r.java++
			if rel, ok := destPath(name, e, rules); !ok {
				r.excluded++
			} else if strings.HasPrefix(rel, "source/") && owners.origin[strings.TrimPrefix(rel, "source/")] == name {
				r.taken++
			}
		}
	}
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return rows[names[i]].taken > rows[names[j]].taken ||
			(rows[names[i]].taken == rows[names[j]].taken && names[i] < names[j])
	})
	total := row{}
	t.Logf("%-40s %6s %6s %6s %8s", "jar", ".java", "taken", "kept", "excluded")
	for _, n := range names {
		r := rows[n]
		t.Logf("%-40s %6d %6d %6d %8d", n, r.java, r.taken, r.kept, r.excluded)
		total.java, total.taken, total.kept, total.excluded = total.java+r.java, total.taken+r.taken, total.kept+r.kept, total.excluded+r.excluded
		if r.taken+r.kept+r.excluded != r.java {
			t.Errorf("%s: %d .java != %d taken + %d kept + %d excluded", n, r.java, r.taken, r.kept, r.excluded)
		}
	}
	t.Logf("%-40s %6d %6d %6d %8d", "TOTAL", total.java, total.taken, total.kept, total.excluded)
	if len(owners.origin) != total.taken {
		t.Errorf("origins record %d files, %d taken", len(owners.origin), total.taken)
	}
}
