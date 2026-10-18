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
	ss, err := OpenStations(checks, func(string) string { return goodStation }, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	if len(ss.list) != 1 || ss.Run(checks[1]).Verdict != Pass || ss.Scope() != "station GoodStation (237 components, 16 links)" {
		t.Errorf("stations %d, scope %q", len(ss.list), ss.Scope())
	}
	paths := map[string]string{RoleTraining: goodStation, RoleReview: "../testdata/stations/bad"}
	ss2, err := OpenStations(checks, func(r string) string { return paths[r] }, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ss2.Close()
	if r := ss2.Run(checks[1]); len(ss2.list) != 2 || r.Verdict != Fail {
		t.Errorf("stations %d, the RA1 check on the bad station: %s %s", len(ss2.list), r.Verdict, r.Evidence)
	}
	// The summary names every station read, in role order, comma-separated.
	if want := "station GoodStation (237 components, 16 links), station BadStation (236 components, 15 links)"; ss2.Scope() != want {
		t.Errorf("two-station scope %q, want %q", ss2.Scope(), want)
	}
	if _, err := OpenStations(checks, func(r string) string { return paths[r] + "x" }, "", "", ""); err == nil {
		t.Error("an unreadable station: no error")
	}
}

// Without --lab only the labs of the TrainingStation run; RA1 and RA2 run when named, in any case.
func TestSelectDefaultsToTheLabs(t *testing.T) {
	all, _ := Select(nil)
	for _, ck := range all {
		if Role(ck.Lab) != RoleTraining {
			t.Errorf("%s (%s) runs without --lab", ck.ID, ck.Lab)
		}
	}
	ra, unknown := Select([]string{"ra1", "RA2"})
	if unknown != "" || len(ra) == 0 {
		t.Fatalf("Select(ra1, RA2): %d checks, unknown %q", len(ra), unknown)
	}
	for _, ck := range ra {
		if ck.Lab != "RA1" && ck.Lab != "RA2" {
			t.Errorf("%s (%s) selected by --lab ra1 --lab RA2", ck.ID, ck.Lab)
		}
	}
	if _, unknown := Select([]string{"RA3"}); unknown != "RA3" {
		t.Errorf("Select(RA3): unknown %q, want RA3", unknown)
	}
}
