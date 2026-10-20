// Package sizegate holds only a test: the 500-line source limit
// (TOOLING-GUIDELINES principle 6). It counts every .go file outside testdata/
// (Go files there are fixtures) and every .py file anywhere, testdata/
// included, so the remaining Python test helpers stay within the limit too.
//
// allowlist.txt lists the exceptions as "<repo-relative path> <max lines>",
// each preceded by a "#" line giving its reason. Entries may only be lowered or
// removed; the test fails on an entry above the file's size or matching no
// file. The one kind of new entry accepted is a frozen reference generator that
// D39(d) moves to the pre-go-retire tag: it is not worth rewriting.
package sizegate
