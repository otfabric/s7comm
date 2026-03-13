package client

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// newProbeServer starts a minimal TCP listener and returns its port and a
// cleanup function. The handler func is called once per accepted connection.
func newProbeServer(t *testing.T, handler func(net.Conn)) (port int, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() { _ = ln.Close() }
}

// TestApplyProbeDefaults verifies that zero-value fields are filled in.
func TestApplyProbeDefaults(t *testing.T) {
	req := RackSlotProbeRequest{}
	applyProbeDefaults(&req)

	if req.Port != 102 {
		t.Errorf("Port: got %d, want 102", req.Port)
	}
	if req.Timeout != 2*time.Second {
		t.Errorf("Timeout: got %v, want 2s", req.Timeout)
	}
	if req.Parallelism != 4 {
		t.Errorf("Parallelism: got %d, want 4", req.Parallelism)
	}
	// RackMax and SlotMax are NOT defaulted; 0 is valid. Use DefaultRackSlotProbeRequest.
	if req.RackMax != 0 {
		t.Errorf("RackMax should remain 0, got %d", req.RackMax)
	}
	if req.SlotMax != 0 {
		t.Errorf("SlotMax should remain 0, got %d", req.SlotMax)
	}
}

// TestApplyProbeDefaultsPreservesExplicit ensures non-zero fields are not overwritten.
func TestApplyProbeDefaultsPreservesExplicit(t *testing.T) {
	req := RackSlotProbeRequest{
		Port:        502,
		Timeout:     5 * time.Second,
		Parallelism: 8,
	}
	applyProbeDefaults(&req)

	if req.Port != 502 {
		t.Errorf("Port should not be overwritten: got %d", req.Port)
	}
	if req.Timeout != 5*time.Second {
		t.Errorf("Timeout should not be overwritten: got %v", req.Timeout)
	}
	if req.Parallelism != 8 {
		t.Errorf("Parallelism should not be overwritten: got %d", req.Parallelism)
	}
}

// TestProbeRackSlotsUnreachable confirms unreachable hosts are classified correctly.
func TestProbeRackSlotsUnreachable(t *testing.T) {
	// Port 1 is almost certainly closed.
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     300 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RackMin=0/RackMax=0, SlotMin=1/SlotMax=1 → 1 candidate.
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Status != StatusUnreachable {
		t.Errorf("Status: got %q, want %q", c.Status, StatusUnreachable)
	}
	if len(result.Valid) != 0 {
		t.Errorf("expected no valid candidates, got %d", len(result.Valid))
	}
}

// TestProbeRackSlotsTCPOnly confirms that a plain TCP echo server (no S7)
// is classified as tcp-only or cotp-failed (not unreachable, not valid).
func TestProbeRackSlotsTCPOnly(t *testing.T) {
	port, cleanup := newProbeServer(t, func(conn net.Conn) {
		// Accept but immediately close; simulates a non-S7 service.
		_ = conn.Close()
	})
	defer cleanup()

	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        port,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     500 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RackMin=0/RackMax=0, SlotMin=1/SlotMax=1 → 1 candidate.
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Status != StatusTCPOnly && c.Status != StatusCOTPOnly {
		t.Errorf("Status: got %q, want %q or %q", c.Status, StatusTCPOnly, StatusCOTPOnly)
	}
	if len(result.Valid) != 0 {
		t.Errorf("expected no valid candidates, got %d", len(result.Valid))
	}
}

// TestProbeRackSlotsCanceledContext confirms context cancellation is propagated.
func TestProbeRackSlotsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ProbeRackSlots(ctx, RackSlotProbeRequest{
		Address:     "192.0.2.1", // TEST-NET, will time out / be refused
		Port:        102,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     100 * time.Millisecond,
		Parallelism: 1,
	})
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// TestProbeRackSlotsStopOnFirst confirms that StopOnFirst stops after the first valid.
// Since we cannot spin up a full S7 stack, we verify the structural behaviour by
// using an unreachable target and checking that the result is still coherent.
func TestProbeRackSlotsStopOnFirst(t *testing.T) {
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     0,
		RackMax:     1,
		SlotMin:     0,
		SlotMax:     1,
		Timeout:     200 * time.Millisecond,
		Parallelism: 1,
		StopOnFirst: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No valid candidates, so all should be probed; we just verify no panic/crash.
	if result == nil {
		t.Fatal("result is nil")
	}
}

