package client

import (
	"context"
	"github.com/otfabric/s7comm/model"
	"testing"
	"time"
)

func TestCompareRead_EmptyCandidates(t *testing.T) {
	result, err := CompareRead(context.Background(), CompareReadRequest{
		Address:    "192.168.0.1",
		Candidates: nil,
		Area:       model.AreaDB,
		DBNumber:   1,
		Offset:     0,
		Size:       8,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.ByCandidate) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.ByCandidate))
	}
	if result.RackSlotInsensitive {
		t.Error("RackSlotInsensitive should be false when no candidates")
	}
}

func TestCompareRead_SingleCandidate(t *testing.T) {
	result, err := CompareRead(context.Background(), CompareReadRequest{
		Address:    "127.0.0.1",
		Port:       1,
		Candidates: []RackSlot{{Rack: 0, Slot: 1}},
		Area:       model.AreaDB,
		DBNumber:   1,
		Offset:     0,
		Size:       8,
		Timeout:    100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ByCandidate) != 1 {
		t.Fatalf("expected 1 candidate result, got %d", len(result.ByCandidate))
	}
	// Connection will fail (port 1 closed), so we get TransportErr
	if result.ByCandidate[0].Result.Status != ReadStatusTransportErr {
		t.Errorf("expected transport error for unreachable, got %q", result.ByCandidate[0].Result.Status)
	}
	if result.RackSlotInsensitive {
		t.Error("RackSlotInsensitive should be false with single candidate or failed read")
	}
}

func TestRackSlot_ZeroValue(t *testing.T) {
	var r RackSlot
	if r.Rack != 0 || r.Slot != 0 {
		t.Errorf("zero value RackSlot: Rack=%d Slot=%d", r.Rack, r.Slot)
	}
}
