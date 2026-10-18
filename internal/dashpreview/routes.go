package dashpreview

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// loadsOrEmpty mirrors `json.loads(body or "{}")` with ValueError -> {}. A
// RecursionError is not a ValueError: it escapes the handler, returned here
// as the exception line for rp.drop.
func loadsOrEmpty(body string) (pyjson.Value, string) {
	if body == "" {
		body = "{}"
	}
	v, err := pyjson.Loads([]rune(body))
	var re *pyjson.RecursionError
	if errors.As(err, &re) {
		return v, "RecursionError: " + re.Error()
	}
	if err != nil {
		return pyjson.NewObject(), ""
	}
	return v, ""
}

// attrError is the AttributeError `req.get` raises on a non-dict body.
func attrError(v pyjson.Value) string {
	return fmt.Sprintf("AttributeError: '%s' object has no attribute 'get'", v.TypeName())
}

func getOr(v pyjson.Value, key string, def pyjson.Value) pyjson.Value {
	if x, ok := v.Get(key); ok {
		return x
	}
	return def
}

func (h *Handler) doGET(rp *reply) {
	p := h.strip(rp.r.RequestURI)
	if p == "/hmi" {
		h.hmi(rp)
		return
	}
	xhrOnly := strings.HasPrefix(p, "/api/") || h.Login != nil && p == "/__mock/change_log"
	if xhrOnly && !h.hasXHR(rp.r) {
		rp.redirect(h.Prefix + "/index.html")
		return
	}
	if h.Login != nil && (p == "/api/config/session" || p == "/__mock/change_log") {
		h.mu.Lock()
		var v pyjson.Value
		if p == "/api/config/session" {
			act := h.sessActive(rp.r)
			v = pyjson.NewObject()
			v.Set("active", pyjson.Value{Kind: pyjson.Bool, B: act})
			user, remaining := pyjson.Value{}, int64(0)
			if act {
				user = h.sessUser
				remaining = int64(max(0, h.expires-h.now()))
			}
			v.Set("user", user)
			v.Set("remaining", pyjson.IntValue(remaining))
			v.Set("ttl", pyjson.IntValue(int64(h.Login.TTL)))
		} else {
			v = pyjson.Value{Kind: pyjson.Array, A: append([]pyjson.Value{}, h.changeLog...)}
		}
		h.mu.Unlock()
		rp.json(v, 200, "")
		return
	}
	if strings.HasPrefix(p, "/api/") {
		if h.LiveBaseURL != "" {
			h.proxy(rp, p, nil, nil)
		} else {
			rp.json(h.Mock, 200, "")
		}
		return
	}
	h.serveFile(rp, p)
}

func (h *Handler) doPOST(rp *reply) {
	r := rp.r
	p := h.strip(r.RequestURI)
	if strings.HasPrefix(p, "/api/") && !h.hasXHR(r) {
		rp.redirect(h.Prefix + "/index.html")
		return
	}
	length := 0
	if cl := r.Header.Get("Content-Length"); cl != "" {
		n, err := strconv.Atoi(strings.TrimSpace(cl))
		if err != nil {
			rp.drop(fmt.Sprintf("ValueError: invalid literal for int() with base 10: %s", pyjson.StrRepr(cl)))
			return
		}
		length = n
	}
	raw := []byte{}
	if length > 0 {
		raw, _ = io.ReadAll(io.LimitReader(r.Body, int64(length)))
	}
	body := pycompat.DecodeReplace(raw)
	h.logf("  [POST %s] %s\n", p, body)
	var row *pyjson.Value
	if h.Login != nil && (p == "/api/config/login" || p == "/api/config/logout") {
		h.login(rp, p, body)
		return
	}
	if h.Login != nil && strings.HasPrefix(p, "/api/") {
		h.mu.Lock()
		if !h.sessActive(r) {
			h.mu.Unlock()
			rp.json(errObj("config_login_required"), 403, "")
			return
		}
		h.expires = h.now() + float64(h.Login.TTL)
		req, exc := loadsOrEmpty(body)
		if exc == "" && req.Kind != pyjson.Object {
			exc = attrError(req)
		}
		if exc != "" {
			h.mu.Unlock()
			rp.drop(exc)
			return
		}
		row = h.changeRow(req)
		h.mu.Unlock()
	}
	logRow := func(code int) {
		if row == nil {
			return
		}
		entry := pyjson.NewObject()
		for _, k := range row.Keys {
			entry.Set(k, row.M[k])
		}
		entry.Set("result", pyjson.IntValue(int64(code)))
		entry.Set("ok", pyjson.Value{Kind: pyjson.Bool, B: code >= 200 && code < 300})
		h.mu.Lock()
		h.changeLog = append(h.changeLog, entry)
		h.mu.Unlock()
	}
	if h.LiveBaseURL != "" && strings.HasPrefix(p, "/api/") {
		h.proxy(rp, p, raw, logRow)
		return
	}
	logRow(200)
	ok := pyjson.NewObject()
	ok.Set("ok", pyjson.Value{Kind: pyjson.Bool, B: true})
	rp.json(ok, 200, "")
}

func errObj(code string) pyjson.Value {
	v := pyjson.NewObject()
	v.Set("ok", pyjson.Value{Kind: pyjson.Bool})
	v.Set("error", pyjson.String(code))
	return v
}

