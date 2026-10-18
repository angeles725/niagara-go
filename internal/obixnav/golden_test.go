package obixnav

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// stationAddr is where the golden cases expect the fake station: the Python capture ran against
// a scripted station on this fixed port, and the URLs are part of the frozen output. The port is
// outside the TCP ranges Windows reserves here (netsh int ipv4 show excludedportrange, the
// 49152+ dynamic ports) and below the ephemeral range WSL mirrored networking gives Linux and
// holds on the Windows side (ip_local_port_range 44619-48715 on the capture host). It is inside
// the stock Linux ephemeral range (32768-60999), so a plain Linux host may hand it out to an
// outgoing connection; listenWhenFree waits while anything holds it.
const stationAddr = "127.0.0.1:39213"

// exchange is one recorded request/response pair of testdata/inputs/recording.json.
type exchange struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Auth     string `json:"auth"`
	Body     string `json:"body"`
	Status   int    `json:"status"`
	Response string `json:"response"`
}

// replayStation serves the recording: a request that was not recorded gets 599 and is reported.
type replayStation struct {
	mu         sync.Mutex
	byKey      map[string]exchange
	unexpected []string
}

func key(method, path, auth, body string) string {
	return method + " " + path + "\n" + auth + "\n" + body
}

func (rs *replayStation) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	k := key(r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"), string(body))
	rs.mu.Lock()
	ex, ok := rs.byKey[k]
	if !ok {
		rs.unexpected = append(rs.unexpected, k)
	}
	rs.mu.Unlock()
	if !ok {
		w.WriteHeader(599)
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(ex.Status)
	io.WriteString(w, ex.Response)
}

func (rs *replayStation) takeUnexpected() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	u := rs.unexpected
	rs.unexpected = nil
	return u
}

func startStation(t *testing.T) *replayStation {
	t.Helper()
	raw, err := os.ReadFile("testdata/inputs/recording.json")
	if err != nil {
		t.Fatal(err)
	}
	var exs []exchange
	if err := json.Unmarshal(raw, &exs); err != nil {
		t.Fatal(err)
	}
	rs := &replayStation{byKey: map[string]exchange{}}
	for _, ex := range exs {
		rs.byKey[key(ex.Method, ex.Path, ex.Auth, ex.Body)] = ex
	}
	ln, err := listenWhenFree(stationAddr, 2*time.Minute)
	if err != nil {
		t.Fatalf("the golden cases need %s free for the fake station: %v", stationAddr, err)
	}
	srv := httptest.NewUnstartedServer(rs)
	srv.Listener.Close()
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return rs
}

// listenWhenFree binds addr, waiting while another test process (a parallel `go test ./...` in
// another worktree) holds the fixed port, so concurrent runs queue instead of failing.
func listenWhenFree(addr string, wait time.Duration) (net.Listener, error) {
	deadline := time.Now().Add(wait)
	for {
		ln, err := net.Listen("tcp", addr)
		if err == nil || time.Now().After(deadline) {
			return ln, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// TestGolden runs every case (usage errors included) through Main against the replayed station;
// intended differences come from each case's intended.json. connection-refused requires linux:
// its text is Linux's ECONNREFUSED message (Windows CPython's urllib says "[WinError 10061]
// <system message>" there), and its refused port, 47914, stays inside the WSL ephemeral range,
// because under WSL mirrored networking a closed port outside it times out instead of refusing.
func TestGolden(t *testing.T) {
	station := startStation(t)
	golden.Run(t, "testdata", Main)
	if u := station.takeUnexpected(); len(u) > 0 {
		t.Errorf("requests not in the recording:\n%s", strings.Join(u, "\n---\n"))
	}
}

// TestConnectionRefusedAnyOS is the portable twin of the connection-refused golden, whose text
// and fixed port are Linux's: it dials a listener the test has just closed, an ephemeral port
// that refuses on every OS (and under WSL mirrored networking), and checks that Main reports the
// refusal the OS gave, rendered the way urllib prints it, with exit 2.
func TestConnectionRefusedAnyOS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	_, dialErr := net.DialTimeout("tcp", addr, 10*time.Second)
	if !errors.Is(dialErr, syscall.ECONNREFUSED) && pycompat.OSErrorName(dialErr) != "ConnectionRefusedError" {
		t.Fatalf("dialing the closed listener %s did not refuse: %v", addr, dialErr)
	}
	pass := filepath.Join(t.TempDir(), "pass.txt")
	if err := os.WriteFile(pass, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := "http://" + addr + "/obix"
	var out, errb bytes.Buffer
	code := Main([]string{"--base", base, "--pass-file", pass, "cond"}, cli.Stdio{Out: &out, Err: &errb})
	want := fmt.Sprintf("obix-nav: cond failed against %s: <urlopen error %s>\n", base, pycompat.WinOSErrorText(dialErr))
	if code != 2 || out.Len() != 0 || errb.String() != want {
		t.Fatalf("exit %d, stdout %q, stderr %q; want exit 2, no stdout, stderr %q", code, out.String(), errb.String(), want)
	}
}
