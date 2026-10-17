//go:build !linux

package argparse

// stdoutColumns is not implemented off Linux: without $COLUMNS the width is 80.
func stdoutColumns() int { return 0 }
