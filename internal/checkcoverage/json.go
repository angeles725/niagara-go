package checkcoverage

import (
	"fmt"
	"strings"
)

// jsonString is json.dumps of a str with ensure_ascii=False.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// jsonReport is the --json document, json.dumps(..., indent=2, ensure_ascii=False).
func jsonReport(results map[string]*coverage, order []string, nfiles int, covered bool) string {
	var b strings.Builder
	hits := func(name string, hs []hit, last bool) {
		fmt.Fprintf(&b, "      %s: ", jsonString(name))
		if len(hs) == 0 {
			b.WriteString("[]")
		} else {
			b.WriteString("[\n")
			for i, h := range hs {
				fmt.Fprintf(&b, "        {\n          \"block\": %s,\n          \"h1\": %s\n        }",
					jsonString(h.block), jsonString(h.h1))
				if i < len(hs)-1 {
					b.WriteString(",")
				}
				b.WriteString("\n")
			}
			b.WriteString("      ]")
		}
		if !last {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "{\n  \"scanned_blocks\": %d,\n  \"names\": {\n", nfiles)
	for i, n := range order {
		fmt.Fprintf(&b, "    %s: {\n", jsonString(n))
		hits("subject", results[n].subject, false)
		hits("body", results[n].body, true)
		b.WriteString("    }")
		if i < len(order)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	verdict := "clear"
	if covered {
		verdict = "prior-coverage-found"
	}
	fmt.Fprintf(&b, "  },\n  \"verdict\": %s\n}", jsonString(verdict))
	return b.String()
}
