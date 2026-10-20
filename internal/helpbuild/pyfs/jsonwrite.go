package pyfs

import (
	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// JSON index writes for the builders (niagara_help_lib/atomic.write_json): the
// text pyjson.Dumps gives, written with atomicfile.WriteText (temp file +
// rename in the same directory, mode 0644).

// The json.dumps option sets the builders write with: DumpDefault is
// json.dumps with no keyword arguments, DumpCompact is
// json.dumps(separators=(",", ":")) (the jar-listing cache format),
// DumpIndent2 is json.dumps(indent=2) (ensure_ascii on, "," and ": ") and
// DumpIndent2Unicode is json.dumps(indent=2, ensure_ascii=False).
var (
	DumpDefault        = pyjson.DumpOpts{EnsureASCII: true, Indent: -1, ItemSep: ", ", KeySep: ": "}
	DumpCompact        = pyjson.DumpOpts{EnsureASCII: true, Indent: -1, ItemSep: ",", KeySep: ":"}
	DumpIndent2        = pyjson.DumpOpts{EnsureASCII: true, Indent: 2, ItemSep: ",", KeySep: ": "}
	DumpIndent2Unicode = pyjson.DumpOpts{Indent: 2, ItemSep: ",", KeySep: ": "}
)

// WriteJSON writes pyjson.Dumps(v, o) to path atomically (atomic.write_json).
func WriteJSON(path string, v pyjson.Value, o pyjson.DumpOpts) error {
	return atomicfile.WriteText(path, pyjson.Dumps(v, o))
}
