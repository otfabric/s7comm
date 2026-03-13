package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/otfabric/s7comm/model"
	"github.com/otfabric/s7comm/transport"
	"github.com/otfabric/s7comm/wire"
)

// Client is an S7 protocol client
type Client struct {
	host          string
	opts          *options
	conn          *transport.Conn
	reqMu         sync.Mutex
	mu            sync.RWMutex
	pduRef        uint32
	localTSAP     uint16
	remoteTSAP    uint16
	pduSize       int
	maxAmqCalling int
	maxAmqCalled  int
}

// New creates a new S7 client for the given host
func New(host string, opts ...Option) *Client {
	o := defaultOptions()
	for _, f := range opts {
		f(o)
	}
	return &Client{host: host, opts: o}
}

func (c *Client) Connect(ctx context.Context) error {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.opts.autoRackSlot && !c.opts.useExplicit {
		return c.autoConnect(ctx)
	}

	return c.connectOnce(ctx, c.opts.rack, c.opts.slot)
}

func (c *Client) autoConnect(ctx context.Context) error {
	if c.opts.bruteRackSlot {
		for rack := 0; rack <= 3; rack++ {
			for slot := 0; slot <= 5; slot++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := c.connectOnce(ctx, rack, slot); err == nil {
					c.opts.rack = rack
					c.opts.slot = slot
					return nil
				}
			}
		}
		return fmt.Errorf("auto rack/slot connect failed for rack 0-3 slot 0-5")
	}

	common := [][2]int{{0, 1}, {0, 2}, {0, 0}, {1, 1}, {0, 3}}
	for _, rs := range common {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.connectOnce(ctx, rs[0], rs[1]); err == nil {
			c.opts.rack = rs[0]
			c.opts.slot = rs[1]
			return nil
		}
	}

	return fmt.Errorf("auto rack/slot connect failed on common rack/slot pairs")
}

func (c *Client) connectOnce(ctx context.Context, rack, slot int) error {
	if c.conn != nil {
		_ = c.closeConnLocked()
	}

	addr := net.JoinHostPort(c.host, fmt.Sprint(c.opts.port))
	dialer := net.Dialer{Timeout: c.opts.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("TCP connect: %w", err)
	}

	c.conn = transport.New(conn, c.opts.timeout)
	c.opts.rack = rack
	c.opts.slot = slot

	// COTP connection
	if err := c.cotpConnect(ctx); err != nil {
		_ = c.closeConnLocked()
		return fmt.Errorf("COTP connect: %w", err)
	}

	// S7 setup communication
	setup, err := c.s7Setup(ctx)
	if err != nil {
		_ = c.closeConnLocked()
		return fmt.Errorf("S7 setup: %w", err)
	}
	c.pduSize = setup.PDUSize
	c.maxAmqCalling = setup.MaxAmqCalling
	c.maxAmqCalled = setup.MaxAmqCalled

	return nil
}

func (c *Client) closeConnLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) cotpConnect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var localTSAP, remoteTSAP uint16
	if c.opts.useExplicit {
		localTSAP = c.opts.localTSAP
		remoteTSAP = c.opts.remoteTSAP
	} else {
		localTSAP = wire.BuildTSAP(1, 0, 0)
		remoteTSAP = wire.BuildTSAP(3, c.opts.rack, c.opts.slot)
	}
	c.localTSAP = localTSAP
	c.remoteTSAP = remoteTSAP

	cr := wire.EncodeCOTPCR(localTSAP, remoteTSAP)
	frame := wire.EncodeTPKT(cr)

	if err := c.conn.SendContext(ctx, frame); err != nil {
		return err
	}

	resp, err := c.conn.ReceiveContext(ctx)
	if err != nil {
		return err
	}

	_, cotpData, err := wire.ParseTPKT(resp)
	if err != nil {
		return err
	}

	cotp, _, err := wire.ParseCOTP(cotpData)
	if err != nil {
		return err
	}

	if cotp.PDUType != wire.COTPTypeCC {
		return fmt.Errorf("expected COTP CC, got 0x%02X", cotp.PDUType)
	}

	return nil
}

func (c *Client) s7Setup(ctx context.Context) (*wire.SetupCommResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req := wire.EncodeSetupCommRequest(c.opts.maxAmqCalling, c.opts.maxAmqCalled, c.opts.maxPDU)
	cotp := wire.EncodeCOTPData()
	frame := wire.EncodeTPKT(append(cotp, req...))

	if err := c.conn.SendContext(ctx, frame); err != nil {
		return nil, err
	}

	resp, err := c.conn.ReceiveContext(ctx)
	if err != nil {
		return nil, err
	}

	_, cotpData, err := wire.ParseTPKT(resp)
	if err != nil {
		return nil, err
	}

	_, s7Data, err := wire.ParseCOTP(cotpData)
	if err != nil {
		return nil, err
	}

	header, paramData, err := wire.ParseS7Header(s7Data)
	if err != nil {
		return nil, err
	}

	if header.ErrorClass != 0 || header.ErrorCode != 0 {
		return nil, wire.NewS7Error(header.ErrorClass, header.ErrorCode)
	}

	setup, err := wire.ParseSetupCommResponse(paramData)
	if err != nil {
		return nil, err
	}

	return setup, nil
}

func (c *Client) nextPDURef() uint16 {
	return uint16(atomic.AddUint32(&c.pduRef, 1))
}

func (c *Client) sendReceive(ctx context.Context, req []byte) ([]byte, []byte, error) {
	cotp := wire.EncodeCOTPData()
	frame := wire.EncodeTPKT(append(cotp, req...))
	if err := c.conn.SendContext(ctx, frame); err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	resp, err := c.conn.ReceiveContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	_, cotpData, err := wire.ParseTPKT(resp)
	if err != nil {
		return nil, nil, err
	}

	_, s7Data, err := wire.ParseCOTP(cotpData)
	if err != nil {
		return nil, nil, err
	}

	header, rest, err := wire.ParseS7Header(s7Data)
	if err != nil {
		return nil, nil, err
	}

	if header.ErrorClass != 0 || header.ErrorCode != 0 {
		return nil, nil, wire.NewS7Error(header.ErrorClass, header.ErrorCode)
	}

	need := int(header.ParamLength) + int(header.DataLength)
	if len(rest) < need {
		return nil, nil, fmt.Errorf("short S7 payload: need %d bytes, got %d", need, len(rest))
	}

	paramData := rest[:header.ParamLength]
	dataData := rest[header.ParamLength : header.ParamLength+header.DataLength]

	return paramData, dataData, nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeConnLocked()
}

// ConnectionInfo returns info about current connection
func (c *Client) ConnectionInfo() model.ConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return model.ConnectionInfo{
		Host:          c.host,
		Port:          c.opts.port,
		LocalTSAP:     c.localTSAP,
		RemoteTSAP:    c.remoteTSAP,
		Rack:          c.opts.rack,
		Slot:          c.opts.slot,
		PDUSize:       c.pduSize,
		MaxAmqCalling: c.maxAmqCalling,
		MaxAmqCalled:  c.maxAmqCalled,
	}
}

// PDUSize returns the negotiated PDU size
func (c *Client) PDUSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.pduSize
}
