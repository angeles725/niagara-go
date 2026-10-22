package nlicense

import (
	"errors"
	"strings"
)

// lxml's serialisation of a parsed tree (ElementTree.write, no XML
// declaration, utf-8, no pretty printing) and the tool's canonical encoding.

var textEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#13;")
var attrEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;",
	"\n", "&#10;", "\r", "&#13;", "\t", "&#9;")

func writeNode(b *strings.Builder, n *node) {
	switch n.kind {
	case textNode:
		b.WriteString(textEsc.Replace(n.text))
	case commentNode:
		b.WriteString("<!--" + n.text + "-->")
	case piNode:
		b.WriteString("<?" + n.name)
		if n.text != "" {
			b.WriteString(" " + n.text)
		}
		b.WriteString("?>")
	case doctypeNode:
		b.WriteString(n.name + "\n")
	case elemNode:
		b.WriteString("<" + n.name)
		for _, a := range n.attrs {
			b.WriteString(" " + a.name + `="` + attrEsc.Replace(a.value) + `"`)
		}
		if len(n.children) == 0 {
			b.WriteString("/>")
			return
		}
		b.WriteString(">")
		for _, c := range n.children {
			writeNode(b, c)
		}
		b.WriteString("</" + n.name + ">")
	}
}

// serialize mirrors ElementTree(root).write(path, xml_declaration=False,
// encoding="utf-8"): misc nodes before the root (a DOCTYPE ends with a
// newline), the root, then the misc nodes after it.
func (d *document) serialize() []byte {
	var b strings.Builder
	for _, n := range d.before {
		writeNode(&b, n)
	}
	writeNode(&b, d.root)
	for _, n := range d.after {
		writeNode(&b, n)
	}
	return []byte(b.String())
}

// errCythonTag is what canonical_encode raises on a comment or PI child: lxml
// gives those nodes a function as .tag, and StringIO.write rejects it.
var errCythonTag = errors.New("TypeError: string argument expected, got '_cython_3_2_9.cython_function_or_method'")

const xmlNS = "http://www.w3.org/XML/1998/namespace"

// clark resolves a qname to lxml's {uri}local form within scope.
func clark(qname string, scope map[string]string, isAttr bool) string {
	prefix, local, ok := strings.Cut(qname, ":")
	if !ok {
		if isAttr {
			return qname
		}
		local, prefix = qname, ""
	}
	if prefix == "xml" {
		return "{" + xmlNS + "}" + local
	}
	if uri := scope[prefix]; uri != "" {
		return "{" + uri + "}" + local
	}
	return local
}

func childScope(n *node, scope map[string]string) map[string]string {
	var out map[string]string
	for _, a := range n.attrs {
		if a.name == "xmlns" || strings.HasPrefix(a.name, "xmlns:") {
			if out == nil {
				out = make(map[string]string, len(scope)+1)
				for k, v := range scope {
					out[k] = v
				}
			}
			out[strings.TrimPrefix(strings.TrimPrefix(a.name, "xmlns"), ":")] = a.value
		}
	}
	if out == nil {
		return scope
	}
	return out
}

// canonicalEncode mirrors the tool's canonical_encode (LicenseUtil.encode):
// "<tag a="v">\n", element children recursively, "</tag>\n"; text dropped
// unless keepText, attribute values raw, namespaced names in Clark notation.
// With keepText, a text node that is not whitespace only is written raw followed
// by "\n", as LicenseUtil.encode writes the XText that XParser keeps
// (organized/baja/baja/decompiled/com/tridium/sys/license/LicenseUtil.java:141-143).
func canonicalEncode(n *node, scope map[string]string, b *strings.Builder, keepText bool) error {
	scope = childScope(n, scope)
	tag := clark(n.name, scope, false)
	b.WriteString("<" + tag)
	for _, a := range n.attrs {
		if a.name == "xmlns" || strings.HasPrefix(a.name, "xmlns:") {
			continue
		}
		b.WriteString(" " + clark(a.name, scope, true) + `="` + a.value + `"`)
	}
	b.WriteString(">\n")
	for _, c := range n.children {
		switch c.kind {
		case elemNode:
			if err := canonicalEncode(c, scope, b, keepText); err != nil {
				return err
			}
		case textNode:
			if keepText && strings.Trim(c.text, " \t\r\n") != "" {
				b.WriteString(c.text + "\n")
			}
		case commentNode, piNode:
			return errCythonTag
		}
	}
	b.WriteString("</" + tag + ">\n")
	return nil
}

// canonicalBytes mirrors canonical_bytes_for_verify: the root without its
// <signature> children, text dropped (a license holds none that is signed).
func canonicalBytes(root *node) ([]byte, error) { return signedBytes(root, false) }

// certCanonicalBytes is what a certificate's signature covers: the root without
// its <signature>, the <publicKey> text kept (CertificateFile.java:85-86).
func certCanonicalBytes(root *node) ([]byte, error) { return signedBytes(root, true) }

func signedBytes(root *node, keepText bool) ([]byte, error) {
	var b strings.Builder
	clone := *root
	clone.children = nil
	for _, c := range root.children {
		if !isSignature(root, c) {
			clone.children = append(clone.children, c)
		}
	}
	if err := canonicalEncode(&clone, map[string]string{}, &b, keepText); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// isSignature reports whether c, a child of root, is findall("signature")-visible.
func isSignature(root, c *node) bool {
	return c.kind == elemNode && clark(c.name, childScope(c, childScope(root, map[string]string{})), false) == "signature"
}

// find mirrors _Element.find(tag) for an un-namespaced tag: the first direct
// element child whose Clark name is tag.
func find(root *node, tag string) *node {
	scope := childScope(root, map[string]string{})
	for _, c := range root.children {
		if c.kind == elemNode && clark(c.name, childScope(c, scope), false) == tag {
			return c
		}
	}
	return nil
}

// elemText mirrors _Element.text: the leading text node, or nil.
func elemText(n *node) *string {
	if len(n.children) > 0 && n.children[0].kind == textNode {
		return &n.children[0].text
	}
	return nil
}

// setSignature mirrors _set_signature: drop every <signature> child together
// with its tail, then append <signature>b64</signature>.
func setSignature(root *node, b64 string) {
	var kept []*node
	for i := 0; i < len(root.children); i++ {
		c := root.children[i]
		if isSignature(root, c) {
			if i+1 < len(root.children) && root.children[i+1].kind == textNode {
				i++
			}
			continue
		}
		kept = append(kept, c)
	}
	sig := &node{kind: elemNode, name: "signature", children: []*node{{kind: textNode, text: b64}}}
	root.children = append(kept, sig)
}
