package dashpreview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Frozen from tools/dashboard-preview.py (Python 3.15):
//   - testdata/golden/: CLI cases that exit without serving (shared runner).
//   - testdata/startup-golden.json: banners of a served run (--port 0, then
//     SIGINT), the absolute testdata path written as {TD}.
//   - testdata/http-golden.json: the tool's own make_handler on 127.0.0.1:0
//     with a fixed clock, token and change_log time, replayed request by request
//     (status, headers minus Date/Server, body, server log lines, and what the
//     --live upstream received).

func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestStartupBanners(t *testing.T) {
	raw, err := os.ReadFile("testdata/startup-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]struct {
		Args           []string
		Stdout, Stderr string
		Exit           int
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	td, _ := filepath.Abs("testdata")
	t.Chdir("testdata")
	t.Setenv("COLUMNS", "100")
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := newParser()
			var out, errb syncBuf
			s := cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb}
			ns, _, done := p.Parse(c.Args, s)
			if done {
				t.Fatal("parse failed")
			}
			o := Options{RC: ns.String("rc"), Prefix: ns.String("prefix"), Port: ns.Int("port"), Mock: ns.String("mock"),
				AutoMock: ns.Bool("auto_mock"), Reader: ns.String("reader"), Live: ns.String("live"),
				ConfigLogin: ns.Bool("config_login"), ConfigPassword: ns.String("config_password"),
				ConfigTTL: ns.Int("config_ttl"), Editor: ns.Bool("editor")}
			ctx, cancel := context.WithCancel(context.Background())
			done2 := make(chan int)
			go func() { done2 <- Run(ctx, o, s) }()
			// Run prints the "Ctrl+C" banner right before Serve; stop it once the
			// banner is out (Serve then returns ErrServerClosed), or take an early exit.
			var code int
			deadline := time.After(10 * time.Second)
		wait:
			for {
				select {
				case code = <-done2:
					break wait
				case <-deadline:
					cancel()
					t.Fatalf("no startup banner within 10s: stdout %q stderr %q", out.String(), errb.String())
				case <-time.After(time.Millisecond):
					if strings.Contains(out.String(), "  Ctrl+C para detener.") {
						cancel()
						code = <-done2
						break wait
					}
				}
			}
			cancel()
			norm := func(x string) string { return strings.ReplaceAll(x, td, "{TD}") }
			if code != c.Exit || norm(out.String()) != c.Stdout || norm(errb.String()) != c.Stderr {
				t.Errorf("exit %d stdout %q stderr %q\nwant %d %q %q", code, norm(out.String()), norm(errb.String()), c.Exit, c.Stdout, c.Stderr)
			}
		})
	}
}

type httpStep struct {
	Advance *float64 `json:"advance"`
	Request struct {
		Method  string
		Path    string
		Headers map[string]string
		BodyB64 *string `json:"body_b64"`
	}
	Status      int
	Dropped     bool
	RespHeaders [][2]string `json:"resp_headers"`
	Body        *string
	BodyB64     *string `json:"body_b64"`
	Stderr      string
	Upstream    []map[string]any
}

type httpConfig struct {
	Config struct {
		Prefix      string
		Editor      bool
		ConfigLogin *struct {
			Password string
			TTL      int
		} `json:"config_login"`
		Live     any
		MockFile *string `json:"mock_file"`
		AutoMock bool    `json:"auto_mock"`
	}
	Steps []httpStep
}

// upstream mirrors the harness's fake station.
func upstream(seen *[]map[string]any, mu *sync.Mutex) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec := map[string]any{"method": r.Method, "path": r.RequestURI, "xhr": nil, "ctype": nil, "body": string(body)}
		if v, ok := r.Header["X-Requested-With"]; ok {
			rec["xhr"] = v[0]
		}
		if v, ok := r.Header["Content-Type"]; ok {
			rec["ctype"] = v[0]
		}
		mu.Lock()
		*seen = append(*seen, rec)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=x-up")
		if strings.HasSuffix(r.RequestURI, "/fail") {
			w.WriteHeader(409)
			_, _ = io.WriteString(w, `{"ok":false}`)
			return
		}
		_, _ = io.WriteString(w, `{"from": "upstream", "path": "`+r.RequestURI+`"}`)
	}))
}

func TestHTTPGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/http-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var configs map[string]httpConfig
	if err := json.Unmarshal(raw, &configs); err != nil {
		t.Fatal(err)
	}
	rc, _ := filepath.Abs("testdata/ux/src/rc")
	for name, c := range configs {
		t.Run(name, func(t *testing.T) { runHTTPConfig(t, rc, c) })
	}
}

