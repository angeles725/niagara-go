package pycompat

import "strings"

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;")

// HTMLEscape is html.escape(s, quote=True).
func HTMLEscape(s string) string { return htmlEscaper.Replace(s) }
