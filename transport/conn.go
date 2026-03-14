// Package transport provides TCP connection handling for S7 using go-tpkt for TPKT framing.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/otfabric/go-tpkt"
)

var ErrConnectionNotEstablished = errors.New("connection not established")

// Conn wraps a TCP connection with TPKT framing (go-tpkt) for S7/COTP payloads.
type Conn struct {
	conn    net.Conn
	reader  *tpkt.Reader
	writer  *tpkt.Writer
	timeout time.Duration
	tracer  Tracer
}

// Tracer is an interface for protocol tracing
type Tracer interface {
	Trace(direction string, data []byte)
}

// New creates a new S7 connection wrapper. Send accepts TPDU payload (e.g. COTP bytes);
// Receive returns the TPDU payload of the next TPKT frame.
func New(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{
		conn:    conn,
		reader:  tpkt.NewReader(conn),
		writer:  tpkt.NewWriter(conn),
		timeout: timeout,
	}
}

// SetTracer sets a tracer for protocol debugging
func (c *Conn) SetTracer(t Tracer) {
	c.tracer = t
}

// Send sends a TPDU payload as one TPKT frame (using go-tpkt).
func (c *Conn) Send(data []byte) error {
	return c.SendContext(context.Background(), data)
}

// SendContext sends a TPDU payload as one TPKT frame with context cancellation support.
func (c *Conn) SendContext(ctx context.Context, data []byte) error {
	if c.conn == nil {
		return ErrConnectionNotEstablished
	}
	if err := setWriteDeadline(c.conn, c.timeout, ctx); err != nil {
		return err
	}
	done := cancelWriteOnContextDone(c.conn, ctx)
	defer close(done)

	if err := ctx.Err(); err != nil {
		return err
	}
	if c.tracer != nil {
		c.tracer.Trace("TX", data)
	}
	_, err := c.writer.WriteFrame(data)
	return err
}

// Receive reads the next TPKT frame and returns its payload (e.g. COTP TPDU).
func (c *Conn) Receive() ([]byte, error) {
	return c.ReceiveContext(context.Background())
}

// ReceiveContext reads the next TPKT frame with context cancellation and returns its payload.
func (c *Conn) ReceiveContext(ctx context.Context) ([]byte, error) {
	if c.conn == nil {
		return nil, ErrConnectionNotEstablished
	}
	if err := setReadDeadline(c.conn, c.timeout, ctx); err != nil {
		return nil, err
	}
	done := cancelReadOnContextDone(c.conn, ctx)
	defer close(done)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	payload, err := c.reader.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read TPKT frame: %w", err)
	}
	if c.tracer != nil {
		c.tracer.Trace("RX", payload)
	}
	return payload, nil
}

func setReadDeadline(conn net.Conn, timeout time.Duration, ctx context.Context) error {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || d.Before(deadline) {
			deadline = d
		}
	}
	return conn.SetReadDeadline(deadline)
}

func setWriteDeadline(conn net.Conn, timeout time.Duration, ctx context.Context) error {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || d.Before(deadline) {
			deadline = d
		}
	}
	return conn.SetWriteDeadline(deadline)
}

func cancelReadOnContextDone(conn net.Conn, ctx context.Context) chan struct{} {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetReadDeadline(time.Now())
		case <-done:
		}
	}()
	return done
}

func cancelWriteOnContextDone(conn net.Conn, ctx context.Context) chan struct{} {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetWriteDeadline(time.Now())
		case <-done:
		}
	}()
	return done
}

// Close closes the connection
func (c *Conn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	if c.conn != nil {
		return c.conn.LocalAddr()
	}
	return nil
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}
	return nil
}
