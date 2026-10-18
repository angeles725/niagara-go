package domain

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/license"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Driver anatomy (drivers.py): a driver network is a top-level class in an -rt
// submodule whose superclass chain reaches BDeviceNetwork; its anatomy comes
// from the class, inheritance and module indexes, its source (getDeviceType()),
// the licensing calls and the module lexicons (help.guide.base).

// families is FAMILIES, in test order.
var families = [][2]string{{"BDdfNetwork", "DDF"}, {"BNNetwork", "NDriver"}, {"BBasicNetwork", "BasicDriver"}}

// managerBases is _MANAGER_BASES.
var managerBases = []string{"BAbstractManager", "BDeviceManager", "BPointManager"}

// simple is _simple: "javax.baja.driver.BDeviceNetwork<X>" -> "BDeviceNetwork".
func simple(typeName string) string {
	s, _, _ := strings.Cut(typeName, "<")
	s = pycompat.Strip(s)
	return s[strings.LastIndex(s, ".")+1:]
}

// item is one (name, entry) of the corpus.
type item struct {
	name string
	e    *index.Object
}

func (it item) module() string { return core.Str(it.e, "module") }
func (it item) path() string   { return core.Str(it.e, "path") }

// familyDir is entry["path"].split("/", 1)[0].
func familyDir(path string) string { d, _, _ := strings.Cut(path, "/"); return d }

// corpus is _Corpus: the class index grouped for anatomy lookups.
type corpus struct {
	chains      *index.Object
	bySub       map[string][]item
	subs        []string // bySub keys
	byFamily    map[string][]item
	byName      map[string][]item
	chainMemory map[*index.Object][]string
}

func newCorpus(classes, inheritance *index.Object) *corpus {
	cx := &corpus{chains: inheritance.Object("class_to_chain"), bySub: map[string][]item{},
		byFamily: map[string][]item{}, byName: map[string][]item{}, chainMemory: map[*index.Object][]string{}}
	for i, name := range classes.Keys() {
		for _, e := range core.Objects(classes.Values()[i]) {
			it := item{name, e}
			if core.Truthy(e, "outer_class") || it.module() == "docSource-doc" {
				continue
			}
			if _, seen := cx.bySub[it.module()]; !seen {
				cx.subs = append(cx.subs, it.module())
			}
			cx.bySub[it.module()] = append(cx.bySub[it.module()], it)
			cx.byFamily[familyDir(it.path())] = append(cx.byFamily[familyDir(it.path())], it)
			cx.byName[name] = append(cx.byName[name], it)
		}
	}
	return cx
}

// chain is _Corpus.chain: the entry's own superclass, then that class's chain.
func (cx *corpus) chain(e *index.Object) []string {
	if c, ok := cx.chainMemory[e]; ok {
		return c
	}
	var out []string
	ext, _ := e.Get("extends")
	s, _ := ext.(string)
	if parent := simple(s); parent != "" {
		out = append([]string{parent}, func() []string {
			if cx.chains == nil {
				return nil
			}
			return core.Strs(cx.chains, parent)
		}()...)
	}
	cx.chainMemory[e] = out
	return out
}

