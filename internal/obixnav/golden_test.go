package obixnav

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
// intended differences come from each case's intended.json.
func TestGolden(t *testing.T) {
	station := startStation(t)
	golden.Run(t, "testdata", Main)
	if u := station.takeUnexpected(); len(u) > 0 {
		t.Errorf("requests not in the recording:\n%s", strings.Join(u, "\n---\n"))
	}
}
