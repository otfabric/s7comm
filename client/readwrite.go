package client

import (
	"context"
	"fmt"
	"time"

	"github.com/otfabric/s7comm/model"
	"github.com/otfabric/s7comm/wire"
)

// ReadArea reads data from an S7 memory area and returns a structured result with status and lengths.
// Use result.OK() for success; result.Err() for a non-success outcome; result.Data for the payload.
func (c *Client) ReadArea(ctx context.Context, addr model.Address) (*ReadResult, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	reqLen := addr.Size
	if reqLen < 0 {
		reqLen = 0
	}
	out := &ReadResult{
		RequestedLength: reqLen,
		ReturnedLength:  0,
		Data:            nil,
	}

	c.mu.RLock()
	pduSize := c.pduSize
	rateLimit := c.opts.rateLimit
	c.mu.RUnlock()

	maxData := pduSize - 18
	if maxData < 1 {
		maxData = 200
	}

	result := make([]byte, 0, addr.Size)
	offset := addr.Start
	chunks := planReadChunks(addr.Size, maxData)

	for _, chunkSize := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if rateLimit > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(rateLimit):
			}
		}

		s7Addr := wire.S7AnyAddress{
			Area:     byte(addr.Area),
			DBNumber: addr.DBNumber,
			Start:    offset,
			Size:     chunkSize,
		}

		req := wire.EncodeReadVarRequest(c.nextPDURef(), []wire.S7AnyAddress{s7Addr})
		param, data, err := c.sendReceive(ctx, req)
		if err != nil {
			return nil, err
		}

		items, err := wire.ParseReadVarResponse(param, data)
		if err != nil {
			out.Status = ReadStatusProtocolErr
			out.Error = err.Error()
			out.ReturnedLength = len(result)
			out.Data = result
			return out, nil
		}

		if len(items) == 0 {
			out.Status = ReadStatusEmptyRead
			out.Error = "no data returned"
			out.ReturnedLength = len(result)
			out.Data = result
			return out, nil
		}

		item := items[0]
		if err := wire.ReturnCodeError(item.ReturnCode); err != nil {
			out.Status = ReadStatusRejected
			out.ReturnCode = item.ReturnCode
			out.Error = err.Error()
			out.ItemStatus = wireReturnCodeString(item.ReturnCode)
			out.ReturnedLength = len(result)
			out.Data = result
			return out, nil
		}

		result = append(result, item.Data...)
		offset += chunkSize
	}

	out.ReturnedLength = len(result)
	out.Data = result
	out.Status = ClassifyReadOutcome(reqLen, out.ReturnedLength)
	if out.Status == ReadStatusEmptyRead && out.Error == "" {
		out.Error = "target returned no data for requested range"
	}
	if out.Status == ReadStatusShortRead && out.Error == "" {
		out.Error = fmt.Sprintf("short read: requested %d, got %d", reqLen, out.ReturnedLength)
	}
	return out, nil
}

// ClassifyReadOutcome returns the ReadStatus for a read that requested requested bytes and returned returned bytes.
func ClassifyReadOutcome(requested, returned int) ReadStatus {
	if requested <= 0 {
		if returned == 0 {
			return ReadStatusSuccess
		}
		return ReadStatusSuccess
	}
	if returned == 0 {
		return ReadStatusEmptyRead
	}
	if returned < requested {
		return ReadStatusShortRead
	}
	return ReadStatusSuccess
}

func wireReturnCodeString(code byte) string {
	switch code {
	case wire.RetCodeSuccess:
		return "success"
	case wire.RetCodeHWFault:
		return "hardware fault"
	case wire.RetCodeAccessFault:
		return "access denied"
	case wire.RetCodeAddressFault:
		return "invalid address"
	case wire.RetCodeDataTypeFault:
		return "data type not supported"
	case wire.RetCodeDataSizeFault:
		return "data size mismatch"
	case wire.RetCodeBusy:
		return "object busy"
	case wire.RetCodeNotAvailable:
		return "object not available"
	default:
		return fmt.Sprintf("return code 0x%02X", code)
	}
}

// WriteArea writes data to an S7 memory area
func (c *Client) WriteArea(ctx context.Context, addr model.Address, data []byte) error {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.RLock()
	rateLimit := c.opts.rateLimit
	c.mu.RUnlock()

	s7Addr := wire.S7AnyAddress{
		Area:     byte(addr.Area),
		DBNumber: addr.DBNumber,
		Start:    addr.Start,
		Size:     len(data),
	}

	req := wire.EncodeWriteVarRequest(c.nextPDURef(), s7Addr, data)
	if rateLimit > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rateLimit):
		}
	}
	param, respData, err := c.sendReceive(ctx, req)
	if err != nil {
		return err
	}

	return wire.ParseWriteVarResponse(param, respData)
}

// ReadDB reads from a data block. Returns *ReadResult; use result.OK() and result.Data.
func (c *Client) ReadDB(ctx context.Context, dbNum, offset, size int) (*ReadResult, error) {
	return c.ReadArea(ctx, model.Address{
		Area:     model.AreaDB,
		DBNumber: dbNum,
		Start:    offset,
		Size:     size,
	})
}

// WriteDB is a convenience function to write to a data block
func (c *Client) WriteDB(ctx context.Context, dbNum, offset int, data []byte) error {
	return c.WriteArea(ctx, model.Address{
		Area:     model.AreaDB,
		DBNumber: dbNum,
		Start:    offset,
		Size:     len(data),
	}, data)
}

// ReadInputs reads from the input area. Returns *ReadResult; use result.OK() and result.Data.
func (c *Client) ReadInputs(ctx context.Context, offset, size int) (*ReadResult, error) {
	return c.ReadArea(ctx, model.Address{
		Area:  model.AreaInputs,
		Start: offset,
		Size:  size,
	})
}

// ReadOutputs reads from the output area. Returns *ReadResult; use result.OK() and result.Data.
func (c *Client) ReadOutputs(ctx context.Context, offset, size int) (*ReadResult, error) {
	return c.ReadArea(ctx, model.Address{
		Area:  model.AreaOutputs,
		Start: offset,
		Size:  size,
	})
}

// ReadMerkers reads from the merker area. Returns *ReadResult; use result.OK() and result.Data.
func (c *Client) ReadMerkers(ctx context.Context, offset, size int) (*ReadResult, error) {
	return c.ReadArea(ctx, model.Address{
		Area:  model.AreaMerkers,
		Start: offset,
		Size:  size,
	})
}

func planReadChunks(total, maxChunk int) []int {
	if total <= 0 {
		return nil
	}
	if maxChunk <= 0 {
		maxChunk = total
	}

	out := make([]int, 0, (total/maxChunk)+1)
	remaining := total
	for remaining > 0 {
		n := remaining
		if n > maxChunk {
			n = maxChunk
		}
		out = append(out, n)
		remaining -= n
	}
	return out
}
