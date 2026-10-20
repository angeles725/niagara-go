package pdftext

import (
	"strings"
)

// Helpers only the tests use; production code has no caller for them.

// Stdout is the text the run printed: the Log entries in order, each
// followed by a newline except a progress entry (one starting with "\r",
// written without a newline).
func (r *PDFResult) Stdout() string {
	var b strings.Builder
	for _, e := range r.Log {
		b.WriteString(e)
		if !strings.HasPrefix(e, "\r") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