func (h *Handler) login(rp *reply, p, body string) {
	h.mu.Lock()
	if p == "/api/config/logout" {
		h.sessToken, h.sessUser, h.expires = "", pyjson.Value{}, 0
		h.mu.Unlock()
		ok := pyjson.NewObject()
		ok.Set("ok", pyjson.Value{Kind: pyjson.Bool, B: true})
		rp.json(ok, 200, "dp_config_session=; Path="+h.Prefix+"/; Max-Age=0; SameSite=Lax")
		return
	}
	req, exc := loadsOrEmpty(body)
	if exc == "" && req.Kind != pyjson.Object {
		exc = attrError(req)
	}
	if exc != "" {
		h.mu.Unlock()
		rp.drop(exc)
		return
	}
	user := pycompat.Strip(getOr(req, "user", pyjson.String("")).Str())
	if user == "" || getOr(req, "pass", pyjson.String("")).Str() != h.Login.Password {
		h.mu.Unlock()
		rp.json(errObj("auth"), 401, "")
		return
	}
	h.sessToken, h.sessUser = h.Token(), pyjson.String(user)
	h.expires = h.now() + float64(h.Login.TTL)
	tok := h.sessToken
	h.mu.Unlock()
	v := pyjson.NewObject()
	v.Set("ok", pyjson.Value{Kind: pyjson.Bool, B: true})
	v.Set("user", pyjson.String(user))
	v.Set("ttl", pyjson.IntValue(int64(h.Login.TTL)))
	rp.json(v, 200, "dp_config_session="+tok+"; Path="+h.Prefix+"/; SameSite=Lax")
}

// changeRow builds the change_log entry of a session write (caller holds h.mu).
func (h *Handler) changeRow(req pyjson.Value) *pyjson.Value {
	ord := getOr(req, "ord", pyjson.String("")).Str()
	room, slot, _ := strings.Cut(ord, "/")
	old := pyjson.Value{}
	if h.Mock.Kind == pyjson.Object {
		cand := getOr(h.Mock, ord, getOr(h.Mock, slot, pyjson.NewObject()))
		if cand.Kind == pyjson.Object {
			old = getOr(cand, "v", pyjson.Value{})
		}
	}
	row := pyjson.NewObject()
	row.Set("ts", pyjson.String(h.Clock()))
	row.Set("user_email", h.sessUser)
	row.Set("config_session", pyjson.String(h.sessToken[:min(8, len(h.sessToken))]))
	if room == "" {
		row.Set("room", pyjson.Value{})
	} else {
		row.Set("room", pyjson.String(room))
	}
	if slot == "" {
		row.Set("slot", pyjson.String(ord))
	} else {
		row.Set("slot", pyjson.String(slot))
	}
	row.Set("old_value", old)
	row.Set("new_value", getOr(req, "value", pyjson.Value{}))
	area := "config"
	if strings.HasSuffix(slot, "Mode") {
		area = "control"
	}
	row.Set("area", pyjson.String(area))
	row.Set("surface", pyjson.String("B"))
	return &row
}

// proxy mirrors _proxy_api: relay to the station, never minting the XHR
// header, and relay its status (a refused write must not look accepted).
func (h *Handler) proxy(rp *reply, apiPath string, body []byte, beforeReply func(int)) {
	target := strings.TrimRight(h.LiveBaseURL, "/") + h.Prefix + apiPath
	method := http.MethodGet
	var rd io.Reader
	if body != nil {
		method, rd = http.MethodPost, bytes.NewReader(body)
	}
	code, data, ctype := 0, []byte(nil), "application/json"
	req, err := http.NewRequest(method, target, rd)
	if err == nil {
		req.Header.Set("X-Requested-With", rp.r.Header.Get("X-Requested-With"))
		if body != nil {
			ct := rp.r.Header.Get("Content-Type")
			if _, ok := rp.r.Header["Content-Type"]; !ok {
				ct = "application/json"
			}
			req.Header.Set("Content-Type", ct)
		}
		var resp *http.Response
		resp, err = h.httpClient().Do(req)
		if err == nil {
			data, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			code = resp.StatusCode
			if v := resp.Header.Values("Content-Type"); len(v) > 0 {
				ctype = v[0]
			}
		}
	}
	if err != nil {
		e := pyjson.NewObject()
		e.Set("error", pyjson.String(urlopenError(err)))
		code, data, ctype = 502, []byte(pyjson.Dumps(e, pyDefault)), "application/json"
	}
	if beforeReply != nil {
		beforeReply(code)
	}
	rp.send(code, ctype, data, "")
}

func (h *Handler) httpClient() *http.Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.client == nil {
		h.client = &http.Client{Timeout: 15 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DisableCompression: true}}
	}
	return h.client
}

// urlopenError renders str() of the exception urllib.request.urlopen raises.
func urlopenError(err error) string {
	var dns *net.DNSError
	var errno syscall.Errno
	var ue *url.Error
	switch {
	case errors.As(err, &ue) && ue.Timeout(), errors.Is(err, errTimeout):
		return "<urlopen error timed out>"
	case errors.As(err, &dns):
		if dns.IsNotFound {
			return "<urlopen error [Errno -2] Name or service not known>"
		}
		return "<urlopen error [Errno -3] Temporary failure in name resolution>"
	case errors.As(err, &errno):
		return "<urlopen error " + pycompat.WinOSErrorText(err) + ">"
	}
	return "<urlopen error " + err.Error() + ">"
}

var errTimeout = errors.New("timed out")
