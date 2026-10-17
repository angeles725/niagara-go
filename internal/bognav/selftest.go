package bognav

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
)

// selftestXML is the synthetic bog of bognav/selftest.py: tree, a composite property
// (StatusNumeric.value child), a simple slot, module prefixes, cross-component links, a CROSSED
// tile (evap1 <- unit 3), an own-module relay into writables with and without a fallback, an auto
// mode slot and an Alerton PointCache with a handle-less value wrapper.
const selftestXML = `
<?xml version='1.0' encoding='UTF-8'?>
<bajaObjectGraph version='4.0'>
 <p n='Station' h='1' t='b:Station' m='b=baja CRP=ColdRoomPan DPCD=DashboardPan c=control'>
  <p n='Panel' h='10' t='DPCD:RoomPanel'>
   <p n='setpoint' t='b:StatusNumeric'>
    <p n="value" v="3.0"/>
   </p>
   <p n="differentialUp" v="1.5"/>
   <p n='Link' t='b:Link'>
    <p n="sourceOrd" v="h:30"/>
    <p n="sourceSlotName" v="valveOut"/>
    <p n="targetSlotName" v="evap1ValveState"/>
   </p>
  </p>
  <p n='Logic' h='20' t='CRP:ColdRoom'>
   <p n='setpoint' t='b:StatusNumeric'>
    <p n="value" v="3.0"/>
   </p>
   <p n="valveMode" v="auto"/>
   <p n='EvaporatorUnit_3' h='30' t='CRP:EvaporatorUnit'>
    <p n='valveOut' t='b:StatusBoolean'>
     <p n="value" v="false"/>
    </p>
   </p>
   <p n='Link' t='b:Link'>
    <p n="sourceOrd" v="h:10"/>
    <p n="sourceSlotName" v="setpoint"/>
    <p n="targetSlotName" v="setpoint"/>
   </p>
   <p n='Link1' t='b:Link'>
    <p n="sourceOrd" v="h:10"/>
    <p n="sourceSlotName" v="evap1FanMode"/>
    <p n="targetSlotName" v="fanMode"/>
   </p>
  </p>
  <p n='Drivers' h='40' t='c:BooleanWritable'>
   <p n='ro1' h='41' t='c:BooleanWritable'>
    <p n='fallback' t='b:StatusBoolean'>
     <p n="value" v="false"/>
    </p>
    <p n='Link' t='b:Link'>
     <p n="sourceOrd" v="h:30"/>
     <p n="sourceSlotName" v="valveOut"/>
     <p n="targetSlotName" v="in10"/>
    </p>
   </p>
   <p n='ro2' h='42' t='c:BooleanWritable'>
    <p n='Link' t='b:Link'>
     <p n="sourceOrd" v="h:30"/>
     <p n="sourceSlotName" v="valveOut"/>
     <p n="targetSlotName" v="in10"/>
    </p>
   </p>
  </p>
  <p n='DisplayService' h='50' t='disp:BtDisplayService' m='disp=btsDisp bac=bacnet'>
   <p n='pointCache' h='51' t='disp:PointCache'>
    <p n='bts$3abacref$3adopr$3d5_100_85_8_101' h='52' t='disp:PointCacheObj'>
     <p n="value" m="bac=bacnet" t="bac:BacnetBinaryPv" v="inactive"/>
     <p n="polledTime" t="b:AbsTime" v="2022-04-18T12:12:54.046-05:00"/>
    </p>
    <p n='bts$3abacref$3adopr$3d2_50_85_8_101' h='53' t='disp:PointCacheObj'>
     <p n="value" t="b:Float" v="72.5"/>
     <p n="polledTime" t="b:AbsTime" v="2026-09-03T00:00:00.000-05:00"/>
    </p>
   </p>
  </p>
 </p>
</bajaObjectGraph>`

type ns = map[string]any

// run executes one command on b with the selftest namespace (no _scope) and returns its stdout.
func run(b *bog.Bog, fn command, kw ns) string {
	n := argparse.NewNamespace()
	for k, v := range (ns{"json": false, "csv": false, "module": nil, "src": nil, "all": false}) {
		n.Set(k, v)
	}
	for k, v := range kw {
		n.Set(k, v)
	}
	var out bytes.Buffer
	fn(&env{ns: n, out: &out, err: io.Discard, bog: b})
	return out.String()
}

