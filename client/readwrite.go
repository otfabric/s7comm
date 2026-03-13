package client

import (
	"context"
	"time"

	"github.com/otfabric/s7comm/model"
	"github.com/otfabric/s7comm/wire"
)

// ReadArea reads data from an S7 memory area
func (c *Client) ReadArea(ctx context.Context, addr model.Address) ([]byte, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	c.mu.RLock()
	pduSize := c.pduSize
	rateLimit := c.opts.rateLimit
	c.mu.RUnlock()

	// Calculate max data per request (PDU minus overhead)
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
			return nil, err
		}

		if len(items) == 0 {
			return nil, &wire.S7Error{Message: "no data returned"}
		}

		if err := wire.ReturnCodeError(items[0].ReturnCode); err != nil {
			return nil, err
		}

		result = append(result, items[0].Data...)
		offset += chunkSize
	}

	return result, nil
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

// ReadDB is a convenience function to read from a data block
func (c *Client) ReadDB(ctx context.Context, dbNum, offset, size int) ([]byte, error) {
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

// ReadInputs reads from the input area
func (c *Client) ReadInputs(ctx context.Context, offset, size int) ([]byte, error) {
	return c.ReadArea(ctx, model.Address{
		Area:  model.AreaInputs,
		Start: offset,
		Size:  size,
	})
}

// ReadOutputs reads from the output area
func (c *Client) ReadOutputs(ctx context.Context, offset, size int) ([]byte, error) {
	return c.ReadArea(ctx, model.Address{
		Area:  model.AreaOutputs,
		Start: offset,
		Size:  size,
	})
}

// ReadMerkers reads from the merker area
func (c *Client) ReadMerkers(ctx context.Context, offset, size int) ([]byte, error) {
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
