package navbuild

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestMain lets runGoChild re-run this test binary as one Go builder, so the
// full-corpus parity run measures each builder's own peak RSS.
func TestMain(m *testing.M) {
	if index := os.Getenv("NAVBUILD_CHILD_INDEX"); index != "" {
		os.Exit(childBuild(index, os.Getenv("NAVBUILD_CHILD_BASE"), os.Getenv("NAVBUILD_CHILD_ORGANIZED")))
	}
	os.Exit(m.Run())
}

func childBuild(index, base, organized string) int {
	for _, b := range laterBuilders {
		if b.index == index {
			if err := b.build(base, organized); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
	}
	fmt.Fprintln(os.Stderr, "no later builder writes", index)
	return 2
}

// runGoChild builds index into base in a child process and logs its wall time
// and peak RSS.
func runGoChild(t *testing.T, index, base, organized string) {
	t.Helper()
	cmd := measured(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "NAVBUILD_CHILD_INDEX="+index,
		"NAVBUILD_CHILD_BASE="+base, "NAVBUILD_CHILD_ORGANIZED="+organized)
	t0 := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", index, err, out)
	}
	t.Logf("go %s: %v, peak RSS %s", index, time.Since(t0).Round(time.Millisecond), peakRSS(out))
}
