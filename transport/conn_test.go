package transport

import (
	"net"
	"testing"
	"time"

	"otfabric/s7comm/wire"
)

func TestSendReceiveWithNetPipe(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	conn := New(c1, 2*time.Second)

	go func() {
		frame := wire.EncodeTPKT([]byte{0x02, 0xF0, 0x80})
		_, _ = c2.Write(frame)
	}()

	frame, err := conn.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if len(frame) < 4 {
		t.Fatalf("short frame: %d", len(frame))
	}

}
