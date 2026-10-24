package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runTag runs `tag` on a root holding the tag-index.json fixture.
func runTag(t *testing.T, p TagParams) (string, int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "tag-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "indexes", "tag-index.json"), string(data))
	var out bytes.Buffer
	e := &Env{BaseDir: root, Out: &out}
	if err := e.Tag(p); err != nil {
		t.Fatal(err)
	}
	return out.String(), e.misses
}

const tagUnreadable = "\nUNREADABLE (1): haystack-wb.jar: zip: not a valid zip file\n"

// TestTagShowsEveryDefinition: a tag prints each file's definition, then
// the N4 rules applying it, the tag groups and protos holding it and the
// Haystack 3 names migrating to it.
func TestTagShowsEveryDefinition(t *testing.T) {
	got, misses := runTag(t, TagParams{Query: "ahu"})
	want := `TAGS with 'ahu' (1):

TAG hs:ahu  (tag; 2 definitions)
  haystack-rt.jar!data/defs.json
    kind: Marker
    doc: Air Handling Unit:
         an enclosure with a fan.
    is: airHandlingEquip
    supertypes: airHandlingEquip, equip, marker
    lib: lib:phIoT
  haystack-rt.jar!data/tags.csv
    kind: Marker
    validity: hasTags=equip rooftop
  N4 rules (1):
    ahuRule  isType kitControl:Ahu  (haystack-rt.jar!data/HS4toN4.json)
  tag groups (1): ahuEquip
  protos (1):
    ahu equip
  Haystack 3 migrations (1):
    ahuRef (relation) -> ahu (tag)
` + tagUnreadable
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestTagMatchesLikeType: dict:name or name, exactly, then in any letter
// case, then as a glob; a tag group and a relation are names too.
func TestTagMatchesLikeType(t *testing.T) {
	for _, tt := range []struct{ query, want string }{
		{"n:point", "TAG n:point  (tag; 1 definitions)\n"},
		{"AHUEQUIP", "TAG hs:ahuEquip  (tag group; 1 definitions)\n"},
		{"chi*", "TAG n:child  (relation; 1 definitions)\n"},
	} {
		got, misses := runTag(t, TagParams{Query: tt.query})
		if misses != 0 || !strings.Contains(got, tt.want) {
			t.Errorf("%s: misses %d, got:\n%s", tt.query, misses, got)
		}
	}
	got, _ := runTag(t, TagParams{Query: "AHUEQUIP"})
	if !strings.Contains(got, "  N4 rules (1):\n    ahuRule  isType kitControl:Ahu") {
		t.Errorf("a rule applying a group is not named:\n%s", got)
	}
}

// TestTagList: --list names every tag, tag group and relation once, with
// its kind and the files defining it.
func TestTagList(t *testing.T) {
	got, misses := runTag(t, TagParams{List: true})
	want := `TAGS in tag-index.json (5: 3 tags, 1 tag groups, 1 relations; from 7 files):

  hs:ahu        tag        Marker  defs.json, tags.csv
  hs:discharge  tag        Marker  defs.json
  n:point       tag        Marker  Niagara.bog
  hs:ahuEquip   tag group  equip.csv
  n:child       relation   tagdictionary:ComponentChildRelation  Niagara.bog
` + tagUnreadable
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

func TestTagJSON(t *testing.T) {
	got, _ := runTag(t, TagParams{Query: "discharge", JSON: true})
	for _, want := range []string{"{\n  \"query\": \"discharge\",\n  \"hits\": [\n    {\n      \"dict\": \"hs\",\n      \"name\": \"discharge\",\n      \"kind\": \"tag\",",
		`"protos": [
        "discharge air temp sensor point"
      ]`, `"unreadable": [
    {
      "jar": "haystack-wb.jar",`} {
		if !strings.Contains(got, want) {
			t.Errorf("json lacks %s:\n%s", want, got)
		}
	}
}

// TestTagMiss: an unknown name is a miss naming tag-index.json, its names and
// its unreadable files.
func TestTagMiss(t *testing.T) {
	got, misses := runTag(t, TagParams{Query: "nope"})
	if misses != 1 || !strings.Contains(got, "tag-index.json: 5 names from 7 files (1 unreadable: haystack-wb.jar)") {
		t.Errorf("misses %d, got:\n%s", misses, got)
	}
}

// TestTagCLI: tag takes a name, --list and --json; a miss exits 1 and no
// name without --list is a usage error (exit 2).
func TestTagCLI(t *testing.T) {
	dir := repo(t, `{"default": "1.0.0.1", "versions": {"1.0.0.1": {"install_dir": ".", "data_root": "."}}}`, ".")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "tag-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "indexes", "tag-index.json"), string(data))
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"tag", "ahu"}, 0, "TAGS with 'ahu' (1):\n"},
		{[]string{"tag", "--list"}, 0, "TAGS in tag-index.json (5: "},
		{[]string{"tag", "--list", "--json"}, 0, "{\n  \"tags\": ["},
		{[]string{"tag", "ahu", "--json"}, 0, "{\n  \"query\": \"ahu\","},
		{[]string{"tag", "nope"}, 1, ""},
		{[]string{"tag"}, 2, ""},
	} {
		out, errs, code := run(t, dir, tt.args...)
		if code != tt.code || !strings.HasPrefix(out, tt.out) {
			t.Errorf("%v: code %d stdout %q stderr %q", tt.args, code, out, errs)
		}
	}
}
