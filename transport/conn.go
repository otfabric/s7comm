// Package transport provides TCP connection handling for S7 using go-tpkt for TPKT framing.
//
// SendContext and ReceiveContext set read/write deadlines from the context deadline and
// the Conn's timeout; no per-I/O goroutine is used. Cancellation is effective when the
// context has a deadline; otherwise I/O may run until the Conn timeout.
//
// SetTracer is not safe for concurrent use with ongoing Send/Receive; call it before
// starting I/O or when the connection is idle.
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

// SetTracer sets a tracer for protocol debugging. Do not call concurrently with Send/Receive.
func (c *Conn) SetTracer(t Tracer) {
	c.tracer = t
}

// Send sends a TPDU payload as one TPKT frame (using go-tpkt).
func (c *Conn) Send(data []byte) error {
	return c.SendContext(context.Background(), data)
}

// SendContext sends a TPDU payload as one TPKT frame. The write deadline is set from
// ctx.Deadline() and the Conn timeout; no goroutine is spawned.
func (c *Conn) SendContext(ctx context.Context, data []byte) error {
	if c.conn == nil {
		return ErrConnectionNotEstablished
	}
	if err := setWriteDeadline(c.conn, c.timeout, ctx); err != nil {
		return err
	}
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

// ReceiveContext reads the next TPKT frame. The read deadline is set from ctx.Deadline()
// and the Conn timeout; no goroutine is spawned.
func (c *Conn) ReceiveContext(ctx context.Context) ([]byte, error) {
	if c.conn == nil {
		return nil, ErrConnectionNotEstablished
	}
	if err := setReadDeadline(c.conn, c.timeout, ctx); err != nil {
		return nil, err
	}
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

// Close closes the connection and nils internal fields so the Conn is in a clear terminal state.
func (c *Conn) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.reader = nil
		c.writer = nil
		return err
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
