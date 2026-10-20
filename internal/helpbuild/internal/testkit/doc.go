// Package testkit is the test harness the helpbuild packages share: the
// frozen-expectation checks (CheckFrozen and its -update-frozen flag, read
// from the testdata/frozen of the package under test), file and tree helpers
// (WriteFile, TreeLines, CopyTree, ...) and the pyjson value shorthands the
// frozen observations are written with. Only tests import it.
package testkit
