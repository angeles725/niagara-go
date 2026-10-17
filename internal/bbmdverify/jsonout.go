package bbmdverify

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// A minimal ordered value tree written like json.dumps(obj, indent=2)
// (ensure_ascii, Python separators).

type member struct {
	key string
	val any
}

type object []member

// jsonStr is json.dumps(s) with ensure_ascii, i.e. py_encode_basestring_ascii.
func jsonStr(s string) string { return pyjson.EncodeString(s, true) }

func writeJSON(b *strings.Builder, v any, level int) {
	pad := func(l int) string { return "\n" + strings.Repeat("  ", l) }
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		fmt.Fprint(b, x)
	case int:
		fmt.Fprint(b, x)
	case string:
		b.WriteString(jsonStr(x))
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[")
		for i, e := range x {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(pad(level + 1))
			writeJSON(b, e, level+1)
		}
		b.WriteString(pad(level) + "]")
	case object:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{")
		for i, m := range x {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(pad(level+1) + jsonStr(m.key) + ": ")
			writeJSON(b, m.val, level+1)
		}
		b.WriteString(pad(level) + "}")
	}
}

func ints(xs []int) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func (r *report) jsonReport(verdictText string) string {
	o := r.o
	var inst any
	if r.uniReachable {
		inst = r.uniInstance
	}
	responders, crossed, local := object{}, object{}, object{}
	for _, ip := range r.order {
		m := member{ip, ints(sortedUnique(r.responders[ip]))}
		responders = append(responders, m)
		if inSubnet(ip, o.DeviceIP, o.DevicePrefix) {
			crossed = append(crossed, m)
		} else {
			local = append(local, m)
		}
	}
	probe := object{}
	for _, e := range r.bbmd {
		var bdt, fdt any
		if e.bdt != nil {
			rows := []any{}
			for _, b := range e.bdt {
				rows = append(rows, []any{b.addr, b.mask})
			}
			bdt = rows
		}
		if e.fdt != nil {
			rows := []any{}
			for _, f := range e.fdt {
				rows = append(rows, []any{f.addr, f.ttl, f.rem})
			}
			fdt = rows
		}
		probe = append(probe, member{e.target, object{{"is_bbmd", e.isBBMD}, {"bdt", bdt}, {"fdt", fdt}}})
	}
	doc := object{
		{"device_ip", o.DeviceIP},
		{"device_subnet", fmt.Sprintf("%s/%d", o.DeviceIP, o.DevicePrefix)},
		{"local_ip", r.localIP},
		{"local_on_device_subnet", r.onDeviceSubnet},
		{"port", o.Port},
		{"tests", object{
			{"unicast_device", object{{"reachable", r.uniReachable}, {"instance", inst}}},
			{"broadcast", object{{"responders", responders}, {"crossed_from_device_subnet", crossed}, {"same_as_local", local}}},
			{"bbmd_probe", probe},
		}},
		{"verdict", verdictText},
	}
	var b strings.Builder
	writeJSON(&b, doc, 0)
	return b.String()
}
