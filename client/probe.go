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

// ProbeStage indicates the last stage reached during a rack/slot probe.
type ProbeStage string

const (
	ProbeStageTCP   ProbeStage = "tcp"
	ProbeStageCOTP  ProbeStage = "cotp"
	ProbeStageSetup ProbeStage = "setup"
	ProbeStageQuery ProbeStage = "query"
)

// ProbeStatus is the classification of a single rack/slot probe attempt.
type ProbeStatus string

const (
	StatusUnreachable  ProbeStatus = "unreachable"   // TCP connect failed
	StatusTCPOnly      ProbeStatus = "tcp-only"      // TCP ok, COTP failed
	StatusCOTPOnly     ProbeStatus = "cotp-only"     // COTP ok, S7 setup failed
	StatusSetupOnly    ProbeStatus = "setup-only"    // Setup ok, no follow-up (non-strict)
	StatusValidConnect ProbeStatus = "valid-connect" // Setup ok, follow-up failed or not attempted
	StatusValidQuery   ProbeStatus = "valid-query"   // Setup ok and follow-up query succeeded
	StatusRejected     ProbeStatus = "rejected"      // Target explicitly rejected (S7 error)
	StatusTimeout      ProbeStatus = "timeout"       // Any stage timed out
	StatusFlaky        ProbeStatus = "flaky"         // Retries produced mixed results
)

// ConfirmationKind specifies how to confirm a rack/slot after S7 setup (strict mode).
type ConfirmationKind string

const (
	ConfirmNone     ConfirmationKind = "none"      // No follow-up (non-strict)
	ConfirmSZL      ConfirmationKind = "szl"       // SZL module/component ID
	ConfirmCPUState ConfirmationKind = "cpu-state" // SZL CPU state
	ConfirmAny      ConfirmationKind = "any"       // Try SZL, then CPU state, then protection
)

// Confidence indicates how strongly a result is confirmed.
type Confidence string

const (
	ConfidenceNone Confidence = "none"
	ConfidenceLow  Confidence = "low"
	ConfidenceHigh Confidence = "high"
)

// Legacy classification constants for backward compatibility.
const (
	ClassValidQuery   = string(StatusValidQuery)
	ClassValidConnect = string(StatusValidConnect)
	ClassRejected     = string(StatusRejected)
	ClassCOTPFailed   = string(StatusCOTPOnly)
	ClassTCPOnly      = string(StatusTCPOnly)
	ClassUnreachable  = string(StatusUnreachable)
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
	StopOnFirst bool          // stop after first valid candidate (any valid in non-strict)

	// Optional manual TSAP override (bypasses rack/slot-derived TSAP).
	LocalTSAP  *uint16
	RemoteTSAP *uint16

	// Strict mode: only count valid-query as valid; run follow-up confirmation.
	Strict bool
	// Confirm selects the follow-up strategy when Strict is true. Default when Strict is true: ConfirmAny.
	Confirm ConfirmationKind
	// Retries: number of attempts per candidate (Phase 2; reserved).
	Retries int
	// RetryDelay: delay between retries (Phase 2; reserved).
	RetryDelay time.Duration
	// StopOnFirstConfirmed: in strict mode, stop after first valid-query (Phase 2; reserved).
	StopOnFirstConfirmed bool
}

// RackSlotCandidate holds the probe result for a single rack/slot pair.
type RackSlotCandidate struct {
	Rack       int    `json:"rack"`
	Slot       int    `json:"slot"`
	LocalTSAP  uint16 `json:"localTsap"`
	RemoteTSAP uint16 `json:"remoteTsap"`

	Stage       ProbeStage       `json:"stage"`
	Status      ProbeStatus      `json:"status"`
	PDUSize     int              `json:"pduSize,omitempty"`
	ConfirmedBy ConfirmationKind `json:"confirmedBy,omitempty"`
	Confidence  Confidence       `json:"confidence"`
	Error       string           `json:"error,omitempty"`

	// Legacy fields for backward compatibility.
	ReachableTCP   bool   `json:"-"`
	ReachableCOTP  bool   `json:"-"`
	S7SetupOK      bool   `json:"-"`
	SZLQueryOK     bool   `json:"-"`
	Classification string `json:"-"` // same as Status
}

// RackSlotProbeResult holds all candidates, the subset that are valid, and a summary.
type RackSlotProbeResult struct {
	Address    string
	Candidates []RackSlotCandidate
	Valid      []RackSlotCandidate

	// Summary counts (honest reporting).
	SetupAccepted    int // candidates that reached setup success (setup-only, valid-connect, valid-query)
	ConfirmedByQuery int // candidates with valid-query (strict validity)
	Flaky            int // reserved for Phase 2
	TCPOnly          int // tcp-only count
}

