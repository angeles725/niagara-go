package pycompat

import (
	"math"
	"testing"
)

func TestCommas(t *testing.T) {
	for _, tt := range []struct {
		n    int64
		want string
	}{
		{0, "0"}, {7, "7"}, {999, "999"}, {1000, "1,000"}, {6310, "6,310"}, {-999, "-999"},
		{-1000, "-1,000"}, {-1234567, "-1,234,567"}, {math.MinInt64, "-9,223,372,036,854,775,808"},
	} {
		if got := Commas(tt.n); got != tt.want {
			t.Errorf("Commas(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
	if got := Commas(uint64(math.MaxUint64)); got != "18,446,744,073,709,551,615" {
		t.Errorf("Commas(MaxUint64) = %q", got)
	}
	type count int
	if got := Commas(count(1234)) + Commas(int8(-128)) + Commas(uint8(255)); got != "1,234-128255" {
		t.Errorf("Commas of named and small kinds = %q", got)
	}
}
