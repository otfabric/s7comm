package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/otfabric/s7comm/transport"
	"github.com/otfabric/s7comm/wire"
)

// Classification values for RackSlotCandidate.
const (
	ClassValidQuery   = "valid-query"   // S7 setup + SZL read succeeded
	ClassValidConnect = "valid-connect" // S7 setup succeeded; SZL not attempted or failed
	ClassRejected     = "rejected"      // COTP connected, S7 setup rejected
	ClassCOTPFailed   = "cotp-failed"   // TCP reachable, COTP not accepted
	ClassTCPOnly      = "tcp-only"      // TCP reachable, no valid COTP/S7 response
	ClassUnreachable  = "unreachable"   // TCP connect failed
)

// RackSlotProbeRequest configures a host-oriented rack/slot probe.
type RackSlotProbeRequest struct {
	Address     string
	Port        int           // default 102
	RackMin     int           // default 0
	RackMax     int           // default 7
	SlotMin     int           // default 0
	SlotMax     int           // default 31
	Timeout     time.Duration // per-attempt timeout; default 2s
	Parallelism int           // concurrent probes; default 4
	DelayMS     int           // delay between attempts in ms; default 0
	StopOnFirst bool          // stop after first valid candidate

	// Optional manual TSAP override (bypasses rack/slot-derived TSAP).
	LocalTSAP  *uint16
	RemoteTSAP *uint16
}

// RackSlotCandidate holds the probe result for a single rack/slot pair.
type RackSlotCandidate struct {
	Rack           int
	Slot           int
	ReachableTCP   bool
	ReachableCOTP  bool
	S7SetupOK      bool
	SZLQueryOK     bool
	PDUSize        int
	LocalTSAP      uint16
	RemoteTSAP     uint16
	Classification string
	Error          string
}

// RackSlotProbeResult holds all candidates and the subset that are valid.
type RackSlotProbeResult struct {
	Address    string
	Candidates []RackSlotCandidate
	Valid      []RackSlotCandidate
}

