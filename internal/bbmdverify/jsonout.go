package bbmdverify

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The report is a pyjson.Obj tree printed like json.dumps(obj, indent=2).

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
	responders, crossed, local := pyjson.Obj{}, pyjson.Obj{}, pyjson.Obj{}
	for _, ip := range r.order {
		m := pyjson.KV{Key: ip, Val: ints(sortedUnique(r.responders[ip]))}
		responders = append(responders, m)
		if inSubnet(ip, o.DeviceIP, o.DevicePrefix) {
			crossed = append(crossed, m)
		} else {
			local = append(local, m)
		}
	}
	probe := pyjson.Obj{}
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
		probe = append(probe, pyjson.KV{Key: e.target, Val: pyjson.Obj{{"is_bbmd", e.isBBMD}, {"bdt", bdt}, {"fdt", fdt}}})
	}
	doc := pyjson.Obj{
		{"device_ip", o.DeviceIP},
		{"device_subnet", fmt.Sprintf("%s/%d", o.DeviceIP, o.DevicePrefix)},
		{"local_ip", r.localIP},
		{"local_on_device_subnet", r.onDeviceSubnet},
		{"port", o.Port},
		{"tests", pyjson.Obj{
			{"unicast_device", pyjson.Obj{{"reachable", r.uniReachable}, {"instance", inst}}},
			{"broadcast", pyjson.Obj{{"responders", responders}, {"crossed_from_device_subnet", crossed}, {"same_as_local", local}}},
			{"bbmd_probe", probe},
		}},
		{"verdict", verdictText},
	}
	return pyjson.DumpsAny(doc, pyjson.Indent2)
}
