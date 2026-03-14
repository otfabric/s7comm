package transport

import (
	"context"
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

func TestSendWithNetPipe(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	conn := New(c1, 2*time.Second)
	payload := []byte{0x02, 0xF0, 0x80}
	// Read on other end so Send doesn't block
	done := make(chan []byte, 1)
	go func() {
		raw := make([]byte, 1024)
		n, _ := c2.Read(raw)
		done <- raw[:n]
	}()
	if err := conn.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	raw := <-done
	if len(raw) < 7 {
		t.Fatalf("expected TPKT frame, got %d bytes", len(raw))
	}
	decoded, err := tpkt.Decode(raw)
	if err != nil {
		t.Fatalf("tpkt.Decode: %v", err)
	}
	if len(decoded) != 3 || decoded[0] != 0x02 {
		t.Fatalf("expected payload 0x02 0xF0 0x80, got %v", decoded)
	}
}

func TestSendContext(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	go func() { _, _ = c2.Read(make([]byte, 64)) }() // drain so Send completes
	conn := New(c1, 2*time.Second)
	ctx := context.Background()
	// TPKT requires at least 3-byte payload (min packet length 7)
	if err := conn.SendContext(ctx, []byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("SendContext: %v", err)
	}
}

func TestCloseLocalAddrRemoteAddr(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c2.Close() }()

	conn := New(c1, time.Second)
	if conn.LocalAddr() != nil {
		t.Log("LocalAddr (pipe):", conn.LocalAddr())
	}
	if conn.RemoteAddr() != nil {
		t.Log("RemoteAddr (pipe):", conn.RemoteAddr())
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close is no-op
	_ = conn.Close()
}

func TestSetTracer(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	go func() { _, _ = c2.Read(make([]byte, 64)) }() // drain so Send completes
	conn := New(c1, time.Second)
	conn.SetTracer(nil)
	var traced []string
	conn.SetTracer(&tracerFunc{fn: func(d string, _ []byte) { traced = append(traced, d) }})
	// TPKT requires at least 3-byte payload
	if err := conn.Send([]byte{0x02, 0xF0, 0x80}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(traced) < 1 || traced[0] != "TX" {
		t.Fatalf("expected tracer TX, got %v", traced)
	}
}

type tracerFunc struct {
	fn func(direction string, data []byte)
}

func (t *tracerFunc) Trace(direction string, data []byte) {
	if t.fn != nil {
		t.fn(direction, data)
	}
}

func TestSendContextCancelled(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	conn := New(c1, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := conn.SendContext(ctx, []byte{0x02, 0xF0, 0x80})
	if err == nil {
		t.Fatal("SendContext with cancelled context should return error")
	}
}

func TestReceiveContextCancelled(t *testing.T) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	conn := New(c1, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := conn.ReceiveContext(ctx)
	if err == nil {
		t.Fatal("ReceiveContext with cancelled context should return error")
	}
}

func TestConnWithRealTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tr := New(client, time.Second)
	if tr.LocalAddr() == nil {
		t.Error("LocalAddr should be non-nil for TCP")
	}
	if tr.RemoteAddr() == nil {
		t.Error("RemoteAddr should be non-nil for TCP")
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// BenchmarkSend measures Send throughput over a pipe (writer only; other end drains).
func BenchmarkSend(b *testing.B) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := c2.Read(buf); err != nil {
				return
			}
		}
	}()
	conn := New(c1, 5*time.Second)
	payload := []byte{0x02, 0xF0, 0x80}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conn.Send(payload)
	}
}

// BenchmarkReceive measures Receive throughput over a pipe (reader only; other end writes).
func BenchmarkReceive(b *testing.B) {
	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()
	frame, _ := tpkt.Encode([]byte{0x02, 0xF0, 0x80})
	go func() {
		for {
			if _, err := c2.Write(frame); err != nil {
				return
			}
		}
	}()
	conn := New(c1, 5*time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Receive()
	}
}
