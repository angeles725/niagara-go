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

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// verify-predictions is verify --status predicted: the same stdout, stderr and exit.
func TestVerifyPredictionsIsVerifyStatusPredicted(t *testing.T) {
	t.Setenv(stationEnv["training"], "")
	t.Setenv(installEnv, "")
	for _, tail := range [][]string{
		{"--station", goodStation, "--install", fixtureInstall, "--date", "2026-10-17"},
		{"--station", "testdata/stations/bad", "--date", "2026-10-17", "--json"},
		{"--station", "testdata/stations/broken", "--date", "2026-10-17"},
		{"--station", goodStation, "--date", "17/10/2026"},
	} {
		old := append([]string{"--root", "testdata/predict-root", "verify-predictions"}, tail...)
		gen := append([]string{"--root", "testdata/predict-root", "verify", "--status", "predicted"}, tail...)
		c1, o1, e1 := run(old...)
		c2, o2, e2 := run(gen...)
		if c1 != c2 || o1 != o2 || e1 != e2 {
			t.Errorf("%v:\nverify-predictions exit %d\n%s%s\nverify --status predicted exit %d\n%s%s", tail, c1, o1, e1, c2, o2, e2)
		}
	}
}
