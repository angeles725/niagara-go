package dashpreview

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// LoginConfig is the --config-login mock (password, sliding TTL seconds).
type LoginConfig struct {
	Password string
	TTL      int
}

// Handler mirrors make_handler: it serves rc/, mocks or proxies /api/*, and
// optionally injects the label editor and the R14 config-login flow.
type Handler struct {
	RCDir, Prefix string
	Mock          pyjson.Value
	Editor        bool
	Login         *LoginConfig
	LiveBaseURL   string
	Log           io.Writer

	// Test seams (time.time, secrets.token_hex(16), time.strftime("%H:%M:%S")).
	Now   func() float64
	Token func() string
	Clock func() string

	mu        sync.Mutex
	logMu     sync.Mutex
	sessToken string
	sessUser  pyjson.Value
	expires   float64
	changeLog []pyjson.Value
	client    *http.Client
}

func (h *Handler) logf(format string, a ...any) {
	h.logMu.Lock()
	defer h.logMu.Unlock()
	fmt.Fprintf(h.Log, format, a...)
}

// reply is one response: BaseHTTPRequestHandler logs the request line when
// the status is sent.
type reply struct {
	w    http.ResponseWriter
	r    *http.Request
	h    *Handler
	line string
}

func (rp *reply) status(code int) {
	rp.h.logf("  \"%s\" %d -\n", rp.line, code)
}

// send mirrors _send: status, type, length, no-store, optional cookie.
func (rp *reply) send(code int, ctype string, data []byte, cookie string) {
	rp.status(code)
	hd := rp.w.Header()
	hd.Set("Content-Type", ctype)
	hd.Set("Content-Length", strconv.Itoa(len(data)))
	hd.Set("Cache-Control", "no-store")
	if cookie != "" {
		hd.Set("Set-Cookie", cookie)
	}
	rp.w.WriteHeader(code)
	_, _ = rp.w.Write(data)
}

func (rp *reply) json(v pyjson.Value, code int, cookie string) {
	rp.send(code, "application/json", []byte(pyjson.Dumps(v, pyDefault)), cookie)
}

// pyDefault is json.dumps' default: ", " and ": " separators, ensure_ascii.
var pyDefault = pyjson.DumpOpts{EnsureASCII: true, Indent: -1, ItemSep: ", ", KeySep: ": "}

func (rp *reply) redirect(location string) {
	rp.status(302)
	rp.w.Header().Set("Location", location)
	rp.w.WriteHeader(302)
}

// htmlEscape mirrors html.escape(s, quote=False).
var htmlEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

var reasons = map[int][2]string{
	404: {"Not Found", "Nothing matches the given URI"},
	501: {"Not Implemented", "Server does not support this operation"},
}

// sendError mirrors BaseHTTPRequestHandler.send_error.
func (rp *reply) sendError(code int, message string) {
	r := reasons[code]
	if message == "" {
		message = r[0]
	}
	rp.h.logf("  code %d, message %s\n", code, message)
	rp.status(code)
	body := errorPage[0] + strconv.Itoa(code) + errorPage[1] + htmlEscape(message) +
		errorPage[2] + strconv.Itoa(code) + errorPage[3] + htmlEscape(r[1]) + errorPage[4]
	hd := rp.w.Header()
	hd.Set("Connection", "close")
	hd.Set("Content-Type", "text/html;charset=utf-8")
	hd.Set("Content-Length", strconv.Itoa(len(body)))
	rp.w.WriteHeader(code)
	_, _ = io.WriteString(rp.w, body)
}

// drop mirrors an exception escaping the handler: socketserver prints the
// error and closes the connection without a response.
func (rp *reply) drop(err string) {
	rp.h.logf("----------------------------------------\nException occurred during processing of request from %s\n%s\n----------------------------------------\n",
		pyAddr(rp.r.RemoteAddr), err)
	if hj, ok := rp.w.(http.Hijacker); ok {
		if c, _, e := hj.Hijack(); e == nil {
			c.Close()
			return
		}
	}
	panic(http.ErrAbortHandler)
}

