package help

import (
	"reflect"
	"testing"
)

func TestSuggestMatchesPython(t *testing.T) {
	names := []string{"BAlarmService", "BAlarmServices", "BAlarmSrvice", "BAlarmServise", "balarmservice", "BAlarm", "X"}
	want := []string{"BAlarmService", "balarmservice", "BAlarmSrvice", "BAlarmServices", "BAlarmServise"}
	if got := suggest(names, "BAlarmSevice", 5); !reflect.DeepEqual(got, want) {
		t.Errorf("suggest = %q, want %q", got, want)
	}
	if got := suggest(names, "BAlarmSevice", 2); !reflect.DeepEqual(got, want[:2]) {
		t.Errorf("suggest n=2 = %q", got)
	}
	sims := map[string]float64{"balarmservice": 0.9230769230769231, "balarmservices": 0.8571428571428572,
		"balarmsrvice": 0.9166666666666666, "balarm": 0, "x": 0}
	for name, want := range sims {
		if got := typoSimilarity("balarmsevice", name); got != want {
			t.Errorf("typoSimilarity(%q) = %v, want %v", name, got, want)
		}
	}
}
