package client

import (
	"bytes"
	"context"
	"time"

	"github.com/otfabric/s7comm/model"
)

// RackSlot is a rack/slot pair for compare or probe operations.
type RackSlot struct {
	Rack int
	Slot int
}

// CompareReadRequest is the same read across multiple rack/slot candidates.
type CompareReadRequest struct {
	Address    string
	Port       int
	Candidates []RackSlot
	Area       model.Area
	DBNumber   int
	Offset     int
	Size       int
	Timeout    time.Duration
}

// CompareReadCandidate holds one candidate's read result.
type CompareReadCandidate struct {
	Rack   int
	Slot   int
	Result ReadResult
}

// CompareReadResult holds one ReadResult per candidate and whether all successful results were identical.
type CompareReadResult struct {
	Request             CompareReadRequest
	ByCandidate         []CompareReadCandidate
	RackSlotInsensitive bool
}

// CompareRead performs the same read for each rack/slot candidate and reports whether results are identical.
// Each candidate uses a new connection (connect, read, close). If all candidates return ReadStatusSuccess
// and the returned data is identical, RackSlotInsensitive is set true.
func CompareRead(ctx context.Context, req CompareReadRequest) (*CompareReadResult, error) {
	out := &CompareReadResult{Request: req}
	if len(req.Candidates) == 0 {
		return out, nil
	}

	port := req.Port
	if port == 0 {
		port = 102
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	for _, cand := range req.Candidates {
		c := New(req.Address, WithPort(port), WithRackSlot(cand.Rack, cand.Slot), WithTimeout(timeout))
		if err := c.Connect(ctx); err != nil {
			out.ByCandidate = append(out.ByCandidate, CompareReadCandidate{
				Rack:   cand.Rack,
				Slot:   cand.Slot,
				Result: ReadResult{Status: ReadStatusTransportErr, Error: err.Error(), RequestedLength: req.Size, ReturnedLength: 0},
			})
			_ = c.Close()
			continue
		}
		res, err := c.ReadArea(ctx, model.Address{Area: req.Area, DBNumber: req.DBNumber, Start: req.Offset, Size: req.Size})
		_ = c.Close()
		if err != nil {
			out.ByCandidate = append(out.ByCandidate, CompareReadCandidate{
				Rack:   cand.Rack,
				Slot:   cand.Slot,
				Result: ReadResult{Status: ReadStatusTransportErr, Error: err.Error(), RequestedLength: req.Size, ReturnedLength: 0},
			})
			continue
		}
		out.ByCandidate = append(out.ByCandidate, CompareReadCandidate{Rack: cand.Rack, Slot: cand.Slot, Result: *res})
	}

	// Set RackSlotInsensitive if all succeeded and all Data is identical
	if len(out.ByCandidate) < 2 {
		return out, nil
	}
	firstOK := -1
	for i := range out.ByCandidate {
		if out.ByCandidate[i].Result.Status == ReadStatusSuccess {
			firstOK = i
			break
		}
	}
	if firstOK < 0 {
		return out, nil
	}
	ref := out.ByCandidate[firstOK].Result.Data
	allSame := true
	for i := range out.ByCandidate {
		if out.ByCandidate[i].Result.Status != ReadStatusSuccess {
			allSame = false
			break
		}
		if !bytes.Equal(out.ByCandidate[i].Result.Data, ref) {
			allSame = false
			break
		}
	}
	out.RackSlotInsensitive = allSame
	return out, nil
}
