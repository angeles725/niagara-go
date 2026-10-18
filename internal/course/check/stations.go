package check

import "strings"

// Station roles: the station a check group runs on.
const (
	RoleTraining = "training" // the labs (1-11, B): the course TrainingStation
	RoleReview   = "review"   // RA1, RA2: the review-assignment station
	RoleCert     = "cert"     // EXAM: the certification-exam station
)

// Role is the station role of a lab group ("3", "B", "RA1", "EXAM", ...).
func Role(lab string) string {
	switch {
	case strings.HasPrefix(lab, "RA"):
		return RoleReview
	case lab == "EXAM":
		return RoleCert
	}
	return RoleTraining
}

// Roles lists the roles the checks run on, in first-use order; with no check, the training
// station (a run always reads one station, so an unreadable one is still an error).
func Roles(checks []*Check) []string {
	var roles []string
	seen := map[string]bool{}
	for _, ck := range checks {
		if r := Role(ck.Lab); !seen[r] {
			seen[r] = true
			roles = append(roles, r)
		}
	}
	if len(roles) == 0 {
		roles = []string{RoleTraining}
	}
	return roles
}

// Stations are the stations one run reads: one per path, each read once, with nested values when
// one of its checks reads a dotted slot.
type Stations struct {
	byRole map[string]*Station
	list   []*Station
}

// OpenStations opens the station of every role the checks need; path maps a role to its station
// directory or bog. The error is the bog reader's message of the first station that cannot be read.
func OpenStations(checks []*Check, path func(role string) string, install string) (*Stations, error) {
	perPath := map[string][]*Check{}
	var paths []string
	rolePath := map[string]string{}
	for _, r := range Roles(checks) {
		p := path(r)
		rolePath[r] = p
		if _, ok := perPath[p]; !ok {
			perPath[p] = nil
			paths = append(paths, p)
		}
	}
	for _, ck := range checks {
		p := rolePath[Role(ck.Lab)]
		perPath[p] = append(perPath[p], ck)
	}
	ss := &Stations{byRole: map[string]*Station{}}
	byPath := map[string]*Station{}
	for _, p := range paths {
		st, err := Open(p, install, NeedsNested(perPath[p]))
		if err != nil {
			ss.Close()
			return nil, err
		}
		byPath[p] = st
		ss.list = append(ss.list, st)
	}
	for r, p := range rolePath {
		ss.byRole[r] = byPath[p]
	}
	return ss, nil
}

// Run runs a check on the station of its role.
func (ss *Stations) Run(ck *Check) Result { return ss.byRole[Role(ck.Lab)].Run(ck) }

// Scope is the Scope of every station read, comma-separated.
func (ss *Stations) Scope() string {
	parts := make([]string, len(ss.list))
	for i, st := range ss.list {
		parts[i] = st.Scope()
	}
	return strings.Join(parts, ", ")
}

// Close releases the jars of every station.
func (ss *Stations) Close() {
	for _, st := range ss.list {
		st.Close()
	}
}
