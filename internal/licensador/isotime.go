package licensador

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// pyTime is a Python datetime: an instant plus whether it is aware and its offset.
type pyTime struct {
	T      time.Time // the wall clock in the given offset (or naive wall clock as UTC)
	Aware  bool
	Offset int // seconds east of UTC (aware only)
}

var errNaiveAware = errors.New("can't compare offset-naive and offset-aware datetimes")

// cmp mirrors datetime ordering: -1, 0, 1, or the TypeError Python raises.
func (a pyTime) cmp(b pyTime) (int, error) {
	if a.Aware != b.Aware {
		return 0, errNaiveAware
	}
	return a.T.Compare(b.T), nil
}

func (a pyTime) add(d time.Duration) pyTime { a.T = a.T.Add(d); return a }

func awareUTC(t time.Time) pyTime { return pyTime{T: t.UTC(), Aware: true} }

// isoformat mirrors datetime.isoformat().
func (a pyTime) isoformat() string {
	local := a.T.In(time.FixedZone("", a.Offset))
	s := local.Format("2006-01-02T15:04:05")
	if us := local.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	if !a.Aware {
		return s
	}
	off, sign := a.Offset, '+'
	if off < 0 {
		off, sign = -off, '-'
	}
	s += fmt.Sprintf("%c%02d:%02d", sign, off/3600, off/60%60)
	if off%60 != 0 {
		s += fmt.Sprintf(":%02d", off%60)
	}
	return s
}

func (a pyTime) dropMicros() pyTime { a.T = a.T.Truncate(time.Second); return a }

func atoiN(s string, n int) (int, bool) {
	if len(s) != n {
		return 0, false
	}
	for i := 0; i < n; i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	v, _ := strconv.Atoi(s)
	return v, true
}

// parseClock parses HH[:MM[:SS[.f+]]] or the basic HH[MM[SS[.f+]]] form.
func parseClock(s string) (h, m, sec, us int, ok bool) {
	frac := ""
	if i := strings.IndexAny(s, ".,"); i >= 0 {
		s, frac = s[:i], s[i+1:]
		if frac == "" {
			return 0, 0, 0, 0, false
		}
		for _, c := range frac {
			if c < '0' || c > '9' {
				return 0, 0, 0, 0, false
			}
		}
		frac = (frac + "000000")[:6]
		us, _ = strconv.Atoi(frac)
	}
	parts := []string{s}
	if strings.Contains(s, ":") {
		parts = strings.Split(s, ":")
	} else {
		for len(parts[len(parts)-1]) > 2 {
			last := parts[len(parts)-1]
			parts = append(parts[:len(parts)-1], last[:2], last[2:])
		}
	}
	if len(parts) > 3 || (frac != "" && len(parts) != 3) {
		return 0, 0, 0, 0, false
	}
	vals := []int{0, 0, 0}
	for i, p := range parts {
		v, good := atoiN(p, 2)
		if !good {
			return 0, 0, 0, 0, false
		}
		vals[i] = v
	}
	h, m, sec = vals[0], vals[1], vals[2]
	return h, m, sec, us, h < 24 && m < 60 && sec < 60
}

// parseISO mirrors datetime.fromisoformat for the calendar-date forms
// (YYYY-MM-DD / YYYYMMDD, any one-character separator, Z or +-HH[:MM[:SS]] offsets).
func parseISO(s string) (pyTime, error) {
	bad := fmt.Errorf("Invalid isoformat string: %s", pyjson.StrRepr(s))
	var y, mo, d int
	var ok bool
	rest := ""
	if wd, rest, ok := parseWeekDate(s); ok {
		return parseISOClock(pyTime{T: wd}, rest, bad)
	}
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		y, ok = atoiN(s[0:4], 4)
		mo, _ = atoiN(s[5:7], 2)
		d, _ = atoiN(s[8:10], 2)
		rest = s[10:]
	} else if len(s) >= 8 {
		y, ok = atoiN(s[0:4], 4)
		mo, _ = atoiN(s[4:6], 2)
		d, _ = atoiN(s[6:8], 2)
		rest = s[8:]
	}
	if !ok || y < 1 || mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) {
		return pyTime{}, bad
	}
	return parseISOClock(pyTime{T: time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)}, rest, bad)
}

// parseWeekDate parses the ISO week forms YYYY-Www[-D] and YYYYWww[D].
func parseWeekDate(s string) (time.Time, string, bool) {
	var y, w, d int
	var ok bool
	rest := ""
	switch {
	case len(s) >= 8 && s[4:6] == "-W":
		y, ok = atoiN(s[:4], 4)
		w, _ = atoiN(s[6:8], 2)
		d, rest = 1, s[8:]
		if len(s) >= 10 && s[8] == '-' && s[9] >= '1' && s[9] <= '7' {
			d, rest = int(s[9]-'0'), s[10:]
		}
	case len(s) >= 7 && s[4] == 'W':
		y, ok = atoiN(s[:4], 4)
		w, _ = atoiN(s[5:7], 2)
		d, rest = 1, s[7:]
		if len(s) >= 8 && s[7] >= '1' && s[7] <= '7' {
			d, rest = int(s[7]-'0'), s[8:]
		}
	default:
		return time.Time{}, "", false
	}
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC)
	monday := jan4.AddDate(0, 0, -((int(jan4.Weekday()) + 6) % 7))
	t := monday.AddDate(0, 0, (w-1)*7+d-1)
	if _, lastWeek := time.Date(y, 12, 28, 0, 0, 0, 0, time.UTC).ISOWeek(); !ok || y < 1 || w < 1 || w > lastWeek {
		return time.Time{}, "", false
	}
	return t, rest, true
}

func parseISOClock(out pyTime, rest string, bad error) (pyTime, error) {
	if rest == "" {
		return out, nil
	}
	rs := []rune(rest)
	if len(rs) < 2 {
		return pyTime{}, bad
	}
	clock := string(rs[1:])
	tz := ""
	if i := strings.IndexAny(clock, "Z+-"); i >= 0 {
		clock, tz = clock[:i], clock[i:]
	}
	h, m, sec, us, good := parseClock(clock)
	if !good {
		return pyTime{}, bad
	}
	out.T = out.T.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second + time.Duration(us)*time.Microsecond)
	if tz == "" {
		return out, nil
	}
	off := 0
	if tz != "Z" {
		oh, om, osec, ous, good := parseClock(tz[1:])
		if !good || ous != 0 {
			return pyTime{}, bad
		}
		off = oh*3600 + om*60 + osec
		if tz[0] == '-' {
			off = -off
		}
	}
	out.Aware, out.Offset = true, off
	out.T = out.T.Add(-time.Duration(off) * time.Second)
	return out, nil
}

func daysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
