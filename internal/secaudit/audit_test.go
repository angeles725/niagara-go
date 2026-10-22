package secaudit

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/procgroup"
)

// An interrupt during SEC-11's "keytool -list -v", after SEC-02's plain listing ran, still ends
// the audit with the keytool error: no later check runs.
func TestInterruptInSec11StopsTheAudit(t *testing.T) {
	e := &env{
		run: func(argv []string, _ []byte, _ time.Duration, _ ...string) (result, error) {
			if slices.Contains(argv, "-v") {
				return result{}, &procgroup.InterruptError{Signal: os.Interrupt}
			}
			return result{}, nil
		},
		dial:     func(string, int) bool { return false },
		lookPath: func(string) string { return "" },
		winpath:  func(string) string { return "" },
	}
	rep := &report{}
	err := e.audit(homeWithTruststore(t), "", "", nil, rep)
	var in *procgroup.InterruptError
	if !errors.As(err, &in) || !strings.HasPrefix(err.Error(), "keytool: ") {
		t.Errorf("audit = %v, want the keytool *procgroup.InterruptError", err)
	}
	if n := len(rep.rows); n == 0 || rep.rows[n-1].id != "SEC-10" {
		t.Errorf("audit rows %v, want them to end at SEC-10, the check before SEC-11", rep.rows)
	}
}