// ProbeRackSlots probes a single target IP for valid rack/slot combinations.
// It is non-destructive: only connection/setup and optionally a benign SZL read (strict mode) are used.
// Without Strict, "valid" means setup-only or valid-connect or valid-query.
// With Strict, "valid" means only valid-query (setup accepted and confirmed by follow-up).
func ProbeRackSlots(ctx context.Context, req RackSlotProbeRequest) (*RackSlotProbeResult, error) {
	applyProbeDefaults(&req)

	type job struct {
		rack int
		slot int
	}

	var jobs []job
	for rack := req.RackMin; rack <= req.RackMax; rack++ {
		for slot := req.SlotMin; slot <= req.SlotMax; slot++ {
			jobs = append(jobs, job{rack, slot})
		}
	}

	result := &RackSlotProbeResult{Address: req.Address}
	candidates := make([]RackSlotCandidate, len(jobs))

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
			candidates[idx] = c

			// Stop on first valid: non-strict = any setup success; strict = first valid-query
			validForStop := c.S7SetupOK && !req.Strict
			if req.Strict {
				validForStop = c.Status == StatusValidQuery
			}
			if validForStop && (req.StopOnFirst || req.StopOnFirstConfirmed) {
				select {
				case stopCh <- struct{}{}:
				default:
				}
			}
		}()
	}

	wg.Wait()

	result.Candidates = candidates

	// Valid list: non-strict = setup-only | valid-connect | valid-query; strict = only valid-query
	for _, c := range candidates {
		if req.Strict {
			if c.Status == StatusValidQuery {
				result.Valid = append(result.Valid, c)
			}
		} else {
			if c.Status == StatusSetupOnly || c.Status == StatusValidConnect || c.Status == StatusValidQuery {
				result.Valid = append(result.Valid, c)
			}
		}
	}

	// Summary counts
	for _, c := range candidates {
		if c.S7SetupOK {
			result.SetupAccepted++
		}
		if c.Status == StatusValidQuery {
			result.ConfirmedByQuery++
		}
		if c.Status == StatusTCPOnly {
			result.TCPOnly++
		}
		if c.Status == StatusFlaky {
			result.Flaky++
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
	if req.Strict && req.Confirm == ConfirmNone {
		req.Confirm = ConfirmAny
	}
	// RackMax and SlotMax are not defaulted: 0 is a valid rack/slot number.
	// Use DefaultRackSlotProbeRequest for a full-range scan.
}

// runFollowUp sends a benign S7 request (SZL) on the existing conn and returns whether it succeeded.
// pduRef is used for the request header. On success, confirmedBy is set to the strategy that worked.
func runFollowUp(ctx context.Context, conn *transport.Conn, pduRef uint16, strategy ConfirmationKind) (ok bool, confirmedBy ConfirmationKind, errMsg string) {
	trySZL := func(szlID uint16) (bool, string) {
		req := wire.EncodeSZLRequest(pduRef, szlID, 0)
		cotp := wire.EncodeCOTPData()
		frame := wire.EncodeTPKT(append(cotp, req...))
		if err := conn.SendContext(ctx, frame); err != nil {
			return false, err.Error()
		}
		resp, err := conn.ReceiveContext(ctx)
		if err != nil {
			return false, err.Error()
		}
		_, cotpData, err := wire.ParseTPKT(resp)
		if err != nil {
			return false, err.Error()
		}
		_, s7Data, err := wire.ParseCOTP(cotpData)
		if err != nil {
			return false, err.Error()
		}
		hdr, rest, err := wire.ParseS7Header(s7Data)
		if err != nil {
			return false, err.Error()
		}
		if hdr.ErrorClass != 0 || hdr.ErrorCode != 0 {
			return false, fmt.Sprintf("S7 error 0x%02X/0x%02X", hdr.ErrorClass, hdr.ErrorCode)
		}
		need := int(hdr.ParamLength) + int(hdr.DataLength)
		if len(rest) < need {
			return false, "short S7 payload"
		}
		dataSlice := rest[hdr.ParamLength : hdr.ParamLength+hdr.DataLength]
		if _, err := wire.ParseSZLResponse(nil, dataSlice); err != nil {
			return false, err.Error()
		}
		return true, ""
	}

	switch strategy {
	case ConfirmSZL:
		ok, errMsg = trySZL(wire.SZLModuleID)
		if ok {
			return true, ConfirmSZL, ""
		}
		return false, ConfirmNone, errMsg
	case ConfirmCPUState:
		ok, errMsg = trySZL(wire.SZLCPUState)
		if ok {
			return true, ConfirmCPUState, ""
		}
		return false, ConfirmNone, errMsg
	case ConfirmAny:
		if ok, _ := trySZL(wire.SZLModuleID); ok {
			return true, ConfirmSZL, ""
		}
		if ok, _ := trySZL(wire.SZLCPUState); ok {
			return true, ConfirmCPUState, ""
		}
		if ok, errMsg = trySZL(wire.SZLProtectionInfo); ok {
			return true, ConfirmAny, ""
		}
		return false, ConfirmNone, errMsg
	default:
		return false, ConfirmNone, "no confirmation strategy"
	}
}

func probeOne(ctx context.Context, req RackSlotProbeRequest, rack, slot int) RackSlotCandidate {
	c := RackSlotCandidate{Rack: rack, Slot: slot, Confidence: ConfidenceNone}

	addr := net.JoinHostPort(req.Address, fmt.Sprint(req.Port))
	dialer := net.Dialer{Timeout: req.Timeout}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		c.Stage = ProbeStageTCP
		c.Status = StatusUnreachable
		c.Error = err.Error()
		applyLegacyCandidate(&c)
		return c
	}
	c.ReachableTCP = true

	conn := transport.New(netConn, req.Timeout)
	defer func() { _ = conn.Close() }()

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

	// COTP connect
	cr := wire.EncodeCOTPCR(localTSAP, remoteTSAP)
	frame := wire.EncodeTPKT(cr)
	if err := conn.SendContext(ctx, frame); err != nil {
		c.Stage = ProbeStageTCP
		c.Status = StatusTCPOnly
		c.Error = fmt.Sprintf("COTP send: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	resp, err := conn.ReceiveContext(ctx)
	if err != nil {
		c.Stage = ProbeStageTCP
		c.Status = StatusTCPOnly
		c.Error = fmt.Sprintf("COTP recv: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	_, cotpData, err := wire.ParseTPKT(resp)
	if err != nil {
		c.Stage = ProbeStageTCP
		c.Status = StatusTCPOnly
		c.Error = fmt.Sprintf("TPKT parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	cotp, _, err := wire.ParseCOTP(cotpData)
	if err != nil {
		c.Stage = ProbeStageTCP
		c.Status = StatusTCPOnly
		c.Error = fmt.Sprintf("COTP parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	if cotp.PDUType != wire.COTPTypeCC {
		c.Stage = ProbeStageCOTP
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("expected COTP CC, got 0x%02X", cotp.PDUType)
		c.ReachableCOTP = true
		applyLegacyCandidate(&c)
		return c
	}
	c.ReachableCOTP = true

	// S7 setup
	setupReq := wire.EncodeSetupCommRequest(1, 1, 480)
	cotpDT := wire.EncodeCOTPData()
	frame = wire.EncodeTPKT(append(cotpDT, setupReq...))
	if err := conn.SendContext(ctx, frame); err != nil {
		c.Stage = ProbeStageCOTP
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("S7 setup send: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	resp, err = conn.ReceiveContext(ctx)
	if err != nil {
		c.Stage = ProbeStageCOTP
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("S7 setup recv: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	_, cotpData, err = wire.ParseTPKT(resp)
	if err != nil {
		c.Stage = ProbeStageCOTP
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("S7 setup TPKT parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	_, s7Data, err := wire.ParseCOTP(cotpData)
	if err != nil {
		c.Stage = ProbeStageCOTP
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("S7 setup COTP parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	header, paramData, err := wire.ParseS7Header(s7Data)
	if err != nil {
		c.Stage = ProbeStageSetup
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("S7 header parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	if header.ErrorClass != 0 || header.ErrorCode != 0 {
		c.Stage = ProbeStageSetup
		c.Status = StatusRejected
		c.Error = fmt.Sprintf("S7 error 0x%02X/0x%02X", header.ErrorClass, header.ErrorCode)
		applyLegacyCandidate(&c)
		return c
	}
	setup, err := wire.ParseSetupCommResponse(paramData)
	if err != nil {
		c.Stage = ProbeStageSetup
		c.Status = StatusCOTPOnly
		c.Error = fmt.Sprintf("setup comm parse: %s", err)
		applyLegacyCandidate(&c)
		return c
	}
	c.PDUSize = setup.PDUSize
	c.S7SetupOK = true

	if !req.Strict {
		c.Stage = ProbeStageSetup
		c.Status = StatusSetupOnly
		c.Confidence = ConfidenceLow
		applyLegacyCandidate(&c)
		return c
	}

	// Strict: run follow-up confirmation
	c.Stage = ProbeStageQuery
	ok, confirmedBy, errStr := runFollowUp(ctx, conn, 2, req.Confirm)
	if ok {
		c.Status = StatusValidQuery
		c.ConfirmedBy = confirmedBy
		c.Confidence = ConfidenceHigh
		c.SZLQueryOK = true
	} else {
		c.Status = StatusValidConnect
		c.Confidence = ConfidenceLow
		if errStr != "" {
			c.Error = "follow-up failed: " + errStr
		}
	}
	applyLegacyCandidate(&c)
	return c
}

func applyLegacyCandidate(c *RackSlotCandidate) {
	c.Classification = string(c.Status)
	c.ReachableTCP = c.Status != StatusUnreachable
	c.ReachableCOTP = c.Status != StatusUnreachable && c.Status != StatusTCPOnly
	c.S7SetupOK = c.Status == StatusSetupOnly || c.Status == StatusValidConnect || c.Status == StatusValidQuery
	c.SZLQueryOK = c.Status == StatusValidQuery
}