func link(b *bog.Bog, pred func(*bog.Link) bool) []*bog.Link {
	var out []*bog.Link
	for _, lk := range b.Links {
		if pred(lk) {
			out = append(out, lk)
		}
	}
	return out
}

func eq(p *string, s string) bool { return p != nil && *p == s }

// selftest is cmd_selftest: in-memory parser and command assertions, printed one per line;
// exit 1 when any fails.
func selftest(w io.Writer) int {
	b := bog.Parse("<selftest>", selftestXML[1:])
	var fails []string
	check := func(ok bool, label string) {
		if ok {
			fmt.Fprintf(w, "  ok   %s\n", label)
			return
		}
		fails = append(fails, label)
		fmt.Fprintf(w, "  FAIL %s\n", label)
	}
	c := func(h string) *bog.Comp {
		x, _ := b.Comps.Get(h)
		if x == nil {
			return &bog.Comp{}
		}
		return x
	}
	slot := func(h, name string) *bog.Slot {
		s, _ := c(h).Slots.Get(name)
		if s == nil {
			return &bog.Slot{}
		}
		return s
	}
	keys := append([]string(nil), b.Comps.Keys()...)
	check(strings.Join(sortedCopy(keys), ",") == "1,10,20,30,40,41,42,50,51,52,53", "all handled components parsed")
	check(c("10").Path == "Station/Panel", "path built from tree (Station/Panel)")
	check(eq(c("20").ParentH, "1"), "parent handle tracked")
	check(eq(c("30").ParentH, "20"), "nested EvaporatorUnit parent handle tracked")
	_, hasSetpoint := c("10").Slots.Get("setpoint")
	check(hasSetpoint, "composite property recorded as slot")
	check(slot("10", "setpoint").Type == "b:StatusNumeric", "composite slot type")
	check(eq(slot("10", "setpoint").Value, "3.0"), "nested StatusNumeric.value child surfaced on the slot (not None)")
	cv, _ := slot("10", "setpoint").Child.Get("value")
	check(cv == "3.0", "nested child dict captured (the child-ORD write target)")
	check(typeDisplay("", "differentialUp", map[string]string{"differentialUp": "double"}) == "double (from source)",
		"--src fills a bog-absent type from source")
	check(typeDisplay("", "x", map[string]string{}) == "frozen simple (type in source)",
		"bog-absent type without --src labelled frozen simple (not untyped)")
	check(eq(slot("10", "differentialUp").Value, "1.5"), "simple slot value")
	crp, _ := b.Prefixes.Get("CRP")
	check(crp == "ColdRoomPan", "module prefix decl parsed")
	logic := link(b, func(lk *bog.Link) bool { return eq(lk.ContainerH, "20") && eq(lk.TgtSlot, "setpoint") })
	ok := len(logic) == 1
	if ok {
		r := b.LinkRow(logic[0])
		ok = r.Source == "Station/Panel.setpoint" && r.Target == "Station/Logic.setpoint" && r.SrcResolved
	}
	check(ok, "cross-component link sourceOrd h:10 resolved to a path")
	sp := "setpoint"
	h20, h10 := "20", "10"
	check(matchRef(&h20, "Station/Logic", &sp, "h:20") && !matchRef(&h20, "Station/Logic", &sp, "h:99") &&
		matchRef(&h10, "Station/Panel", &sp, "h:10"), "--to/--from match an endpoint by h:handle exactly")
	check(matchRef(&h20, "Station/Logic", &sp, "Logic") && matchRef(&h10, "Station/Panel", &sp, "Panel"),
		"--to/--from still match by path when the needle is not a handle")
	fan := link(b, func(lk *bog.Link) bool { return eq(lk.TgtSlot, "fanMode") })
	check(len(fan) == 1 && !slotFilter(fan[0], "fanMode", "Panel", "", false),
		"--from X --slot S filters the SOURCE slot (target named S is not a hit)")
	check(len(fan) == 1 && slotFilter(fan[0], "evap1FanMode", "Panel", "", false) && slotFilter(fan[0], "fanMode", "", "Logic", false) &&
		slotFilter(fan[0], "fanMode", "", "", false),
		"--slot matches the source end with --from, the target end with --to, either end alone")
	check(len(fan) == 1 && slotFilter(fan[0], "fanMode", "Panel", "", true), "--slot-any restores either-end matching")
	k, _ := writability("b:StatusNumeric")
	check(k == "complex", "StatusNumeric classified as complex (child-leaf write)")
	k, _ = writability("b:Double")
	check(k == "simple", "plain b:Double classified simple")
	k, _ = writability("double")
	check(k == "simple", "lowercase Java primitive double classified simple")
	k, _ = writability("NumericPoint")
	check(k == "other", "NumericPoint not false-matched as simple")
	k, _ = writability("")
	check(k == "bare", "untyped value slot classified bare")

	check(c("41").HasFallback && !c("42").HasFallback, "relay data: ro1 has a fallback, ro2 has none (CHECK11 view)")
	check(strings.Contains(run(b, cmdRelays, nil), "2 own-module->writable relay targets; 1 WITHOUT"), "relays: 2 targets, 1 without fallback")
	tiles := run(b, cmdTiles, nil)
	check(strings.Contains(tiles, "CROSSED") && strings.Contains(tiles, "evap1"), "tiles: evap1 tile flagged CROSSED (fed by EvaporatorUnit_3)")
	check(strings.Contains(run(b, cmdHoa, nil), "0 active priority-array override"), "hoa: 0 active overrides, mode slot auto (CHECK8)")
	check(strings.TrimSpace(run(b, cmdPath, ns{"handle": "h:30"})) == "Station/Logic/EvaporatorUnit_3", "path: handle -> path reverse lookup")
	check(strings.Contains(run(b, cmdFind, ns{"type": "DPCD:RoomPanel"}), "Station/Panel"), "find: flat list by type")

	check(c("52").ValueType == "bac:BacnetBinaryPv", "handle-less `value` wrapper type captured on the PointCacheObj (the bug fix)")
	noSlots := run(b, cmdFind, ns{"type": "bac:BacnetBinaryPv", "slots": false})
	check(strings.Contains(noSlots, "handle-less slot") && strings.Contains(noSlots, "rerun with --slots"),
		"find: bac:BacnetBinaryPv with no --slots warns instead of a silent zero")
	check(strings.Contains(run(b, cmdFind, ns{"type": "bac:BacnetBinaryPv", "slots": true}), "Station/DisplayService/pointCache"),
		"find --slots: the handle-less bac:BacnetBinaryPv value wrapper is now listed")
	types := run(b, cmdTypes, ns{"slots": true, "type_filter": nil})
	check(strings.Contains(types, "disp:PointCacheObj") && strings.Contains(types, "bac:BacnetBinaryPv"),
		"types --slots: component types AND handle-less slot types counted")
	pcKW := ns{"device": nil, "stale": false, "live": false, "stale_before": nil, "summary": false}
	pc := run(b, cmdPointcache, pcKW)
	check(strings.Contains(pc, "binary-value:100@dev101") && strings.Contains(pc, "present-value"),
		"pointcache: bacref dopr ORD decoded to obj-type:inst@dev + property name")
	check(strings.Contains(pc, "STALE") && strings.Contains(pc, "live"), "pointcache: freshness classified (2022 STALE vs 2026 live)")
	pcKW["device"], pcKW["stale"] = "101", true
	stale := run(b, cmdPointcache, pcKW)
	check(strings.Contains(stale, "2022-04-18") && !strings.Contains(stale, "2026-09-03"),
		"pointcache --stale --device: filters to the dead binding only")
	if len(fails) > 0 {
		fmt.Fprintf(w, "\nSELFTEST FAILED: %d check(s)\n", len(fails))
		return cli.ExitNoMatch
	}
	fmt.Fprintln(w, "\nSELFTEST OK")
	return cli.ExitFound
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
