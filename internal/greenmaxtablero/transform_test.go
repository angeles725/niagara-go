package greenmaxtablero

import (
	"strings"
	"testing"
)

func TestCheckWellFormed(t *testing.T) {
	for doc, ok := range map[string]bool{
		`<?xml version="1.0"?><!-- c --><a><b/></a>`: true,
		"<a/>\n<!-- trailing -->\n":                  true,
		"<a/><b/>":                                   false,
		"<a></a>junk":                                false,
		"<a><b></a>":                                 false,
		"<a>&nbsp;</a>":                              false,
		"<!-- only a comment -->":                    false,
		"<a>":                                        false,
	} {
		if err := checkWellFormed(doc); (err == nil) != ok {
			t.Errorf("checkWellFormed(%q) = %v, want ok=%v", doc, err, ok)
		}
	}
}

func TestBadCoordinateIsARefusal(t *testing.T) {
	px := `<CanvasPane><Picture layout="1.2.3,4.0,60.0,30.0"><ValueBinding ord="slot:Relay$5b1$5d$2eBO"><IBooleanToSimple/></ValueBinding></Picture></CanvasPane>`
	_, _, _, err := transform(px, "GM1", false)
	if err == nil || !strings.Contains(err.Error(), "could not convert string to float: '1.2.3'") {
		t.Errorf("err = %v", err)
	}
}

func TestStatsRepr(t *testing.T) {
	s := stats{circuits: 2, missing: []int{}, bumped: &[2]float64{600, 702}}
	if got := s.String(); !strings.HasSuffix(got, "view_bumped=(600.0, 702)") || !strings.Contains(got, "missing=[]") {
		t.Errorf("stats = %q", got)
	}
}
