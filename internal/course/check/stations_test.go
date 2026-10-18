package check

import (
	"slices"
	"testing"
)

func TestRoles(t *testing.T) {
	for lab, want := range map[string]string{"1": RoleTraining, "11": RoleTraining, "B": RoleTraining,
		"RA1": RoleReview, "RA2": RoleReview, "EXAM": RoleCert} {
		if got := Role(lab); got != want {
			t.Errorf("Role(%q) = %s, want %s", lab, got, want)
		}
	}
	checks := []*Check{{Lab: "EXAM"}, {Lab: "4"}, {Lab: "RA1"}, {Lab: "EXAM"}}
	if got := Roles(checks); !slices.Equal(got, []string{RoleCert, RoleTraining, RoleReview}) {
		t.Errorf("Roles = %v", got)
	}
	if got := Roles(nil); !slices.Equal(got, []string{RoleTraining}) {
		t.Errorf("Roles(nil) = %v, want the training station", got)
	}
}

// Two roles on one path read the station once; a role's station is the one of its path.
func TestOpenStationsSharesAPath(t *testing.T) {
	checks := []*Check{&Spec[0], {ID: "CRA1.x", Lab: "RA1", Kind: kindType, Type: "training:FirstComponent", Op: opMin, N: 1}}
	ss, err := OpenStations(checks, func(string) string { return goodStation }, "")
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	if len(ss.list) != 1 || ss.Run(checks[1]).Verdict != Pass || ss.Scope() != "station GoodStation (237 components, 16 links)" {
		t.Errorf("stations %d, scope %q", len(ss.list), ss.Scope())
	}
	paths := map[string]string{RoleTraining: goodStation, RoleReview: "../testdata/stations/bad"}
	ss2, err := OpenStations(checks, func(r string) string { return paths[r] }, "")
	if err != nil {
		t.Fatal(err)
	}
	defer ss2.Close()
	if r := ss2.Run(checks[1]); len(ss2.list) != 2 || r.Verdict != Fail {
		t.Errorf("stations %d, the RA1 check on the bad station: %s %s", len(ss2.list), r.Verdict, r.Evidence)
	}
	if _, err := OpenStations(checks, func(r string) string { return paths[r] + "x" }, ""); err == nil {
		t.Error("an unreadable station: no error")
	}
}
