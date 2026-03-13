package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/otfabric/s7comm/model"
)

// RangeProbeRequest configures a read-range scan over an area.
// The client must be connected; the scan uses the client's connection.
type RangeProbeRequest struct {
	Area        model.Area
	DBNumber    int
	Start       int
	End         int
	Step        int // distance between probe offsets; if 0, use ProbeSize
	ProbeSize   int // bytes to read per probe
	Retries     int
	RetryDelay  time.Duration
	Repeat      int // repeat each probe N times for stability
	Interval    time.Duration
	Parallelism int // max concurrent probes; if <=0, sequential
}

// ReadProbeObservation is one probe at one offset.
type ReadProbeObservation struct {
	Offset  int
	Request model.Address
	Result  ReadResult
	Stable  *bool
	AllZero *bool
}

// ReadableSpan is a consolidated contiguous range [Start, End) with a single status.
type ReadableSpan struct {
	Start   int
	End     int
	Status  ReadStatus
	Stable  *bool
	AllZero *bool
	Notes   []string
}

// RangeProbeSummary aggregates spans by classification.
type RangeProbeSummary struct {
	ReadableSpans     []ReadableSpan
	EmptySpans        []ReadableSpan
	FailedSpans       []ReadableSpan
	InconclusiveSpans []ReadableSpan
}

// RangeProbeResult holds spans and raw observations.
type RangeProbeResult struct {
	Area     model.Area
	DBNumber int
	Spans    []ReadableSpan
	Probes   []ReadProbeObservation
	Summary  RangeProbeSummary
}

// ProbeReadableRanges scans the area in the configured range and consolidates adjacent readable probes into spans.
// The client must be connected when the range is non-empty. The scan is read-only.
func (c *Client) ProbeReadableRanges(ctx context.Context, req RangeProbeRequest) (*RangeProbeResult, error) {
	step := req.Step
	if step <= 0 {
		step = req.ProbeSize
	}
	if step <= 0 {
		step = 1
	}
	probeSize := req.ProbeSize
	if probeSize <= 0 {
		probeSize = 8
	}

	out := &RangeProbeResult{Area: req.Area, DBNumber: req.DBNumber}

	var offsets []int
	for o := req.Start; o < req.End; o += step {
		offsets = append(offsets, o)
	}

	if len(offsets) > 0 {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil {
			return nil, errors.New("client not connected")
		}
	}

	observations := make([]ReadProbeObservation, len(offsets))
	var mu sync.Mutex
	parallelism := req.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for i, offset := range offsets {
		select {
		case <-ctx.Done():
			wg.Wait()
			return out, ctx.Err()
		default:
		}

		i, offset := i, offset
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			obs := c.probeOneOffset(ctx, req, offset, probeSize)
			obs.Offset = offset
			obs.Request = model.Address{Area: req.Area, DBNumber: req.DBNumber, Start: offset, Size: probeSize}
			mu.Lock()
			observations[i] = obs
			mu.Unlock()
		}()
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return out, err
	}

	out.Probes = observations
	out.Spans, out.Summary = ConsolidateSpans(observations, step, probeSize)
	return out, nil
}

