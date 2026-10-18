package pyjson

// Defaults is json.dumps(obj, indent=indent) with every other keyword at its
// default: ensure_ascii=True, no sort_keys, and separators (", ", ": ") on one
// line or (",", ": ") when indented. indent < 0 is indent=None.
func Defaults(indent int) DumpOpts {
	if indent < 0 {
		return DumpOpts{EnsureASCII: true, Indent: -1, ItemSep: ", ", KeySep: ": "}
	}
	return DumpOpts{EnsureASCII: true, Indent: indent, ItemSep: ",", KeySep: ": "}
}

// Indent2 is json.dumps(obj, indent=2).
var Indent2 = Defaults(2)
