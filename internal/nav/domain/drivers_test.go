package domain

import "testing"

// TestSimple pins drivers._simple (values printed by Python).
func TestSimple(t *testing.T) {
	for in, want := range map[string]string{" javax.baja.driver.BDeviceNetwork<X, Y> ": "BDeviceNetwork",
		"": "", "Plain": "Plain"} {
		if got := simple(in); got != want {
			t.Errorf("simple(%q) = %q, Python %q", in, got, want)
		}
	}
}
