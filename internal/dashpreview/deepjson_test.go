package dashpreview

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestPostDeepJSONDropsConnection: a runaway-nested POST body makes Python's
// json.loads raise RecursionError, which `except ValueError` does not catch,
// so the exception escapes the handler and socketserver drops the connection.
func TestPostDeepJSONDropsConnection(t *testing.T) {
	for _, path := range []string{"/dp/api/config/login", "/dp/api/set"} {
		var log bytes.Buffer
		h := &Handler{RCDir: "testdata/ux/src/rc", Prefix: "/dp", Mock: pyjson.NewObject(),
			Login: &LoginConfig{Password: "pw", TTL: 60}, Log: &log}
		h.sessToken, h.expires = "tok", h.now()+60
		body := strings.Repeat("[", pyjson.MaxDepth+1)
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Content-Length", strconv.Itoa(len(body)))
		r.Header.Set("X-Requested-With", "XMLHttpRequest")
		r.AddCookie(&http.Cookie{Name: "dp_config_session", Value: "tok"})
		dropped := func() (d bool) {
			defer func() { d = recover() == http.ErrAbortHandler }()
			h.ServeHTTP(httptest.NewRecorder(), r)
			return false
		}()
		want := "\nRecursionError: maximum recursion depth exceeded while decoding a JSON array from a unicode string\n"
		if !dropped || !strings.Contains(log.String(), want) {
			t.Errorf("%s: dropped %v, log tail %q", path, dropped, log.String()[max(0, log.Len()-300):])
		}
	}
}
