package helpbuild

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

// The part of niagara_help_lib/bajadoc_xml.py the class index reads: the
// walk of iter_classes and, of parse_file/parse_class, the class attributes
// (name, package, module, kind). The members, types and descriptions belong
// to the bajadoc-clean/ text (S7b).

// bajadocWalk is bajadoc_xml.iter_classes' walk: os.walk(root) with sorted
// folders and files, every *.bajadoc but module-index.bajadoc. visit gets
// the path (os.path.join from root) and the path relative to root with "/"
// separators.
func bajadocWalk(root string, visit func(path, rel string)) {
	pyWalk(root, func(dir string, dirs, files []string) []string {
		sort.Strings(dirs)
		sort.Strings(files)
		sub, err := filepath.Rel(root, dir) // dir is root joined with folder names
		if err != nil {
			sub = "."
		}
		for _, name := range files {
			if !strings.HasSuffix(name, ".bajadoc") || name == "module-index.bajadoc" {
				continue
			}
			rel := name
			if sub != "." {
				rel = filepath.ToSlash(sub) + "/" + name
			}
			visit(pycompat.Join(dir, name), rel)
		}
		return dirs
	})
}

// parsedBajadoc is bajadocClassHeads' result for one file.
type parsedBajadoc struct {
	heads  []bajadocClassHead
	failed bool
	err    error
}

// parseBajadocFiles runs bajadocClassHeads on every path, on half the CPUs
// (the files are independent; a large tree holds 115 MB of XML), and
// returns the results in the order of paths.
func parseBajadocFiles(paths []string) []parsedBajadoc {
	out := make([]parsedBajadoc, len(paths))
	workers := max(1, min(runtime.GOMAXPROCS(0)/2, len(paths)))
	next := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f := &out[i]
				f.heads, f.failed, f.err = bajadocClassHeads(paths[i])
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// bajadocClassHead is the part of a parse_class dict the class index keeps.
type bajadocClassHead struct {
	Name, Package, Module, Kind string
}

// bajadocClassHeads is bajadoc_xml.parse_file reduced to the class heads:
// ET.parse, and when that is not well-formed, ET.fromstring of the bytes
// with the forbidden character references replaced (failed reports a file
// that stays unparseable: PARSE_FAILURES, no class). Every <class> child of
// the root with a qualifiedName counts. err != nil is what ET.parse raises
// besides ParseError (the file cannot be read): Python does not catch it.
func bajadocClassHeads(path string) (heads []bajadocClassHead, failed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return nil, false, errors.New(pycompat.OSErrorText(&fs.PathError{Op: pe.Op, Path: path, Err: pe.Err}))
		}
		return nil, false, err
	}
	root, perr := pyxml.Parse(data, pyxml.Mode{ETree: true})
	if perr != nil {
		root, perr = pyxml.Parse(sanitizeCharRefs(data), pyxml.Mode{ETree: true})
		if perr != nil {
			return nil, true, nil
		}
	}
	for _, el := range root.Children {
		if el.Tag != "class" {
			continue
		}
		if q, _ := el.Get("qualifiedName"); q == "" {
			continue
		}
		heads = append(heads, classHead(el))
	}
	return heads, false, nil
}

// classHead is parse_class's name, package, module and kind (bajadoc_xml._kind).
func classHead(el *pyxml.Element) bajadocClassHead {
	attr := func(name string) string { v, _ := el.Get(name); return v }
	h := bajadocClassHead{Name: attr("name"), Package: attr("packageName"), Module: attr("module")}
	if profile := attr("runtimeProfile"); profile != "" {
		h.Module += "-" + profile
	}
	category := attr("category")
	switch {
	case attr("enum") == "true" || category == "enum":
		h.Kind = "enum"
	case attr("annotation") == "true" || category == "annotation":
		h.Kind = "annotation_type"
	case attr("interface") == "true":
		h.Kind = "interface"
	default:
		h.Kind = "class"
	}
	return h
}

// bdCharRef is bajadoc_xml._CHARREF (a bytes pattern).
var bdCharRef = regexp.MustCompile(`&#(x[0-9a-fA-F]+|[0-9]+);`)

// sanitizeCharRefs is bajadoc_xml._sanitize: a character reference to a
// code point XML forbids (&#x0; in JDK annotation defaults) becomes "?".
func sanitizeCharRefs(data []byte) []byte {
	return bdCharRef.ReplaceAllFunc(data, func(m []byte) []byte {
		ref := string(m[2 : len(m)-1])
		base := 10
		if ref[0] == 'x' {
			ref, base = ref[1:], 16
		}
		if cp, ok := charRefValue(ref, base); ok && isXMLCharCode(cp) {
			return m
		}
		return []byte("?")
	})
}

// charRefValue parses the digits of a reference; ok is false when the value
// is beyond any code point (Python's int has no limit, and the value is
// then no XML character either).
func charRefValue(digits string, base int) (uint64, bool) {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return 0, true
	}
	v, err := strconv.ParseUint(digits, base, 32)
	return v, err == nil
}

// isXMLCharCode is bajadoc_xml._xml_char.
func isXMLCharCode(cp uint64) bool {
	return cp == 9 || cp == 10 || cp == 13 || 0x20 <= cp && cp <= 0xD7FF ||
		0xE000 <= cp && cp <= 0xFFFD || 0x10000 <= cp && cp <= 0x10FFFF
}
