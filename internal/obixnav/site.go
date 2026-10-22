package obixnav

import (
	"fmt"
	"strings"
)

// defrostKind is how a room's evaporators defrost, which decides what config expects of them.
type defrostKind int

const (
	noDefrost         defrostKind = iota // no per-evap defrost: wiring and config skip the room
	airDefrost                           // fan-only defrost
	resistanceDefrost                    // electric resistance, terminated on resistance temperature
)

// room is one PANCCADIA cold room: its number (CuartoN on the DashboardService facade), its
// Programacion ColdRoom container, and its defrost kind.
type room struct {
	num      int
	coldRoom string
	defrost  defrostKind
}

// site is every room obix-nav audits, in room order. Cuarto3 is the -20 C resistance room;
// Cuarto5's units have hasDefrost=false because its defrost is a kitControl sequence.
var site = []room{
	{1, "ColdRoom_1", airDefrost},
	{2, "ColdRoom_2", airDefrost},
	{3, "ColdRoom_3", resistanceDefrost},
	{4, "ColdRoom_4", airDefrost},
	{5, "ColdRoom_5", noDefrost},
}

func findRoom(num int) (room, bool) {
	for _, rm := range site {
		if rm.num == num {
			return rm, true
		}
	}
	return room{}, false
}

// siteRooms returns the room numbers of site, only those with a per-evap defrost when
// defrostOnly is set.
func siteRooms(defrostOnly bool) []int {
	var nums []int
	for _, rm := range site {
		if !defrostOnly || rm.defrost != noDefrost {
			nums = append(nums, rm.num)
		}
	}
	return nums
}

// roomsText names a sorted room list for an error message: "rooms 1-4" when contiguous, else
// "rooms 1, 3, 4".
func roomsText(nums []int) string {
	if len(nums) > 1 && nums[len(nums)-1]-nums[0] == len(nums)-1 {
		return fmt.Sprintf("rooms %d-%d", nums[0], nums[len(nums)-1])
	}
	s := make([]string, len(nums))
	for i, n := range nums {
		s[i] = fmt.Sprint(n)
	}
	return "rooms " + strings.Join(s, ", ")
}
