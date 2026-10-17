// Package golden runs frozen CLI cases (captured from the Python tools) against
// a Go command and requires byte-identical stdout, stderr and exit code.
//
// Layout, one directory per case under <testdata>/golden/<case>/:
//
//	cmd.json  {"args": [...], "stdin_file": "...", "env": {...}, "cwd_fixture": "..."}
//	stdout    expected standard output (exact bytes)
//	stderr    expected standard error (exact bytes; missing file = empty)
//	exit      expected exit code as decimal text
//
// stdin_file and cwd_fixture are relative to <testdata>; the command runs with
// <testdata> (or <testdata>/<cwd_fixture>) as its working directory. Every case
// gets COLUMNS=100 and NO_COLOR=1 unless its env overrides them.
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

	Stdout []byte `json:"-"`
	Stderr []byte `json:"-"`
	Exit   int    `json:"-"`
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
	return c, nil
}
