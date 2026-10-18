//go:build !unix

package procgroup

// runner is the process under test of the signal tests, which run on Unix
// only.
func runner() int { return 4 }