// sortedSet is sorted(set(xs)).
func sortedSet(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// subclasses is _Corpus.subclasses: the sorted names of items whose chain holds base.
func (cx *corpus) subclasses(items []item, base string) []string {
	var names []string
	for _, it := range items {
		if contains(cx.chain(it.e), base) {
			names = append(names, it.name)
		}
	}
	return sortedSet(names)
}

// inherited is _Corpus.inherited: subclasses of base in the network's
// submodule, else in the submodule of its nearest superclass network.
func (cx *corpus) inherited(e *index.Object, base string) ([]string, string) {
	own := core.Str(e, "module")
	subs := []string{own}
	for _, parent := range cx.chain(e) {
		for _, it := range cx.byName[parent] {
			if strings.HasSuffix(it.module(), "-rt") {
				subs = append(subs, it.module())
			}
		}
	}
	for _, sub := range subs {
		if found := cx.subclasses(cx.bySub[sub], base); len(found) > 0 {
			return found, sub
		}
	}
	return []string{}, own
}

func family(chain []string) string {
	for _, f := range families {
		if contains(chain, f[0]) {
			return f[1]
		}
	}
	return "standalone"
}

// deviceTypes is _DEVICE_TYPE.findall on the network's source:
// \bgetDeviceType\s*\(\s*\)\s*\{\s*return\s+([\w$]+)\.TYPE\b.
func deviceTypes(organized, path string) []string {
	text, ok := license.ReadText(filepath.Join(organized, path))
	if !ok {
		return nil
	}
	var out []string
	for i := 0; ; {
		k := strings.Index(text[i:], "getDeviceType")
		if k < 0 {
			return out
		}
		p := i + k
		if name, end, ok := deviceTypeAt(text, p); ok {
			out, i = append(out, name), end
		} else {
			i = p + 1
		}
	}
}

func deviceTypeAt(text string, p int) (string, int, bool) {
	if wordBefore(text, p) {
		return "", 0, false
	}
	i := p + len("getDeviceType")
	for _, tok := range []string{"(", ")", "{", "return"} {
		i = skipSpace(text, i)
		if !strings.HasPrefix(text[i:], tok) {
			return "", 0, false
		}
		i += len(tok)
	}
	j := skipSpace(text, i)
	if j == i {
		return "", 0, false
	}
	k := j
	for k < len(text) {
		r, n := runeAt(text, k)
		if r != '$' && !pycompat.IsWordRune(r) {
			break
		}
		k += n
	}
	if k == j || !strings.HasPrefix(text[k:], ".TYPE") || wordAt(text, k+len(".TYPE")) {
		return "", 0, false
	}
	return text[j:k], k + len(".TYPE"), true
}

// networkRows loads the indexes, the corpus root and the license features
// Drivers reads, then returns its rows and scope, or ok=false (after an
// exit-2 error, or silently when an index is empty).
func networkRows(env *registry.Env) (rows []*DriverRow, scope string, ok bool) {
	ci := env.Load("class-index.json", core.LoadOpts{})
	var inh, inv *index.Object
	if ci != nil && ci.Len() > 0 {
		inh = env.Load("inheritance.json", core.LoadOpts{})
	}
	if inh != nil && inh.Len() > 0 {
		inv = env.Load("module-inventory.json", core.LoadOpts{})
	}
	if inv == nil || inv.Len() == 0 {
		return nil, "", false
	}
	organized := env.SourceRoot(ci.Object("_meta"))
	if organized == "" {
		return nil, "", false
	}
	report := license.Scan(env)
	if report == nil {
		return nil, "", false
	}
	rows, scope = Drivers(DriverInputs{ClassIndex: ci, Inheritance: inh, Inventory: inv,
		Organized: organized, Features: FeaturesByModule(report.Calls)})
	return rows, scope, true
}

func anatomy(cx *corpus, it item, organized string, features map[string][]string, docs *docIndex) *DriverRow {
	sub, dir := it.module(), familyDir(it.path())
	fam := cx.byFamily[dir]
	declared := sortedSet(deviceTypes(organized, it.path()))
	r := &DriverRow{Network: it.name, Module: sub, Package: core.Str(it.e, "package"), Path: it.path(),
		Abstract: contains(core.Strs(it.e, "modifiers"), "abstract"), Extends: simple(extendsOf(it.e)),
		Family: family(append([]string{it.name}, cx.chain(it.e)...))}
	r.Devices, r.DevicesFrom = cx.inherited(it.e, "BDevice")
	r.DevicesVia = "subclasses"
	if len(declared) > 0 {
		r.Devices, r.DevicesFrom, r.DevicesVia = declared, sub, "getDeviceType()"
	}
	r.PointDeviceExts, r.PointDeviceExtsFrom = cx.inherited(it.e, "BPointDeviceExt")
	r.ProxyExts, r.ProxyExtsFrom = cx.inherited(it.e, "BProxyExt")
	var managers []string
	for _, f := range fam {
		ch := cx.chain(f.e)
		isManager := false
		for _, b := range managerBases {
			isManager = isManager || contains(ch, b)
		}
		if isManager || (strings.HasSuffix(f.module(), "-ux") && strings.HasPrefix(f.name, "B") &&
			strings.HasSuffix(f.name, "Manager")) {
			managers = append(managers, f.name)
		}
	}
	r.Managers = sortedSet(managers)
	r.DiscoveryJobs = []string{}
	for _, n := range cx.subclasses(fam, "BJob") {
		if strings.Contains(n, "Discover") || strings.Contains(n, "Learn") {
			r.DiscoveryJobs = append(r.DiscoveryJobs, n)
		}
	}
	r.License = sortedSet(features[sub])
	r.doc = docs.target(organized, dir, sub)
	return r
}

func extendsOf(e *index.Object) string {
	v, _ := e.Get("extends")
	s, _ := v.(string)
	return s
}
