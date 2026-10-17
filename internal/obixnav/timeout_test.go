package obixnav

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// chunkedServer writes n chunks of body, sleeping gap before each, then holds the response open
// for hold (a stall) before finishing it.
func chunkedServer(t *testing.T, n int, gap, hold time.Duration) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		for i := 0; i < n; i++ {
			time.Sleep(gap)
			fmt.Fprintf(w, "<chunk%d/>", i)
			w.(http.Flusher).Flush()
		}
		select {
		case <-time.After(hold):
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv
}

// urllib's timeout bounds each socket operation, not the whole request: a body that keeps
// arriving in time succeeds although it takes several timeouts in total.
func TestTimeoutIsPerSocketOperation(t *testing.T) {
	const timeout = 150 * time.Millisecond
	srv := chunkedServer(t, 6, 60*time.Millisecond, 0)
	c := newClient(srv.URL, "pw")
	start := time.Now()
	body, err := c.fetch(http.MethodGet, srv.URL, nil, timeout)
	if err != nil {
		t.Fatalf("a slow but progressing body failed after %s: %v", time.Since(start), err)
	}
	if elapsed := time.Since(start); elapsed <= timeout {
		t.Fatalf("the body took %s, not longer than one timeout: the test proves nothing", elapsed)
	}
	if !strings.HasSuffix(body, "<chunk5/>") {
		t.Errorf("body %q", body)
	}
}

// A body that stops arriving times out with urllib's message.
func TestStalledBodyTimesOut(t *testing.T) {
	srv := chunkedServer(t, 1, 0, 5*time.Second)
	c := newClient(srv.URL, "pw")
	start := time.Now()
	_, err := c.fetch(http.MethodGet, srv.URL, nil, 150*time.Millisecond)
	if err == nil || err.Error() != "<urlopen error timed out>" {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("the stall was detected only after %s", elapsed)
	}
}

// A server that never sends headers times out too.
func TestStalledHeadersTimeOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	t.Cleanup(func() { close(release); srv.Close() })
	_, err := newClient(srv.URL, "pw").fetch(http.MethodGet, srv.URL, nil, 150*time.Millisecond)
	if err == nil || err.Error() != "<urlopen error timed out>" {
		t.Fatalf("err = %v", err)
	}
}
