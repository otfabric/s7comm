// Package transport provides TCP connection handling for S7.
package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var ErrConnectionNotEstablished = errors.New("connection not established")

// Conn wraps a TCP connection with S7-specific handling
type Conn struct {
	conn    net.Conn
	timeout time.Duration
	tracer  Tracer
}

// Tracer is an interface for protocol tracing
type Tracer interface {
	Trace(direction string, data []byte)
}

// New creates a new S7 connection wrapper
func New(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{
		conn:    conn,
		timeout: timeout,
	}
}

// SetTracer sets a tracer for protocol debugging
func (c *Conn) SetTracer(t Tracer) {
	c.tracer = t
}

// Send sends a complete frame over the connection
func (c *Conn) Send(data []byte) error {
	return c.SendContext(context.Background(), data)
}

// SendContext sends a complete frame over the connection with context cancellation support.
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
	_, err := c.conn.Write(data)
	return err
}

// Receive reads a complete TPKT frame from the connection
func (c *Conn) Receive() ([]byte, error) {
	return c.ReceiveContext(context.Background())
}

// ReceiveContext reads a complete TPKT frame from the connection with context cancellation support.
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

	// Read TPKT header (4 bytes)
	header := make([]byte, 4)
	_, err := io.ReadFull(c.conn, header)
	if err != nil {
		return nil, fmt.Errorf("read TPKT header: %w", err)
	}

	// Validate TPKT version
	if header[0] != 3 {
		return nil, fmt.Errorf("invalid TPKT version: %d", header[0])
	}

	// Get length and read rest of frame
	length := int(header[2])<<8 | int(header[3])
	if length < 4 {
		return nil, fmt.Errorf("invalid TPKT length: %d", length)
	}

	frame := make([]byte, length)
	copy(frame[:4], header)
	_, err = io.ReadFull(c.conn, frame[4:])
	if err != nil {
		return nil, fmt.Errorf("read TPKT payload: %w", err)
	}

	if c.tracer != nil {
		c.tracer.Trace("RX", frame)
	}
	return frame, nil
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
