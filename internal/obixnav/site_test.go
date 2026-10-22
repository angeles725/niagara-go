package obixnav

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// The derived room lists and range texts are the values the commands used to hard-code.
func TestSiteDerivedRoomLists(t *testing.T) {
	cases := []struct {
		name  string
		nums  []int
		want  []int
		label string
	}{
		{"every room (map, dump)", siteRooms(false), []int{1, 2, 3, 4, 5}, "rooms 1-5"},
		{"per-evap defrost rooms (wiring, config)", siteRooms(true), []int{1, 2, 3, 4}, "rooms 1-4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !reflect.DeepEqual(c.nums, c.want) {
				t.Errorf("rooms %v, want %v", c.nums, c.want)
			}
			if got := roomsText(c.nums); got != c.label {
				t.Errorf("roomsText = %q, want %q", got, c.label)
			}
		})
	}
}

func TestRoomsTextListsAGappedSet(t *testing.T) {
	if got := roomsText([]int{1, 3, 4}); got != "rooms 1, 3, 4" {
		t.Errorf("roomsText = %q", got)
	}
}

func TestFindRoom(t *testing.T) {
	cases := []struct {
		num      int
		ok       bool
		coldRoom string
		defrost  defrostKind
	}{
		{1, true, "ColdRoom_1", airDefrost},
		{3, true, "ColdRoom_3", resistanceDefrost},
		{5, true, "ColdRoom_5", noDefrost},
		{0, false, "", noDefrost},
		{7, false, "", noDefrost},
	}
	for _, c := range cases {
		rm, ok := findRoom(c.num)
		if ok != c.ok || rm.coldRoom != c.coldRoom || rm.defrost != c.defrost {
			t.Errorf("findRoom(%d) = %+v, %v", c.num, rm, ok)
		}
	}
}

// The resistance-room warnings name the audited room, which today is Cuarto3.
func TestDefrostFlagsNameTheResistanceRoom(t *testing.T) {
	rm, _ := findRoom(3)
	gd := func(string) *string { return nil }
	flags, err := defrostFlags(gd, value{kind: 'f', f: 0}, rm)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"terminateOnResistanceTemp=None (Cuarto3 debe true)", "resistanceTempThreshold=0 (Cuarto3 lo necesita)"}
	if !reflect.DeepEqual(flags, want) {
		t.Errorf("flags %q, want %q", flags, want)
	}
}

func TestPassFileDefaultsToTheEnvironment(t *testing.T) {
	cases := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"env var", "/run/secrets/obix", []string{"cond"}, "/run/secrets/obix"},
		{"flag wins over env", "/run/secrets/obix", []string{"--pass-file", "p.txt", "cond"}, "p.txt"},
		{"neither", "", []string{"cond"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]string{PassFileEnv: c.env}
			var out, errb bytes.Buffer
			o, _, done := parse(c.args, cli.Stdio{Out: &out, Err: &errb, Getenv: func(k string) string { return env[k] }})
			if done || o.PassFile != c.want {
				t.Errorf("PassFile %q (done %v %s), want %q", o.PassFile, done, errb.String(), c.want)
			}
		})
	}
}

func TestNoPassFileGivenNamesTheWays(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{Command: "cond"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || !strings.Contains(errb.String(), "--pass-file") || !strings.Contains(errb.String(), PassFileEnv) {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}
