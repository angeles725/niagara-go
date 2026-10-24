package tagdict

import "github.com/angeles725/niagara-go/internal/pyjson"

// Append passes each record of f to add as its tag-index.json row, with the
// section it belongs in ("tags", "protos", "groups", "relations", "rules",
// "migrations"). Every row names file ("<jar>!<path>"); a field left empty
// is not written.
func (f *File) Append(file string, add func(section string, v pyjson.Value)) {
	for _, t := range f.Tags {
		o := record(t.Dict, t.Name, file)
		for _, kv := range [][2]string{{"kind", t.Kind}, {"doc", t.Doc}} {
			setStr(&o, kv[0], kv[1])
		}
		for _, kv := range []struct {
			k string
			v []string
		}{{"is", t.Is}, {"supertypes", t.Supertypes}, {"tagOn", t.TagOn}} {
			setList(&o, kv.k, kv.v)
		}
		for _, kv := range [][2]string{{"of", t.Of}, {"lib", t.Lib}, {"smartType", t.SmartType}, {"units", t.Units}} {
			setStr(&o, kv[0], kv[1])
		}
		setFields(&o, "implied", t.Implied)
		setFields(&o, "validity", t.Validity)
		add("tags", o)
	}
	for _, p := range f.Protos {
		add("protos", pyjson.ObjectOf(field("file", file), field("proto", p)))
	}
	for _, g := range f.Groups {
		o := record(g.Dict, g.Name, file)
		setList(&o, "tags", g.Tags)
		setFields(&o, "validity", g.Validity)
		add("groups", o)
	}
	for _, r := range f.Relations {
		o := record(r.Dict, r.Name, file)
		setStr(&o, "type", r.Type)
		add("relations", o)
	}
	for _, r := range f.Rules {
		o := record(r.Dict, r.Name, file)
		setStr(&o, "condition", r.Condition)
		setRuleTags(&o, "tags", r.Tags)
		setList(&o, "groups", r.Groups)
		setRuleTags(&o, "relations", r.Relations)
		add("rules", o)
	}
	for _, m := range f.Migrations {
		o := pyjson.ObjectOf(field("file", file), field("from", m.From))
		setStr(&o, "fromType", m.FromType)
		setStr(&o, "query", m.Query)
		setList(&o, "to", m.To)
		setStr(&o, "toType", m.ToType)
		add("migrations", o)
	}
}

func field(k, v string) pyjson.Field { return pyjson.Field{Key: k, Value: pyjson.String(v)} }

func record(dict, name, file string) pyjson.Value {
	return pyjson.ObjectOf(field("dict", dict), field("name", name), field("file", file))
}

func setStr(o *pyjson.Value, k, v string) {
	if v != "" {
		o.Set(k, pyjson.String(v))
	}
}

func setList(o *pyjson.Value, k string, v []string) {
	if len(v) > 0 {
		o.Set(k, pyjson.StringList(v))
	}
}

// setFields writes rule cells as an object of name: value.
func setFields(o *pyjson.Value, k string, fs []Field) {
	if len(fs) == 0 {
		return
	}
	v := pyjson.NewObject()
	for _, f := range fs {
		v.Set(f.Name, pyjson.String(f.Value))
	}
	o.Set(k, v)
}

// setRuleTags writes a rule's tags or relations as [{"name", "type"?, "validity"?}].
func setRuleTags(o *pyjson.Value, k string, ts []RuleTag) {
	if len(ts) == 0 {
		return
	}
	a := make([]pyjson.Value, len(ts))
	for i, t := range ts {
		a[i] = pyjson.ObjectOf(field("name", t.Name))
		setStr(&a[i], "type", t.Type)
		setStr(&a[i], "validity", t.Validity)
	}
	o.Set(k, pyjson.Value{Kind: pyjson.Array, A: a})
}
