package tagdict

import (
	"reflect"
	"testing"
)

// tags415 is a trimmed haystack-rt tags.csv as 4.13 to 4.15 ship it: a
// version row, a row naming the column groups, the header row, then tags.
const tags415 = `version,3.0.2 N.2,2019 Jul 25,,,,,,,,,
,,,impliedTagRules,,,,Validity Rules,,,,
name,kind,SmartType,hasTags,isType,hasAncestors,filter,hasTags,isType,hasAncestor,filter,units
absorption,Marker,,,,,,equip chiller,,,,
ahu,Marker,,equip,,,,,,,,
siteRef,Ref,haystack:SiteRelation,,,,,equip point,,,,
power,Number,,,,,,point,,,,kW
`

// tags43 is the 4.3 layout: no version row, other implied columns, and the
// units column named only by its group.
const tags43 = `,,,impliedTagRules,,,,Validity Rules,,,units
name,kind,SmartType,hasTags,hasAncestors,hasRelation,isType,hasTags,hasAncestor,isType,
ahu,Marker,,equip,,,,,,,
power,Number,,,,,,point,,,kW
`

func TestReadTagTables(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		n, ahu     int
	}{{"4.15", tags415, 4, 1}, {"4.3", tags43, 2, 0}} {
		f, err := Read("com/tridium/haystack/data/tags.csv", []byte(tc.data))
		if err != nil || f.Kind != "tags" || len(f.Tags) != tc.n {
			t.Fatalf("%s: %+v %v", tc.name, f, err)
		}
		ahu, power := f.Tags[tc.ahu], f.Tags[len(f.Tags)-1]
		want := Tag{Dict: "hs", Name: "ahu", Kind: "Marker", Implied: []Field{{"hasTags", "equip"}}}
		if !reflect.DeepEqual(ahu, want) {
			t.Errorf("%s: ahu = %+v", tc.name, ahu)
		}
		if power.Units != "kW" || !reflect.DeepEqual(power.Validity, []Field{{"hasTags", "point"}}) {
			t.Errorf("%s: power = %+v", tc.name, power)
		}
	}
	f, _ := Read("tags.csv", []byte(tags415))
	if f.Tags[2].SmartType != "haystack:SiteRelation" || f.Tags[2].Kind != "Ref" {
		t.Errorf("siteRef = %+v", f.Tags[2])
	}
}

func TestReadGroupAndMigrationTables(t *testing.T) {
	equip := `,,Validity Rules,,,,tagGroup
name,Remove,hasTags,isType,hasAncestor,filter,tags
activePowerSensor,,point,control:ControlPoint,elecPanel,,active power  sensor
`
	f, err := Read("equip.csv", []byte(equip))
	want := Group{Dict: "hs", Name: "activePowerSensor", Tags: []string{"active", "power", "sensor"},
		Validity: []Field{{"hasTags", "point"}, {"isType", "control:ControlPoint"}, {"hasAncestor", "elecPanel"}}}
	if err != nil || f.Kind != "groups" || len(f.Groups) != 1 || !reflect.DeepEqual(f.Groups[0], want) {
		t.Fatalf("%+v %v", f, err)
	}
	mig := `h3Name,h3Type,h3Query,h4Names,h4Type
apparent,tag,(hs:apparent and hs:temp),air feelsLike | apparent,tag
ahuRef,relation,,airRef,relation
`
	f, err = Read("haystack3MigrationConfig.csv", []byte(mig))
	wantM := Migration{From: "apparent", FromType: "tag", Query: "(hs:apparent and hs:temp)",
		To: []string{"air feelsLike", "apparent"}, ToType: "tag"}
	if err != nil || f.Kind != "migrations" || len(f.Migrations) != 2 || !reflect.DeepEqual(f.Migrations[0], wantM) {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestReadFailsClosedOnCSV(t *testing.T) {
	for name, data := range map[string]string{
		"no header":      "a,b\n1,2\n",
		"unquoted quote": "name,kind\n\"ahu,Marker\n",
		"nameless row":   "name,kind\n,Marker\n",
		"empty":          "",
	} {
		if f, err := Read("x.csv", []byte(data)); err == nil {
			t.Errorf("%s: read as %+v", name, f)
		}
	}
}
