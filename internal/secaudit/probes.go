package secaudit

// The external probes of the audit (keytool, openssl, icacls, TCP connects). They are fields of
// env so tests replace them; the defaults run the real programs.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// result is a finished subprocess: its output bytes and exit status.
type result struct {
	stdout, stderr []byte
	code           int
}

// startError is a program that could not be started, rendered like Python's OSError.
type startError struct{ msg string }

func (e *startError) Error() string { return e.msg }

// timeoutError is subprocess.TimeoutExpired.
type timeoutError struct{ msg string }

func (e *timeoutError) Error() string { return e.msg }

// env holds the probes and file tests one audit uses.
type env struct {
	// run starts argv with stdin; extraEnv ("KEY=value") is added to this child's environment only.
	run      func(argv []string, stdin []byte, timeout time.Duration, extraEnv ...string) (result, error)
	dial     func(host string, port int) bool
	lookPath func(name string) string // "" when not found
	winpath  func(path string) string // the Windows form of a WSL path, "" for none
}

func defaultEnv() *env {
	return &env{run: runProgram, dial: dialTCP, lookPath: lookPath, winpath: toWinpath}
}

// runProgram is subprocess.run(argv, input=stdin, capture_output=True, timeout=timeout), with
// extraEnv appended to the inherited environment of the child.
func runProgram(argv []string, stdin []byte, timeout time.Duration, extraEnv ...string) (result, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return result{}, &timeoutError{fmt.Sprintf("Command %s timed out after %s seconds",
			pycompat.Repr(argv), strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64))}
	}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return result{out.Bytes(), errb.Bytes(), exitErr.ExitCode()}, nil
	case err != nil:
		return result{}, &startError{startMessage(argv[0], err)}
	}
	return result{out.Bytes(), errb.Bytes(), 0}, nil
}

// startMessage is str(OSError) of a program that did not start.
func startMessage(name string, err error) string {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Sprintf("[Errno 2] No such file or directory: %s", pycompat.StrRepr(name))
	}
	return pyOSError(err)
}

func dialTCP(host string, port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func lookPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// keytoolOutcome is one "keytool -list" run against the default store password.
type keytoolOutcome struct {
	opened    bool   // exit 0: the store opened with "changeit"
	wrongPass bool   // keytool refused the password ("... password was incorrect")
	failure   string // "exit N: <first output line, 120 characters at most>" for any non-zero exit
	stdout    string
}

// storepassVar carries the default store password to keytool ("-storepass:env"): a password on
// the command line is visible to every local user in the process list.
const storepassVar = "NIAGARA_SECAUDIT_STOREPASS"

// wrongPassMarker is how keytool (JDK 8 and later, JKS and PKCS12) reports a wrong store password.
// keytool prints its errors on stdout, so both streams are searched.
const wrongPassMarker = "password was incorrect"

// keytoolList runs "keytool -list [-v]" on the store at path with the default password.
func (e *env) keytoolList(path string, verbose bool) (keytoolOutcome, error) {
	argv := []string{"keytool", "-list"}
	if verbose {
		argv = append(argv, "-v")
	}
	r, err := e.run(append(argv, "-keystore", path, "-storepass:env", storepassVar), nil, 0, storepassVar+"=changeit")
	if err != nil {
		return keytoolOutcome{}, err
	}
	out := keytoolOutcome{stdout: pycompat.DecodeReplace(r.stdout)}
	if r.code == 0 {
		out.opened = true
		return out, nil
	}
	errText := pycompat.DecodeReplace(r.stderr)
	out.wrongPass = strings.Contains(strings.ToLower(out.stdout+"\n"+errText), wrongPassMarker)
	text := pycompat.Strip(out.stdout)
	if text == "" {
		text = pycompat.Strip(errText)
	}
	first, _, _ := strings.Cut(text, "\n") // -v adds a stack trace after the error line
	out.failure = fmt.Sprintf("exit %d: %s", r.code, head(pycompat.Strip(first), 120))
	return out, nil
}

// keytoolDefault is the SEC-02 probe of the store at path; nil when it is no file.
func (e *env) keytoolDefault(path string) (*keytoolOutcome, error) {
	if !isFile(path) {
		return nil, nil
	}
	o, err := e.keytoolList(path, false)
	return &o, err
}

var bitsRE = regexp.MustCompile(`([0-9]+)-bit`)

// keytoolKeysizes are the key sizes "keytool -list -v" prints for the store at path. failure is
// set (and sizes nil) when keytool did not exit 0: the sizes were not read, which is not "none".
func (e *env) keytoolKeysizes(path string) (sizes []int, failure string, err error) {
	if !isFile(path) {
		return nil, "", nil
	}
	o, err := e.keytoolList(path, true)
	if err != nil {
		return nil, "", err
	}
	if !o.opened {
		return nil, o.failure, nil
	}
	for _, m := range bitsRE.FindAllStringSubmatch(o.stdout, -1) {
		n, _ := strconv.Atoi(m[1])
		sizes = append(sizes, n)
	}
	return sizes, "", nil
}

// tlsSubject is the "subject=..." line of the certificate host:port serves; "" on any failure.
func (e *env) tlsSubject(host string, port int) string {
	r, err := e.run([]string{"openssl", "s_client", "-connect", host + ":" + strconv.Itoa(port)}, nil, 15*time.Second)
	if err != nil || !utf8.Valid(r.stdout) {
		return ""
	}
	c, err := e.run([]string{"openssl", "x509", "-noout", "-subject"}, r.stdout, 0)
	if err != nil || !utf8.Valid(c.stdout) {
		return ""
	}
	return pycompat.Strip(string(c.stdout))
}

// icaclsPaths is where WSL keeps icacls when System32 is not on PATH.
var icaclsPaths = []string{"/mnt/c/Windows/System32/icacls.exe"}

// findIcacls is icacls.exe on PATH, else its System32 copy; "" when neither exists.
func (e *env) findIcacls() string {
	if p := e.lookPath("icacls.exe"); p != "" {
		return p
	}
	for _, p := range icaclsPaths {
		if isFile(p) {
			return p
		}
	}
	return ""
}

// runIcacls returns the decoded listing, or the reason there is none: failure to start, timeout
// or a non-zero exit.
func (e *env) runIcacls(exe, winpath string) (string, string) {
	r, err := e.run([]string{exe, winpath}, nil, 60*time.Second)
	if err != nil {
		return "", "failed: " + err.Error()
	}
	stdout, stderr := decodeConsole(r.stdout), decodeConsole(r.stderr)
	if r.code != 0 {
		text := stderr
		if text == "" {
			text = stdout
		}
		return "", fmt.Sprintf("exit %d: %s", r.code, head(pycompat.Strip(text), 120))
	}
	return stdout, ""
}