func runHTTPConfig(t *testing.T, rc string, c httpConfig) {
	clock := 1_800_000_000.0
	var logBuf syncBuf
	h := &Handler{RCDir: rc, Prefix: c.Config.Prefix, Mock: pyjson.NewObject(), Editor: c.Config.Editor, Log: &logBuf,
		Now: func() float64 { return clock }, Token: func() string { return strings.Repeat("0123456789abcdef", 2) },
		Clock: func() string { return "12:00:00" }}
	if c.Config.ConfigLogin != nil {
		h.Login = &LoginConfig{Password: c.Config.ConfigLogin.Password, TTL: c.Config.ConfigLogin.TTL}
	}
	if c.Config.MockFile != nil {
		v, err := loadMock(filepath.Join("testdata", *c.Config.MockFile))
		if err != nil {
			t.Fatal(err)
		}
		h.Mock = v
	}
	if c.Config.AutoMock {
		src, _ := os.ReadFile("testdata/ux/src/com/acme/DemoReader.java")
		e, n, l, err := classifyReaderArrays(parseStringArrays(string(src)))
		if err != nil {
			t.Fatal(err)
		}
		h.Mock = generateAutoMock(e, n, l)
	}
	var seen []map[string]any
	var seenMu sync.Mutex
	switch c.Config.Live {
	case true:
		up := upstream(&seen, &seenMu)
		defer up.Close()
		h.LiveBaseURL = up.URL
	case "down":
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		h.LiveBaseURL = "http://" + ln.Addr().String()
		ln.Close()
	}
	// handled receives once per request after ServeHTTP has returned (also
	// when it aborts a dropped connection), so the request's log lines are
	// complete before a step checks them or advances the clock.
	handled := make(chan struct{}, len(c.Steps)+1) // never blocks a handler, even after a failed step
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { handled <- struct{}{} }()
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	waitHandled := func(label string) {
		select {
		case <-handled:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: handler did not return within 10s", label)
		}
	}
	for i, st := range c.Steps {
		if st.Advance != nil {
			clock += *st.Advance
			continue
		}
		before := len(logBuf.String())
		seenMu.Lock()
		n0 := len(seen)
		seenMu.Unlock()
		var body io.Reader
		if st.Request.BodyB64 != nil {
			b, _ := base64.StdEncoding.DecodeString(*st.Request.BodyB64)
			body = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(st.Request.Method, srv.URL+"/", body)
		req.URL.Opaque = st.Request.Path
		for k, v := range st.Request.Headers {
			req.Header.Set(k, v)
		}
		req.Close = true
		resp, err := http.DefaultTransport.RoundTrip(req)
		label := st.Request.Method + " " + st.Request.Path
		if st.Dropped {
			if err == nil {
				resp.Body.Close()
				t.Errorf("step %d %s: got %d, want a dropped connection", i, label, resp.StatusCode)
			}
			waitHandled(label)
			if got := strings.SplitN(logBuf.String()[before:], strings.Repeat("-", 40), 2)[0]; got != st.Stderr {
				t.Errorf("step %d %s log %q, want %q", i, label, got, st.Stderr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("step %d %s: %v", i, label, err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		waitHandled(label)
		checkStep(t, i, label, st, resp, got, logBuf.String()[before:])
		seenMu.Lock()
		gotSeen := seen[n0:]
		seenMu.Unlock()
		a, _ := json.Marshal(gotSeen)
		b, _ := json.Marshal(st.Upstream)
		if len(st.Upstream) == 0 && len(gotSeen) == 0 {
			continue
		}
		if string(a) != string(b) {
			t.Errorf("step %d %s upstream saw %s, want %s", i, label, a, b)
		}
	}
}

func checkStep(t *testing.T, i int, label string, st httpStep, resp *http.Response, got []byte, log string) {
	t.Helper()
	if resp.StatusCode != st.Status {
		t.Errorf("step %d %s: status %d want %d", i, label, resp.StatusCode, st.Status)
	}
	var want []byte
	if st.Body != nil {
		want = []byte(*st.Body)
	} else if st.BodyB64 != nil {
		want, _ = base64.StdEncoding.DecodeString(*st.BodyB64)
	}
	if st.Request.Method == http.MethodHead {
		want = nil
	}
	if !bytes.Equal(got, want) {
		t.Errorf("step %d %s body:\n%q\nwant\n%q", i, label, got, want)
	}
	for _, kv := range st.RespHeaders {
		if kv[0] == "Connection" {
			continue // consumed by the client transport; TestErrorPageHeaders checks it
		}
		if g := resp.Header.Values(kv[0]); len(g) != 1 || g[0] != kv[1] {
			t.Errorf("step %d %s header %s=%q, want %q", i, label, kv[0], g, kv[1])
		}
	}
	if log != st.Stderr {
		t.Errorf("step %d %s log %q, want %q", i, label, log, st.Stderr)
	}
}

// TestErrorPageHeaders checks send_error's headers on the recorder (a client
// transport consumes Connection: close).
func TestErrorPageHeaders(t *testing.T) {
	h := &Handler{RCDir: "testdata/ux/src/rc", Prefix: "/dp", Mock: pyjson.NewObject(), Log: io.Discard}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/dp/nope.html", nil))
	if rec.Code != 404 || rec.Header().Get("Connection") != "close" || rec.Header().Get("Content-Type") != "text/html;charset=utf-8" {
		t.Errorf("code %d headers %v", rec.Code, rec.Header())
	}
}
