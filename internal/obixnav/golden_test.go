package obixnav

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// stationAddr is where the golden cases expect the fake station: the Python capture ran against
// a scripted station on this fixed port, and the URLs are part of the frozen output.
const stationAddr = "127.0.0.1:47913"

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
	ln, err := net.Listen("tcp", stationAddr)
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

// optionsFromArgs maps the argv of a non-usage golden case onto Options (a test fixture, not a
// parser: usage, help and error text belong to internal/argparse).
func optionsFromArgs(t *testing.T, args []string) Options {
	t.Helper()
	var o Options
	var pos []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--base" || a == "--pass-file" || a == "--room":
			if i+1 >= len(args) {
				t.Fatalf("golden argv %q", args)
			}
			v := args[i+1]
			i++
			switch a {
			case "--base":
				o.Base = v
			case "--pass-file":
				o.PassFile = v
			default:
				n, err := strconv.Atoi(v)
				if err != nil {
					t.Fatalf("golden --room %q", v)
				}
				o.Room = n
			}
		case a == "--json":
			o.JSON = true
		case strings.HasPrefix(a, "-"):
			t.Fatalf("golden argv %q is not a plain case; name it usage-*", args)
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		t.Fatalf("golden argv %q has no command", args)
	}
	o.Command = pos[0]
	switch {
	case o.Command == "read" && len(pos) == 2:
		o.Ord = pos[1]
	case o.Command == "dump" && len(pos) == 2:
		o.DumpRoom = pos[1]
	case len(pos) != 1:
		t.Fatalf("golden argv %q", args)
	}
	return o
}

// intended lists the cases where Python ends in a traceback; Go prints one line instead.
var intended = map[string]struct {
	exit   int
	stderr string
}{
	"map-room-7-unknown":         {2, "obix-nav: map: no room 7 (rooms 1-5)\n"},
	"config-room-5-no-air-entry": {2, "obix-nav: config: no room 5 (rooms 1-4)\n"},
	"dump-room-x":                {2, "obix-nav: dump: invalid literal for int() with base 10: 'x'\n"},
}

// TestGolden checks Run against the frozen Python cases at the logic level (options already
// parsed). Once Main exists (internal/argparse), golden.Run(t, "testdata", Main) replaces it,
// with the same fake station running.
func TestGolden(t *testing.T) {
	cases, err := golden.Load("testdata")
	if err != nil {
		t.Fatal(err)
	}
	station := startStation(t)
	t.Chdir("testdata")
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if strings.HasPrefix(c.Name, "usage-") {
				t.Skip("awaiting internal/argparse")
			}
			var out, errb bytes.Buffer
			code := Run(optionsFromArgs(t, c.Args), cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			if u := station.takeUnexpected(); len(u) > 0 {
				t.Errorf("requests not in the recording:\n%s", strings.Join(u, "\n---\n"))
			}
			wantExit, wantErr := c.Exit, c.Stderr
			if d, ok := intended[c.Name]; ok {
				wantExit, wantErr = d.exit, []byte(d.stderr)
			}
			if code != wantExit {
				t.Errorf("exit = %d, want %d", code, wantExit)
			}
			if !bytes.Equal(out.Bytes(), c.Stdout) {
				t.Errorf("stdout: %s", golden.Diff(c.Stdout, out.Bytes()))
			}
			if !bytes.Equal(errb.Bytes(), wantErr) {
				t.Errorf("stderr: %s", golden.Diff(wantErr, errb.Bytes()))
			}
		})
	}

}
