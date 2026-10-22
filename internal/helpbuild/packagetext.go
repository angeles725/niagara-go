package helpbuild

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/htmltext"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Package descriptions (no Python twin): the prose <description> of each
// package-index.bajadoc under devguide/ and guides/ becomes one clean-text
// page package-text/<package>.txt, indexed by the full-text source of the
// same name and printed by `nh package`. The class list of the listing
// stays indexed class by class (bajadoc-xml); only its prose was dropped.

const (
	packageTextDir    = "package-text"
	packageIndexName  = "package-index.bajadoc"
	packageTextScript = "package_text_builder"
)

// packageDesc is one module's description of a package.
type packageDesc struct {
	Pkg, Module, Text string
}

var bajadocCharRef = regexp.MustCompile(`&#(x[0-9a-fA-F]+|[0-9]+);`)

// sanitizeCharRefs turns the character references XML forbids into '?'
// (bajadocxml's sanitize, which the class reader applies on a retry).
func sanitizeCharRefs(data []byte) []byte {
	return bajadocCharRef.ReplaceAllFunc(data, func(m []byte) []byte {
		ref := string(m[2 : len(m)-1])
		var cp int64
		var err error
		if ref[0] == 'x' || ref[0] == 'X' {
			cp, err = strconv.ParseInt(ref[1:], 16, 64)
		} else {
			cp, err = strconv.ParseInt(ref, 10, 64)
		}
		if err == nil && (cp == 9 || cp == 10 || cp == 13 || (0x20 <= cp && cp <= 0xD7FF) ||
			(0xE000 <= cp && cp <= 0xFFFD) || (0x10000 <= cp && cp <= 0x10FFFF)) {
			return m
		}
		return []byte("?")
	})
}

// readPackageDesc reads the <package> element of a package-index.bajadoc:
// its name, its module ("<module>-<runtimeProfile>") and its description
// as plain text (the escaped HTML converted like a help page). ok is false
// when the document does not parse, even sanitized, or has no package;
// Text is "" for an empty description.
func readPackageDesc(data []byte) (d packageDesc, ok bool) {
	d, err := parsePackageDesc(data)
	if err != nil {
		if d, err = parsePackageDesc(sanitizeCharRefs(data)); err != nil {
			return packageDesc{}, false
		}
	}
	return d, d.Pkg != ""
}

func parsePackageDesc(data []byte) (packageDesc, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var d packageDesc
	var desc strings.Builder
	depth, inPkg, inDesc := 0, false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return packageDesc{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "package" && !inPkg && d.Pkg == "":
				inPkg = true
				var module, profile string
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "name":
						d.Pkg = a.Value
					case "module":
						module = a.Value
					case "runtimeProfile":
						profile = a.Value
					}
				}
				d.Module = module
				if profile != "" {
					d.Module += "-" + profile
				}
			case inPkg && t.Name.Local == "description" && depth == 3:
				inDesc = true
			}
		case xml.EndElement:
			if inDesc && t.Name.Local == "description" {
				inDesc = false
			}
			if inPkg && t.Name.Local == "package" {
				inPkg = false
			}
			depth--
		case xml.CharData:
			if inDesc {
				desc.Write(t)
			}
		}
	}
	_, text := htmltext.HTMLToText(desc.String())
	d.Text = text
	return d, nil
}

// packageDescs walks devguide/ then guides/ (os.walk order, sorted) for
// package-index.bajadoc files. An unreadable file is an error; one that
// does not parse is a warning, skipped.
func packageDescs(baseDir string) (descs []packageDesc, warnings []string, err error) {
	for _, top := range []string{"devguide", "guides"} {
		root := pycompat.Join(baseDir, top)
		if !pycompat.IsDir(root) {
			continue
		}
		pycompat.Walk(root, func(dir string, dirs, files []string) []string {
			if err != nil {
				return nil
			}
			sort.Strings(dirs)
			for _, name := range files {
				if name != packageIndexName {
					continue
				}
				path := pycompat.Join(dir, name)
				data, rerr := os.ReadFile(path)
				if rerr != nil {
					err = rerr
					return nil
				}
				d, ok := readPackageDesc(data)
				if !ok {
					warnings = append(warnings, "WARNING: "+packageTextScript+": cannot parse "+path)
					continue
				}
				descs = append(descs, d)
			}
			return dirs
		})
		if err != nil {
			return nil, warnings, err
		}
	}
	return descs, warnings, nil
}

// packagePages groups the non-empty descriptions by package: one page per
// package, its modules in walk order, a description repeated by another
// module (or by the guides/ copy) written once.
func packagePages(descs []packageDesc) []textPage {
	type group struct {
		modules []string
		texts   []packageDesc
	}
	groups := map[string]*group{}
	var order []string
	for _, d := range descs {
		if d.Text == "" {
			continue
		}
		g := groups[d.Pkg]
		if g == nil {
			g = &group{}
			groups[d.Pkg] = g
			order = append(order, d.Pkg)
		}
		dup := false
		for _, x := range g.texts {
			dup = dup || x.Text == d.Text
		}
		if !dup {
			g.texts = append(g.texts, d)
		}
		if !containsString(g.modules, d.Module) {
			g.modules = append(g.modules, d.Module)
		}
	}
	sort.Strings(order)
	pages := make([]textPage, 0, len(order))
	for _, pkg := range order {
		g := groups[pkg]
		var b strings.Builder
		fmt.Fprintf(&b, "TITLE: %s package\nPACKAGE: %s\nMODULE: %s\n\n", pkg, pkg, strings.Join(g.modules, ", "))
		for i, d := range g.texts {
			if i > 0 {
				b.WriteString("\n")
			}
			if len(g.texts) > 1 {
				fmt.Fprintf(&b, "DESCRIPTION (%s):\n", d.Module)
			} else {
				b.WriteString("DESCRIPTION:\n")
			}
			b.WriteString(d.Text + "\n")
		}
		pages = append(pages, textPage{pkg + ".txt", b.String()})
	}
	return pages
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// buildPackageText is the package_text_builder step of rebuild-all.
func buildPackageText(baseDir string) (*textResult, error) {
	descs, warnings, err := packageDescs(baseDir)
	res := &textResult{Warnings: warnings}
	if err != nil {
		return res, err
	}
	pages := packagePages(descs)
	if err := writeTextSource(baseDir, packageTextDir, pages); err != nil {
		return res, err
	}
	res.Files = len(pages)
	res.Log = append(res.Log, fmt.Sprintf("%s/: %d package descriptions (from %d %s files)",
		packageTextDir, len(pages), len(descs), packageIndexName))
	return res, nil
}

// PackageDescription is the description block of package-text/<pkg>.txt
// under baseDir, the lines after its header; nil when the package has no
// description page.
func PackageDescription(baseDir, pkg string) []string {
	text, err := pycompat.ReadTextReplace(pycompat.Join(baseDir, packageTextDir, pkg+".txt"))
	if err != nil {
		return nil
	}
	_, body, _ := strings.Cut(text, "\n\n")
	return strings.Split(strings.TrimRight(body, "\n"), "\n")
}
