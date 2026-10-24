package classfile

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/modjar"
)

const nre = "Ljavax/baja/nre/annotations/"

func slotLines(ss []Slot) string {
	var out []string
	for _, s := range ss {
		out = append(out, fmt.Sprintf("%s %s type=%s ret=%s def=%s flags=%s facets=%s override=%t via=%s",
			s.Kind, s.Name, s.Type, s.ReturnType, s.Default, s.Flags, strings.Join(s.Facets, "|"), s.Override, s.Via))
	}
	return strings.Join(out, "\n")
}

// Slots come from every Niagara slot annotation shape in annotation order,
// then from public static final Property/Action/Topic fields no annotation
// declared, in field order.
func TestNiagaraSlots(t *testing.T) {
	c := newClass()
	p := c.pool
	prop := func(name string, extra ...any) []byte {
		return evAnn(p.ann(nre+"NiagaraProperty", append([]any{"name", p.evStr(name), "type", p.evStr("BStatusBoolean")}, extra...)...))
	}
	c.attrs = [][]byte{
		p.anns(true, p.ann(nre+"NiagaraType"), p.ann("La/Other;", "name", p.evStr("ignored"))),
		p.anns(false,
			p.ann(nre+"NiagaraProperties", "value", evArray(
				prop("out", "defaultValue", p.evStr("new BStatusBoolean()"), "flags", p.evInt(267)),
				prop("facets", "override", evConst('Z', p.raw(3, 0, 0, 0, 1)), "facets", evArray(
					evAnn(p.ann(nre+"Facet", "value", p.evStr("BFacets.make()"))),
					evAnn(p.ann(nre+"Facet", "name", p.evStr("k"), "value", p.evStr("v"))))))),
			p.ann(nre+"NiagaraAction", "name", p.evStr("reset"), "parameterType", p.evStr("BBoolean"),
				"returnType", p.evStr("BString"), "defaultValue", p.evStr("BBoolean.TRUE")),
			p.ann(nre+"NiagaraTopics", "value", evArray(evAnn(p.ann(nre+"NiagaraTopic", "name", p.evStr("fired"), "eventType", p.evStr("BValue"))))),
			p.ann(nre+"NiagaraSlots", "properties", evArray(prop("in")), "topics", evArray())),
	}
	psf := uint16(AccPublic | AccStatic | AccFinal)
	c.fields = [][]byte{
		p.member(psf, "in", "Ljavax/baja/sys/Property;"),
		p.member(psf, "extra", "Ljavax/baja/sys/Property;"),
		p.member(psf, "TYPE", "Ljavax/baja/sys/Type;"),
		p.member(AccPublic|AccStatic, "notFinal", "Ljavax/baja/sys/Property;"),
		p.member(AccStatic|AccFinal, "notPublic", "Ljavax/baja/sys/Action;"),
		p.member(psf, "go", "Ljavax/baja/sys/Action;"),
		p.member(psf, "changed", "Ljavax/baja/sys/Topic;"),
	}
	cf, err := Parse(c.bytes())
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"property out type=BStatusBoolean ret= def=new BStatusBoolean() flags=267 facets= override=false via=annotation",
		"property facets type=BStatusBoolean ret= def= flags= facets=BFacets.make()|k=v override=true via=annotation",
		"action reset type=BBoolean ret=BString def=BBoolean.TRUE flags= facets= override=false via=annotation",
		"topic fired type=BValue ret= def= flags= facets= override=false via=annotation",
		"property in type=BStatusBoolean ret= def= flags= facets= override=false via=annotation",
		"property extra type= ret= def= flags= facets= override=false via=field",
		"action go type= ret= def= flags= facets= override=false via=field",
		"topic changed type= ret= def= flags= facets= override=false via=field",
	}, "\n")
	if got := slotLines(NiagaraSlots(cf)); got != want {
		t.Errorf("slots:\n%s\nwant:\n%s", got, want)
	}
}

func TestNiagaraSlotsNone(t *testing.T) {
	cf, err := Parse(newClass().bytes())
	if err != nil {
		t.Fatal(err)
	}
	if ss := NiagaraSlots(cf); len(ss) != 0 {
		t.Errorf("slots = %+v", ss)
	}
}

// The real 4.15 kitControl BBooleanLatch, read from the installed jar.
func TestNiagaraSlotsRealBooleanLatch(t *testing.T) {
	const jar = "/mnt/c/Niagara/Niagara-4.15.6.20/modules/kitControl-rt.jar"
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("Niagara 4.15.6.20 kitControl-rt.jar not installed at %s: %v", jar, err)
	}
	j, err := modjar.Open(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	i := j.Last("com/tridium/kitControl/util/BBooleanLatch.class")
	if i < 0 {
		t.Fatal("BBooleanLatch.class not in kitControl-rt.jar")
	}
	b, err := j.Read(i, 0)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := ParseReader(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if cf.ThisClass != "com/tridium/kitControl/util/BBooleanLatch" || cf.SuperClass != "com/tridium/kitControl/util/BLatch" {
		t.Errorf("class = %s extends %s", cf.ThisClass, cf.SuperClass)
	}
	got := slotLines(NiagaraSlots(cf))
	t.Logf("BBooleanLatch slots:\n%s", got)
	want := strings.Join([]string{
		"property facets type=BFacets ret= def=BFacets.makeBoolean() flags= facets= override=true via=annotation",
		"property out type=BStatusBoolean ret= def=new BStatusBoolean() flags=267 facets= override=false via=annotation",
		"property in type=BStatusBoolean ret= def=new BStatusBoolean() flags=8 facets= override=false via=annotation",
	}, "\n")
	if got != want {
		t.Errorf("slots:\n%s\nwant:\n%s", got, want)
	}
}
