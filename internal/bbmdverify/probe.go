package bbmdverify

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Options are the parsed bacnet-bbmd-verify.py arguments.
type Options struct {
	DeviceIP     string
	DevicePrefix int
	LocalBcast   string
	BBMD         string // "" = not given
	Port         int
	SrcPort      int
	Timeout      float64 // seconds
	JSON         bool
}

var colors = map[string]string{"crit": "\033[91m", "high": "\033[93m", "ok": "\033[92m",
	"dim": "\033[90m", "hdr": "\033[96m", "off": "\033[0m"}

// color mirrors _color: always on, like the Python tool (NO_COLOR is ignored).
func color(s, k string) string { return colors[k] + s + colors["off"] }

type frame struct {
	ip   string
	fn   byte
	inst int // -1 = no I-Am
	raw  []byte
}

// collect drains responses until the deadline (Python's collect()).
func collect(conn *net.UDPConn, deadline time.Time, wantIAm bool) []frame {
	var out []frame
	buf := make([]byte, 1500)
	for time.Until(deadline) > 0 {
		_ = conn.SetReadDeadline(deadline)
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		data := append([]byte(nil), buf[:n]...)
		fn, payload, ok := parseBVLC(data)
		if !ok {
			continue
		}
		inst := -1
		if wantIAm {
			inst, _ = extractIAm(fn, payload)
		}
		out = append(out, frame{from.IP.To4().String(), fn, inst, data})
	}
	return out
}

// localIPToward mirrors local_ip_toward: the source address of a UDP connect.
func localIPToward(ip string, port int) (string, error) {
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: port})
	if err != nil {
		return "", err
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// verdict mirrors verdict(): the text and its colour key.
func verdict(onDeviceSubnet, unicast, crossed, anyBBMD bool) (string, string) {
	switch {
	case onDeviceSubnet:
		return "INCONCLUSIVE — this host is ON the device subnet, so its broadcast never crossed a " +
			"router. Run it from the supervisor's subnet, or fix --device-prefix.", "high"
	case crossed || anyBBMD:
		s := "REBIND OK — a broadcast Who-Is reaches the device subnet"
		if anyBBMD {
			s += " (BBMD confirmed)"
		}
		return s + ". Instance-based re-binding will work on IP change.", "ok"
	case unicast:
		return "REBIND WILL FAIL — the device answers UNICAST (pinned IP:port works, like " +
			"ICMP) but NOT the broadcast Who-Is, and no BBMD answered. On an IP change " +
			"Niagara cannot re-resolve it. Add a BBMD/Foreign-Device path or use a static " +
			"IP / DHCP reservation.", "crit"
	}
	return "INCONCLUSIVE — the device did not answer even the directed unicast Who-Is. " +
		"Check --device-ip, --port, firewall, and that you run this on a routed host.", "high"
}

type bbmdResult struct {
	target string
	isBBMD bool
	bdt    []bdtEntry // nil = None
	fdt    []fdtEntry // nil = None
}

type report struct {
	o              Options
	localIP        string
	onDeviceSubnet bool
	uniReachable   bool
	uniInstance    int
	order          []string         // responder IPs in first-seen order
	responders     map[string][]int // instances in add() order
	bbmd           []bbmdResult
}

// Run executes the three probes and prints the report (main()).
func Run(o Options, s cli.Stdio) int {
	if err := checkNetwork(o.DeviceIP, o.DevicePrefix); err != nil {
		fmt.Fprintf(s.Err, "bacnet-bbmd-verify: invalid --device-ip/--device-prefix (%s); "+
			"give an IPv4 literal and a prefix 0-32\n", err)
		return 2
	}
	localIP, err := localIPToward(o.DeviceIP, o.Port)
	if err != nil {
		fmt.Fprintf(s.Err, "bacnet-bbmd-verify: no route from this host to %s (%s); nothing was probed\n",
			o.DeviceIP, pyOSError(err))
		return 2
	}
	r := &report{o: o, localIP: localIP, onDeviceSubnet: inSubnet(localIP, o.DeviceIP, o.DevicePrefix),
		uniInstance: -1, responders: map[string][]int{}}
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = setSockOpts(fd) }); err != nil {
			return err
		}
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf("0.0.0.0:%d", o.SrcPort))
	if err != nil {
		fmt.Fprintln(s.Out, color(fmt.Sprintf("  ! could not bind local UDP :%d (%s). Retry with --src-port 0 (ephemeral).",
			o.SrcPort, pyOSError(err)), "high"))
		return 2
	}
	conn := pc.(*net.UDPConn)
	if !o.JSON {
		fmt.Fprintln(s.Out, color("\n  BACnet/IP BBMD & Who-Is reachability verifier (read-only) — corpus B444\n", "hdr"))
	}
	window := time.Duration(o.Timeout * float64(time.Second))
	// send resolves host on every call, like Python's sock.sendto((host, port)).
	send := func(pkt []byte, host string) error {
		ip, err := resolveHost(host)
		if err != nil {
			return err
		}
		_, err = conn.WriteToUDP(pkt, &net.UDPAddr{IP: ip, Port: o.Port})
		return err
	}
	err = r.probe(conn, send, window)
	conn.Close()
	var gai *gaiError
	if errors.As(err, &gai) {
		fmt.Fprintf(s.Err, "socket.gaierror: %s\n", gai)
		return 1
	}
	if err != nil {
		fmt.Fprintf(s.Err, "OSError: %s\n", pyOSError(err))
		return 1
	}
	return r.print(s)
}