func pyAddr(addr string) string {
	i := strings.LastIndexByte(addr, ':')
	if i < 0 {
		return addr
	}
	return fmt.Sprintf("('%s', %s)", strings.Trim(addr[:i], "[]"), addr[i+1:])
}

func (h *Handler) now() float64 {
	if h.Now != nil {
		return h.Now()
	}
	return float64(time.Now().UnixNano()) / 1e9
}

func (h *Handler) cookieToken(r *http.Request) (string, bool) {
	for _, part := range strings.Split(r.Header.Get("Cookie"), ";") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		if k == "dp_config_session" {
			return v, true
		}
	}
	return "", false
}

// sessActive mirrors _sess_active (caller holds h.mu).
func (h *Handler) sessActive(r *http.Request) bool {
	tok, ok := h.cookieToken(r)
	return h.sessToken != "" && h.now() < h.expires && ok && tok == h.sessToken
}

func (h *Handler) strip(path string) string {
	path, _, _ = strings.Cut(path, "?")
	if h.Prefix != "" && strings.HasPrefix(path, h.Prefix) {
		path = path[len(h.Prefix):]
	}
	if path == "" {
		return "/"
	}
	return path
}

// serveFile mirrors _serve_file.
func (h *Handler) serveFile(rp *reply, rel string) {
	rel = strings.TrimLeft(rel, "/")
	if rel == "" {
		rel = "index.html"
	}
	if strings.Contains(rel, "..") || strings.HasPrefix(rel, `\`) {
		rp.sendError(404, "")
		return
	}
	full := filepath.Join(append([]string{h.RCDir}, strings.Split(rel, "/")...)...)
	if strings.HasSuffix(rel, "/") {
		full += string(filepath.Separator)
	}
	st, err := os.Stat(full)
	if err != nil || !st.Mode().IsRegular() || strings.HasSuffix(rel, "/") {
		rp.sendError(404, "Not found: "+rel)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		rp.drop(fmt.Sprintf("OSError: %v", err))
		return
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(full)))
	if strings.Trim(strings.TrimSuffix(filepath.Base(full), filepath.Ext(full)), ".") == "" {
		ext = ""
	}
	if (h.Editor || h.Login != nil) && ext == ".html" {
		data = h.inject(data)
	}
	ctype, ok := mimeTypes[ext]
	if !ok {
		ctype = "application/octet-stream"
	}
	rp.send(200, ctype, data, "")
}

// inject inserts the snippets before the last </body> (case-insensitive), or appends them.
func (h *Handler) inject(data []byte) []byte {
	txt := []rune(pycompat.DecodeReplace(data))
	snip := ""
	if h.Editor {
		snip += editorSnippet
	}
	if h.Login != nil {
		snip += strings.ReplaceAll(configLoginSnippet, "__PREFIX__", h.Prefix)
	}
	low := make([]rune, len(txt))
	for i, r := range txt {
		low[i] = unicode.ToLower(r)
	}
	idx := strings.LastIndex(string(low), "</body>")
	if idx < 0 {
		return []byte(string(txt) + snip)
	}
	at := len([]rune(string(low)[:idx]))
	return []byte(string(txt[:at]) + snip + string(txt[at:]))
}

func (h *Handler) hmi(rp *reply) {
	rp.send(200, "text/html; charset=utf-8", []byte(hmiHead+h.Prefix+hmiTail), "")
}

func (h *Handler) hasXHR(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// ServeHTTP dispatches like BaseHTTPRequestHandler: do_GET, do_POST, else 501.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp := &reply{w: w, r: r, h: h, line: r.Method + " " + r.RequestURI + " " + r.Proto}
	switch r.Method {
	case http.MethodGet:
		h.doGET(rp)
	case http.MethodPost:
		h.doPOST(rp)
	default:
		rp.sendError(501, "Unsupported method ("+pyjson.StrRepr(r.Method)+")")
	}
}
