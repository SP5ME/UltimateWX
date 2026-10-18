package mqtt

import (
	"bufio"
	"net"
	"testing"
)

func TestMQTTPacketRoundTrip(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		_ = writePacket(left, 0x30, append(append([]byte{0, 3}, []byte("wx/")...), []byte(`{"device":"test"}`)...))
	}()
	header, payload, err := readPacket(bufio.NewReader(right))
	if err != nil {
		t.Fatal(err)
	}
	if header != 0x30 || string(payload[2:5]) != "wx/" {
		t.Fatalf("unexpected packet 0x%x %q", header, payload)
	}
}