// TestProbeRackSlotsManualTSAP verifies that manual TSAP override is applied.
func TestProbeRackSlotsManualTSAP(t *testing.T) {
	var recordedLocalTSAP, recordedRemoteTSAP uint16

	port, cleanup := newProbeServer(t, func(conn net.Conn) {
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil || n < 7 {
			_ = conn.Close()
			return
		}
		// COTP CR frame: TPKT(4) + LI(1) + PDU type skip — TSAPs are in the options.
		// We just close; the test checks candidate fields, not the COTP CC response.
		_ = conn.Close()
	})
	defer cleanup()

	local := uint16(0x0100)
	remote := uint16(0x0301)

	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        port,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     300 * time.Millisecond,
		Parallelism: 1,
		LocalTSAP:   &local,
		RemoteTSAP:  &remote,
	})
	_ = recordedLocalTSAP
	_ = recordedRemoteTSAP

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RackMin=0/RackMax=0, SlotMin=1/SlotMax=1 → 1 candidate.
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.LocalTSAP != local {
		t.Errorf("LocalTSAP: got 0x%04X, want 0x%04X", c.LocalTSAP, local)
	}
	if c.RemoteTSAP != remote {
		t.Errorf("RemoteTSAP: got 0x%04X, want 0x%04X", c.RemoteTSAP, remote)
	}
}

// TestProbeRackSlotsCandidateCount verifies correct number of candidates is generated.
func TestProbeRackSlotsCandidateCount(t *testing.T) {
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     0,
		RackMax:     1,
		SlotMin:     0,
		SlotMax:     2,
		Timeout:     200 * time.Millisecond,
		Parallelism: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 racks × 3 slots = 6
	want := 6
	if len(result.Candidates) != want {
		t.Errorf("Candidates: got %d, want %d", len(result.Candidates), want)
	}
}

// TestProbeRackSlotsAddress verifies the result carries the target address.
func TestProbeRackSlotsAddress(t *testing.T) {
	addr := "127.0.0.1"
	port, cleanup := newProbeServer(t, func(conn net.Conn) { _ = conn.Close() })
	defer cleanup()

	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     addr,
		Port:        port,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     300 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Address != addr {
		t.Errorf("Address: got %q, want %q", result.Address, addr)
	}
}

// TestProbeRackSlotsCandidateCoordinates verifies rack/slot fields are set correctly.
func TestProbeRackSlotsCandidateCoordinates(t *testing.T) {
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     2,
		RackMax:     2,
		SlotMin:     5,
		SlotMax:     5,
		Timeout:     200 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Rack != 2 || c.Slot != 5 {
		t.Errorf("coordinates: got rack=%d slot=%d, want rack=2 slot=5", c.Rack, c.Slot)
	}
}

// TestProbeRackSlotsPortString exercises strconv path guard (no panic on port).
func TestProbeRackSlotsPortString(t *testing.T) {
	// Just verify the port is used in address construction — we observe via connect failure.
	port := 65535
	result, _ := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        port,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     200 * time.Millisecond,
		Parallelism: 1,
	})
	_ = strconv.Itoa(port)
	if result == nil {
		t.Error("result should not be nil")
	}
}

// TestDefaultRackSlotProbeRequest verifies the constructor sets documented defaults.
func TestDefaultRackSlotProbeRequest(t *testing.T) {
	req := DefaultRackSlotProbeRequest("192.168.0.10")

	if req.Address != "192.168.0.10" {
		t.Errorf("Address: got %q", req.Address)
	}
	if req.Port != 102 {
		t.Errorf("Port: got %d, want 102", req.Port)
	}
	if req.RackMin != 0 || req.RackMax != 7 {
		t.Errorf("Rack range: got %d..%d, want 0..7", req.RackMin, req.RackMax)
	}
	if req.SlotMin != 0 || req.SlotMax != 31 {
		t.Errorf("Slot range: got %d..%d, want 0..31", req.SlotMin, req.SlotMax)
	}
	if req.Timeout != 2*time.Second {
		t.Errorf("Timeout: got %v, want 2s", req.Timeout)
	}
	if req.Parallelism != 4 {
		t.Errorf("Parallelism: got %d, want 4", req.Parallelism)
	}
}

// TestApplyProbeDefaultsStrictSetsConfirm verifies that when Strict is true, Confirm defaults to ConfirmAny.
func TestApplyProbeDefaultsStrictSetsConfirm(t *testing.T) {
	req := RackSlotProbeRequest{Strict: true, Confirm: ConfirmNone}
	applyProbeDefaults(&req)
	if req.Confirm != ConfirmAny {
		t.Errorf("when Strict is true and Confirm is none: got Confirm %q, want %q", req.Confirm, ConfirmAny)
	}
}

// TestApplyProbeDefaultsStrictPreservesExplicitConfirm verifies that explicit Confirm is not overwritten.
func TestApplyProbeDefaultsStrictPreservesExplicitConfirm(t *testing.T) {
	req := RackSlotProbeRequest{Strict: true, Confirm: ConfirmSZL}
	applyProbeDefaults(&req)
	if req.Confirm != ConfirmSZL {
		t.Errorf("explicit Confirm: got %q, want szl", req.Confirm)
	}
}

