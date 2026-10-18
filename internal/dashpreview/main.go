package dashpreview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Prog is the installed command name used in usage and error lines (I4-2).
const Prog = "dashboard-preview"

// Options are the parsed dashboard-preview.py arguments.
type Options struct {
	RC, Prefix     string
	Port           int
	Mock           string
	AutoMock       bool
	Reader, Live   string
	ConfigLogin    bool
	ConfigPassword string
	ConfigTTL      int
	Editor         bool
}

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: Prog, Description: "Local preview server for a Niagara dashboard -ux module."})
	p.Add(argparse.Arg{Default: "./src/rc", Help: "path to the module's rc/ folder (default ./src/rc)"}, "--rc")
	p.Add(argparse.Arg{Default: "/dashboardpan", Help: "servlet mount prefix (default /dashboardpan)"}, "--prefix")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 8080, Help: "port (default 8080)"}, "--port")
	p.Add(argparse.Arg{Help: "JSON file served for every GET /api/<name> (default: empty {})"}, "--mock")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "generate equipment mock by parsing the module's *Reader.java; " +
		"mutually exclusive with --mock and --live"}, "--auto-mock")
	p.Add(argparse.Arg{Metavar: "Reader.java", Help: "path to the *Reader.java when using --auto-mock; " +
		"if omitted the file is auto-located under the sibling src/ of --rc"}, "--reader")
	p.Add(argparse.Arg{Metavar: "BASE_URL", Help: "proxy /api/* GETs AND POSTs (writes) to BASE_URL/<prefix>/api/* (real station), " +
		"relaying the station status code; mutually exclusive with --mock and --auto-mock"}, "--live")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "preview the R14 'second login before any write' flow: injects a native modal + session chip + " +
		"change_log strip and adds stateful /config/login|logout|session mock endpoints; writes need a session"}, "--config-login")
	p.Add(argparse.Arg{Default: "1234", Help: "demo password accepted by the mock login (default 1234)"}, "--config-password")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 300, Help: "mock config-session TTL in seconds, sliding (default 300 = the HMI product default, lead 2026-09-06)"}, "--config-ttl")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "inject a preview-only label editor (drag points/labels, export rx/ry/rlx/rly/rside); " +
		"never touches the module rc/"}, "--editor")
	return p
}

// Main is the dashboard-preview entry point; it serves until SIGINT.
func Main(args []string, s cli.Stdio) int {
	p := newParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	o := Options{RC: ns.String("rc"), Prefix: ns.String("prefix"), Port: ns.Int("port"), Mock: ns.String("mock"),
		AutoMock: ns.Bool("auto_mock"), Reader: ns.String("reader"), Live: ns.String("live"),
		ConfigLogin: ns.Bool("config_login"), ConfigPassword: ns.String("config_password"),
		ConfigTTL: ns.Int("config_ttl"), Editor: ns.Bool("editor")}
	modes := 0
	for _, on := range []bool{o.Mock != "", o.AutoMock, o.Live != ""} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return p.Error(s.Err, "--mock, --auto-mock, and --live are mutually exclusive; pick one")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return Run(ctx, o, s)
}

func randomToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Run validates the options, prints the banner and serves until ctx ends
// (Ctrl+C in Python: KeyboardInterrupt, "detenido.").
func Run(ctx context.Context, o Options, s cli.Stdio) int {
	rcDir, _ := filepath.Abs(o.RC)
	if st, err := os.Stat(rcDir); err != nil || !st.IsDir() {
		fmt.Fprintf(s.Err, "ERROR: --rc no es un directorio: %s\n", rcDir)
		return 1
	}
	prefix := ""
	if t := strings.Trim(o.Prefix, "/"); t != "" {
		prefix = "/" + t
	}
	h := &Handler{RCDir: rcDir, Prefix: prefix, Mock: pyjson.NewObject(), Editor: o.Editor, Log: s.Err,
		Token: randomToken, Clock: func() string { return time.Now().Format("15:04:05") }}
	label := "(vacio)"
	switch {
	case o.Mock != "":
		v, err := loadMock(o.Mock)
		if err != nil {
			fmt.Fprintln(s.Err, err)
			return 1
		}
		h.Mock, label = v, o.Mock
	case o.AutoMock:
		reader := o.Reader
		if reader == "" {
			reader = findReaderJava(rcDir)
		}
		if st, err := os.Stat(reader); reader == "" || err != nil || !st.Mode().IsRegular() {
			fmt.Fprintln(s.Err, "ERROR: --auto-mock: no se encontro *Reader.java. "+
				"Especifica --reader <path> o asegurate de que exista bajo el src/ del modulo.")
			return 1
		}
		src, err := os.ReadFile(reader)
		if err != nil {
			fmt.Fprintf(s.Err, "OSError: %v\n", err)
			return 1
		}
		ents, nums, labels, err := classifyReaderArrays(parseStringArrays(pycompat.DecodeReplace(src)))
		if err != nil {
			fmt.Fprintln(s.Err, err)
			return 1
		}
		if len(ents) == 0 {
			fmt.Fprintf(s.Err, "ERROR: --auto-mock: no se pudo extraer el array de entidades de %s\n", reader)
			return 1
		}
		h.Mock = generateAutoMock(ents, nums, labels)
		label = fmt.Sprintf("auto-mock desde %s (%d entidades, %d slots numericos, %d label slots)",
			filepath.Base(reader), len(ents), len(nums), len(labels))
		fmt.Fprintf(s.Err, "  [auto-mock] entidades: %s\n", pycompat.Repr(ents))
		fmt.Fprintf(s.Err, "  [auto-mock] num_slots:  %s\n", pycompat.Repr(nums))
		fmt.Fprintf(s.Err, "  [auto-mock] label_slots:%s\n", pycompat.Repr(labels))
		fmt.Fprintf(s.Err, "  [auto-mock] keys generadas: %d\n", len(h.Mock.Keys))
	case o.Live != "":
		h.LiveBaseURL = strings.TrimRight(o.Live, "/")
		label = "live proxy -> " + h.LiveBaseURL
	default:
		fmt.Fprintln(s.Err, "  (sin --mock/--auto-mock/--live: /api/* devuelve {} — el diseño/paleta se ve, los valores salen '--')")
	}
	if o.ConfigLogin {
		h.Login = &LoginConfig{Password: o.ConfigPassword, TTL: o.ConfigTTL}
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", o.Port))
	if err != nil {
		fmt.Fprintf(s.Err, "OSError: %s\n", pycompat.WinOSErrorText(err))
		return 1
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 30 * time.Second}
	fmt.Fprintf(s.Out, "dashboard-preview -> http://localhost:%d%s/\n", o.Port, prefix)
	fmt.Fprintf(s.Out, "  serving: %s\n", rcDir)
	fmt.Fprintf(s.Out, "  mock:    %s\n", label)
	if o.Editor {
		fmt.Fprintln(s.Out, "  editor:  ON (editor de labels inyectado; NO toca el modulo)")
	}
	if h.Login != nil {
		fmt.Fprintf(s.Out, "  config-login: ON — cualquier usuario + contrasena '%s', sesion %ds (deslizante); "+
			"escribir sin sesion -> 403 + modal; Salir cierra la sesion\n", h.Login.Password, h.Login.TTL)
	}
	fmt.Fprintln(s.Out, "  Ctrl+C para detener. Edita rc/ y refresca (sin recompilar).")
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(s.Err, "OSError: %v\n", err)
		return 1
	}
	fmt.Fprintln(s.Out, "\ndetenido.")
	return 0
}

// loadMock mirrors open(path, encoding="utf-8") + json.load, whose failures
// are uncaught in Python (traceback, exit 1): Go prints the final line.
func loadMock(path string) (pyjson.Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pyjson.Value{}, fmt.Errorf("%s: %s", pycompat.OSErrorName(err), pycompat.OSErrorText(err))
	}
	text := []rune(pycompat.DecodeReplace(data))
	if pycompat.DecodeReplace(data) != string(data) {
		return pyjson.Value{}, errors.New("UnicodeDecodeError: 'utf-8' codec can't decode the mock file")
	}
	v, err := pyjson.Loads(text)
	var re *pyjson.RecursionError
	if errors.As(err, &re) {
		return pyjson.Value{}, fmt.Errorf("RecursionError: %v", err)
	}
	if err != nil {
		return pyjson.Value{}, fmt.Errorf("json.decoder.JSONDecodeError: %v", err)
	}
	return v, nil
}
