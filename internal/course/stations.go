package course

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/check"
)

// The variables that stand in for --install and --station (blank counts as unset). The station
// variable is the one of the station role of each selected check: the labs run on the
// TrainingStation, RA1 and RA2 on the review station, EXAM on the exam station.
const installEnv = "COURSE_NIAGARA_INSTALL"

// sourceEnv stands in for --source: a Niagara source tree, one directory per module part (the
// 4.15.6.20 sources of a niagara-research checkout: niagara-help/versions/4.15.6.20/source).
const sourceEnv = "COURSE_NIAGARA_SOURCE"

// labsrcEnv stands in for --labsrc: the participant's lab module sources the annotation checks read
// (the Niagara dev home holding training/ and trafficLightDriver/).
const labsrcEnv = "COURSE_LAB_SOURCES"

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
	source     string
	hasSource  bool
	labsrc     string
	hasLabsrc  bool
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

// anyStation reports whether a station was given at all: --station or one role's variable. A run
// reads at least one station, so verify checks it before the root and the --date (the usage error
// of a missing --station comes first, as when --station was required).
func (o stationOpts) anyStation() bool {
	if o.hasStation {
		return true
	}
	for _, env := range stationEnv {
		if getenv(env) != "" {
			return true
		}
	}
	return false
}

// missingStation is the usage error of no --station: with the groups of a role whose variable is
// unset, it names them and the variable ("EXAM needs --station or COURSE_CERT_STATION").
func (o stationOpts) missingStation(role string, checks []*check.Check) int {
	msg := "the following arguments are required: --station"
	if role != "" {
		label, verb := roleGroups(role, checks)
		msg += fmt.Sprintf(" (%s %s --station or %s)", label, verb, stationEnv[role])
	}
	return o.usage(msg)
}

// roleGroups names the groups of the checks that run on role: "EXAM", "RA1, RA2", "lab 4",
// "labs 1, 4", or "the labs" for more than three labs; and the verb that agrees with it.
func roleGroups(role string, checks []*check.Check) (string, string) {
	var labs []string
	for _, ck := range checks {
		if check.Role(ck.Lab) == role && !slices.Contains(labs, ck.Lab) {
			labs = append(labs, ck.Lab)
		}
	}
	label := strings.Join(labs, ", ")
	switch {
	case len(labs) == 0 || (role == check.RoleTraining && len(labs) > 3):
		return "the labs", "need" // no check (the station is still read) or most of the labs
	case role == check.RoleTraining && len(labs) == 1:
		label = "lab " + label
	case role == check.RoleTraining:
		label = "labs " + label
	}
	if len(labs) == 1 {
		return label, "needs"
	}
	return label, "need"
}

// sourceDir is --source, else COURSE_NIAGARA_SOURCE, else "" (the source-text checks SKIP).
func (o stationOpts) sourceDir() string {
	if o.hasSource {
		return o.source
	}
	return getenv(sourceEnv)
}

// labsrcDir is --labsrc, else COURSE_LAB_SOURCES, else "" (the annotation checks SKIP).
func (o stationOpts) labsrcDir() string {
	if o.hasLabsrc {
		return o.labsrc
	}
	return getenv(labsrcEnv)
}

// scope is the summary's "station ..., install <dir>", with ", source <dir>" when a check reads the
// source tree and ", labsrc <dir>" when one reads the lab sources (an annotation row, or a files or file-text row over them).
func (o stationOpts) scope(ss *check.Stations, checks []*check.Check) string {
	out := fmt.Sprintf("%s, install %s", ss.Scope(), orNone(o.installDir()))
	if check.NeedsSource(checks) {
		out += ", source " + orNoSource(o.sourceDir())
	}
	if check.NeedsLabSources(checks) {
		out += ", labsrc " + orNoLabsrc(o.labsrcDir())
	}
	return out
}

func orNoLabsrc(s string) string {
	if s == "" {
		return "(none: lab-source checks skipped)"
	}
	return s
}

func orNoSource(s string) string {
	if s == "" {
		return "(none: source checks skipped)"
	}
	return s
}

// open reads the station of every role the checks run on: --station for all of them, else the
// role's variable. A role with neither is the usage error of a missing --station, naming the
// role's groups and variable; a station that cannot be read is exit 2.
func (o stationOpts) open(s cli.Stdio, checks []*check.Check) (*check.Stations, int) {
	path := func(role string) string {
		if o.hasStation {
			return o.station
		}
		return getenv(stationEnv[role])
	}
	for _, r := range check.Roles(checks) {
		if path(r) == "" {
			return nil, o.missingStation(r, checks)
		}
	}
	ss, err := check.OpenStations(checks, path, o.installDir(), o.sourceDir(), o.labsrcDir())
	if err != nil {
		fmt.Fprintf(s.Err, "course: cannot check the station: %v\n", err)
		return nil, cli.ExitCannotSearch
	}
	return ss, cli.ExitFound
}
