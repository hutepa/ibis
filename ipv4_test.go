package ibis

import (
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/songgao/ibis/ibisutil"
)

const BUFFERSIZE = 1522

func startRead(ch chan<- []byte, ifce *Interface) {
	go func() {
		for {
			buffer := make([]byte, BUFFERSIZE)
			n, err := ifce.Read(buffer)
			if err == nil {
				buffer = buffer[:n:n]
				ch <- buffer
			}
		}
	}()
}

func startBroadcast(t *testing.T, dst net.IP) {
	if err := exec.Command("ping", "-b", "-c", "2", dst.String()).Start(); err != nil {
		t.Fatal(err)
	}
}

func TestBroadcast(t *testing.T) {
	var (
		self = net.IPv4(10, 0, 42, 1)
		mask = net.IPv4Mask(255, 255, 255, 0)
		brd  = net.IPv4(10, 0, 42, 255)
	)

	ifce, err := NewTAP("test")
	if err != nil {
		t.Fatalf("creating TAP error: %v\n", err)
	}

	setupIfce(t, net.IPNet{IP: self, Mask: mask}, ifce.Name())
	startBroadcast(t, brd)

	dataCh := make(chan []byte, 8)
	startRead(dataCh, ifce)

	timeout := time.NewTimer(8 * time.Second).C

readFrame:
	for {
		select {
		case buffer := <-dataCh:
			ethertype := ibisutil.MACEthertype(buffer)
			if ethertype != ibisutil.IPv4 {
				continue readFrame
			}
			if !ibisutil.IsBroadcast(ibisutil.MACDestination(buffer)) {
				continue readFrame
			}
			packet := ibisutil.MACPayload(buffer)
			if !ibisutil.IsIPv4(packet) {
				continue readFrame
			}
			if !ibisutil.IPv4Source(packet).Equal(self) {
				continue readFrame
			}
			if !ibisutil.IPv4Destination(packet).Equal(brd) {
				continue readFrame
			}
			if ibisutil.IPv4Protocol(packet) != ibisutil.ICMP {
				continue readFrame
			}
			t.Logf("received broadcast frame: %#v\n", buffer)
			break readFrame
		case <-timeout:
			t.Fatal("Waiting for broadcast packet timeout")
		}
	}
}
