// Package golden runs frozen CLI cases (captured from the Python tools) against
// a Go command and requires byte-identical stdout, stderr and exit code.
//
// Layout, one directory per case under <testdata>/golden/<case>/:
//
//	cmd.json  {"args": [...], "stdin_file": "...", "env": {...}, "cwd_fixture": "...",
//	           "requires": {"machine_id_sha256": "...", "goos": ["linux", ...]}}
//	stdout    expected standard output (exact bytes)
//	stderr    expected standard error (exact bytes; missing file = empty)
//	exit      expected exit code as decimal text
//	intended.json  optional: {"reason": "...", "stdout"?: "...", "stderr"?: "...", "exit"?: N}
//
// stdout/stderr/exit always hold what the Python tool printed. An intended
// difference (e.g. one error line where Python crashed with a traceback) is
// declared in intended.json: its fields replace the expectation, and its
// reason is mandatory so every divergence is explained next to the fixture.
//
// stdin_file and cwd_fixture are relative to <testdata>; the command runs with
// <testdata> (or <testdata>/<cwd_fixture>) as its working directory. Every case
// gets COLUMNS=100 and NO_COLOR=1 unless its env overrides them. "{tmp}" in
// args or env values is a fresh temporary directory (see bind.go); a case with
// "requires" is skipped, with a reason, on any other host or operating system.
package golden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DefaultEnv is applied to every case before the case's own env.
var DefaultEnv = map[string]string{"COLUMNS": "100", "NO_COLOR": "1"}

// Case is one frozen invocation and its expected result.
type Case struct {
	Name       string            `json:"-"`
	Args       []string          `json:"args"`
	StdinFile  string            `json:"stdin_file,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	CwdFixture string            `json:"cwd_fixture,omitempty"`
	Requires   Requires          `json:"requires,omitempty"`

	Stdout []byte `json:"-"`
	Stderr []byte `json:"-"`
	Exit   int    `json:"-"`

	// Intended is non-nil when the case declares an intended difference.
	Intended *Intended `json:"-"`
}

// Intended overrides parts of the Python expectation, with a reason.
type Intended struct {
	Reason string  `json:"reason"`
	Stdout *string `json:"stdout,omitempty"`
	Stderr *string `json:"stderr,omitempty"`
	Exit   *int    `json:"exit,omitempty"`
}

// Environ returns DefaultEnv merged with the case's env.
func (c Case) Environ() map[string]string {
	env := map[string]string{}
	for k, v := range DefaultEnv {
		env[k] = v
	}
	for k, v := range c.Env {
		env[k] = v
	}
	return env
}

// Load reads every case under testdata/golden, sorted by name.
func Load(testdata string) ([]Case, error) {
	root := filepath.Join(testdata, "golden")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var cases []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := loadCase(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("golden case %s: %w", e.Name(), err)
		}
		c.Name = e.Name()
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no golden cases under %s", root)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

func loadCase(dir string) (Case, error) {
	var c Case
	raw, err := os.ReadFile(filepath.Join(dir, "cmd.json"))
	if err != nil {
		return c, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("cmd.json: %w", err)
	}
	if err := c.Requires.validate(); err != nil {
		return c, fmt.Errorf("cmd.json: %w", err)
	}
	if c.Stdout, err = os.ReadFile(filepath.Join(dir, "stdout")); err != nil {
		return c, err
	}
	if c.Stderr, err = os.ReadFile(filepath.Join(dir, "stderr")); err != nil && !os.IsNotExist(err) {
		return c, err
	}
	exit, err := os.ReadFile(filepath.Join(dir, "exit"))
	if err != nil {
		return c, err
	}
	if c.Exit, err = strconv.Atoi(strings.TrimSpace(string(exit))); err != nil {
		return c, fmt.Errorf("exit: %w", err)
	}
	return c, applyIntended(dir, &c)
}

func applyIntended(dir string, c *Case) error {
	raw, err := os.ReadFile(filepath.Join(dir, "intended.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var in Intended
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return fmt.Errorf("intended.json: %w", err)
	}
	if strings.TrimSpace(in.Reason) == "" {
		return fmt.Errorf("intended.json: reason is required")
	}
	if in.Stdout == nil && in.Stderr == nil && in.Exit == nil {
		return fmt.Errorf("intended.json: overrides nothing")
	}
	if in.Stdout != nil {
		c.Stdout = []byte(*in.Stdout)
	}
	if in.Stderr != nil {
		c.Stderr = []byte(*in.Stderr)
	}
	if in.Exit != nil {
		c.Exit = *in.Exit
	}
	c.Intended = &in
	return nil
}
