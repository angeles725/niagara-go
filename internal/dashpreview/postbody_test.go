package dashpreview

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// bodyReader yields data, then err instead of io.EOF.
type bodyReader struct {
	data []byte
	err  error
}

func (b *bodyReader) Read(p []byte) (int, error) {
	if len(b.data) == 0 {
		return 0, b.err
	}
	n := copy(p, b.data)
	b.data = b.data[n:]
	return n, nil
}

// post sends a POST /dp/api/x with the given body and Content-Length, and returns the log,
// the status (0 when the handler dropped the connection) and whether it dropped it.
func post(t *testing.T, body io.Reader, length string) (log string, code int, dropped bool) {
	t.Helper()
	var logBuf bytes.Buffer
	h := &Handler{RCDir: "testdata/ux/src/rc", Prefix: "/dp", Mock: pyjson.NewObject(), Log: &logBuf}
	r := httptest.NewRequest("POST", "/dp/api/x", body)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	r.Header.Set("Content-Length", length)
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if v := recover(); v != nil {
				if v != http.ErrAbortHandler {
					panic(v)
				}
				dropped = true
			}
		}()
		h.ServeHTTP(rec, r)
	}()
	if !dropped {
		code = rec.Code
	}
	return logBuf.String(), code, dropped
}

func TestPOSTBodyReadErrorDropsTheRequest(t *testing.T) {
	log, code, dropped := post(t, &bodyReader{data: []byte(`{"a"`), err: errors.New("connection reset by peer")}, "20")
	if !dropped {
		t.Fatalf("a body read error was ignored: status %d, log:\n%s", code, log)
	}
	if !strings.Contains(log, "Exception occurred during processing of request") ||
		!strings.Contains(log, "OSError: connection reset by peer") {
		t.Fatalf("log does not report the read error:\n%s", log)
	}
	if strings.Contains(log, "[POST") {
		t.Fatalf("a body that could not be read was still processed:\n%s", log)
	}
}

func TestPOSTShortBodyIsReadLikePython(t *testing.T) {
	// rfile.read(length) returns what arrived before the client closed: not an error.
	log, code, dropped := post(t, &bodyReader{data: []byte(`{"a":1}`), err: io.ErrUnexpectedEOF}, "20")
	if dropped || code != 200 {
		t.Fatalf("short body: dropped=%v status=%d, log:\n%s", dropped, code, log)
	}
	if !strings.Contains(log, `[POST /api/x] {"a":1}`) {
		t.Fatalf("short body not logged as read:\n%s", log)
	}
}
