package searchdoor

import (
	"reflect"
	"strings"
	"sync"
	"testing"
)

// fakeBackend answers per module_nav command (argv[1]) and may advance the
// fake clock, so the CODE source's shared deadline is tested without sleeping
// (the Python goldens cannot freeze it: the reasons print the budget left).
type fakeBackend struct {
	clock    *float64
	handlers map[string]func(argv []string, timeout float64) Raw
	mu       sync.Mutex // the sources call Run concurrently
	calls    []string   // read once the search has returned
}

func (f *fakeBackend) Run(_ source, argv []string, timeout float64) Raw {
	f.mu.Lock()
	f.calls = append(f.calls, strings.Join(argv[1:], " "))
	f.mu.Unlock()
	if h, ok := f.handlers[argv[1]]; ok {
		return h(argv, timeout)
	}
	return Raw{State: Empty, Out: "0 matches", RC: 1, HasRC: true}
}

func newFakeDoor(handlers map[string]func([]string, float64) Raw) (*door, *fakeBackend, *float64) {
	clock := new(float64)
	fb := &fakeBackend{clock: clock, handlers: handlers}
	return &door{backend: fb, now: func() float64 { return *clock }}, fb, clock
}

// spend is a handler that uses the whole budget it was given and times out.
func spend(clock *float64) func([]string, float64) Raw {
	return func(_ []string, timeout float64) Raw {
		*clock += timeout
		return Raw{State: Unavailable, Reason: "unavailable: timed out after " + formatG3(timeout) + "s"}
	}
}

func codeOutcome(t *testing.T, d *door, query string) *Outcome {
	t.Helper()
	return guarded(d.codeJob("module_nav.py", query, 1, "SOURCE 3 · CODE", "testdata/course"))
}

func TestCodeBudgetSpentByTheFirstTokenLookup(t *testing.T) {
	handlers := map[string]func([]string, float64) Raw{}
	d, fb, clock := newFakeDoor(handlers)
	handlers["token"] = spend(clock)
	o := codeOutcome(t, d, "BSlowToken BOtherType alarm")
	wantNotes := []string{
		"token BSlowToken: unavailable: timed out after 1s; ranking the question",
		"token BOtherType: skipped, the 1s budget of this source is spent",
	}
	if !reflect.DeepEqual(o.Notes, wantNotes) {
		t.Errorf("notes = %q, want %q", o.Notes, wantNotes)
	}
	if o.State != Unavailable || o.Reason != "unavailable: the 1s budget was spent on token lookups before rank" {
		t.Errorf("code = %s %q", o.State, o.Reason)
	}
	course := o.Subs[0]
	if want := "unavailable: timeout, the CODE budget was spent before the course code (read 0 of 4 .java files)"; course.Reason != want {
		t.Errorf("course reason = %q, want %q", course.Reason, want)
	}
	if !reflect.DeepEqual(fb.calls, []string{"token BSlowToken -n 100000"}) {
		t.Errorf("backend calls = %q: no call may run past the deadline", fb.calls)
	}
}

func TestCodeBudgetSpentByRankBeforeTheDegradedGrep(t *testing.T) {
	handlers := map[string]func([]string, float64) Raw{}
	d, _, clock := newFakeDoor(handlers)
	handlers["rank"] = func(argv []string, timeout float64) Raw {
		*clock += timeout
		return Classify(2, "", "module_nav: no class index\n", 0)
	}
	o := codeOutcome(t, d, "alarm history")
	if o.Command != "module_nav grep '(?i)alarm|history' (DEGRADED)" ||
		o.Reason != "unavailable: the 1s budget was spent before the degraded grep" {
		t.Errorf("code = %q %q", o.Command, o.Reason)
	}
	if len(o.Notes) != 1 || !strings.HasPrefix(o.Notes[0], "DEGRADED: module_nav rank cannot search (error: module_nav: no class index (exit 2))") {
		t.Errorf("notes = %q", o.Notes)
	}
}

func TestCodeBudgetSpentByTheDriversProbe(t *testing.T) {
	handlers := map[string]func([]string, float64) Raw{}
	d, fb, clock := newFakeDoor(handlers)
	handlers["drivers"] = func(argv []string, timeout float64) Raw {
		*clock += timeout
		return Raw{State: OK, Out: "usage", RC: 0, HasRC: true}
	}
	o := codeOutcome(t, d, "list drivers")
	s := o.Subs[0]
	if !s.Lead || s.Title != "structure · driver anatomy" ||
		s.Reason != "unavailable: timeout, the CODE budget was spent before the structure command ran" {
		t.Errorf("structure = lead %v %q %q", s.Lead, s.Title, s.Reason)
	}
	if !reflect.DeepEqual(fb.calls, []string{"drivers --help"}) {
		t.Errorf("backend calls = %q", fb.calls)
	}
}

func TestCourseIndexRunsOutMidway(t *testing.T) {
	calls := 0
	d := &door{now: func() float64 {
		calls++
		if calls > 2 {
			return 10
		}
		return 0
	}}
	o := d.courseSearch("testdata/course", "alarm", 1)
	if want := "unavailable: timeout, the CODE budget ran out building the course index (read 2 of 4 .java files)"; o.Reason != want {
		t.Errorf("reason = %q, want %q", o.Reason, want)
	}
}

func TestTokenHitsLeadAndRankIsDeduplicated(t *testing.T) {
	handlers := map[string]func([]string, float64) Raw{
		"token": func(argv []string, _ float64) Raw {
			return Raw{State: OK, Out: "TOKEN: 'BFoo' (2)\n  BFoo  3  foo-rt\n  BUser  9  bar-rt", RC: 0, HasRC: true}
		},
		"rank": func(argv []string, _ float64) Raw {
			return Raw{State: OK, Out: "Rank 'q': 2 matching classes\n1\t2.000\tBBar\tbar-rt\tp\n2\t1.000\tBFoo\tfoo-rt\tp",
				RC: 0, HasRC: true}
		},
	}
	d, _, _ := newFakeDoor(handlers)
	o := guarded(d.codeJob("module_nav.py", "BFoo usage", 1, "SOURCE 3 · CODE", ""))
	var keys []string
	for _, it := range o.Items {
		keys = append(keys, it.Key)
	}
	if !reflect.DeepEqual(keys, []string{"BFoo", "BBar", "BUser"}) {
		t.Errorf("keys = %q, want the exact token class, then rank (deduplicated), then mentions", keys)
	}
	if o.Command != "module_nav token BFoo + module_nav rank" {
		t.Errorf("command = %q", o.Command)
	}
}

func TestAPanickingSourceIsTypedUnavailable(t *testing.T) {
	o := guarded(job{"docs", "SOURCE 2", "niagara_help find", func() *Outcome { panic("boom") }})
	if o.State != Unavailable || o.Key != "docs" ||
		o.Reason != "unavailable: the front door failed on this source: panic: boom" {
		t.Errorf("outcome = %+v", o)
	}
}

func TestReEscapeAndCodeRegex(t *testing.T) {
	if got := codeRegex([]string{"a.b", "c d", "x-y"}); got != `(?i)a\.b|c\ d|x\-y` {
		t.Errorf("codeRegex = %q", got)
	}
}
