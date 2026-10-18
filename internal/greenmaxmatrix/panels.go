package greenmaxmatrix

import (
	"regexp"
	"strconv"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
)

// Component name matchers (Python `$` also matches before a final newline).
var (
	relayRE    = regexp.MustCompile(pyRe(`^Relay\[([0-9]+)\]\.BO$`))
	schedRE    = regexp.MustCompile(pyRe(`^(?:Schedule|R)([0-9]+)$`))               // Schedule{k} (GM01) or R{k} (GM02+)
	writableRE = regexp.MustCompile(pyRe(`^(?:SchdlGM0*[0-9]+\s+R|HorR)([0-9]+)$`)) // command writable
	labelRE    = regexp.MustCompile(pyRe(`^labelR([0-9]+)$`))
	tabRE      = regexp.MustCompile(pyRe(`^tabR([0-9]+)$`))
	gmFolderRE = regexp.MustCompile(pyRe(`^GM0*([0-9]+)ilum$`)) // supervisor folder / JACE device
)

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// supPanel is one GM<n>ilum NiagaraPointFolder of the supervisor.
type supPanel struct {
	relays, scheds map[int]bool
	writables      map[int]*bog.Comp
	labels, tabs   map[int]string
	schedLinked    map[int]bool
}

// jacePanel is one GM<n>ilum BacnetDevice of the JACE.
type jacePanel struct {
	bo       map[int]string // relay -> BACnet objectId
	feedOK   map[int]bool
	feedPrio map[int]string
	device   string
}

func newJace(device string) *jacePanel {
	return &jacePanel{bo: map[int]string{}, feedOK: map[int]bool{}, feedPrio: map[int]string{}, device: device}
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// strconstValue is a kitControl:StringConst's out value, html-unescaped and stripped.
func strconstValue(c *bog.Comp) string {
	slot, ok := c.Slots.Get("out")
	if !ok || slot == nil {
		return ""
	}
	v := slot.Value
	if v == nil {
		if cv, ok := slot.Child.Get("value"); ok {
			v = &cv
		}
	}
	if v == nil || *v == "" {
		return ""
	}
	return pycompat.Strip(pyhtml.Unescape(*v))
}

// supPanels maps each GM number to its supervisor folder content and confirmed schedule links.
func supPanels(b *bog.Bog) map[int]*supPanel {
	panels := map[int]*supPanel{}
	for _, c := range b.Comps.All() {
		g := gmFolderRE.FindStringSubmatch(c.Name)
		if g == nil || c.SimpleType() != "NiagaraPointFolder" {
			continue
		}
		gm := atoi(g[1])
		e := panels[gm]
		if e == nil {
			e = &supPanel{map[int]bool{}, map[int]bool{}, map[int]*bog.Comp{}, map[int]string{}, map[int]string{}, map[int]bool{}}
			panels[gm] = e
		}
		for _, ch := range c.Children {
			cc, ok := b.Comps.Get(ch)
			if !ok || cc == nil {
				continue
			}
			nm := cc.Name
			if m := relayRE.FindStringSubmatch(nm); m != nil {
				e.relays[atoi(m[1])] = true
			} else if m := writableRE.FindStringSubmatch(nm); m != nil { // before schedule (HorR vs R)
				e.writables[atoi(m[1])] = cc
			} else if m := schedRE.FindStringSubmatch(nm); m != nil {
				e.scheds[atoi(m[1])] = true
			} else if m := labelRE.FindStringSubmatch(nm); m != nil {
				e.labels[atoi(m[1])] = strconstValue(cc)
			} else if m := tabRE.FindStringSubmatch(nm); m != nil {
				e.tabs[atoi(m[1])] = strconstValue(cc)
			}
		}
	}
	// schedule.out -> writable.in10, both naming patterns
	type ref struct{ gm, k int }
	wh := map[string]ref{}
	for gm, e := range panels {
		for k, c := range e.writables {
			wh[c.Handle] = ref{gm, k}
		}
	}
	for _, lk := range b.Links {
		if str(lk.TgtSlot) != "in10" || lk.ContainerH == nil {
			continue
		}
		r, ok := wh[*lk.ContainerH]
		if !ok || lk.SrcH == nil {
			continue
		}
		if src, ok := b.Comps.Get(*lk.SrcH); ok && src != nil &&
			(schedRE.MatchString(src.Name) || contains(src.Name, "Schedule")) {
			panels[r.gm].schedLinked[r.k] = true
		}
	}
	return panels
}

// deviceGM climbs from c (itself first, at most 8 steps) to its owning GM<n>ilum BacnetDevice.
func deviceGM(b *bog.Bog, c *bog.Comp) (int, bool) {
	for seen := 0; c != nil && seen < 8; seen++ {
		if g := gmFolderRE.FindStringSubmatch(c.Name); g != nil && c.SimpleType() == "BacnetDevice" {
			return atoi(g[1]), true
		}
		if c.ParentH == nil {
			return 0, false
		}
		c, _ = b.Comps.Get(*c.ParentH)
	}
	return 0, false
}

var driveSlots = map[string]bool{"in10": true, "in16": true, "in9": true}

// jacePanels maps each GM number to its BACnet device, relay objectIds and driven relays.
func jacePanels(b *bog.Bog) map[int]*jacePanel {
	panels := map[int]*jacePanel{}
	for _, c := range b.Comps.All() {
		if g := gmFolderRE.FindStringSubmatch(c.Name); g != nil && c.SimpleType() == "BacnetDevice" {
			if gm := atoi(g[1]); panels[gm] == nil {
				panels[gm] = newJace(c.Name)
			}
		}
	}
	type ref struct{ gm, k int }
	boToGM := map[string]ref{}
	for _, c := range b.Comps.All() {
		m := relayRE.FindStringSubmatch(c.Name)
		if m == nil {
			continue
		}
		gm, ok := deviceGM(b, c)
		if !ok {
			continue
		}
		k := atoi(m[1])
		objid := ""
		for sn, sv := range c.Slots.All() {
			if contains(sn, "objectId") {
				objid = str(sv.Value)
			}
		}
		if panels[gm] == nil {
			panels[gm] = newJace("")
		}
		panels[gm].bo[k] = objid
		boToGM[c.Handle] = ref{gm, k}
	}
	for _, lk := range b.Links {
		if !driveSlots[str(lk.TgtSlot)] || lk.ContainerH == nil {
			continue
		}
		r, ok := boToGM[*lk.ContainerH]
		if !ok {
			continue
		}
		e := panels[r.gm]
		e.feedOK[r.k] = true
		e.feedPrio[r.k] = str(lk.TgtSlot)
	}
	return panels
}
