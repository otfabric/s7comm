package client

import (
	"bytes"
	"context"
	"fmt"

	"otfabric/s7comm/model"
	"otfabric/s7comm/wire"
)

const (
	maxUploadChunks  = 4096
	maxUploadRetries = 3
)

// ListBlocks returns a list of blocks of the specified type
func (c *Client) ListBlocks(ctx context.Context, bt model.BlockType) ([]model.BlockInfo, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	req := wire.EncodeBlockListRequest(c.nextPDURef(), byte(bt))
	_, data, err := c.sendReceive(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := wire.ParseSZLResponse(nil, data)
	if err != nil {
		return nil, err
	}

	entries, err := wire.ParseBlockListResponse(resp.Data)
	if err != nil {
		return nil, err
	}

	var blocks []model.BlockInfo
	for _, e := range entries {
		blocks = append(blocks, model.BlockInfo{
			Type:     model.BlockType(e.BlockType),
			Number:   int(e.BlockNumber),
			Language: model.BlockLang(e.Language),
			Flags:    e.Flags,
		})
	}

	return blocks, nil
}

// ListAllBlocks returns all blocks in the PLC
func (c *Client) ListAllBlocks(ctx context.Context) ([]model.BlockInfo, error) {
	var all []model.BlockInfo
	types := []model.BlockType{
		model.BlockOB,
		model.BlockDB,
		model.BlockFC,
		model.BlockFB,
		model.BlockSFC,
		model.BlockSFB,
	}

	for _, bt := range types {
		blocks, err := c.ListBlocks(ctx, bt)
		if err != nil {
			continue // Skip types that fail
		}
		all = append(all, blocks...)
	}

	return all, nil
}

// GetBlockInfo returns detailed info about a specific block
func (c *Client) GetBlockInfo(ctx context.Context, bt model.BlockType, num int) (*model.BlockInfo, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	// Use SZL 0x0113 with block type and number
	szlIndex := uint16(bt)<<8 | uint16(num&0xFF)
	req := wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLBlockInfo, szlIndex)
	_, data, err := c.sendReceive(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := wire.ParseSZLResponse(nil, data)
	if err != nil {
		return nil, err
	}

	info := &model.BlockInfo{
		Type:   bt,
		Number: num,
	}

	// Parse block info from SZL response
	if len(resp.Data) >= 8 {
		info.LoadMemory = int(resp.Data[0])<<8 | int(resp.Data[1])
		info.LocalData = int(resp.Data[2])<<8 | int(resp.Data[3])
		info.MC7Size = int(resp.Data[4])<<8 | int(resp.Data[5])
	}

	return info, nil
}

// UploadBlock uploads a block from the PLC
func (c *Client) UploadBlock(ctx context.Context, bt model.BlockType, num int) (*model.BlockData, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	req := wire.EncodeStartUploadRequest(c.nextPDURef(), byte(bt), num)
	param, _, err := c.sendReceive(ctx, req)
	if err != nil {
		return nil, err
	}

	sessionID, err := wire.ParseStartUploadResponse(param)
	if err != nil {
		return nil, fmt.Errorf("parse start upload: %w", err)
	}
	defer func() {
		_, _, _ = c.sendReceive(context.Background(), wire.EncodeEndUploadRequest(c.nextPDURef(), sessionID))
	}()

	var payload bytes.Buffer
	chunkCount := 0
	done := false
	for i := 0; i < maxUploadChunks; i++ {
		var chunk *wire.UploadChunk
		var lastErr error
		for attempt := 0; attempt < maxUploadRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			req = wire.EncodeUploadRequest(c.nextPDURef(), sessionID)
			param, data, e := c.sendReceive(ctx, req)
			if e != nil {
				lastErr = e
				continue
			}

			chunk, e = wire.ParseUploadResponse(param, data)
			if e != nil {
				lastErr = e
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			return nil, fmt.Errorf("upload chunk %d failed after %d retries: %w", i, maxUploadRetries, lastErr)
		}

		if len(chunk.Data) > 0 {
			_, _ = payload.Write(chunk.Data)
			chunkCount++
		}
		if chunk.Done {
			done = true
			break
		}
	}
	if !done {
		return nil, fmt.Errorf("upload did not complete within %d chunks", maxUploadChunks)
	}
	if payload.Len() == 0 && chunkCount == 0 {
		return nil, fmt.Errorf("upload completed with no payload")
	}

	return &model.BlockData{
		Info: model.BlockInfo{
			Type:   bt,
			Number: num,
		},
		Data: payload.Bytes(),
	}, nil
}