func (r *report) probe(conn *net.UDPConn, send func([]byte, string) error, window time.Duration) error {
	o := r.o
	if err := send(buildWhoIs(nil, false), o.DeviceIP); err != nil {
		return err
	}
	for _, f := range collect(conn, time.Now().Add(window), true) {
		if f.ip == o.DeviceIP && f.inst >= 0 && !r.uniReachable {
			r.uniReachable, r.uniInstance = true, f.inst
		}
	}
	if err := send(buildWhoIs(nil, true), o.LocalBcast); err != nil {
		return err
	}
	for _, f := range collect(conn, time.Now().Add(window), true) {
		if f.inst < 0 {
			continue
		}
		if _, ok := r.responders[f.ip]; !ok {
			r.order = append(r.order, f.ip)
		}
		r.responders[f.ip] = append(r.responders[f.ip], f.inst)
	}
	var targets []string
	for _, t := range []string{o.BBMD, o.DeviceIP} {
		if t != "" && !contains(targets, t) {
			targets = append(targets, t)
		}
	}
	for _, tgt := range targets {
		if err := send(buildReadBDT(), tgt); err != nil {
			return err
		}
		e := bbmdResult{target: tgt}
		for _, f := range collect(conn, time.Now().Add(window), false) {
			if bdt, ok := parseBDTAck(f.raw); ok {
				e.isBBMD, e.bdt = true, bdt
			}
		}
		if e.isBBMD {
			if err := send(buildReadFDT(), tgt); err != nil {
				return err
			}
			for _, f := range collect(conn, time.Now().Add(window), false) {
				if fdt, ok := parseFDTAck(f.raw); ok {
					e.fdt = fdt
				}
			}
		}
		r.bbmd = append(r.bbmd, e)
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// sortedUnique is sorted(set(xs)).
func sortedUnique(xs []int) []int {
	out := pySetOrder(xs)
	sort.Ints(out)
	return out
}

func (r *report) crossed() bool {
	for _, ip := range r.order {
		if inSubnet(ip, r.o.DeviceIP, r.o.DevicePrefix) {
			return true
		}
	}
	return false
}

func (r *report) print(s cli.Stdio) int {
	o := r.o
	anyBBMD := false
	for _, e := range r.bbmd {
		anyBBMD = anyBBMD || e.isBBMD
	}
	text, key := verdict(r.onDeviceSubnet, r.uniReachable, r.crossed(), anyBBMD)
	code := 1
	if key == "ok" {
		code = 0
	}
	if o.JSON {
		fmt.Fprintln(s.Out, r.jsonReport(text))
		return code
	}
	fmt.Fprintf(s.Out, "  1) Directed UNICAST Who-Is -> %s:0x%04X\n", o.DeviceIP, o.Port)
	if r.uniReachable {
		fmt.Fprintln(s.Out, color(fmt.Sprintf("     I-Am received (instance %d). Unicast/routing works — same as ICMP.", r.uniInstance), "ok"))
	} else {
		fmt.Fprintln(s.Out, color("     no I-Am. Device unreachable even by unicast.", "high"))
	}
	fmt.Fprintf(s.Out, "\n  2) Local BROADCAST Who-Is -> %s:0x%04X\n", o.LocalBcast, o.Port)
	if len(r.order) == 0 {
		fmt.Fprintln(s.Out, color("     no responders at all.", "high"))
	}
	ips := append([]string(nil), r.order...)
	sort.Strings(ips)
	for _, ip := range ips {
		tag := color("[local subnet]", "dim")
		if inSubnet(ip, o.DeviceIP, o.DevicePrefix) {
			tag = color("[device subnet — CROSSED]", "ok")
		}
		set := pySetOrder(r.responders[ip])
		parts := make([]string, len(set))
		for i, v := range set {
			parts[i] = fmt.Sprint(v)
		}
		fmt.Fprintf(s.Out, "     %-16s instances {%s} %s\n", ip, strings.Join(parts, ", "), tag)
	}
	fmt.Fprintln(s.Out, "\n  3) BBMD probe (Read-BDT / Read-FDT)")
	for _, e := range r.bbmd {
		if !e.isBBMD {
			fmt.Fprintln(s.Out, color(fmt.Sprintf("     %s did not answer Read-BDT (not a BBMD, or blocked).", e.target), "dim"))
			continue
		}
		fmt.Fprintln(s.Out, color(fmt.Sprintf("     %s IS a BBMD.", e.target), "ok"))
		fmt.Fprintln(s.Out, "       BDT (subnets it distributes broadcasts to):")
		for _, b := range e.bdt {
			fmt.Fprintf(s.Out, "         peer %s  mask %s\n", b.addr, b.mask)
		}
		if len(e.fdt) > 0 {
			fmt.Fprintln(s.Out, "       FDT (registered foreign devices):")
			for _, f := range e.fdt {
				fmt.Fprintf(s.Out, "         %s  ttl %ds  remaining %ds\n", f.addr, f.ttl, f.rem)
			}
		}
	}
	fmt.Fprintf(s.Out, "\n  %s %s\n\n", color("VERDICT:", "hdr"), color(text, key))
	return code
}
