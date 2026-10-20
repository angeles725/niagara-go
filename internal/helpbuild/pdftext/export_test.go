package pdftext

import "github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"

// Helpers only the tests use; production code has no caller for them.

// Stdout is the text the run printed (testkit.PDFStdout of the Log).
func (r *PDFResult) Stdout() string { return testkit.PDFStdout(r.Log) }
