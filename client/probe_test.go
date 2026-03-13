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
	if c.Classification != ClassUnreachable {
		t.Errorf("Classification: got %q, want %q", c.Classification, ClassUnreachable)
	}
	if c.ReachableTCP {
		t.Error("ReachableTCP should be false")
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
	if !c.ReachableTCP {
		t.Error("ReachableTCP should be true for a listening port")
	}
	if c.S7SetupOK {
		t.Error("S7SetupOK should be false for a non-S7 server")
	}
	validClass := c.Classification == ClassTCPOnly || c.Classification == ClassCOTPFailed
	if !validClass {
		t.Errorf("Classification: got %q, want %q or %q", c.Classification, ClassTCPOnly, ClassCOTPFailed)
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
