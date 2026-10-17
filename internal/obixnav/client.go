package obixnav

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// httpError is urllib's HTTPError: str() is "HTTP Error <code>: <reason>".
type httpError struct {
	code   int
	reason string
}

func (e *httpError) Error() string { return fmt.Sprintf("HTTP Error %d: %s", e.code, e.reason) }

// urlError is urllib's URLError: str() is "<urlopen error <reason>>".
type urlError struct{ reason string }

func (e *urlError) Error() string { return "<urlopen error " + e.reason + ">" }

// baseError is a 404 on the top-level container a probe hangs from: a wrong --base (exit 2).
type baseError string

func (e baseError) Error() string { return string(e) }

func strerror(errno syscall.Errno) string {
	msg := errno.Error()
	if msg == "" {
		return msg
	}
	return strings.ToUpper(msg[:1]) + msg[1:]
}

// netReason renders a transport failure the way urllib's URLError reason prints.
func netReason(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timed out"
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return fmt.Sprintf("[Errno %d] %s", int(errno), strerror(errno))
	}
	return err.Error()
}

// client talks to one station: HTTP Basic as user API, TLS verification off (self-signed JACE).
type client struct {
	base      string
	auth      string
	clients   map[time.Duration]*http.Client // one per timeout, see httpClient
	anchorsOK map[string]bool
}

func newClient(base, password string) *client {
	return &client{base: base, auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("API:"+password)),
		clients: map[time.Duration]*http.Client{}, anchorsOK: map[string]bool{}}
}

// deadlineConn re-arms the deadline before every read and write: like a Python socket timeout it
// bounds each operation, so a slow response that keeps arriving never times out.
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c *deadlineConn) Write(p []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

// httpClient returns the client for one urlopen timeout. urllib applies its timeout to every
// socket operation (connect, TLS handshake, each send and recv), never to the whole request, so
// there is no http.Client.Timeout: the dialer, the handshake and the wait for the headers are
// bounded by timeout, and deadlineConn bounds every read of the body.
func (c *client) httpClient(timeout time.Duration) *http.Client {
	if hc, ok := c.clients[timeout]; ok {
		return hc
	}
	dialer := &net.Dialer{Timeout: timeout}
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- self-signed station cert, read-only
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &deadlineConn{Conn: conn, timeout: timeout}, nil
		}}
	hc := &http.Client{Transport: tr}
	c.clients[timeout] = hc
	return hc
}

// fetch performs one request; a non-2xx status is an *httpError, a transport failure a *urlError.
func (c *client) fetch(method, url string, body []byte, timeout time.Duration) (string, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		return "", &urlError{err.Error()}
	}
	req.Header.Set("Authorization", c.auth)
	if body != nil {
		req.Header.Set("Content-Type", "text/xml")
	}
	resp, err := c.httpClient(timeout).Do(req)
	if err != nil {
		return "", &urlError{netReason(err)}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		reason := strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode))
		return "", &httpError{resp.StatusCode, strings.TrimPrefix(reason, " ")}
	}
	if err != nil {
		return "", &urlError{netReason(err)}
	}
	return pycompat.DecodeReplace(data), nil
}

func (c *client) getConfig(path string) (string, error) {
	return c.fetch(http.MethodGet, c.base+"/config"+path+"/", nil, 15*time.Second)
}

func is404(err error) bool {
	var he *httpError
	return errors.As(err, &he) && he.code == 404
}

// checkAnchor fails with a baseError unless the top-level container of path exists.
func (c *client) checkAnchor(path string) error {
	trimmed := strings.Trim(path, "/")
	anchor := "/" + strings.SplitN(trimmed, "/", 2)[0]
	if c.anchorsOK[anchor] {
		return nil
	}
	var err error
	if anchor == "/"+trimmed {
		err = &httpError{404, "Not Found"}
	} else {
		_, err = c.getConfig(anchor)
	}
	if is404(err) {
		return baseError(fmt.Sprintf("404 on %s/config%s/ itself: wrong --base or the station has no such tree",
			c.base, anchor))
	}
	if err != nil {
		return err
	}
	c.anchorsOK[anchor] = true
	return nil
}

// get returns the body of <base>/config<path>/, or "" when the component is absent (404, noted on
// warn) while its top-level container exists. Every other failure propagates: an unreachable or
// misaddressed station must never read as an empty component.
func (c *client) get(path string, warn io.Writer) (string, error) {
	body, err := c.getConfig(path)
	if !is404(err) {
		return body, err
	}
	if err := c.checkAnchor(path); err != nil {
		return "", err
	}
	fmt.Fprintf(warn, "obix-nav: 404 %s/config%s/ (component absent; read as empty)\n", c.base, path)
	return "", nil
}

// result is one Batch read: the tag and the val attribute (nil for an err or a missing val).
type result struct {
	tag string
	val *string
}

// batchOut keeps the Python dict's insertion order.
type batchOut struct {
	keys []string
	m    map[string]result
}

func (b *batchOut) get(o string) result {
	if r, ok := b.m[o]; ok {
		return r
	}
	return result{tag: "err"}
}

var (
	batchElemRE = regexp.MustCompile(`<(real|bool|int|reltime|str|abstime|enum|err)\b([^>]*)/?>`)
	batchHrefRE = regexp.MustCompile(`href="[^"]*?/config([^"]*?)/?"`)
	batchValRE  = regexp.MustCompile(`\bval="([^"]*)"`)
)

// batch POSTs one oBIX BatchIn of Reads. Batch URIs are absolute server paths that keep /obix.
func (c *client) batch(ords []string) (*batchOut, error) {
	prefix := c.base
	if i := strings.LastIndex(c.base, "/obix"); i >= 0 {
		prefix = c.base[:i]
	} else if prefix != "" {
		prefix = prefix[:len(prefix)-1] // Python base[:-1]
	}
	prefix += "/obix"
	var items strings.Builder
	for _, o := range ords {
		fmt.Fprintf(&items, `<uri is="obix:Read" val="%s/config%s"/>`, prefix, o)
	}
	body := []byte(`<list is="obix:BatchIn">` + items.String() + "</list>")
	xml, err := c.fetch(http.MethodPost, c.base+"/batch/", body, 25*time.Second)
	if err != nil {
		return nil, err
	}
	out := &batchOut{m: map[string]result{}}
	for _, m := range batchElemRE.FindAllStringSubmatch(xml, -1) {
		tag, attrs := m[1], m[2]
		h := batchHrefRE.FindStringSubmatch(attrs)
		if h == nil {
			continue
		}
		if _, dup := out.m[h[1]]; dup {
			continue
		}
		r := result{tag: tag}
		if v := batchValRE.FindStringSubmatch(attrs); v != nil && tag != "err" {
			r.val = &v[1]
		}
		out.keys = append(out.keys, h[1])
		out.m[h[1]] = r
	}
	for _, o := range ords {
		if _, ok := out.m[o]; !ok {
			out.keys = append(out.keys, o)
			out.m[o] = result{tag: "err"}
		}
	}
	return out, nil
}

// value is num(): None, a float, or the text when float() refuses it.
type value struct {
	kind byte // 'n', 'f' or 's'
	f    float64
	s    string
}

func num(r result) value {
	if r.val == nil {
		return value{kind: 'n'}
	}
	if f, err := pycompat.ParseFloat(*r.val); err == nil {
		return value{kind: 'f', f: f}
	}
	return value{kind: 's', s: *r.val}
}

// str is Python str() of the value.
func (v value) str() string {
	switch v.kind {
	case 'n':
		return "None"
	case 'f':
		return pycompat.FloatRepr(v.f)
	}
	return v.s
}