// ProbeRackSlots probes a single target IP for valid rack/slot combinations.
// It is non-destructive: only connection/setup and a benign SZL read are used.
func ProbeRackSlots(ctx context.Context, req RackSlotProbeRequest) (*RackSlotProbeResult, error) {
	applyProbeDefaults(&req)

	type job struct {
		rack int
		slot int
	}

	// Build ordered job list.
	var jobs []job
	for rack := req.RackMin; rack <= req.RackMax; rack++ {
		for slot := req.SlotMin; slot <= req.SlotMax; slot++ {
			jobs = append(jobs, job{rack, slot})
		}
	}

	result := &RackSlotProbeResult{Address: req.Address}
	candidates := make([]RackSlotCandidate, len(jobs))

	// foundFirst signals workers to stop when StopOnFirst is set.
	var foundFirst int32
	var foundMu sync.Mutex
	_ = foundFirst

	sem := make(chan struct{}, req.Parallelism)
	var wg sync.WaitGroup

	stopCh := make(chan struct{})

outer:
	for i, j := range jobs {
		select {
		case <-ctx.Done():
			break outer
		case <-stopCh:
			break outer
		default:
		}

		if req.DelayMS > 0 && i > 0 {
			select {
			case <-ctx.Done():
				break outer
			case <-time.After(time.Duration(req.DelayMS) * time.Millisecond):
			}
		}

		idx := i
		rack := j.rack
		slot := j.slot

		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			c := probeOne(ctx, req, rack, slot)
			foundMu.Lock()
			candidates[idx] = c
			foundMu.Unlock()

			if c.S7SetupOK && req.StopOnFirst {
				select {
				case stopCh <- struct{}{}:
				default:
				}
			}
		}()
	}

	wg.Wait()

	result.Candidates = candidates
	for _, c := range candidates {
		if c.S7SetupOK {
			result.Valid = append(result.Valid, c)
		}
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// DefaultRackSlotProbeRequest returns a RackSlotProbeRequest populated with
// the recommended defaults for a full rack/slot scan of a single target.
func DefaultRackSlotProbeRequest(address string) RackSlotProbeRequest {
	return RackSlotProbeRequest{
		Address:     address,
		Port:        102,
		RackMin:     0,
		RackMax:     7,
		SlotMin:     0,
		SlotMax:     31,
		Timeout:     2 * time.Second,
		Parallelism: 4,
	}
}

func applyProbeDefaults(req *RackSlotProbeRequest) {
	if req.Port == 0 {
		req.Port = 102
	}
	if req.Timeout == 0 {
		req.Timeout = 2 * time.Second
	}
	if req.Parallelism < 1 {
		req.Parallelism = 4
	}
	// RackMax and SlotMax are not defaulted: 0 is a valid rack/slot number.
	// Use DefaultRackSlotProbeRequest for a full-range scan.
}

func probeOne(ctx context.Context, req RackSlotProbeRequest, rack, slot int) RackSlotCandidate {
	c := RackSlotCandidate{Rack: rack, Slot: slot}

	addr := net.JoinHostPort(req.Address, fmt.Sprint(req.Port))
	dialer := net.Dialer{Timeout: req.Timeout}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		c.Classification = ClassUnreachable
		c.Error = err.Error()
		return c
	}
	c.ReachableTCP = true

	conn := transport.New(netConn, req.Timeout)
	defer func() { _ = conn.Close() }()

	// Derive or override TSAPs.
	var localTSAP, remoteTSAP uint16
	if req.LocalTSAP != nil {
		localTSAP = *req.LocalTSAP
	} else {
		localTSAP = wire.BuildTSAP(1, 0, 0)
	}
	if req.RemoteTSAP != nil {
		remoteTSAP = *req.RemoteTSAP
	} else {
		remoteTSAP = wire.BuildTSAP(3, rack, slot)
	}
	c.LocalTSAP = localTSAP
	c.RemoteTSAP = remoteTSAP

	// COTP connect.
	cr := wire.EncodeCOTPCR(localTSAP, remoteTSAP)
	frame := wire.EncodeTPKT(cr)
	if err := conn.SendContext(ctx, frame); err != nil {
		c.Classification = ClassTCPOnly
		c.Error = fmt.Sprintf("COTP send: %s", err)
		return c
	}
	resp, err := conn.ReceiveContext(ctx)
	if err != nil {
		c.Classification = ClassTCPOnly
		c.Error = fmt.Sprintf("COTP recv: %s", err)
		return c
	}
	_, cotpData, err := wire.ParseTPKT(resp)
	if err != nil {
		c.Classification = ClassTCPOnly
		c.Error = fmt.Sprintf("TPKT parse: %s", err)
		return c
	}
	cotp, _, err := wire.ParseCOTP(cotpData)
	if err != nil {
		c.Classification = ClassTCPOnly
		c.Error = fmt.Sprintf("COTP parse: %s", err)
		return c
	}
	if cotp.PDUType != wire.COTPTypeCC {
		c.Classification = ClassCOTPFailed
		c.Error = fmt.Sprintf("expected COTP CC, got 0x%02X", cotp.PDUType)
		return c
	}
	c.ReachableCOTP = true

	// S7 setup communication.
	setupReq := wire.EncodeSetupCommRequest(1, 1, 480)
	cotpDT := wire.EncodeCOTPData()
	frame = wire.EncodeTPKT(append(cotpDT, setupReq...))
	if err := conn.SendContext(ctx, frame); err != nil {
		c.Classification = ClassCOTPFailed
		c.Error = fmt.Sprintf("S7 setup send: %s", err)
		return c
	}
	resp, err = conn.ReceiveContext(ctx)
	if err != nil {
		c.Classification = ClassCOTPFailed
		c.Error = fmt.Sprintf("S7 setup recv: %s", err)
		return c
	}
	_, cotpData, err = wire.ParseTPKT(resp)
	if err != nil {
		c.Classification = ClassCOTPFailed
		c.Error = fmt.Sprintf("S7 setup TPKT parse: %s", err)
		return c
	}
	_, s7Data, err := wire.ParseCOTP(cotpData)
	if err != nil {
		c.Classification = ClassCOTPFailed
		c.Error = fmt.Sprintf("S7 setup COTP parse: %s", err)
		return c
	}
	header, paramData, err := wire.ParseS7Header(s7Data)
	if err != nil {
		c.Classification = ClassRejected
		c.Error = fmt.Sprintf("S7 header parse: %s", err)
		return c
	}
	if header.ErrorClass != 0 || header.ErrorCode != 0 {
		c.Classification = ClassRejected
		c.Error = fmt.Sprintf("S7 error 0x%02X/0x%02X", header.ErrorClass, header.ErrorCode)
		return c
	}
	setup, err := wire.ParseSetupCommResponse(paramData)
	if err != nil {
		c.Classification = ClassRejected
		c.Error = fmt.Sprintf("setup comm parse: %s", err)
		return c
	}
	c.S7SetupOK = true
	c.PDUSize = setup.PDUSize
	c.Classification = ClassValidConnect

	// Optional benign SZL read to elevate confidence.
	szlReq := wire.EncodeSZLRequest(1, wire.SZLModuleID, 0)
	cotpDT = wire.EncodeCOTPData()
	frame = wire.EncodeTPKT(append(cotpDT, szlReq...))
	if err := conn.SendContext(ctx, frame); err == nil {
		if resp, err = conn.ReceiveContext(ctx); err == nil {
			if _, cotpData, err = wire.ParseTPKT(resp); err == nil {
				if _, s7Data, err = wire.ParseCOTP(cotpData); err == nil {
					if hdr, rest, err := wire.ParseS7Header(s7Data); err == nil {
						if hdr.ErrorClass == 0 && hdr.ErrorCode == 0 {
							need := int(hdr.ParamLength) + int(hdr.DataLength)
							if len(rest) >= need {
								dataSlice := rest[hdr.ParamLength : hdr.ParamLength+hdr.DataLength]
								if _, err = wire.ParseSZLResponse(nil, dataSlice); err == nil {
									c.SZLQueryOK = true
									c.Classification = ClassValidQuery
								}
							}
						}
					}
				}
			}
		}
	}

	return c
}
