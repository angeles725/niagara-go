package course

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/check"
)

// The variables that stand in for --install and --station (blank counts as unset). The station
// variable is the one of the station role of each selected check: the labs run on the
// TrainingStation, RA1 and RA2 on the review station, EXAM on the exam station.
const installEnv = "COURSE_NIAGARA_INSTALL"

var stationEnv = map[string]string{
	check.RoleTraining: "COURSE_TRAINING_STATION",
	check.RoleReview:   "COURSE_REVIEW_STATION",
	check.RoleCert:     "COURSE_CERT_STATION",
}

func getenv(name string) string { return strings.TrimSpace(os.Getenv(name)) }

// stationOpts are the --station and --install of check, verify-predictions and verify.
type stationOpts struct {
	station    string
	hasStation bool
	install    string
	hasInstall bool
	// usage prints the subcommand's usage error and returns its exit (2).
	usage func(message string) int
}

// installDir is --install, else COURSE_NIAGARA_INSTALL, else "" (the jar checks SKIP).
func (o stationOpts) installDir() string {
	if o.hasInstall {
		return o.install
	}
	return getenv(installEnv)
}

// open reads the station of every role the checks run on: --station for all of them, else the
// role's variable. A role with neither is the usage error of a missing --station; a station
// that cannot be read is exit 2.
func (o stationOpts) open(s cli.Stdio, checks []*check.Check) (*check.Stations, int) {
	path := func(role string) string {
		if o.hasStation {
			return o.station
		}
		return getenv(stationEnv[role])
	}
	for _, r := range check.Roles(checks) {
		if path(r) == "" {
			return nil, o.usage("the following arguments are required: --station")
		}
	}
	ss, err := check.OpenStations(checks, path, o.installDir())
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return nil, cli.ExitCannotSearch
	}
	return ss, cli.ExitFound
}
