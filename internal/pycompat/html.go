package pycompat

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pyhtml"
)

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;")

// HTMLEscape is html.escape(s, quote=True).
func HTMLEscape(s string) string { return htmlEscaper.Replace(s) }

// HTMLUnescape is html.unescape: it forwards to pyhtml.Unescape, the home of the html
// module family (decision D37), for the callers that still name it here.
func HTMLUnescape(s string) string { return pyhtml.Unescape(s) }
