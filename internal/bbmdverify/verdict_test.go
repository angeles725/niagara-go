package bbmdverify

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestVerdictBBMDAloneIsNotOK: a BBMD answering Read-BDT does not show that
// the broadcast reached the device subnet; only an I-Am from that subnet does.
func TestVerdictBBMDAloneIsNotOK(t *testing.T) {
	for _, unicast := range []bool{false, true} {
		text, key := verdict(false, unicast, false, true)
		if key == "ok" || !strings.HasPrefix(text, "REBIND NOT CONFIRMED") {
			t.Errorf("unicast=%v, BBMD, no cross: %s %q", unicast, key, text)
		}
	}
	if text, key := verdict(false, false, true, true); key != "ok" || !strings.Contains(text, "(BBMD confirmed)") {
		t.Errorf("crossed + BBMD: %s %q", key, text)
	}
}

// TestForwardedIAmFromDeviceSubnetCrosses: through BBMDs the device's I-Am
// reaches us as a Forwarded-NPDU from the local BBMD; its originating
// B/IP address, not the UDP source, says it came from the device subnet.
func TestForwardedIAmFromDeviceSubnetCrosses(t *testing.T) {
	fwd := func(orig string) frame {
		return frame{ip: "192.168.1.1", fn: fnForwardedNPDU, inst: 1234, orig: addr6(orig, 0xBAC0)}
	}
	r := &report{o: Options{DeviceIP: "10.2.3.4", DevicePrefix: 24}, responders: map[string][]int{}}
	r.addBroadcast([]frame{fwd("10.9.9.9")})
	if r.crossed() {
		t.Error("forwarded I-Am from another subnet counted as crossed")
	}
	r.addBroadcast([]frame{fwd("10.2.3.77")})
	if !r.crossed() {
		t.Error("forwarded I-Am from the device subnet not counted as crossed")
	}
	if got := r.responders["192.168.1.1"]; len(got) != 2 {
		t.Errorf("responders keyed by UDP source like Python: %v", r.responders)
	}
}

// TestForwardedIAmThroughCollect drives the broadcast leg end to end: a
// loopback fake BBMD forwards the device's I-Am as a Forwarded-NPDU, collect
// reads it off the socket, and the verdict follows the originating address.
func TestForwardedIAmThroughCollect(t *testing.T) {
	for _, tc := range []struct{ orig, want string }{
		{"10.2.3.77", "REBIND OK"},
		{"10.9.9.9", "REBIND NOT CONFIRMED"},
	} {
		t.Run(tc.orig, func(t *testing.T) {
			loop := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
			conn, err := net.ListenUDP("udp4", loop)
			if err != nil {
				t.Skipf("loopback UDP unavailable: %v", err)
			}
			defer conn.Close()
			bbmd, err := net.ListenUDP("udp4", loop)
			if err != nil {
				t.Skipf("loopback UDP unavailable: %v", err)
			}
			defer bbmd.Close()
			pkt := bvlcWrap(fnForwardedNPDU, cat(addr6(tc.orig, 0xBAC0), []byte{1, 0}, iam(1234, 8)))
			if _, err := bbmd.WriteToUDP(pkt, conn.LocalAddr().(*net.UDPAddr)); err != nil {
				t.Fatal(err)
			}
			frames := collect(conn, time.Now().Add(300*time.Millisecond), true)
			if len(frames) != 1 || frames[0].inst != 1234 || frames[0].ip != "127.0.0.1" {
				t.Fatalf("collect: %+v", frames)
			}
			r := &report{o: Options{DeviceIP: "10.2.3.4", DevicePrefix: 24}, responders: map[string][]int{}}
			r.addBroadcast(frames)
			text, _ := verdict(false, false, r.crossed(), true)
			if !strings.HasPrefix(text, tc.want) {
				t.Errorf("orig %s: verdict %q, want prefix %q", tc.orig, text, tc.want)
			}
		})
	}
}
