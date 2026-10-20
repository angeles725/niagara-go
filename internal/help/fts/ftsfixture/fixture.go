// Package ftsfixture installs the full-text test checkout for the help,
// scope and searchdoor tests (rawfixture holds the copy itself; fts's own
// tests use that one, since this package imports fts). On Windows the
// checkout's indexes get the fingerprint fts computes there: the stored ones
// were built on Linux, and NTFS keeps mtimes in 100 ns ticks and spells the
// source directories with "\", so they would always be judged stale.
package ftsfixture

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture/rawfixture"
)

// FixedNS is the retired fulltext_gen.py's FIXED_NS, the mtime of every fixture file.
const FixedNS = rawfixture.FixedNS

// Root is the placeholder for the checkout folder in the stored fingerprints
// and in the recorded outputs.
const Root = rawfixture.Root

// Versions maps each fixture version to its data root inside the checkout.
var Versions = rawfixture.Versions

// HostFingerprint is the fingerprint fts computes for the data root base.
func HostFingerprint(base string) string { return fts.Fingerprint(fts.SourceDirs(base)) }

// Checkout is rawfixture.Checkout with, on Windows, each installed index's
// stored fingerprint rewritten to HostFingerprint, so a fresh checkout's
// indexes are up to date there as they are on Linux.
func Checkout(t testing.TB, testdata string, withoutIndexes bool) string {
	t.Helper()
	checkout := rawfixture.Checkout(t, testdata, withoutIndexes)
	if !withoutIndexes {
		rawfixture.RefreshAll(t, checkout, HostFingerprint)
	}
	return checkout
}

// Install copies a fixture database to dest with <ROOT> in its stored
// fingerprint replaced by root (rawfixture.Install); rawfixture.Refresh makes
// it up to date on Windows.
func Install(t testing.TB, fixture, dest, root string) {
	t.Helper()
	rawfixture.Install(t, fixture, dest, root)
}

// SetFingerprint rewrites the stored fingerprint of db: to value, or (value
// empty) with old replaced by new.
func SetFingerprint(t testing.TB, db, value, old, new string) {
	t.Helper()
	rawfixture.SetFingerprint(t, db, value, old, new)
}

// rootPath is Root and the "/"-joined rest of a path in a recorded output.
var rootPath = regexp.MustCompile(regexp.QuoteMeta(Root) + `(/[^\s)'"]*)`)

// AtRoot is an output recorded on Linux with Root replaced by checkout. Python
// builds the path after it with os.path.join, so on Windows its "/" become
// "\" (Root/indexes/fulltext.sqlite is checkout\indexes\fulltext.sqlite).
func AtRoot(recorded, checkout string) string {
	s := rootPath.ReplaceAllStringFunc(recorded, func(m string) string {
		return checkout + filepath.FromSlash(m[len(Root):])
	})
	return strings.ReplaceAll(s, Root, checkout)
}
