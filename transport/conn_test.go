package transport

import (
	"net"
	"testing"
	"time"

	"github.com/otfabric/go-tpkt"
)

func TestSendReceiveWithNetPipe(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	conn := New(c1, 2*time.Second)

	// Other end sends one TPKT frame (COTP DT minimal payload)
	go func() {
		payload := []byte{0x02, 0xF0, 0x80}
		frame, _ := tpkt.Encode(payload)
		_, _ = c2.Write(frame)
	}()

	payload, err := conn.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	// Receive returns TPKT payload only (COTP bytes)
	if len(payload) != 3 {
		t.Fatalf("expected payload len 3, got %d", len(payload))
	}
}
