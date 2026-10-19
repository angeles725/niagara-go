package navbuild

import "regexp"

// The members of an index's _meta block that differ between two builds of
// the same corpus: when it was built, how long it took, and the absolute
// corpus path it was built from.
var volatileMeta = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`("timestamp":\s*)"[^"]*"`), `${1}"<TIMESTAMP>"`},
	{regexp.MustCompile(`("build_time_sec":\s*)-?[0-9][0-9.eE+-]*`), `${1}0`},
	{regexp.MustCompile(`("source(?:_root)?":\s*)"(?:[^"\\]|\\.)*"`), `${1}"<SOURCE>"`},
}

// NormalizeVolatile, a helper of the golden tests, returns doc with the
// volatile members of its leading "_meta" object replaced by fixed
// placeholders (timestamp -> "<TIMESTAMP>", whatever its format;
// build_time_sec -> 0; source and source_root -> "<SOURCE>"), so an index
// built at any time or place compares byte for byte with its golden. Only the _meta
// object is touched; a document that does not start with one is returned
// unchanged.
func NormalizeVolatile(doc []byte) []byte {
	start, end, ok := metaSpan(doc)
	if !ok {
		return doc
	}
	meta := doc[start:end]
	for _, v := range volatileMeta {
		meta = v.re.ReplaceAll(meta, []byte(v.repl))
	}
	out := make([]byte, 0, len(doc))
	out = append(out, doc[:start]...)
	out = append(out, meta...)
	return append(out, doc[end:]...)
}

// metaSpan is the byte range of the object value of a leading "_meta" key.
func metaSpan(doc []byte) (start, end int, ok bool) {
	i := skipWS(doc, 0)
	if i >= len(doc) || doc[i] != '{' {
		return 0, 0, false
	}
	i = skipWS(doc, i+1)
	const key = `"_meta"`
	if len(doc)-i < len(key) || string(doc[i:i+len(key)]) != key {
		return 0, 0, false
	}
	i = skipWS(doc, i+len(key))
	if i >= len(doc) || doc[i] != ':' {
		return 0, 0, false
	}
	start = skipWS(doc, i+1)
	if start >= len(doc) || doc[start] != '{' {
		return 0, 0, false
	}
	depth := 0
	for j := start; j < len(doc); j++ {
		switch doc[j] {
		case '"':
			for j++; j < len(doc) && doc[j] != '"'; j++ {
				if doc[j] == '\\' {
					j++
				}
			}
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return start, j + 1, true
			}
		}
	}
	return 0, 0, false
}

func skipWS(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}
