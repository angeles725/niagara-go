package tagdict

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// kinds are the Haystack 4 kind defs (docHaystack::Kinds) and their names: a
// def's kind is the first of them among its supertypes.
var kinds = map[string]string{
	"marker": "Marker", "bool": "Bool", "na": "NA", "number": "Number", "remove": "Remove",
	"str": "Str", "uri": "Uri", "ref": "Ref", "symbol": "Symbol", "date": "Date", "time": "Time",
	"dateTime": "DateTime", "coord": "Coord", "xstr": "XStr", "list": "List", "dict": "Dict",
	"grid": "Grid", "bin": "Bin",
}

// grid is a Haystack JSON grid; other is the object of HS4toN4.json.
type grid struct {
	Kind string `json:"_kind"`
	Cols []struct {
		Name string `json:"name"`
	} `json:"cols"`
	Rows     []map[string]json.RawMessage `json:"rows"`
	TagRules []map[string]string          `json:"tagRules"`
}

// symbol is a Haystack JSON symbol value's name, "" when raw is not one.
func symbol(raw json.RawMessage) string {
	var s struct {
		Kind string `json:"_kind"`
		Val  string `json:"val"`
	}
	if json.Unmarshal(raw, &s) != nil || s.Kind != "symbol" {
		return ""
	}
	return s.Val
}

// symbols are the names of a list of symbols.
func symbols(raw json.RawMessage) []string {
	var list []json.RawMessage
	_ = json.Unmarshal(raw, &list)
	var out []string
	for _, v := range list {
		if s := symbol(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func str(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// readJSON reads defs.json, protos.json or HS4toN4.json, told apart by shape.
func readJSON(data []byte) (*File, error) {
	var g grid
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	switch {
	case g.TagRules != nil:
		return readRules(g.TagRules)
	case g.Kind == "grid" && len(g.Cols) > 0 && g.Cols[0].Name == "def":
		return readDefs(g.Rows)
	case g.Kind == "grid" && len(g.Cols) > 0 && g.Cols[0].Name == "proto":
		f := &File{Kind: "protos"}
		for i, r := range g.Rows {
			p := str(r["proto"])
			if p == "" {
				return nil, fmt.Errorf("protos row %d has no proto", i+1)
			}
			f.Protos = append(f.Protos, p)
		}
		return f, nil
	}
	return nil, errors.New("JSON is neither a Haystack defs or protos grid nor a tagRules list")
}

// readDefs reads the def rows of defs.json, resolving each def's kind and
// supertypes through the is lists of the whole file.
func readDefs(rows []map[string]json.RawMessage) (*File, error) {
	f := &File{Kind: "defs"}
	is := map[string][]string{}
	for i, r := range rows {
		name := symbol(r["def"])
		if name == "" {
			return nil, fmt.Errorf("defs row %d has no def symbol", i+1)
		}
		t := Tag{Dict: Haystack, Name: name, Doc: str(r["doc"]), Is: symbols(r["is"]), TagOn: symbols(r["tagOn"]),
			Of: symbol(r["of"]), Lib: symbol(r["lib"])}
		is[name] = t.Is
		f.Tags = append(f.Tags, t)
	}
	for i := range f.Tags {
		t := &f.Tags[i]
		t.Supertypes = supertypes(t.Name, is)
		for _, s := range t.Supertypes {
			if k, ok := kinds[s]; ok {
				t.Kind = k
				break
			}
		}
	}
	return f, nil
}

// supertypes are every def above name, breadth first, each once.
func supertypes(name string, is map[string][]string) []string {
	seen := map[string]bool{name: true}
	var out []string
	for queue := is[name]; len(queue) > 0; queue = queue[1:] {
		if s := queue[0]; !seen[s] {
			seen[s] = true
			out = append(out, s)
			queue = append(queue, is[s]...)
		}
	}
	return out
}

// readRules reads HS4toN4.json: the Niagara rules that apply Haystack tags,
// {"name", "condition" (IS_TYPE, ALWAYS, TAG_ON), "isType"?, "tagType"?}.
func readRules(rows []map[string]string) (*File, error) {
	f := &File{Kind: "rules"}
	for i, r := range rows {
		if r["name"] == "" || r["condition"] == "" {
			return nil, fmt.Errorf("tagRules row %d lacks its name or condition", i+1)
		}
		cond := camel(r["condition"])
		if r["isType"] != "" {
			cond += " " + r["isType"]
		}
		f.Rules = append(f.Rules, Rule{Dict: Haystack, Name: r["name"], Condition: cond,
			Tags: []RuleTag{{Name: r["name"], Type: r["tagType"]}}})
	}
	return f, nil
}

// camel is IS_TYPE as isType.
func camel(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
