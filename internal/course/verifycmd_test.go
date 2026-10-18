package course

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

const (
	goodStation    = "testdata/stations/good"
	fixtureInstall = "testdata/install"
)

// Without --date the suggestion lines carry today's date from the injected clock.
func TestVerifyPredictionsDefaultDate(t *testing.T) {
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Date(2031, 2, 3, 23, 30, 0, 0, time.Local) }
	var out, errb bytes.Buffer
	code := Main([]string{"--root", "testdata/predict-root", "verify-predictions", "--station", goodStation,
		"--install", fixtureInstall}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if code != 0 || !strings.Contains(out.String(), "L4.13: *(verified live 2031-02-03)*") {
		t.Errorf("exit %d, stdout:\n%s\nstderr: %s", code, out.String(), errb.String())
	}
}
