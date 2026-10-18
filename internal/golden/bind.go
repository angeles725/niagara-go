package golden

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TmpPlaceholder in cmd.json args or env values is replaced by a fresh
// temporary directory per case; the same directory printed by the command is
// folded back to the placeholder before comparing with stdout/stderr.
const TmpPlaceholder = "{tmp}"

// MachineIDFile is read for Requires.MachineIDSHA256 (overridable in tests).
var MachineIDFile = "/etc/machine-id"

// hostGOOS is the operating system Requires.GOOS is checked against
// (overridable in tests).
var hostGOOS = runtime.GOOS

// knownGOOS are the GOOS values go/build knows; a name outside the list is a
// typo that would skip the case everywhere, so loading the case fails.
var knownGOOS = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios",
	"js", "linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos"}

// Requires restricts a case to a host. A case whose requirement does not hold
// is skipped with its reason, never passed.
type Requires struct {
	// MachineIDSHA256 is the hex SHA-256 of the trimmed machine id of the
	// capture host (cases whose output depends on the host identity).
	MachineIDSHA256 string `json:"machine_id_sha256,omitempty"`
	// GOOS lists the operating systems the case runs on (runtime.GOOS
	// values); on any other the case is skipped. Absent means every OS.
	GOOS []string `json:"goos,omitempty"`
}

// validate rejects a goos list that could never run: present but empty, or
// naming an unknown operating system.
func (r Requires) validate() error {
	if r.GOOS != nil && len(r.GOOS) == 0 {
		return fmt.Errorf("requires.goos is empty: the case would never run")
	}
	for _, g := range r.GOOS {
		if !slices.Contains(knownGOOS, g) {
			return fmt.Errorf("requires.goos: unknown GOOS %q", g)
		}
	}
	return nil
}

func (c Case) usesTmp() bool {
	for _, a := range c.Args {
		if strings.Contains(a, TmpPlaceholder) {
			return true
		}
	}
	for _, v := range c.Env {
		if strings.Contains(v, TmpPlaceholder) {
			return true
		}
	}
	return false
}

// bind returns the case with {tmp} substituted, and the directory used ("" if none).
func bind(t *testing.T, c Case) (Case, string) {
	t.Helper()
	if !c.usesTmp() {
		return c, ""
	}
	tmp := t.TempDir()
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = strings.ReplaceAll(a, TmpPlaceholder, tmp)
	}
	env := map[string]string{}
	for k, v := range c.Env {
		env[k] = strings.ReplaceAll(v, TmpPlaceholder, tmp)
	}
	c.Args, c.Env = args, env
	return c, tmp
}

func unbind(r result, tmp string) result {
	if tmp == "" {
		return r
	}
	r.stdout = bytes.ReplaceAll(r.stdout, []byte(tmp), []byte(TmpPlaceholder))
	r.stderr = bytes.ReplaceAll(r.stderr, []byte(tmp), []byte(TmpPlaceholder))
	return r
}

// skipUnlessHost skips the case when its host requirement does not hold.
func skipUnlessHost(t *testing.T, c Case) {
	t.Helper()
	if goos := c.Requires.GOOS; goos != nil && !slices.Contains(goos, hostGOOS) {
		t.Skipf("OS-bound case: requires.goos %v does not include %s", goos, hostGOOS)
	}
	want := c.Requires.MachineIDSHA256
	if want == "" {
		return
	}
	raw, err := os.ReadFile(MachineIDFile)
	if err != nil {
		t.Skipf("host-bound case: cannot read %s: %v", MachineIDFile, err)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		t.Skipf("host-bound case: machine id differs from the capture host")
	}
}
