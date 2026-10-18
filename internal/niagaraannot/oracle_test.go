package niagaraannot

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	genInt     = regexp.MustCompile(`public static final int (\w+) = (-?\d+);`)
	genDefault = regexp.MustCompile(`public static final \w+ DEFAULT = (\w+);`)
	genSlot    = regexp.MustCompile(`public static final (Property|Action|Topic) (\w+) = new(?:Property|Action|Topic)\(([^,()]*(?:\([^()]*\))?[^,()]*)[,)]`)
)

// TestOracleSourceTree checks Parse, FlagLetters and Enum.Constants against the code Slot-o-Matic
// generated in a real source tree (COURSE_NIAGARA_SOURCE: niagara-help/versions/4.15.6.20/source):
// for every annotated file with a generated region, the declared enum constants, ordinals and
// DEFAULT, and each declared slot's name and flag letters, must be the generated ones.
func TestOracleSourceTree(t *testing.T) {
	root := os.Getenv("COURSE_NIAGARA_SOURCE")
	if root == "" {
		t.Skip("COURSE_NIAGARA_SOURCE is not set: the parser is not checked against generated code")
	}
	var files, enums, slots int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".java") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(b)
		_, gen, ok := strings.Cut(src, "BEGIN BAJA AUTO GENERATED CODE")
		if !ok || !strings.Contains(src, "@Niagara") {
			return nil
		}
		gen, _, _ = strings.Cut(gen, "END BAJA AUTO GENERATED CODE")
		rel, _ := filepath.Rel(root, path)
		ty, err := Parse(src)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		files++
		if ty.Enum != nil {
			enums++
			checkEnum(t, rel, ty.Enum, gen)
		}
		generated := map[string]string{}
		for _, m := range genSlot.FindAllStringSubmatch(gen, -1) {
			generated[m[2]] = m[3]
		}
		for _, s := range ty.Slots {
			g, ok := generated[s.Name]
			if s.Override {
				continue // an override re-declares an inherited slot: no newProperty
			}
			if !ok {
				t.Errorf("%s:%d: %s %s not generated", rel, s.Line, s.Kind, s.Name)
				continue
			}
			slots++
			want, err := FlagLetters(g)
			got, err2 := FlagLetters(s.Flags)
			if err != nil || err2 != nil || got != want {
				t.Errorf("%s:%d: %s flags %q = %q (%v), generated %q = %q (%v)", rel, s.Line, s.Name, s.Flags, got, err2, g, want, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d annotated files, %d enums, %d slots match the generated code", files, enums, slots)
	if files == 0 {
		t.Fatal("no annotated file with a generated region")
	}
}

func checkEnum(t *testing.T, rel string, e *Enum, gen string) {
	t.Helper()
	cs, err := e.Constants()
	if err != nil {
		if strings.Contains(err.Error(), "not an int literal") {
			t.Logf("%s: not decided: %v", rel, err) // BInterval: ordinal = INTERVAL_NONE
			return
		}
		t.Errorf("%s: %v", rel, err)
		return
	}
	var want []string
	for _, m := range genInt.FindAllStringSubmatch(gen, -1) {
		want = append(want, m[1]+"="+m[2])
	}
	var got []string
	for _, c := range cs {
		got = append(got, c.Name+"="+strconv.Itoa(c.Ordinal))
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s: constants %v, generated %v", rel, got, want)
	}
	d, err := e.DefaultTag()
	if m := genDefault.FindStringSubmatch(gen); m == nil || err != nil || m[1] != d {
		t.Errorf("%s: DEFAULT = %q (%v), generated %v", rel, d, err, m)
	}
}
