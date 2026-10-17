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
	"time"
	"unicode/utf8"
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
	run      func(argv []string, stdin []byte, timeout time.Duration) (result, error)
	dial     func(host string, port int) bool
	lookPath func(name string) string // "" when not found
	winpath  func(path string) string // the Windows form of a WSL path, "" for none
}

func defaultEnv() *env {
	return &env{run: runProgram, dial: dialTCP, lookPath: lookPath, winpath: toWinpath}
}

// runProgram is subprocess.run(argv, input=stdin, capture_output=True, timeout=timeout).
func runProgram(argv []string, stdin []byte, timeout time.Duration) (result, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return result{}, &timeoutError{fmt.Sprintf("Command %s timed out after %s seconds",
			pyListRepr(argv), strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64))}
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
		return fmt.Sprintf("[Errno 2] No such file or directory: %s", pyStrRepr(name))
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

// keytoolDefault reports whether the store at path opens with "changeit"; nil when it is no file.
func (e *env) keytoolDefault(path string) (*bool, error) {
	if !isFile(path) {
		return nil, nil
	}
	r, err := e.run([]string{"keytool", "-list", "-keystore", path, "-storepass", "changeit"}, nil, 0)
	if err != nil {
		return nil, err
	}
	stderr := bytes.ToLower([]byte(decodeReplace(r.stderr)))
	ok := true
	for _, x := range []string{"tampered", "incorrect", "invalid keystore"} {
		if bytes.Contains(stderr, []byte(x)) {
			ok = false
		}
	}
	return &ok, nil
}

var bitsRE = regexp.MustCompile(`([0-9]+)-bit`)

// keytoolKeysizes are the key sizes "keytool -list -v" prints for the store at path.
func (e *env) keytoolKeysizes(path string) ([]int, error) {
	if !isFile(path) {
		return nil, nil
	}
	r, err := e.run([]string{"keytool", "-list", "-v", "-keystore", path, "-storepass", "changeit"}, nil, 0)
	if err != nil {
		return nil, err
	}
	var sizes []int
	for _, m := range bitsRE.FindAllStringSubmatch(decodeReplace(r.stdout), -1) {
		n, _ := strconv.Atoi(m[1])
		sizes = append(sizes, n)
	}
	return sizes, nil
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
	return pyStrip(string(c.stdout))
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
		return "", fmt.Sprintf("exit %d: %s", r.code, head(pyStrip(text), 120))
	}
	return stdout, ""
}
