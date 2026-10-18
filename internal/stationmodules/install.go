package stationmodules

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

// part is one module-part from its META-INF/module.xml (parse_module_xml).
type part struct {
	name, module, profile, symbol, vendor, version string
	types                                          map[string]bool
	deps                                           []string
	jar                                            string
}

// partIndex is the {partName: meta} dict of index_install, in insertion order.
type partIndex struct {
	order  []string
	byName map[string]*part
}

func (x *partIndex) set(p *part) {
	if _, ok := x.byName[p.name]; !ok {
		x.order = append(x.order, p.name)
	}
	x.byName[p.name] = p
}

func (x *partIndex) each(fn func(*part)) {
	for _, n := range x.order {
		fn(x.byName[n])
	}
}

// readZipMember is _read_zip_member: the member's bytes, or false for any read failure.
// Python returns None only for a missing member, a bad ZIP or an OSError and crashes on a
// corrupt deflate stream or an encrypted member; here every failure is None.
func readZipMember(jar, name string) ([]byte, bool) {
	z, err := zip.OpenReader(jar)
	if err != nil {
		return nil, false
	}
	defer z.Close()
	var hit *zip.File
	for _, f := range z.File {
		if f.Name == name {
			hit = f // zipfile keeps the last entry of a repeated name
		}
	}
	if hit == nil {
		return nil, false
	}
	r, err := hit.Open()
	if err != nil {
		return nil, false
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	return data, err == nil
}

// parseModuleXML is parse_module_xml: nil when ElementTree would reject the document.
func parseModuleXML(raw []byte) *part {
	root, err := pyxml.Parse(raw, pyxml.Mode{ETree: true})
	if err != nil {
		return nil
	}
	attr := func(k string) string { v, _ := root.Get(k); return v }
	p := &part{name: attr("name"), module: attr("moduleName"), profile: attr("runtimeProfile"),
		symbol: attr("preferredSymbol"), vendor: attr("vendor"), version: attr("vendorVersion"),
		types: map[string]bool{}}
	var iter func(*pyxml.Element)
	iter = func(e *pyxml.Element) {
		switch e.Tag {
		case "type":
			if n, _ := e.Get("name"); n != "" {
				p.types[n] = true
			}
		case "dependency":
			if n, _ := e.Get("name"); n != "" {
				p.deps = append(p.deps, n)
			}
		}
		for _, c := range e.Children {
			iter(c)
		}
	}
	iter(root)
	return p
}

// indexInstall indexes every module-part jar under <root>/modules; ok is false when there
// is no modules/ directory.
func indexInstall(root string) (*partIndex, string, bool) {
	dir := pycompat.Join(root, "modules")
	if !pycompat.IsDir(dir) {
		return nil, dir, false
	}
	x := &partIndex{byName: map[string]*part{}}
	entries, _ := os.ReadDir(dir) // sorted by name, as sorted(os.listdir()) is
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jar") {
			continue
		}
		jar := pycompat.Join(dir, e.Name())
		raw, ok := readZipMember(jar, "META-INF/module.xml")
		if !ok {
			continue
		}
		if p := parseModuleXML(raw); p != nil && p.name != "" {
			p.jar = jar
			x.set(p)
		}
	}
	return x, dir, true
}

// indexSw maps partName -> staged jar paths under <root>/sw, walked like os.walk (top-down,
// files of a directory before its subdirectories, symlinked directories not entered).
// Directory entries are visited in sorted order; Python uses the file system's order.
func indexSw(root string) map[string][]string {
	staged := map[string][]string{}
	sw := pycompat.Join(root, "sw")
	if !pycompat.IsDir(sw) {
		return staged
	}
	var walk func(string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range entries {
			p := pycompat.Join(dir, e.Name())
			if pycompat.IsDir(p) {
				dirs = append(dirs, p)
			} else if strings.HasSuffix(e.Name(), ".jar") {
				n := strings.TrimSuffix(e.Name(), ".jar")
				staged[n] = append(staged[n], p)
			}
		}
		for _, d := range dirs {
			if st, err := os.Lstat(d); err == nil && st.Mode()&os.ModeSymlink == 0 {
				walk(d)
			}
		}
	}
	walk(sw)
	return staged
}

// jarVersion is the vendorVersion of a jar's module.xml, "" when unknown.
func jarVersion(path string) string {
	raw, ok := readZipMember(path, "META-INF/module.xml")
	if !ok {
		return ""
	}
	if p := parseModuleXML(raw); p != nil {
		return p.version
	}
	return ""
}

// locateFix picks a staged jar whose real manifest version matches want: the first exact
// match, else the highest same-x.y.z build, else the highest real version (never a bare
// "1.0" stub unless nothing else exists). Ties keep the later candidate, as sorted()[-1].
func locateFix(cands []string, want string, version func(string) string) (string, string) {
	type cv struct{ path, ver string }
	scored := make([]cv, len(cands))
	for i, c := range cands {
		scored[i] = cv{c, version(c)}
	}
	highest := func(list []cv) (string, string) {
		sort.SliceStable(list, func(i, j int) bool { return list[i].ver < list[j].ver })
		last := list[len(list)-1]
		return last.path, last.ver
	}
	if want != "" {
		for _, s := range scored {
			if s.ver == want {
				return s.path, s.ver
			}
		}
		base := strings.Join(head(strings.Split(want, "."), 3), ".")
		var near []cv
		for _, s := range scored {
			if strings.HasPrefix(s.ver, base+".") || s.ver == base {
				near = append(near, s)
			}
		}
		if len(near) > 0 {
			return highest(near)
		}
	}
	var real []cv
	for _, s := range scored {
		if s.ver != "" && s.ver != "1.0" {
			real = append(real, s)
		}
	}
	if len(real) == 0 {
		real = scored
	}
	return highest(real)
}

func head(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// applyFix is apply_fix: "exists" when the target is already there, else a shutil.copy2
// (content, permission bits and modification time; the access time is set to the source's
// modification time).
func applyFix(name, src, modulesDir string) (string, string, error) {
	dst := pycompat.Join(modulesDir, name+".jar")
	if _, err := os.Stat(dst); err == nil {
		return dst, "exists", nil
	}
	in, err := os.Open(src)
	if err != nil {
		return dst, "", err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return dst, "", err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return dst, "", err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Close()
	} else {
		out.Close()
	}
	if err == nil {
		err = os.Chmod(dst, st.Mode().Perm())
	}
	if err == nil {
		err = os.Chtimes(dst, st.ModTime(), st.ModTime())
	}
	return dst, "copied", err
}
