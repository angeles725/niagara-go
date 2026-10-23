package tagdict

import (
	"reflect"
	"testing"
)

// niagaraBog is a trimmed tagdictionary-rt bog/Niagara.bog file.xml (4.15).
const niagaraBog = `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0" reversibleEncodingKeySource="none">
<p h="59" m="b=baja" t="b:UnrestrictedFolder">
 <!-- /Niagara -->
 <p n="Niagara" h="1" m="td=tagdictionary" t="td:NiagaraTagDictionary">
  <p n="version" t="b:String" v="1.5"/>
  <p n="tagDefinitions" h="2" t="td:TagInfoList">
   <p n="device" h="5" t="td:SimpleTagInfo">
    <p n="validity" h="6" t="td:IsTypeCondition">
     <p n="objectType" t="b:TypeSpec" v="driver:Device"/>
    </p>
   </p>
   <p n="geoCity" h="8d" t="td:SimpleTagInfo">
    <p n="validity" h="8e" t="td:And">
     <p n="a" f="r" h="8f" t="td:BooleanFilter">
      <p n="filter" v="hs:site"/>
     </p>
     <p n="a1" f="r" h="90" t="td:IsTypeCondition">
      <p n="objectType" t="b:TypeSpec" v="baja:Component"/>
     </p>
    </p>
    <p n="defValue" t="b:String" v=""/>
   </p>
  </p>
  <p n="tagGroupDefinitions" h="23" t="td:TagGroupInfoList">
  </p>
  <p n="relationDefinitions" h="24" t="td:RelationInfoList">
   <p n="parentDevice" h="79" t="td:ParentDeviceRelation">
   </p>
  </p>
  <p n="tagRules" h="27" t="td:TagRuleList">
   <p n="point$20tags" h="45" t="td:TagRule">
    <p n="condition" h="46" t="td:IsTypeCondition">
     <p n="objectType" t="b:TypeSpec" v="control:ControlPoint"/>
    </p>
    <p n="tagList" h="47" t="td:TagInfoList">
     <p n="point" h="48" t="td:SimpleTagInfo">
      <p n="validity" h="49" t="td:Always"/>
     </p>
     <p n="history" h="4a" t="td:HistoryIdTag">
     </p>
    </p>
    <p n="tagGroupList" h="43" t="td:TagGroupInfoList">
    </p>
    <p n="relationList" h="44" t="td:RelationInfoList">
     <p n="parentDevice" h="115" t="td:ParentDeviceRelation"/>
    </p>
   </p>
  </p>
 </p>
</p>
</bajaObjectGraph>
`

func TestReadNiagaraDictionary(t *testing.T) {
	for name, data := range map[string][]byte{
		"zipped":        zipped(t, niagaraBog),
		"plain":         []byte(niagaraBog),
		"comment-first": []byte("<!-- Copyright -->\n" + niagaraBog),
	} {
		f, err := Read("bog/Niagara.bog", data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		wantTags := []Tag{
			{Dict: "n", Name: "device", Kind: "Marker", Validity: []Field{{"condition", "isType driver:Device"}}},
			{Dict: "n", Name: "geoCity", Kind: "baja:String",
				Validity: []Field{{"condition", "(filter hs:site and isType baja:Component)"}}},
		}
		if f.Kind != "niagara" || !reflect.DeepEqual(f.Tags, wantTags) {
			t.Errorf("%s: kind %q tags %+v", name, f.Kind, f.Tags)
		}
		if !reflect.DeepEqual(f.Relations, []Relation{{"n", "parentDevice", "tagdictionary:ParentDeviceRelation"}}) {
			t.Errorf("%s: relations %+v", name, f.Relations)
		}
		wantRule := Rule{Dict: "n", Name: "point tags", Condition: "isType control:ControlPoint",
			Tags:      []RuleTag{{"point", "tagdictionary:SimpleTagInfo", "always"}, {"history", "tagdictionary:HistoryIdTag", ""}},
			Relations: []RuleTag{{"parentDevice", "tagdictionary:ParentDeviceRelation", ""}}}
		if len(f.Rules) != 1 || !reflect.DeepEqual(f.Rules[0], wantRule) {
			t.Errorf("%s: rules %+v", name, f.Rules)
		}
	}
}

// TestReadNiagaraWithoutOptionalLists: a dictionary holding only tag
// definitions reads, with no groups, relations or rules.
func TestReadNiagaraWithoutOptionalLists(t *testing.T) {
	data := `<bajaObjectGraph><p m="td=tagdictionary" t="td:NiagaraTagDictionary">
<p n="tagDefinitions" t="td:TagInfoList"><p n="point" t="td:SimpleTagInfo"/></p></p></bajaObjectGraph>`
	f, err := Read("bog/Niagara.bog", []byte(data))
	if err != nil || f.Records() != 1 || f.Tags[0].Name != "point" {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestReadNiagaraFailsClosed(t *testing.T) {
	for name, data := range map[string]string{
		"malformed":     `<bajaObjectGraph><p t="x`,
		"no dictionary": `<bajaObjectGraph><p m="b=baja" t="b:Folder"/></bajaObjectGraph>`,
		"no namespace":  `<bajaObjectGraph><p m="x=other" t="x:Dict"><p n="tagDefinitions" t="x:L"/></p></bajaObjectGraph>`,
	} {
		if f, err := Read("bog/Niagara.bog", []byte(data)); err == nil {
			t.Errorf("%s: read as %+v", name, f)
		}
	}
}