func (c *Client) probeOneOffset(ctx context.Context, req RangeProbeRequest, offset, probeSize int) ReadProbeObservation {
	addr := model.Address{Area: req.Area, DBNumber: req.DBNumber, Start: offset, Size: probeSize}

	repeat := req.Repeat
	if repeat < 1 {
		repeat = 1
	}
	var repeated []*ReadResult
	for r := 0; r < repeat; r++ {
		if r > 0 && req.Interval > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(req.Interval):
			}
		}
		res, err := c.ReadArea(ctx, addr)
		if err != nil {
			repeated = append(repeated, &ReadResult{Status: ReadStatusTransportErr, Error: err.Error(), RequestedLength: probeSize, ReturnedLength: 0})
			continue
		}
		repeated = append(repeated, res)
	}

	obs := ReadProbeObservation{}
	if len(repeated) == 0 {
		obs.Result = ReadResult{Status: ReadStatusTransportErr, Error: "no reads", RequestedLength: probeSize, ReturnedLength: 0}
		return obs
	}

	obs.Result = *repeated[len(repeated)-1]
	if repeat > 1 && len(repeated) > 1 {
		mixed := false
		for j := 1; j < len(repeated); j++ {
			if repeated[j].Status != repeated[0].Status || !byteSlicesEqual(repeated[j].Data, repeated[0].Data) {
				mixed = true
				break
			}
		}
		if mixed {
			obs.Result.Status = ReadStatusInconclusive
			obs.Result.Error = "repeat reads produced mixed results"
		}
	}

	if req.Retries > 0 {
		var attemptResults []*ReadResult
		for attempt := 0; attempt <= req.Retries; attempt++ {
			if attempt > 0 && req.RetryDelay > 0 {
				select {
				case <-ctx.Done():
					break
				case <-time.After(req.RetryDelay):
				}
			}
			res, err := c.ReadArea(ctx, addr)
			if err != nil {
				attemptResults = append(attemptResults, &ReadResult{Status: ReadStatusTransportErr, Error: err.Error(), RequestedLength: probeSize, ReturnedLength: 0})
				continue
			}
			attemptResults = append(attemptResults, res)
		}
		if len(attemptResults) > 1 {
			allSame := true
			for j := 1; j < len(attemptResults); j++ {
				if attemptResults[j].Status != attemptResults[0].Status || !byteSlicesEqual(attemptResults[j].Data, attemptResults[0].Data) {
					allSame = false
					break
				}
			}
			if !allSame {
				obs.Result.Status = ReadStatusInconclusive
				obs.Result.Error = "retries produced mixed results"
			}
		}
	}

	if repeat > 1 && len(repeated) > 0 {
		allSame := true
		allZero := true
		for _, r := range repeated {
			if r.Status != repeated[0].Status || !byteSlicesEqual(r.Data, repeated[0].Data) {
				allSame = false
			}
			if r.Status == ReadStatusSuccess && len(r.Data) > 0 {
				for _, b := range r.Data {
					if b != 0 {
						allZero = false
						break
					}
				}
			} else {
				allZero = false
			}
		}
		obs.Stable = &allSame
		obs.AllZero = &allZero
	}

	return obs
}

func byteSlicesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ConsolidateSpans merges adjacent probes (by step) with the same status into spans and fills the summary.
// Exported for tests.
func ConsolidateSpans(observations []ReadProbeObservation, step, probeSize int) ([]ReadableSpan, RangeProbeSummary) {
	var spans []ReadableSpan
	var summary RangeProbeSummary

	if len(observations) == 0 {
		return spans, summary
	}
	if step <= 0 {
		step = probeSize
	}

	byStatus := make(map[ReadStatus][]ReadableSpan)
	for _, s := range []ReadStatus{ReadStatusSuccess, ReadStatusEmptyRead, ReadStatusRejected, ReadStatusTimeout, ReadStatusTransportErr, ReadStatusProtocolErr, ReadStatusShortRead, ReadStatusInconclusive} {
		byStatus[s] = nil
	}

	i := 0
	for i < len(observations) {
		status := observations[i].Result.Status
		start := observations[i].Offset
		end := start + probeSize
		j := i + 1
		for j < len(observations) && observations[j].Result.Status == status && observations[j].Offset == observations[j-1].Offset+step {
			end = observations[j].Offset + probeSize
			j++
		}
		span := ReadableSpan{Start: start, End: end, Status: status}
		spans = append(spans, span)
		byStatus[status] = append(byStatus[status], span)
		i = j
	}

	summary.ReadableSpans = byStatus[ReadStatusSuccess]
	summary.EmptySpans = byStatus[ReadStatusEmptyRead]
	summary.InconclusiveSpans = byStatus[ReadStatusInconclusive]
	for _, s := range []ReadStatus{ReadStatusRejected, ReadStatusTimeout, ReadStatusTransportErr, ReadStatusProtocolErr, ReadStatusShortRead} {
		summary.FailedSpans = append(summary.FailedSpans, byStatus[s]...)
	}

	return spans, summary
}
