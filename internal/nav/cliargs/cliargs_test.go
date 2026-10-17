package cliargs

import (
	"errors"
	"math/big"
	"testing"

	"github.com/angeles725/niagara-go/internal/argparse"
)

func TestPositiveInt(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr string
	}{
		{"3", 3, ""},
		{" 7 ", 7, ""},
		{"0", 0, "must be a positive integer, got '0'"},
		{"-2", 0, "must be a positive integer, got '-2'"},
		{"x", 0, "must be a positive integer, got 'x'"},
		{"it's", 0, `must be a positive integer, got "it's"`},
		{"1_000", 1000, ""},
	}
	for _, tt := range tests {
		v, err := PositiveInt.Convert(tt.in)
		if tt.wantErr == "" {
			if err != nil || v != tt.want {
				t.Errorf("%q: got %v, %v", tt.in, v, err)
			}
			continue
		}
		var te *argparse.TypeError
		if !errors.As(err, &te) || te.Msg != tt.wantErr {
			t.Errorf("%q: error %v, want %q", tt.in, err, tt.wantErr)
		}
	}
}

func TestPositiveIntBeyondInt64(t *testing.T) {
	v, err := PositiveInt.Convert("123456789012345678901234567890")
	if b, ok := v.(*big.Int); err != nil || !ok || b.String() != "123456789012345678901234567890" {
		t.Errorf("got %v (%T), %v", v, v, err)
	}
}