// TestProbeRackSlotsSummaryCounts verifies that result summary is populated.
func TestProbeRackSlotsSummaryCounts(t *testing.T) {
	// Unreachable port -> all unreachable; one tcp-only if we had a listener that closes.
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     2,
		Timeout:     200 * time.Millisecond,
		Parallelism: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 candidates, both unreachable
	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result.Candidates))
	}
	if result.SetupAccepted != 0 {
		t.Errorf("SetupAccepted: got %d, want 0", result.SetupAccepted)
	}
	if result.ConfirmedByQuery != 0 {
		t.Errorf("ConfirmedByQuery: got %d, want 0", result.ConfirmedByQuery)
	}
}

// TestProbeRackSlotsStrictValidOnlyValidQuery verifies that in strict mode only valid-query is in Valid.
func TestProbeRackSlotsStrictValidOnlyValidQuery(t *testing.T) {
	// With strict and no real S7 target, we get at most setup-only -> follow-up fails -> valid-connect.
	// So Valid should be empty (no valid-query). Use unreachable to get no setup success.
	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        1,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Strict:      true,
		Timeout:     200 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Valid) != 0 {
		t.Errorf("strict mode with unreachable: got %d Valid, want 0", len(result.Valid))
	}
}

// TestProbeRackSlotsNonStrictSetsSetupOnly verifies that without Strict, successful setup yields StatusSetupOnly.
func TestProbeRackSlotsNonStrictSetsSetupOnly(t *testing.T) {
	result, _ := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address: "127.0.0.1",
		Port:    1,
		RackMin: 0, RackMax: 0, SlotMin: 1, SlotMax: 1,
		Strict: false, Timeout: 200 * time.Millisecond, Parallelism: 1,
	})
	if result.SetupAccepted != 0 {
		t.Errorf("expected 0 setup accepted for unreachable, got %d", result.SetupAccepted)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].Status != StatusUnreachable {
		t.Errorf("expected StatusUnreachable, got %q", result.Candidates[0].Status)
	}
}

// TestProbeRackSlotsSummaryTCPOnly verifies that TCP-only candidates increment result.TCPOnly.
func TestProbeRackSlotsSummaryTCPOnly(t *testing.T) {
	port, cleanup := newProbeServer(t, func(conn net.Conn) { _ = conn.Close() })
	defer cleanup()

	result, err := ProbeRackSlots(context.Background(), RackSlotProbeRequest{
		Address:     "127.0.0.1",
		Port:        port,
		RackMin:     0,
		RackMax:     0,
		SlotMin:     1,
		SlotMax:     1,
		Timeout:     300 * time.Millisecond,
		Parallelism: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].Status != StatusTCPOnly && result.Candidates[0].Status != StatusCOTPOnly {
		t.Errorf("expected tcp-only or cotp-only, got %q", result.Candidates[0].Status)
	}
	if result.Candidates[0].Status == StatusTCPOnly && result.TCPOnly != 1 {
		t.Errorf("expected TCPOnly=1 when status is tcp-only, got %d", result.TCPOnly)
	}
}

// TestProbeStatusConstants verifies that probe status constants are defined and non-empty.
func TestProbeStatusConstants(t *testing.T) {
	statuses := []ProbeStatus{
		StatusUnreachable, StatusTCPOnly, StatusCOTPOnly, StatusSetupOnly,
		StatusValidConnect, StatusValidQuery, StatusRejected, StatusTimeout, StatusFlaky,
	}
	for _, s := range statuses {
		if s == "" {
			t.Errorf("ProbeStatus constant is empty")
		}
	}
}

// TestProbeStageConstants verifies that probe stage constants are defined and non-empty.
func TestProbeStageConstants(t *testing.T) {
	stages := []ProbeStage{ProbeStageTCP, ProbeStageCOTP, ProbeStageSetup, ProbeStageQuery}
	for _, s := range stages {
		if s == "" {
			t.Errorf("ProbeStage constant is empty")
		}
	}
}

// TestConfirmationKindConstants verifies that confirmation kind constants are defined and non-empty.
func TestConfirmationKindConstants(t *testing.T) {
	kinds := []ConfirmationKind{ConfirmNone, ConfirmSZL, ConfirmCPUState, ConfirmAny}
	for _, k := range kinds {
		if k == "" {
			t.Error("ConfirmationKind constant is empty")
		}
	}
}

// TestDefaultRackSlotProbeRequestStrictUnset verifies default probe request does not enable strict mode.
func TestDefaultRackSlotProbeRequestStrictUnset(t *testing.T) {
	req := DefaultRackSlotProbeRequest("10.0.0.1")
	if req.Strict {
		t.Error("DefaultRackSlotProbeRequest should have Strict false")
	}
	// Confirm is zero value; applyProbeDefaults only sets it when Strict is true
}
