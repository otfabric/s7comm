package client

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/otfabric/s7comm/model"
	"github.com/otfabric/s7comm/wire"
)

var (
	orderRegex    = regexp.MustCompile(`(?i)6ES7[0-9A-Z\-\/]{4,}`)
	firmwareRegex = regexp.MustCompile(`(?i)V\s*\d+(?:\.\d+){1,3}`)
	serialRegex   = regexp.MustCompile(`(?i)[A-Z0-9]{8,24}`)
)

// Identify reads device identification from SZL
func (c *Client) Identify(ctx context.Context) (*model.DeviceInfo, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	info := &model.DeviceInfo{}

	// SZL 0x0011 - Module identification
	req := wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLModuleID, 0)
	_, data, err := c.sendReceive(ctx, req)
	if err == nil {
		resp, err := wire.ParseSZLResponse(nil, data)
		if err == nil && len(resp.Data) >= 20 {
			info.OrderNumber = trimString(resp.Data[2:22])
			populateInfoFromRaw(info, resp.Data)
		}
	}

	// SZL 0x001C - Component identification
	req = wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLComponentID, 0)
	_, data, err = c.sendReceive(ctx, req)
	if err == nil {
		resp, err := wire.ParseSZLResponse(nil, data)
		if err == nil && len(resp.Data) >= 34 {
			info.ModuleName = trimString(resp.Data[2:26])
			info.PlantID = trimString(resp.Data[26:34])
			populateInfoFromRaw(info, resp.Data)
		}
	}

	setIdentifyDefaults(info)

	return info, nil
}

// GetCPUState returns the current CPU state
func (c *Client) GetCPUState(ctx context.Context) (model.CPUState, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	req := wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLCPUState, 0)
	_, data, err := c.sendReceive(ctx, req)
	if err != nil {
		return model.CPUStateUnknown, err
	}

	resp, err := wire.ParseSZLResponse(nil, data)
	if err != nil {
		return model.CPUStateUnknown, err
	}

	if len(resp.Data) >= 4 {
		state := resp.Data[2]
		switch state {
		case 0x08:
			return model.CPUStateRun, nil
		case 0x04:
			return model.CPUStateStop, nil
		case 0x01:
			return model.CPUStateStartup, nil
		}
	}

	return model.CPUStateUnknown, nil
}

// GetProtectionLevel returns the PLC protection level
func (c *Client) GetProtectionLevel(ctx context.Context) (model.ProtectionLevel, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	req := wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLProtectionInfo, 0)
	_, data, err := c.sendReceive(ctx, req)
	if err != nil {
		return model.ProtectionNone, err
	}

	resp, err := wire.ParseSZLResponse(nil, data)
	if err != nil {
		return model.ProtectionNone, err
	}

	if len(resp.Data) >= 4 {
		level := resp.Data[2]
		return model.ProtectionLevel(level & 0x03), nil
	}

	return model.ProtectionNone, nil
}

// ReadDiagBuffer reads the diagnostic buffer
func (c *Client) ReadDiagBuffer(ctx context.Context) (*model.DiagBuffer, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	req := wire.EncodeSZLRequest(c.nextPDURef(), wire.SZLDiagBuffer, 0)
	_, data, err := c.sendReceive(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := wire.ParseSZLResponse(nil, data)
	if err != nil {
		return nil, err
	}

	buf := &model.DiagBuffer{}
	// Parse diagnostic entries from resp.Data
	// Format varies by PLC model
	offset := 0
	for offset+20 <= len(resp.Data) {
		entry := model.DiagEntry{
			EventID:    uint16(resp.Data[offset])<<8 | uint16(resp.Data[offset+1]),
			EventClass: resp.Data[offset+2],
			Priority:   resp.Data[offset+3],
		}
		buf.Entries = append(buf.Entries, entry)
		offset += 20
	}
	buf.TotalCount = len(buf.Entries)

	return buf, nil
}

func trimString(data []byte) string {
	end := len(data)
	for end > 0 && (data[end-1] == 0 || data[end-1] == ' ') {
		end--
	}
	return string(data[:end])
}

func populateInfoFromRaw(info *model.DeviceInfo, raw []byte) {
	tokens := extractPrintableTokens(raw)
	joined := strings.Join(tokens, " ")

	if info.OrderNumber == "" {
		if m := orderRegex.FindString(joined); m != "" {
			info.OrderNumber = normalizeToken(m)
		}
	}
	if info.FWVersion == "" {
		if m := firmwareRegex.FindString(joined); m != "" {
			info.FWVersion = normalizeToken(m)
		}
	}

	for _, tok := range tokens {
		tok = normalizeToken(tok)
		u := strings.ToUpper(tok)
		if info.OrderNumber == "" && strings.Contains(u, "6ES7") {
			info.OrderNumber = tok
		}
		if info.CPUType == "" && (strings.Contains(u, "CPU") || strings.Contains(u, "S7-")) {
			info.CPUType = tok
		}
		if info.FWVersion == "" && strings.Contains(u, "V") && strings.IndexFunc(u, unicode.IsDigit) >= 0 {
			info.FWVersion = tok
		}
		if info.SerialNumber == "" && isLikelySerial(u) {
			info.SerialNumber = tok
		}
	}

	if info.SerialNumber == "" {
		for _, m := range serialRegex.FindAllString(joined, -1) {
			u := strings.ToUpper(m)
			if isLikelySerial(u) && !strings.Contains(u, "6ES7") {
				info.SerialNumber = normalizeToken(m)
				break
			}
		}
	}
}

func extractPrintableTokens(raw []byte) []string {
	parts := strings.FieldsFunc(string(raw), func(r rune) bool {
		return r == 0 || !unicode.IsPrint(r)
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = normalizeToken(strings.TrimSpace(p))
		if len(p) >= 4 {
			out = append(out, p)
		}
	}
	return out
}

func normalizeToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'[](){};,")
	s = strings.ReplaceAll(s, "  ", " ")
	return s
}

func isLikelySerial(s string) bool {
	if len(s) < 8 || len(s) > 24 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

func setIdentifyDefaults(info *model.DeviceInfo) {
	if info.OrderNumber == "" {
		info.OrderNumber = "N/A"
	}
	if info.SerialNumber == "" {
		info.SerialNumber = "N/A"
	}
	if info.ModuleName == "" {
		info.ModuleName = "N/A"
	}
	if info.CPUType == "" {
		info.CPUType = "N/A"
	}
	if info.FWVersion == "" {
		info.FWVersion = "N/A"
	}
}
